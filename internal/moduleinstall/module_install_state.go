package moduleinstall

import (
	keycfg "admin/internal/key"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	moduleInstallStatusUnknown          = "unknown"
	moduleInstallStatusInstalling       = "installing"
	moduleInstallStatusInstalled        = "installed"
	moduleInstallStatusFailed           = "failed"
	moduleInstallStatusMissing          = "missing"
	moduleInstallStatusAgentUnreachable = "agent-unreachable"

	moduleInstallHealthUnknown   = "unknown"
	moduleInstallHealthStarting  = "starting"
	moduleInstallHealthHealthy   = "healthy"
	moduleInstallHealthUnhealthy = "unhealthy"
	moduleInstallHealthDegraded  = "degraded"
)

type desiredModuleInstallState struct {
	AgentID          string `json:"agent_id"`
	Module           string `json:"module"`
	Version          string `json:"version,omitempty"`
	Runtime          string `json:"runtime"`
	AppHost          string `json:"app_host"`
	AppPort          int32  `json:"app_port"`
	Endpoint         string `json:"endpoint"`
	ArtifactURL      string `json:"artifact_url,omitempty"`
	ArtifactChecksum string `json:"artifact_checksum,omitempty"`
	Source           string `json:"source"`
	Generation       int64  `json:"generation"`
	Status           string `json:"status"`
	RequestedAt      string `json:"requested_at"`
	LastError        string `json:"last_error,omitempty"`
	LastUpdateAt     string `json:"last_update_at"`
}

type actualModuleInstallState struct {
	AgentID         string `json:"agent_id"`
	Module          string `json:"module"`
	Version         string `json:"version,omitempty"`
	Runtime         string `json:"runtime"`
	ServiceName     string `json:"service_name,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"`
	Status          string `json:"status"`
	Health          string `json:"health,omitempty"`
	DriftReason     string `json:"drift_reason,omitempty"`
	ReconcilePolicy string `json:"reconcile_policy,omitempty"`
	ReconcileAction string `json:"reconcile_action,omitempty"`
	LastOperation   string `json:"last_operation"`
	LastError       string `json:"last_error,omitempty"`
	ObservedAt      string `json:"observed_at"`
}

type moduleInstallStateHandle struct {
	desired desiredModuleInstallState
	actual  actualModuleInstallState
	active  bool
}

func (s *desiredModuleInstallState) UnmarshalJSON(data []byte) error {
	type desiredAlias desiredModuleInstallState
	aux := struct {
		desiredAlias
		LegacyChecksum string `json:"checksum,omitempty"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*s = normalizeDesiredInstallState(desiredModuleInstallState(aux.desiredAlias))
	if s.ArtifactChecksum == "" {
		s.ArtifactChecksum = strings.TrimSpace(aux.LegacyChecksum)
	}
	return nil
}

func (s *actualModuleInstallState) UnmarshalJSON(data []byte) error {
	type actualAlias actualModuleInstallState
	var aux actualAlias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*s = normalizeActualInstallState(actualModuleInstallState(aux))
	return nil
}

func (s *ModuleInstallService) syncActualInstallStateFromAgentInventory(
	ctx context.Context,
	target moduleInstallTarget,
) error {
	if s == nil || s.runtimeRepo == nil {
		return nil
	}
	agentID := strings.TrimSpace(target.AgentID)
	if agentID == "" || strings.TrimSpace(target.AgentGRPCEndpoint) == "" {
		return nil
	}
	inventory, err := listInstalledModulesOnAgent(ctx, target)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, item := range inventory.Items {
		state := actualModuleInstallState{
			AgentID:       agentID,
			Module:        canonicalModuleName(item.Module),
			Version:       strings.TrimSpace(item.Version),
			Runtime:       strings.TrimSpace(item.Runtime),
			ServiceName:   strings.TrimSpace(item.ServiceName),
			Endpoint:      strings.TrimSpace(item.Endpoint),
			Status:        strings.TrimSpace(item.Status),
			Health:        strings.TrimSpace(item.Health),
			LastOperation: "inventory-sync",
			ObservedAt:    now,
		}
		if state.Module == "" {
			continue
		}
		if state.Status == "" {
			state.Status = "unknown"
		}
		if err := s.persistActualInstallState(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (s *ModuleInstallService) beginInstallStateTracking(
	ctx context.Context,
	target moduleInstallTarget,
	moduleName string,
	runtime string,
	appHost string,
	appPort int32,
	endpoint string,
	version string,
	artifactURL string,
	checksum string,
	source string,
) (*moduleInstallStateHandle, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, nil
	}
	agentID := strings.TrimSpace(target.AgentID)
	if agentID == "" {
		return nil, nil
	}
	now := time.Now().UTC()
	handle := &moduleInstallStateHandle{
		active: true,
		desired: desiredModuleInstallState{
			AgentID:          agentID,
			Module:           canonicalModuleName(moduleName),
			Version:          strings.TrimSpace(version),
			Runtime:          strings.TrimSpace(runtime),
			AppHost:          strings.TrimSpace(appHost),
			AppPort:          appPort,
			Endpoint:         strings.TrimSpace(endpoint),
			ArtifactURL:      strings.TrimSpace(artifactURL),
			ArtifactChecksum: strings.TrimSpace(checksum),
			Source:           strings.TrimSpace(source),
			Generation:       now.UnixNano(),
			Status:           moduleInstallDesiredStatusInstalling,
			RequestedAt:      now.Format(time.RFC3339Nano),
			LastUpdateAt:     now.Format(time.RFC3339Nano),
		},
		actual: actualModuleInstallState{
			AgentID:         agentID,
			Module:          canonicalModuleName(moduleName),
			Version:         strings.TrimSpace(version),
			Runtime:         strings.TrimSpace(runtime),
			Status:          moduleInstallStatusInstalling,
			Health:          moduleInstallHealthStarting,
			DriftReason:     moduleInstallDriftNone,
			ReconcilePolicy: moduleInstallReconcilePolicyManual,
			ReconcileAction: moduleInstallReconcileActionNone,
			LastOperation:   "install",
			ObservedAt:      now.Format(time.RFC3339Nano),
		},
	}
	if err := s.persistDesiredInstallState(ctx, handle.desired); err != nil {
		return nil, err
	}
	if err := s.persistActualInstallState(ctx, handle.actual); err != nil {
		return nil, err
	}
	return handle, nil
}

func (s *ModuleInstallService) markInstallStateFailed(ctx context.Context, handle *moduleInstallStateHandle, err error) {
	if s == nil || handle == nil || !handle.active {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	message := ""
	if err != nil {
		message = strings.TrimSpace(err.Error())
	}
	handle.desired.Status = moduleInstallDesiredStatusFailed
	handle.desired.LastError = message
	handle.desired.LastUpdateAt = now
	handle.actual.Status = moduleInstallStatusFailed
	handle.actual.Health = moduleInstallHealthUnhealthy
	handle.actual.DriftReason = moduleInstallDriftNone
	handle.actual.ReconcileAction = moduleInstallReconcileActionNone
	handle.actual.LastError = message
	handle.actual.ObservedAt = now
	_ = s.persistDesiredInstallState(ctx, handle.desired)
	_ = s.persistActualInstallState(ctx, handle.actual)
}

func (s *ModuleInstallService) markInstallStateInstalled(
	ctx context.Context,
	handle *moduleInstallStateHandle,
	version string,
	serviceName string,
	endpoint string,
	health string,
) {
	if s == nil || handle == nil || !handle.active {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if strings.TrimSpace(version) != "" {
		handle.desired.Version = strings.TrimSpace(version)
		handle.actual.Version = strings.TrimSpace(version)
	}
	handle.desired.Status = moduleInstallDesiredStatusInstalled
	handle.desired.LastError = ""
	handle.desired.LastUpdateAt = now
	handle.actual.ServiceName = strings.TrimSpace(serviceName)
	if strings.TrimSpace(endpoint) != "" {
		handle.actual.Endpoint = strings.TrimSpace(endpoint)
	}
	handle.actual.Status = moduleInstallStatusInstalled
	handle.actual.Health = normalizeObservedInstallHealth(health)
	handle.actual.DriftReason = moduleInstallDriftNone
	handle.actual.ReconcileAction = moduleInstallReconcileActionNone
	handle.actual.LastError = ""
	handle.actual.ObservedAt = now
	_ = s.persistDesiredInstallState(ctx, handle.desired)
	_ = s.persistActualInstallState(ctx, handle.actual)
}

func (s *ModuleInstallService) persistDesiredInstallState(ctx context.Context, state desiredModuleInstallState) error {
	if s == nil || s.runtimeRepo == nil {
		return nil
	}
	state = normalizeDesiredInstallState(state)
	body, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal desired install state failed: %w", err)
	}
	return s.runtimeRepo.Upsert(ctx, keycfg.RuntimeInstallDesiredKey(state.AgentID, state.Module), strings.TrimSpace(string(body)))
}

func (s *ModuleInstallService) persistActualInstallState(ctx context.Context, state actualModuleInstallState) error {
	if s == nil || s.runtimeRepo == nil {
		return nil
	}
	state = normalizeActualInstallState(state)
	body, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal actual install state failed: %w", err)
	}
	return s.runtimeRepo.Upsert(ctx, keycfg.RegistryModuleKey(state.AgentID, state.Module), strings.TrimSpace(string(body)))
}

func resolvedInstalledServiceName(result *ModuleInstallResult, moduleName string) string {
	if result != nil && strings.TrimSpace(result.ServiceName) != "" {
		return strings.TrimSpace(result.ServiceName)
	}
	name := canonicalModuleName(moduleName)
	if name == "" {
		name = normalizeModuleName(moduleName)
	}
	if name == "" {
		return ""
	}
	return "aurora-" + name + ".service"
}

func normalizeDesiredInstallState(state desiredModuleInstallState) desiredModuleInstallState {
	state.AgentID = strings.TrimSpace(state.AgentID)
	state.Module = canonicalModuleName(state.Module)
	state.Version = strings.TrimSpace(state.Version)
	state.Runtime = firstNonEmpty(strings.TrimSpace(state.Runtime), ModuleInstallRuntimeName)
	state.AppHost = strings.TrimSpace(state.AppHost)
	state.Endpoint = strings.TrimSpace(state.Endpoint)
	state.ArtifactURL = strings.TrimSpace(state.ArtifactURL)
	state.ArtifactChecksum = strings.TrimSpace(state.ArtifactChecksum)
	state.Source = strings.TrimSpace(state.Source)
	state.Status = normalizeDesiredInstallStatus(state.Status)
	state.RequestedAt = strings.TrimSpace(state.RequestedAt)
	state.LastError = strings.TrimSpace(state.LastError)
	state.LastUpdateAt = strings.TrimSpace(state.LastUpdateAt)
	if state.RequestedAt == "" {
		state.RequestedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if state.LastUpdateAt == "" {
		state.LastUpdateAt = state.RequestedAt
	}
	return state
}

func normalizeActualInstallState(state actualModuleInstallState) actualModuleInstallState {
	state.AgentID = strings.TrimSpace(state.AgentID)
	state.Module = canonicalModuleName(state.Module)
	state.Version = strings.TrimSpace(state.Version)
	state.Runtime = firstNonEmpty(strings.TrimSpace(state.Runtime), ModuleInstallRuntimeName)
	state.ServiceName = strings.TrimSpace(state.ServiceName)
	state.Endpoint = strings.TrimSpace(state.Endpoint)
	state.Status = normalizeObservedInstallStatus(state.Status)
	state.Health = normalizeObservedInstallHealth(state.Health)
	state.DriftReason = normalizeInstallDriftReason(state.DriftReason)
	state.ReconcilePolicy = normalizeInstallReconcilePolicy(state.ReconcilePolicy)
	state.ReconcileAction = normalizeInstallReconcileAction(state.ReconcileAction)
	state.LastOperation = strings.TrimSpace(state.LastOperation)
	state.LastError = strings.TrimSpace(state.LastError)
	state.ObservedAt = strings.TrimSpace(state.ObservedAt)
	if state.ObservedAt == "" {
		state.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return state
}

func normalizeInstallDriftReason(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return moduleInstallDriftNone
	case moduleInstallDriftMissing:
		return moduleInstallDriftMissing
	case moduleInstallDriftVersionMismatch:
		return moduleInstallDriftVersionMismatch
	case moduleInstallDriftRuntimeUnhealthy:
		return moduleInstallDriftRuntimeUnhealthy
	case moduleInstallDriftAgentUnreachable:
		return moduleInstallDriftAgentUnreachable
	default:
		return strings.TrimSpace(raw)
	}
}

func normalizeInstallReconcileAction(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", moduleInstallReconcileActionNone:
		return moduleInstallReconcileActionNone
	case moduleInstallReconcileActionManual:
		return moduleInstallReconcileActionManual
	case moduleInstallReconcileActionAutoRestart:
		return moduleInstallReconcileActionAutoRestart
	case moduleInstallReconcileActionAutoReinstall:
		return moduleInstallReconcileActionAutoReinstall
	default:
		return strings.TrimSpace(raw)
	}
}

func normalizeObservedInstallStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", moduleInstallStatusUnknown:
		return moduleInstallStatusUnknown
	case "applied", "running", "restarted", moduleInstallStatusInstalled:
		return moduleInstallStatusInstalled
	case moduleInstallStatusInstalling:
		return moduleInstallStatusInstalling
	case moduleInstallStatusFailed:
		return moduleInstallStatusFailed
	case "uninstalled", moduleInstallStatusMissing:
		return moduleInstallStatusMissing
	case moduleInstallStatusAgentUnreachable:
		return moduleInstallStatusAgentUnreachable
	default:
		return strings.TrimSpace(raw)
	}
}

func normalizeObservedInstallHealth(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", moduleInstallHealthUnknown:
		return moduleInstallHealthUnknown
	case moduleInstallHealthStarting:
		return moduleInstallHealthStarting
	case moduleInstallHealthHealthy:
		return moduleInstallHealthHealthy
	case moduleInstallHealthUnhealthy:
		return moduleInstallHealthUnhealthy
	case moduleInstallHealthDegraded:
		return moduleInstallHealthDegraded
	default:
		return strings.TrimSpace(raw)
	}
}
