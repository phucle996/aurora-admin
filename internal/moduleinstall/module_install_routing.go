package moduleinstall

import (
	keycfg "admin/internal/key"
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *ModuleInstallService) preseedInstallRouting(
	ctx context.Context,
	moduleName string,
	target moduleInstallTarget,
	endpoint string,
	endpointPort int32,
	result *ModuleInstallResult,
	rollbacks *rollbackStack,
	logFn InstallLogFn,
) error {
	if s == nil || s.endpointRepo == nil || s.runtimeRepo == nil {
		return fmt.Errorf("module install service is nil")
	}

	endpointValue := encodeEndpointValue(target, endpoint)
	logInstall(logFn, "endpoint", "preseed endpoint key=%s", keycfg.EndpointKey(moduleName))
	if err := s.endpointRepo.Upsert(ctx, moduleName, endpointValue); err != nil {
		return fmt.Errorf("upsert endpoint failed: %w", err)
	}
	result.EndpointValue = endpointValue

	appPortKey := keycfg.RuntimeAppPortKey(moduleName)
	if endpointPort <= 0 || endpointPort > 65535 {
		return fmt.Errorf("invalid endpoint port for runtime app port")
	}
	if err := s.runtimeRepo.Upsert(ctx, appPortKey, fmt.Sprintf("%d", endpointPort)); err != nil {
		return fmt.Errorf("upsert runtime app port failed: %w", err)
	}
	logInstall(logFn, "endpoint", "preseed runtime app port key=%s value=%d", appPortKey, endpointPort)

	if rollbacks != nil {
		rollbacks.Add("endpoint", func(rollbackCtx context.Context) error {
			cleanupCtx, cancel := context.WithTimeout(rollbackCtx, 10*time.Second)
			defer cancel()

			endpointErr := s.endpointRepo.Delete(cleanupCtx, moduleName)
			appPortErr := s.runtimeRepo.Delete(cleanupCtx, appPortKey)
			if endpointErr != nil && appPortErr != nil {
				return fmt.Errorf("endpoint cleanup failed (%v) and app port cleanup failed (%v)", endpointErr, appPortErr)
			}
			if endpointErr != nil {
				return fmt.Errorf("endpoint cleanup failed: %w", endpointErr)
			}
			if appPortErr != nil {
				return fmt.Errorf("app port cleanup failed: %w", appPortErr)
			}
			return nil
		})
	}

	return nil
}

func (s *ModuleInstallService) seedHostRoutingEntry(ctx context.Context, host string, address string) error {
	if s == nil || s.runtimeRepo == nil {
		return fmt.Errorf("module install service is nil")
	}
	cleanHost := strings.TrimSpace(host)
	cleanAddress := strings.TrimSpace(address)
	if cleanHost == "" || cleanAddress == "" {
		return fmt.Errorf("host routing entry is invalid")
	}
	return s.runtimeRepo.Upsert(ctx, keycfg.RuntimeHostEntryKey(cleanHost), cleanAddress)
}

func (s *ModuleInstallService) broadcastHostsToConnectedAgents(
	ctx context.Context,
	entries []hostsEntry,
	skipAgentID string,
) ([]string, []string) {
	if s == nil {
		return nil, []string{"broadcast hosts skipped: module install service is nil"}
	}
	targets, warnings := s.listConnectedAgentInstallTargets(ctx, skipAgentID)
	if len(targets) == 0 {
		return nil, warnings
	}
	updated, syncWarnings := syncHostsForTargets(ctx, entries, targets)
	warnings = append(warnings, syncWarnings...)
	return updated, warnings
}

func (s *ModuleInstallService) listConnectedAgentInstallTargets(
	ctx context.Context,
	skipAgentID string,
) ([]moduleInstallTarget, []string) {
	agents, err := s.ListInstallAgents(ctx)
	if err != nil {
		return nil, []string{fmt.Sprintf("list install agents failed: %v", err)}
	}

	targets := make([]moduleInstallTarget, 0, len(agents))
	warnings := make([]string, 0)
	skip := normalizeInstallAgentID(skipAgentID)

	for _, item := range agents {
		agentID := normalizeInstallAgentID(item.AgentID)
		if agentID == "" || agentID == skip {
			continue
		}
		if strings.ToLower(strings.TrimSpace(item.Status)) != "connected" {
			continue
		}
		endpoint := normalizeAgentGRPCEndpoint(item.AgentGRPCEndpoint)
		if endpoint == "" {
			warnings = append(warnings, fmt.Sprintf("skip broadcast to agent %s: missing grpc endpoint", agentID))
			continue
		}
		targets = append(targets, moduleInstallTarget{
			Scope:             ModuleInstallScopeRemote,
			AgentID:           agentID,
			AgentGRPCEndpoint: endpoint,
			Host:              firstNonEmpty(item.Host, item.IPAddress, hostFromEndpoint(endpoint)),
			Port:              22,
			Username:          firstNonEmpty(item.Username, "aurora"),
		})
	}
	return targets, warnings
}
