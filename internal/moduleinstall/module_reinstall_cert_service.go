package moduleinstall

import (
	"admin/pkg/errorvar"
	"context"
	"fmt"
	"strings"
)

type ModuleReinstallCertRequest struct {
	ModuleName string
}

type ModuleReinstallCertResult struct {
	ModuleName        string
	Endpoint          string
	Warnings          []string
	HealthcheckPassed bool
}

func (s *ModuleInstallService) ReinstallCertWithLog(
	ctx context.Context,
	req ModuleReinstallCertRequest,
	logFn InstallLogFn,
) (*ModuleReinstallCertResult, error) {
	if s == nil || s.endpointRepo == nil || s.certStoreRepo == nil {
		return nil, errorvar.ErrModuleInstallServiceNil
	}

	moduleName := canonicalModuleName(req.ModuleName)
	if moduleName == "" || strings.Contains(moduleName, "admin") {
		return nil, errorvar.ErrModuleNameInvalid
	}

	target, endpoint, err := s.resolveModuleTargetByEndpoint(ctx, moduleName)
	if err != nil {
		return nil, err
	}
	if endpoint == "" {
		return nil, errorvar.ErrModuleEndpointInvalid
	}

	result := &ModuleReinstallCertResult{
		ModuleName: moduleName,
		Endpoint:   endpoint,
		Warnings:   []string{},
	}
	tlsPaths := resolveModuleTLSPaths(moduleName)

	appHost := endpointHost(endpoint)
	if appHost == "" {
		appHost = target.Host
	}

	logInstall(logFn, "reinstall-cert", "start module=%s endpoint=%s target=%s", moduleName, endpoint, target.Host)
	logInstall(logFn, "reinstall-cert", "resolved tls host=%s (from endpoint registry)", appHost)

	bundle, err := installModuleTLSOnTarget(ctx, target, moduleName, appHost, endpoint, logFn)
	if err != nil {
		logInstall(logFn, "reinstall-cert", "[error] %v", err)
		return nil, fmt.Errorf("reinstall cert failed: %w", err)
	}
	if err := s.seedModuleTLSBundle(ctx, moduleName, bundle); err != nil {
		logInstall(logFn, "reinstall-cert", "[error] %v", err)
		return nil, fmt.Errorf("seed module tls bundle failed: %w", err)
	}
	logInstall(logFn, "reinstall-cert", "tls materials reinstalled cert=%s key=%s ca=%s", tlsPaths.CertPath, tlsPaths.KeyPath, tlsPaths.CAPath)

	_, healthErr := ensureCurlAndCheckEndpoint(ctx, target, moduleName, endpoint, func(line string) {
		logInstall(logFn, "healthcheck", "%s", line)
	})
	if healthErr != nil {
		logInstall(logFn, "healthcheck", "[error] %v", healthErr)
		return result, fmt.Errorf("service healthcheck failed after reinstall cert: %w", healthErr)
	}

	result.HealthcheckPassed = true
	logInstall(logFn, "reinstall-cert", "[done] reinstall cert completed module=%s", moduleName)
	return result, nil
}

func (s *ModuleInstallService) resolveModuleTargetByEndpoint(
	ctx context.Context,
	moduleName string,
) (moduleInstallTarget, string, error) {
	items, err := s.endpointRepo.List(ctx)
	if err != nil {
		return moduleInstallTarget{}, "", err
	}

	targetModule := canonicalModuleName(moduleName)
	for _, item := range items {
		name := canonicalModuleName(strings.Trim(item.Name, "/"))
		if name == "" || !strings.EqualFold(name, targetModule) {
			continue
		}

		if target, endpoint, ok := parseEndpointTargetAndEndpoint(item.Value); ok {
			if endpoint == "" {
				return moduleInstallTarget{}, "", errorvar.ErrModuleEndpointInvalid
			}
			if strings.TrimSpace(target.Host) == "" {
				target.Host = normalizeAddress(hostFromEndpoint(target.AgentGRPCEndpoint))
			}
			if strings.TrimSpace(target.Host) == "" {
				target.Host = normalizeAddress(endpointHost(endpoint))
			}
			if strings.TrimSpace(target.Host) == "" {
				target.Host = "agent-target"
			}
			return target, endpoint, nil
		}

		return moduleInstallTarget{}, "", errorvar.ErrModuleEndpointInvalid
	}

	return moduleInstallTarget{}, "", errorvar.ErrModuleEndpointNotFound
}
