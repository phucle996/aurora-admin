package moduleinstall

import (
	keycfg "admin/internal/key"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *ModuleInstallService) pushSharedCORSToInstalledServices(
	ctx context.Context,
	logFn InstallLogFn,
) []string {
	warnings := make([]string, 0)
	if s == nil || s.endpointRepo == nil || s.sharedCorsRepo == nil || s.corsRPCClient == nil {
		return append(warnings, "cannot push CORS via RPC: module install service is nil")
	}

	corsValues, err := s.loadSharedCORSRuntimeValues(ctx)
	if err != nil {
		return append(warnings, fmt.Sprintf("cannot load shared cors for rpc push: %v", err))
	}

	items, err := s.endpointRepo.List(ctx)
	if err != nil {
		return append(warnings, fmt.Sprintf("cannot list endpoints for cors rpc push: %v", err))
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.Trim(items[i].Name, "/") < strings.Trim(items[j].Name, "/")
	})

	for _, item := range items {
		moduleName := canonicalModuleName(strings.Trim(item.Name, "/"))
		if moduleName == "" || strings.EqualFold(moduleName, "admin") {
			continue
		}

		endpoint := resolveEndpointFromStoredValue(item.Value)
		if endpoint == "" {
			warning := fmt.Sprintf("skip cors rpc push: invalid endpoint value for module=%s", moduleName)
			warnings = append(warnings, warning)
			logInstall(logFn, "cors.rpc", "[warn] %s", warning)
			continue
		}

		logInstall(logFn, "cors.rpc", "push shared cors to module=%s endpoint=%s", moduleName, endpoint)
		pushCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		pushErr := s.corsRPCClient.ApplySharedCORS(pushCtx, endpoint, corsValues)
		cancel()
		if pushErr != nil {
			warning := fmt.Sprintf("push shared cors failed module=%s endpoint=%s err=%v", moduleName, endpoint, pushErr)
			warnings = append(warnings, warning)
			logInstall(logFn, "cors.rpc", "[warn] %s", warning)
			continue
		}

		logInstall(logFn, "cors.rpc", "push shared cors success module=%s", moduleName)
	}

	return warnings
}

func (s *ModuleInstallService) loadSharedCORSRuntimeValues(ctx context.Context) (map[string]string, error) {
	type corsKeyMapping struct {
		runtimeKey string
		sharedKey  string
		def        string
	}
	mappings := []corsKeyMapping{
		{runtimeKey: "cors/allow_origins", sharedKey: keycfg.SharedCORSAllowOrigins, def: "[]"},
		{runtimeKey: "cors/allow_methods", sharedKey: keycfg.SharedCORSAllowMethods, def: "[]"},
		{runtimeKey: "cors/allow_headers", sharedKey: keycfg.SharedCORSAllowHeaders, def: "[]"},
		{runtimeKey: "cors/expose_headers", sharedKey: keycfg.SharedCORSExposeHeader, def: "[]"},
		{runtimeKey: "cors/allow_credentials", sharedKey: keycfg.SharedCORSAllowCreds, def: "true"},
		{runtimeKey: "cors/max_age", sharedKey: keycfg.SharedCORSMaxAge, def: "12h"},
	}

	values := make(map[string]string, len(mappings))
	for _, item := range mappings {
		raw, exists, err := s.sharedCorsRepo.Get(ctx, item.sharedKey)
		if err != nil {
			return nil, err
		}
		val := strings.TrimSpace(raw)
		if !exists || val == "" {
			val = item.def
		}
		values[item.runtimeKey] = val
	}
	return values, nil
}

func resolveEndpointFromStoredValue(raw string) string {
	if _, endpoint, ok := parseEndpointTargetAndEndpoint(raw); ok {
		return strings.TrimSpace(endpoint)
	}
	return strings.TrimSpace(parseLegacyEndpointValue(raw))
}
