package service

import (
	"admin/internal/endpointmeta"
	keycfg "admin/internal/key"
	runtimerepo "admin/internal/runtime/repository"
	pkgutils "admin/pkg/utils"
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

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
	case "dbaas", "dbaas-service", "dbaas_service", "dbaas-module", "dbaas_module":
		return "dbaas"
	default:
		return name
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
	return endpointmeta.ExtractEndpoint(raw)
}

func buildModuleEndpointMap(items []runtimerepo.EndpointKV) map[string]string {
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
