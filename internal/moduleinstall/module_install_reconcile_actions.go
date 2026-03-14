package moduleinstall

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *ModuleInstallService) triggerAutoReinstall(
	desired desiredModuleInstallState,
) {
	if s == nil {
		return
	}
	go func() {
		installCtx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()
		_, _ = s.InstallWithLog(installCtx, ModuleInstallRequest{
			ModuleName: desired.Module,
			AgentID:    desired.AgentID,
			AppHost:    desired.AppHost,
		}, nil)
	}()
}

func (s *ModuleInstallService) updateDesiredStateForDrift(
	ctx context.Context,
	desired desiredModuleInstallState,
	status string,
	message string,
) error {
	if s == nil || s.runtimeRepo == nil {
		return nil
	}
	desired.Status = normalizeDesiredInstallStatus(status)
	desired.LastError = strings.TrimSpace(message)
	desired.LastUpdateAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.persistDesiredInstallState(ctx, desired)
}

func (s *ModuleInstallService) executeAutoRestart(
	ctx context.Context,
	target moduleInstallTarget,
	desired desiredModuleInstallState,
	actual actualModuleInstallState,
) error {
	serviceName := strings.TrimSpace(actual.ServiceName)
	if serviceName == "" {
		serviceName = resolvedInstalledServiceName(&ModuleInstallResult{
			ModuleName:  desired.Module,
			ServiceName: actual.ServiceName,
		}, desired.Module)
	}
	if serviceName == "" {
		return fmt.Errorf("service name is required for auto-restart")
	}
	res, err := restartModuleOnAgent(ctx, target, agentRestartModuleRequest{
		APIVersion:  installerRPCVersionV1,
		RequestID:   newAgentOperationRequestID("reconcile-restart", desired.Module),
		Module:      desired.Module,
		ServiceName: serviceName,
	})
	if err != nil {
		return err
	}
	if res == nil || !res.OK {
		if res != nil && strings.TrimSpace(res.ErrorText) != "" {
			return fmt.Errorf("%s", strings.TrimSpace(res.ErrorText))
		}
		return fmt.Errorf("auto-restart failed")
	}
	return nil
}
