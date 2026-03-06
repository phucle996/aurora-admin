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
	ModuleName string
	Scope      string
	Endpoint   string
	TargetHost string
	CertPath   string
	KeyPath    string
	CAPath     string
	Warnings   []string

	HealthcheckPassed bool
	HealthcheckOutput string
}

func (s *ModuleInstallService) ReinstallCertWithLog(
	ctx context.Context,
	req ModuleReinstallCertRequest,
	logFn InstallLogFn,
) (*ModuleReinstallCertResult, error) {
	if s == nil || s.endpointRepo == nil {
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

	tlsPaths := resolveModuleTLSPaths(moduleName)
	result := &ModuleReinstallCertResult{
		ModuleName: moduleName,
		Scope:      target.Scope,
		Endpoint:   endpoint,
		TargetHost: target.Host,
		CertPath:   tlsPaths.CertPath,
		KeyPath:    tlsPaths.KeyPath,
		CAPath:     tlsPaths.CAPath,
		Warnings:   []string{},
	}

	appHost := endpointHost(endpoint)
	if appHost == "" {
		appHost = target.Host
	}

	logInstall(logFn, "reinstall-cert", "start module=%s endpoint=%s target=%s", moduleName, endpoint, target.Host)
	logInstall(logFn, "reinstall-cert", "resolved tls host=%s (from endpoint registry)", appHost)

	if err := installModuleTLSOnTarget(ctx, target, moduleName, appHost, endpoint, logFn); err != nil {
		logInstall(logFn, "reinstall-cert", "[error] %v", err)
		return nil, fmt.Errorf("reinstall cert failed: %w", err)
	}
	logInstall(logFn, "reinstall-cert", "tls materials reinstalled cert=%s key=%s ca=%s", tlsPaths.CertPath, tlsPaths.KeyPath, tlsPaths.CAPath)

	healthOutput, healthErr := ensureCurlAndCheckEndpoint(ctx, target, moduleName, endpoint, func(line string) {
		logInstall(logFn, "healthcheck", "%s", line)
	})
	result.HealthcheckOutput = strings.TrimSpace(healthOutput)
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
			target.Scope = normalizeScope(target.Scope)
			if target.Scope == "" {
				target.Scope = ModuleInstallScopeLocal
			}
			if target.Port <= 0 || target.Port > 65535 {
				target.Port = 22
			}
			if strings.TrimSpace(target.Username) == "" {
				target.Username = "aurora"
			}
			if strings.TrimSpace(target.Host) == "" {
				target.Host = normalizeAddress(endpointHost(endpoint))
			}
			if strings.TrimSpace(target.Host) == "" {
				target.Host = detectLocalIPv4()
			}
			return target, endpoint, nil
		}

		legacyEndpoint := parseLegacyEndpointValue(item.Value)
		if legacyEndpoint == "" {
			return moduleInstallTarget{}, "", errorvar.ErrModuleEndpointInvalid
		}
		localHost := detectLocalIPv4()
		if localHost == "" {
			localHost = "127.0.0.1"
		}
		return moduleInstallTarget{
			Scope:    ModuleInstallScopeLocal,
			Username: "aurora",
			Host:     localHost,
			Port:     22,
		}, legacyEndpoint, nil
	}

	return moduleInstallTarget{}, "", errorvar.ErrModuleEndpointNotFound
}

func parseLegacyEndpointValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if status, endpoint, ok := strings.Cut(value, ":"); ok {
		if isKnownRuntimeStatus(status) {
			return strings.TrimSpace(endpoint)
		}
	}

	if strings.Contains(value, "://") {
		return value
	}
	if endpointHost(value) != "" {
		return value
	}
	return ""
}

func isKnownRuntimeStatus(raw string) bool {
	status := strings.ToLower(strings.TrimSpace(raw))
	switch status {
	case "running",
		"installed",
		"installing",
		"stopped",
		"degraded",
		"error",
		"healthy",
		"unhealthy",
		"maintenance",
		"not_installed",
		"unknown":
		return true
	default:
		return false
	}
}
