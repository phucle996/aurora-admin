package moduleinstall

import (
	keycfg "admin/internal/key"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (s *ModuleInstallService) StartActualStateReconcileLoop(
	ctx context.Context,
	interval time.Duration,
	onError func(error),
) {
	if s == nil || s.runtimeRepo == nil {
		return
	}
	if interval <= 0 {
		interval = time.Minute
	}

	go func() {
		runOnce := func() {
			reconcileCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if err := s.ReconcileActualInstallStates(reconcileCtx); err != nil && onError != nil {
				onError(err)
			}
		}

		runOnce()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runOnce()
			}
		}
	}()
}

func (s *ModuleInstallService) ReconcileActualInstallStates(ctx context.Context) error {
	if s == nil || s.runtimeRepo == nil {
		return nil
	}

	desiredByAgent, err := s.listDesiredInstallStates(ctx)
	if err != nil {
		return err
	}
	actualByAgent, err := s.listActualInstallStates(ctx)
	if err != nil {
		return err
	}
	agents, err := s.listInstallAgentsRuntime(ctx)
	if err != nil {
		return err
	}
	policy, err := s.loadInstallReconcilePolicy(ctx)
	if err != nil {
		return err
	}
	agentsByID := make(map[string]moduleInstallTarget, len(agents))
	for _, agent := range agents {
		agentID := strings.TrimSpace(agent.AgentID)
		if agentID == "" || strings.TrimSpace(agent.AgentGRPCEndpoint) == "" {
			continue
		}
		agentsByID[agentID] = moduleInstallTarget{
			AgentID:           agentID,
			AgentGRPCEndpoint: strings.TrimSpace(agent.AgentGRPCEndpoint),
			Architecture:      strings.TrimSpace(agent.Architecture),
			Host:              strings.TrimSpace(agent.Host),
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for agentID, target := range agentsByID {

		inventory, invErr := listInstalledModulesOnAgent(ctx, target)
		if invErr != nil {
			for moduleName, desired := range desiredByAgent[agentID] {
				state := actualModuleInstallState{
					AgentID:         agentID,
					Module:          moduleName,
					Version:         strings.TrimSpace(desired.Version),
					Runtime:         strings.TrimSpace(desired.Runtime),
					Endpoint:        strings.TrimSpace(desired.Endpoint),
					Status:          moduleInstallStatusAgentUnreachable,
					Health:          moduleInstallHealthUnknown,
					DriftReason:     moduleInstallDriftAgentUnreachable,
					ReconcilePolicy: policy,
					ReconcileAction: selectReconcileAction(policy, moduleInstallDriftAgentUnreachable),
					LastOperation:   "reconcile",
					LastError:       strings.TrimSpace(invErr.Error()),
					ObservedAt:      now,
				}
				if persistErr := s.persistActualInstallState(ctx, state); persistErr != nil {
					return persistErr
				}
			}
			return invErr
		}

		seen := make(map[string]actualModuleInstallState, len(inventory.Items))
		for _, item := range inventory.Items {
			moduleName := canonicalModuleName(item.Module)
			if moduleName == "" {
				continue
			}
			desired, hasDesired := desiredByAgent[agentID][moduleName]
			state := actualModuleInstallState{
				AgentID:         agentID,
				Module:          moduleName,
				Version:         strings.TrimSpace(item.Version),
				Runtime:         strings.TrimSpace(item.Runtime),
				ServiceName:     strings.TrimSpace(item.ServiceName),
				Endpoint:        strings.TrimSpace(item.Endpoint),
				Status:          normalizeObservedInstallStatus(item.Status),
				Health:          normalizeObservedInstallHealth(item.Health),
				ReconcilePolicy: policy,
				LastOperation:   "reconcile",
				ObservedAt:      now,
			}
			if hasDesired {
				state.DriftReason = detectInstallDrift(&desired, state)
				state.ReconcileAction = selectReconcileAction(policy, state.DriftReason)
				if state.DriftReason == moduleInstallDriftVersionMismatch && state.ReconcileAction == moduleInstallReconcileActionAutoReinstall {
					nextStatus := desiredStatusForDrift(desired.Status, state.DriftReason)
					if err := s.updateDesiredStateForDrift(ctx, desired, nextStatus, "version drift detected"); err != nil {
						return err
					}
					desired.Status = nextStatus
					desired.LastError = "version drift detected"
					desiredByAgent[agentID][moduleName] = desired
				}
				if state.DriftReason == moduleInstallDriftRuntimeUnhealthy && state.ReconcileAction == moduleInstallReconcileActionAutoRestart {
					if err := s.executeAutoRestart(ctx, target, desired, state); err != nil {
						if updateErr := s.updateDesiredStateForDrift(ctx, desired, moduleInstallDesiredStatusFailed, err.Error()); updateErr != nil {
							return updateErr
						}
						desired.Status = moduleInstallDesiredStatusFailed
						desired.LastError = strings.TrimSpace(err.Error())
						desiredByAgent[agentID][moduleName] = desired
						state.LastError = strings.TrimSpace(err.Error())
						state.ReconcileAction = moduleInstallReconcileActionManual
					} else {
						state.Health = moduleInstallHealthHealthy
						state.LastOperation = "reconcile-auto-restart"
						state.ReconcileAction = moduleInstallReconcileActionAutoRestart
					}
				}
			} else {
				state.DriftReason = moduleInstallDriftNone
				state.ReconcileAction = moduleInstallReconcileActionNone
			}
			seen[moduleName] = state
			if persistErr := s.persistActualInstallState(ctx, state); persistErr != nil {
				return persistErr
			}
		}

		for moduleName, desired := range desiredByAgent[agentID] {
			if _, ok := seen[moduleName]; ok {
				continue
			}
			state := actualModuleInstallState{
				AgentID:         agentID,
				Module:          moduleName,
				Version:         strings.TrimSpace(desired.Version),
				Runtime:         strings.TrimSpace(desired.Runtime),
				Endpoint:        strings.TrimSpace(desired.Endpoint),
				Status:          moduleInstallStatusMissing,
				Health:          moduleInstallHealthUnknown,
				DriftReason:     moduleInstallDriftMissing,
				ReconcilePolicy: policy,
				ReconcileAction: selectReconcileAction(policy, moduleInstallDriftMissing),
				LastOperation:   "reconcile",
				LastError:       "",
				ObservedAt:      now,
			}
			if existing, ok := actualByAgent[agentID][moduleName]; ok {
				state.ServiceName = strings.TrimSpace(existing.ServiceName)
				if state.Endpoint == "" {
					state.Endpoint = strings.TrimSpace(existing.Endpoint)
				}
				if state.Version == "" {
					state.Version = strings.TrimSpace(existing.Version)
				}
				if state.Runtime == "" {
					state.Runtime = strings.TrimSpace(existing.Runtime)
				}
			}
			if state.ReconcileAction == moduleInstallReconcileActionAutoReinstall {
				nextStatus := desiredStatusForDrift(desired.Status, moduleInstallDriftMissing)
				if err := s.updateDesiredStateForDrift(ctx, desired, nextStatus, "module drift detected: missing"); err != nil {
					return err
				}
				desired.Status = nextStatus
				desired.LastError = "module drift detected: missing"
				desiredByAgent[agentID][moduleName] = desired
			}
			if persistErr := s.persistActualInstallState(ctx, state); persistErr != nil {
				return persistErr
			}
		}

		for moduleName, existing := range actualByAgent[agentID] {
			if _, ok := seen[moduleName]; ok {
				continue
			}
			if _, ok := desiredByAgent[agentID][moduleName]; ok {
				continue
			}
			existing.Status = moduleInstallStatusMissing
			existing.Health = moduleInstallHealthUnknown
			existing.DriftReason = moduleInstallDriftNone
			existing.ReconcilePolicy = policy
			existing.ReconcileAction = moduleInstallReconcileActionNone
			existing.LastOperation = "reconcile"
			existing.LastError = ""
			existing.ObservedAt = now
			if persistErr := s.persistActualInstallState(ctx, existing); persistErr != nil {
				return persistErr
			}
		}
	}

	if policy == moduleInstallReconcilePolicyAutoReinstall {
		for agentID, desiredModules := range desiredByAgent {
			if _, ok := agentsByID[agentID]; !ok {
				continue
			}
			for _, desired := range desiredModules {
				switch normalizeDesiredInstallStatus(desired.Status) {
				case moduleInstallDesiredStatusPending, moduleInstallDesiredStatusUpgradePending:
					if err := s.updateDesiredStateForDrift(ctx, desired, moduleInstallDesiredStatusInstalling, desired.LastError); err != nil {
						return err
					}
					s.triggerAutoReinstall(desired)
				}
			}
		}
	}

	return nil
}

func (s *ModuleInstallService) listDesiredInstallStates(ctx context.Context) (map[string]map[string]desiredModuleInstallState, error) {
	if s == nil || s.runtimeRepo == nil {
		return map[string]map[string]desiredModuleInstallState{}, nil
	}
	kvs, err := s.runtimeRepo.ListByPrefix(ctx, keycfg.RuntimeInstallDesiredPrefix)
	if err != nil {
		return nil, fmt.Errorf("list desired install states failed: %w", err)
	}
	out := make(map[string]map[string]desiredModuleInstallState)
	for _, kv := range kvs {
		var state desiredModuleInstallState
		if err := json.Unmarshal([]byte(kv.Value), &state); err != nil {
			continue
		}
		agentID := strings.TrimSpace(state.AgentID)
		moduleName := canonicalModuleName(state.Module)
		if agentID == "" || moduleName == "" {
			continue
		}
		state.Module = moduleName
		if _, ok := out[agentID]; !ok {
			out[agentID] = make(map[string]desiredModuleInstallState)
		}
		out[agentID][moduleName] = state
	}
	return out, nil
}

func (s *ModuleInstallService) listActualInstallStates(ctx context.Context) (map[string]map[string]actualModuleInstallState, error) {
	if s == nil || s.runtimeRepo == nil {
		return map[string]map[string]actualModuleInstallState{}, nil
	}
	kvs, err := s.runtimeRepo.ListByPrefix(ctx, keycfg.RegistryModulesPrefix)
	if err != nil {
		return nil, fmt.Errorf("list actual install states failed: %w", err)
	}
	out := make(map[string]map[string]actualModuleInstallState)
	for _, kv := range kvs {
		var state actualModuleInstallState
		if err := json.Unmarshal([]byte(kv.Value), &state); err != nil {
			continue
		}
		agentID := strings.TrimSpace(state.AgentID)
		moduleName := canonicalModuleName(state.Module)
		if agentID == "" || moduleName == "" {
			continue
		}
		state.Module = moduleName
		if _, ok := out[agentID]; !ok {
			out[agentID] = make(map[string]actualModuleInstallState)
		}
		out[agentID][moduleName] = state
	}
	return out, nil
}
