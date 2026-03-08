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

type RuntimeBootstrapRequest struct {
	ModuleName string
	AppPort    int32
}

type RuntimeBootstrapService struct {
	runtimeRepo  repository.RuntimeConfigRepository
	endpointRepo repository.EndpointRepository
}

type bootstrapValueSource string

const (
	bootstrapSourceRuntime bootstrapValueSource = "runtime"
)

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

func NewRuntimeBootstrapService(
	runtimeRepo repository.RuntimeConfigRepository,
	endpointRepo repository.EndpointRepository,
) *RuntimeBootstrapService {
	return &RuntimeBootstrapService{
		runtimeRepo:  runtimeRepo,
		endpointRepo: endpointRepo,
	}
}

func (s *RuntimeBootstrapService) BuildRuntimeValues(
	ctx context.Context,
	req RuntimeBootstrapRequest,
) (map[string]string, error) {
	if s == nil || s.runtimeRepo == nil || s.endpointRepo == nil {
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

	if err := buildBootstrapValidationError(missing, empty); err != nil {
		return nil, err
	}

	appPort, err := s.resolveModulePort(ctx, moduleName, req.AppPort)
	if err != nil {
		return nil, err
	}

	values["app/port"] = strconv.Itoa(int(appPort))
	for _, dep := range runtimeEndpointDependencies(moduleName) {
		baseURL, resolveErr := s.resolveModuleBaseURL(ctx, dep.TargetModule)
		if resolveErr != nil {
			return nil, resolveErr
		}
		values[dep.BaseURLOutput] = baseURL
		values[dep.GRPCOutput] = toGRPCEndpoint(baseURL)
	}
	values["cors/allow_origins"] = `["https://localhost:80","https://localhost:443"]`
	values["cors/allow_methods"] = `["GET","POST","PUT","PATCH","DELETE","HEAD","OPTIONS"]`
	values["cors/allow_headers"] = `["Origin","Content-Type","Accept","Authorization"]`
	values["cors/expose_headers"] = "[]"
	values["cors/allow_credentials"] = "true"
	values["cors/max_age"] = "12h"

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
) (int32, error) {
	if requestedPort < 0 || requestedPort > 65535 {
		return 0, fmt.Errorf("app_port is invalid")
	}
	if requestedPort > 0 {
		return requestedPort, nil
	}

	items, err := s.endpointRepo.List(ctx)
	if err != nil {
		if fallback := defaultBootstrapAppPort(moduleName); fallback > 0 {
			return fallback, nil
		}
		return pkgutils.RandomAvailableLocalPort()
	}

	for _, item := range items {
		name := normalizeBootstrapModuleName(item.Name)
		if name != moduleName {
			continue
		}
		endpoint := resolveEndpointFromStoredValue(strings.TrimSpace(item.Value))
		if endpoint == "" {
			continue
		}
		port := strings.TrimSpace(pkgutils.EndpointPort(endpoint))
		if port == "" {
			continue
		}
		parsed, parseErr := strconv.Atoi(port)
		if parseErr != nil || parsed <= 0 || parsed > 65535 {
			continue
		}
		return int32(parsed), nil
	}

	if fallback := defaultBootstrapAppPort(moduleName); fallback > 0 {
		return fallback, nil
	}
	return pkgutils.RandomAvailableLocalPort()
}

func defaultBootstrapAppPort(moduleName string) int32 {
	switch normalizeBootstrapModuleName(moduleName) {
	case "ums":
		return 3005
	case "platform":
		return 8080
	default:
		return 0
	}
}

func resolveEndpointFromStoredValue(raw string) string {
	if _, endpoint, ok := parseEndpointValueWithScope(raw); ok {
		return strings.TrimSpace(endpoint)
	}
	if _, endpoint, ok := strings.Cut(raw, ":"); ok {
		return strings.TrimSpace(endpoint)
	}
	return ""
}

func (s *RuntimeBootstrapService) resolveModuleBaseURL(ctx context.Context, moduleName string) (string, error) {
	targetName := normalizeBootstrapModuleName(moduleName)
	if targetName == "" {
		return "", fmt.Errorf("target module is required")
	}
	items, err := s.endpointRepo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve %s endpoint failed: %w", targetName, err)
	}
	for _, item := range items {
		if normalizeBootstrapModuleName(item.Name) != targetName {
			continue
		}
		endpoint := strings.TrimSpace(resolveEndpointFromStoredValue(item.Value))
		if endpoint == "" {
			continue
		}
		if strings.HasPrefix(endpoint, "https://") || strings.HasPrefix(endpoint, "http://") {
			return strings.TrimRight(endpoint, "/"), nil
		}
		return "https://" + strings.TrimRight(endpoint, "/"), nil
	}
	return "", fmt.Errorf("%s endpoint not found", targetName)
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
			if strings.EqualFold(parsed.Scheme, "http") {
				return net.JoinHostPort(host, "80")
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
