package service

import (
	keycfg "admin/internal/key"
	"admin/internal/repository"
	pkgutils "admin/pkg/utils"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type RuntimeBootstrapRequest struct {
	ModuleName string
	AppPort    int32
}

type RuntimeBootstrapService struct {
	runtimeRepo    repository.RuntimeConfigRepository
	sharedCorsRepo repository.RuntimeConfigRepository
	endpointRepo   repository.EndpointRepository
}

type bootstrapValueSource string

const (
	bootstrapSourceRuntime    bootstrapValueSource = "runtime"
	bootstrapSourceSharedCORS bootstrapValueSource = "shared_cors"
)

type bootstrapValueSpec struct {
	Source    bootstrapValueSource
	StoreKey  string
	OutputKey string
	NonEmpty  bool
}

func NewRuntimeBootstrapService(
	runtimeRepo repository.RuntimeConfigRepository,
	sharedCorsRepo repository.RuntimeConfigRepository,
	endpointRepo repository.EndpointRepository,
) *RuntimeBootstrapService {
	return &RuntimeBootstrapService{
		runtimeRepo:    runtimeRepo,
		sharedCorsRepo: sharedCorsRepo,
		endpointRepo:   endpointRepo,
	}
}

func (s *RuntimeBootstrapService) BuildRuntimeValues(
	ctx context.Context,
	req RuntimeBootstrapRequest,
) (map[string]string, error) {
	if s == nil || s.runtimeRepo == nil || s.sharedCorsRepo == nil || s.endpointRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	moduleName := normalizeBootstrapModuleName(req.ModuleName)
	if moduleName == "" {
		return nil, fmt.Errorf("module_name is required")
	}

	schemaStoreKey := keycfg.RuntimeSchemaKey(moduleName)

	runtimeSpecs := buildRuntimeBootstrapSpecs(moduleName, schemaStoreKey)
	sharedCORSSpecs := buildSharedCORSBootstrapSpecs()

	runtimeLoaded, err := s.loadBySpecs(ctx, s.runtimeRepo, runtimeSpecs, bootstrapSourceRuntime)
	if err != nil {
		return nil, err
	}
	sharedCorsLoaded, err := s.loadBySpecs(ctx, s.sharedCorsRepo, sharedCORSSpecs, bootstrapSourceSharedCORS)
	if err != nil {
		return nil, err
	}

	values := make(map[string]string, len(runtimeSpecs)+len(sharedCORSSpecs)+1)
	missing := make([]string, 0)
	empty := make([]string, 0)

	m1, e1 := applySpecs(runtimeSpecs, runtimeLoaded, values)
	missing = append(missing, m1...)
	empty = append(empty, e1...)

	m2, e2 := applySpecs(sharedCORSSpecs, sharedCorsLoaded, values)
	missing = append(missing, m2...)
	empty = append(empty, e2...)

	if err := buildBootstrapValidationError(missing, empty); err != nil {
		return nil, err
	}

	appPort, err := s.resolveModulePort(ctx, moduleName, req.AppPort)
	if err != nil {
		return nil, err
	}

	values["app/port"] = strconv.Itoa(int(appPort))

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

func buildSharedCORSBootstrapSpecs() []bootstrapValueSpec {
	return []bootstrapValueSpec{
		{Source: bootstrapSourceSharedCORS, StoreKey: keycfg.SharedCORSAllowOrigins, OutputKey: "cors/allow_origins", NonEmpty: true},
		{Source: bootstrapSourceSharedCORS, StoreKey: keycfg.SharedCORSAllowMethods, OutputKey: "cors/allow_methods", NonEmpty: true},
		{Source: bootstrapSourceSharedCORS, StoreKey: keycfg.SharedCORSAllowHeaders, OutputKey: "cors/allow_headers", NonEmpty: true},
		{Source: bootstrapSourceSharedCORS, StoreKey: keycfg.SharedCORSExposeHeader, OutputKey: "cors/expose_headers", NonEmpty: true},
		{Source: bootstrapSourceSharedCORS, StoreKey: keycfg.SharedCORSAllowCreds, OutputKey: "cors/allow_credentials", NonEmpty: true},
		{Source: bootstrapSourceSharedCORS, StoreKey: keycfg.SharedCORSMaxAge, OutputKey: "cors/max_age", NonEmpty: true},
	}
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
	case bootstrapSourceSharedCORS:
		return keycfg.SharedCORSStoreKey(storeKey)
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
	default:
		return name
	}
}

func isPlatformModuleName(name string) bool {
	return normalizeBootstrapModuleName(name) == "platform"
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

	return pkgutils.RandomAvailableLocalPort()
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
