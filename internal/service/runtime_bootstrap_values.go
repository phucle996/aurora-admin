package service

import (
	keycfg "admin/internal/key"
	"admin/internal/repository"
	pkgutils "admin/pkg/utils"
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type bootstrapValueSource string

const bootstrapSourceRuntime bootstrapValueSource = "runtime"

type bootstrapValueSpec struct {
	Source    bootstrapValueSource
	StoreKey  string
	OutputKey string
	NonEmpty  bool
}

type endpointRuntimeDependency struct {
	TargetModule  string
	BaseURLOutput string
	GRPCOutput    string
}

func (s *RuntimeBootstrapService) BuildRuntimeValues(
	ctx context.Context,
	req RuntimeBootstrapRequest,
) (map[string]string, error) {
	if s == nil || s.runtimeRepo == nil || s.endpointRepo == nil || s.certStoreRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	moduleName := normalizeBootstrapModuleName(req.ModuleName)
	if moduleName == "" {
		return nil, fmt.Errorf("module_name is required")
	}

	schemaStoreKey := keycfg.RuntimeSchemaKey(moduleName)
	runtimeSpecs := buildRuntimeBootstrapSpecs(moduleName, schemaStoreKey)

	runtimeLoaded, err := s.loadBySpecs(ctx, s.runtimeRepo, runtimeSpecs, bootstrapSourceRuntime)
	if err != nil {
		return nil, err
	}

	values := make(map[string]string, len(runtimeSpecs)+7)
	missing := make([]string, 0)
	empty := make([]string, 0)

	m1, e1 := applySpecs(runtimeSpecs, runtimeLoaded, values)
	missing = append(missing, m1...)
	empty = append(empty, e1...)

	certValues, certErr := s.loadModuleTLSBundle(ctx, moduleName)
	if certErr != nil {
		return nil, certErr
	}
	for outputKey, certValue := range certValues {
		trimmed := strings.TrimSpace(certValue)
		if trimmed == "" {
			empty = append(empty, "cert_store/"+outputKey)
			continue
		}
		values[outputKey] = trimmed
	}

	if err := buildBootstrapValidationError(missing, empty); err != nil {
		return nil, err
	}

	endpointItems, endpointListErr := s.endpointRepo.List(ctx)
	endpointMap := buildModuleEndpointMap(endpointItems)

	appPort, err := s.resolveModulePort(ctx, moduleName, req.AppPort, endpointMap, endpointListErr)
	if err != nil {
		return nil, err
	}

	values["app/port"] = strconv.Itoa(int(appPort))
	for _, dep := range runtimeEndpointDependencies(moduleName) {
		baseURL, resolveErr := s.resolveModuleBaseURL(dep.TargetModule, endpointMap, endpointListErr)
		if resolveErr != nil {
			return nil, resolveErr
		}
		values[dep.BaseURLOutput] = baseURL
		values[dep.GRPCOutput] = toGRPCEndpoint(baseURL)
	}

	return values, nil
}

func buildRuntimeBootstrapSpecs(moduleName string, schemaStoreKey string) []bootstrapValueSpec {
	specs := []bootstrapValueSpec{
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTAppTZ, OutputKey: "app/timezone", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTAppLogLevel, OutputKey: "app/log_level", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTPgURL, OutputKey: "postgresql/url", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTPgSSLMode, OutputKey: "postgresql/sslmode", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: schemaStoreKey, OutputKey: "postgresql/schema", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisAddr, OutputKey: "redis/addr", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisUser, OutputKey: "redis/username", NonEmpty: false},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisPass, OutputKey: "redis/password", NonEmpty: false},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisDB, OutputKey: "redis/db", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisTLS, OutputKey: "redis/use_tls", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisCA, OutputKey: "redis/ca", NonEmpty: false},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisKey, OutputKey: "redis/client_key", NonEmpty: false},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisCert, OutputKey: "redis/client_cert", NonEmpty: false},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisInsecure, OutputKey: "redis/insecure_skip_verify", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTTTLAccess, OutputKey: "token_ttl/access_ttl", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTTTLRefresh, OutputKey: "token_ttl/refresh_ttl", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTTTLDevice, OutputKey: "token_ttl/device_ttl", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTTTLOTT, OutputKey: "token_ttl/ott_ttl", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTSecretCachePrefix, OutputKey: "token_secret/cache_prefix", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTSecretCacheChannel, OutputKey: "token_secret/cache_channel", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTSecretPollEvery, OutputKey: "token_secret/poll_interval", NonEmpty: true},
	}
	if isPlatformModuleName(moduleName) {
		specs = append(specs, bootstrapValueSpec{
			Source:    bootstrapSourceRuntime,
			StoreKey:  keycfg.RTPlatformKubeconfigCipherKey,
			OutputKey: "platform/kubeconfig_cipher_key",
			NonEmpty:  true,
		})
	}
	return specs
}

func (s *RuntimeBootstrapService) loadBySpecs(
	ctx context.Context,
	repo repository.RuntimeConfigRepository,
	specs []bootstrapValueSpec,
	source bootstrapValueSource,
) (map[string]string, error) {
	keys := collectSpecStoreKeys(specs)
	values, err := repo.GetMany(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("load %s keys failed: %w", source, err)
	}
	if values == nil {
		return map[string]string{}, nil
	}
	return values, nil
}

func collectSpecStoreKeys(specs []bootstrapValueSpec) []string {
	if len(specs) == 0 {
		return nil
	}
	out := make([]string, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		key := strings.TrimSpace(spec.StoreKey)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func applySpecs(
	specs []bootstrapValueSpec,
	loaded map[string]string,
	out map[string]string,
) (missing []string, empty []string) {
	for _, spec := range specs {
		value, exists := loaded[spec.StoreKey]
		if !exists {
			missing = append(missing, formatBootstrapStoreKey(spec.Source, spec.StoreKey))
			continue
		}
		trimmed := strings.TrimSpace(value)
		if spec.NonEmpty && trimmed == "" {
			empty = append(empty, formatBootstrapStoreKey(spec.Source, spec.StoreKey))
			continue
		}
		out[spec.OutputKey] = trimmed
	}
	return missing, empty
}

func formatBootstrapStoreKey(source bootstrapValueSource, storeKey string) string {
	switch source {
	case bootstrapSourceRuntime:
		return keycfg.RuntimeStoreKey(storeKey)
	default:
		return string(source) + "/" + strings.Trim(strings.TrimSpace(storeKey), "/")
	}
}

func buildBootstrapValidationError(missing []string, empty []string) error {
	if len(missing) == 0 && len(empty) == 0 {
		return nil
	}

	sort.Strings(missing)
	sort.Strings(empty)
	parts := make([]string, 0, 2)
	if len(missing) > 0 {
		parts = append(parts, "missing=["+strings.Join(missing, ", ")+"]")
	}
	if len(empty) > 0 {
		parts = append(parts, "empty=["+strings.Join(empty, ", ")+"]")
	}
	return fmt.Errorf("runtime bootstrap validation failed: %s", strings.Join(parts, "; "))
}

func normalizeBootstrapModuleName(raw string) string {
	name := strings.ToLower(strings.Trim(strings.TrimSpace(raw), "/"))
	switch name {
	case "platform", "platform-resource", "platform_resource", "plaform-resource", "plaform_resource":
		return "platform"
	case "paas", "paas-service", "paas_service":
		return "paas"
	case "dbaas", "dbaas-service", "dbaas_service":
		return "dbaas"
	case "dbaas-module", "dbaas_module":
		return "dbaas"
	default:
		return name
	}
}

func isPlatformModuleName(name string) bool {
	return normalizeBootstrapModuleName(name) == "platform"
}

func runtimeEndpointDependencies(moduleName string) []endpointRuntimeDependency {
	switch normalizeBootstrapModuleName(moduleName) {
	case "paas", "dbaas":
		return []endpointRuntimeDependency{
			{
				TargetModule:  "platform",
				BaseURLOutput: "platform/base_url",
				GRPCOutput:    "platform/grpc_endpoint",
			},
		}
	default:
		return nil
	}
}

func (s *RuntimeBootstrapService) resolveModulePort(
	ctx context.Context,
	moduleName string,
	requestedPort int32,
	endpointMap map[string]string,
	endpointListErr error,
) (int32, error) {
	if requestedPort < 0 || requestedPort > 65535 {
		return 0, fmt.Errorf("app_port is invalid")
	}

	canonicalName := normalizeBootstrapModuleName(moduleName)
	if requestedPort > 0 {
		return requestedPort, nil
	}

	if s != nil && s.runtimeRepo != nil {
		appPortRaw, found, getErr := s.runtimeRepo.Get(ctx, keycfg.RuntimeAppPortKey(canonicalName))
		if getErr != nil {
			return 0, fmt.Errorf("resolve runtime app port for %s failed: %w", canonicalName, getErr)
		}
		if found {
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(appPortRaw))
			if parseErr != nil || parsed <= 0 || parsed > 65535 {
				return 0, fmt.Errorf("runtime app port for %s is invalid", canonicalName)
			}
			return int32(parsed), nil
		}
	}

	if endpointListErr != nil {
		return 0, fmt.Errorf("resolve %s endpoint failed: %w", canonicalName, endpointListErr)
	}

	endpoint := strings.TrimSpace(endpointMap[canonicalName])
	if endpoint == "" {
		return 0, fmt.Errorf("%s endpoint not found", canonicalName)
	}
	port := strings.TrimSpace(pkgutils.EndpointPort(endpoint))
	if port == "" {
		return 0, fmt.Errorf("%s endpoint has no port", canonicalName)
	}
	parsed, parseErr := strconv.Atoi(port)
	if parseErr != nil || parsed <= 0 || parsed > 65535 {
		return 0, fmt.Errorf("%s endpoint port is invalid", canonicalName)
	}
	return int32(parsed), nil
}

func resolveEndpointFromStoredValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if _, endpoint, ok := parseEndpointValueWithScope(value); ok {
		return strings.TrimSpace(endpoint)
	}
	if status, endpoint, ok := strings.Cut(value, ":"); ok && isLegacyEndpointStatus(status) {
		return strings.TrimSpace(endpoint)
	}
	return value
}

func isLegacyEndpointStatus(raw string) bool {
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

func buildModuleEndpointMap(items []repository.EndpointKV) map[string]string {
	out := make(map[string]string, len(items))
	for _, item := range items {
		moduleName := normalizeBootstrapModuleName(item.Name)
		if moduleName == "" {
			continue
		}
		if _, exists := out[moduleName]; exists {
			continue
		}
		endpoint := strings.TrimSpace(resolveEndpointFromStoredValue(item.Value))
		if endpoint == "" {
			continue
		}
		out[moduleName] = endpoint
	}
	return out
}

func (s *RuntimeBootstrapService) resolveModuleBaseURL(
	moduleName string,
	endpointMap map[string]string,
	endpointListErr error,
) (string, error) {
	targetName := normalizeBootstrapModuleName(moduleName)
	if targetName == "" {
		return "", fmt.Errorf("target module is required")
	}
	if endpointListErr != nil {
		return "", fmt.Errorf("resolve %s endpoint failed: %w", targetName, endpointListErr)
	}
	endpoint := strings.TrimSpace(endpointMap[targetName])
	if endpoint == "" {
		return "", fmt.Errorf("%s endpoint not found", targetName)
	}
	if strings.HasPrefix(endpoint, "http://") {
		return "", fmt.Errorf("%s endpoint must use https for mTLS", targetName)
	}
	if strings.HasPrefix(endpoint, "https://") {
		return strings.TrimRight(endpoint, "/"), nil
	}
	return "https://" + strings.TrimRight(endpoint, "/"), nil
}

func toGRPCEndpoint(baseURL string) string {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil {
			host := strings.TrimSpace(parsed.Host)
			if host == "" {
				return ""
			}
			if _, _, splitErr := net.SplitHostPort(host); splitErr == nil {
				return host
			}
			return net.JoinHostPort(host, "443")
		}
	}
	host := strings.Trim(raw, "/")
	if host == "" {
		return ""
	}
	if _, _, splitErr := net.SplitHostPort(host); splitErr == nil {
		return host
	}
	return net.JoinHostPort(host, "443")
}
