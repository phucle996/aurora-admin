package moduleinstall

import (
	"admin/internal/repository"
	"fmt"
	"net"
	"net/url"
	"strings"
)

func (s *ModuleInstallService) resolveAdminBootstrapEndpoint(items []repository.EndpointKV, listErr error) (string, error) {
	if s == nil || s.endpointRepo == nil {
		return "", fmt.Errorf("module install service is nil")
	}
	if listErr != nil {
		return "", fmt.Errorf("load admin endpoint failed: %w", listErr)
	}
	for _, item := range items {
		name := canonicalModuleName(strings.Trim(item.Name, "/"))
		if name != "admin" {
			continue
		}
		endpoint := resolveEndpointFromStoredValue(item.Value)
		if endpoint != "" {
			return normalizeGRPCEndpoint(endpoint)
		}
	}
	return "", fmt.Errorf("admin endpoint is empty")
}

func normalizeGRPCEndpoint(raw string) (string, error) {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" {
		return "", fmt.Errorf("admin endpoint is empty")
	}
	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", fmt.Errorf("invalid admin endpoint: %w", err)
		}
		host := strings.TrimSpace(parsed.Hostname())
		if host == "" {
			return "", fmt.Errorf("admin endpoint host is empty")
		}
		port := strings.TrimSpace(parsed.Port())
		if port == "" {
			port = "443"
		}
		return net.JoinHostPort(host, port), nil
	}

	host := strings.TrimSpace(endpointHost(endpoint))
	port := strings.TrimSpace(endpointPort(endpoint))
	if host == "" {
		host = strings.TrimSpace(endpoint)
	}
	if host == "" {
		return "", fmt.Errorf("admin endpoint host is empty")
	}
	if port == "" {
		port = "443"
	}
	return net.JoinHostPort(host, port), nil
}
