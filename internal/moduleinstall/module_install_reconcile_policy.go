package moduleinstall

import (
	keycfg "admin/internal/key"
	"context"
	"fmt"
	"strings"
)

const (
	moduleInstallDesiredStatusPending          = "pending"
	moduleInstallDesiredStatusInstalling       = "installing"
	moduleInstallDesiredStatusInstalled        = "installed"
	moduleInstallDesiredStatusUpgradePending   = "upgrade-pending"
	moduleInstallDesiredStatusUninstallPending = "uninstall-pending"
	moduleInstallDesiredStatusFailed           = "failed"

	moduleInstallReconcilePolicyManual        = "manual"
	moduleInstallReconcilePolicyAutoRestart   = "auto-restart"
	moduleInstallReconcilePolicyAutoReinstall = "auto-reinstall"

	moduleInstallReconcileActionNone          = "none"
	moduleInstallReconcileActionManual        = "manual"
	moduleInstallReconcileActionAutoRestart   = "auto-restart"
	moduleInstallReconcileActionAutoReinstall = "auto-reinstall"

	moduleInstallDriftNone             = ""
	moduleInstallDriftMissing          = "missing"
	moduleInstallDriftVersionMismatch  = "version-mismatch"
	moduleInstallDriftRuntimeUnhealthy = "runtime-unhealthy"
	moduleInstallDriftAgentUnreachable = "agent-unreachable"
)

func normalizeDesiredInstallStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", moduleInstallDesiredStatusInstalling:
		return moduleInstallDesiredStatusInstalling
	case moduleInstallDesiredStatusPending:
		return moduleInstallDesiredStatusPending
	case moduleInstallDesiredStatusInstalled:
		return moduleInstallDesiredStatusInstalled
	case moduleInstallDesiredStatusUpgradePending:
		return moduleInstallDesiredStatusUpgradePending
	case moduleInstallDesiredStatusUninstallPending:
		return moduleInstallDesiredStatusUninstallPending
	case moduleInstallDesiredStatusFailed:
		return moduleInstallDesiredStatusFailed
	default:
		return strings.TrimSpace(raw)
	}
}

func normalizeInstallReconcilePolicy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", moduleInstallReconcilePolicyManual:
		return moduleInstallReconcilePolicyManual
	case moduleInstallReconcilePolicyAutoRestart:
		return moduleInstallReconcilePolicyAutoRestart
	case moduleInstallReconcilePolicyAutoReinstall:
		return moduleInstallReconcilePolicyAutoReinstall
	default:
		return moduleInstallReconcilePolicyManual
	}
}

func selectReconcileAction(policy string, driftReason string) string {
	policy = normalizeInstallReconcilePolicy(policy)
	switch driftReason {
	case moduleInstallDriftMissing, moduleInstallDriftVersionMismatch:
		if policy == moduleInstallReconcilePolicyAutoReinstall {
			return moduleInstallReconcileActionAutoReinstall
		}
		return moduleInstallReconcileActionManual
	case moduleInstallDriftRuntimeUnhealthy:
		if policy == moduleInstallReconcilePolicyAutoRestart || policy == moduleInstallReconcilePolicyAutoReinstall {
			return moduleInstallReconcileActionAutoRestart
		}
		return moduleInstallReconcileActionManual
	case moduleInstallDriftAgentUnreachable:
		return moduleInstallReconcileActionManual
	default:
		return moduleInstallReconcileActionNone
	}
}

func detectInstallDrift(desired *desiredModuleInstallState, actual actualModuleInstallState) string {
	if desired == nil {
		return moduleInstallDriftNone
	}
	if normalizeDesiredInstallStatus(desired.Status) == moduleInstallDesiredStatusUninstallPending {
		return moduleInstallDriftNone
	}
	if normalizeObservedInstallStatus(actual.Status) == moduleInstallStatusMissing {
		return moduleInstallDriftMissing
	}
	if strings.TrimSpace(desired.Version) != "" &&
		strings.TrimSpace(actual.Version) != "" &&
		strings.TrimSpace(desired.Version) != strings.TrimSpace(actual.Version) {
		return moduleInstallDriftVersionMismatch
	}
	switch normalizeObservedInstallHealth(actual.Health) {
	case moduleInstallHealthUnhealthy, moduleInstallHealthDegraded:
		return moduleInstallDriftRuntimeUnhealthy
	default:
		return moduleInstallDriftNone
	}
}

func desiredStatusForDrift(current string, driftReason string) string {
	switch driftReason {
	case moduleInstallDriftMissing:
		return moduleInstallDesiredStatusPending
	case moduleInstallDriftVersionMismatch:
		return moduleInstallDesiredStatusUpgradePending
	default:
		return normalizeDesiredInstallStatus(current)
	}
}

func (s *ModuleInstallService) loadInstallReconcilePolicy(ctx context.Context) (string, error) {
	if s == nil || s.runtimeRepo == nil {
		return moduleInstallReconcilePolicyManual, nil
	}
	value, found, err := s.runtimeRepo.Get(ctx, keycfg.RTInstallReconcilePolicy)
	if err != nil {
		return "", fmt.Errorf("load install reconcile policy failed: %w", err)
	}
	if !found {
		return moduleInstallReconcilePolicyManual, nil
	}
	return normalizeInstallReconcilePolicy(value), nil
}
