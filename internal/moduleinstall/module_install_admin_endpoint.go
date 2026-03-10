package moduleinstall

import (
	keycfg "admin/internal/key"
	"admin/internal/repository"
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func (s *ModuleInstallService) resolveAdminBootstrapEndpoint(
	ctx context.Context,
	items []repository.EndpointKV,
	listErr error,
) (string, error) {
	if s == nil || s.endpointRepo == nil {
		return "", fmt.Errorf("module install service is nil")
	}
	if listErr != nil {
		return "", fmt.Errorf("load admin endpoint failed: %w", listErr)
	}

	adminHost := ""
	for _, item := range items {
		name := canonicalModuleName(strings.Trim(item.Name, "/"))
		if name != "admin" {
			continue
		}
		endpoint := resolveEndpointFromStoredValue(item.Value)
		if endpoint == "" {
			continue
		}
		host, explicitPort, err := parseEndpointHostAndPort(endpoint)
		if err != nil {
			return "", err
		}
		adminHost = host
		// If admin endpoint already has explicit port, use it directly.
		if explicitPort > 0 {
			return net.JoinHostPort(adminHost, strconv.Itoa(explicitPort)), nil
		}
		break
	}
	if adminHost == "" {
		return "", fmt.Errorf("admin endpoint is empty")
	}

	// gRPC must go direct to Admin backend port, not via nginx 443.
	if s.runtimeRepo == nil {
		return "", fmt.Errorf("runtime repo is nil; cannot resolve direct admin grpc endpoint")
	}
	portRaw, found, err := s.runtimeRepo.Get(ctx, keycfg.RuntimeAppPortKey("admin"))
	if err != nil {
		return "", fmt.Errorf("load admin runtime app port failed: %w", err)
	}
	if !found {
		return "", fmt.Errorf("admin runtime app port is not configured")
	}
	port, err := strconv.Atoi(strings.TrimSpace(portRaw))
	if err != nil || port <= 0 || port > 65535 {
		return "", fmt.Errorf("admin runtime app port is invalid")
	}
	return net.JoinHostPort(adminHost, strconv.Itoa(port)), nil
}

func parseEndpointHostAndPort(raw string) (host string, port int, err error) {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" {
		return "", 0, fmt.Errorf("admin endpoint is empty")
	}

	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", 0, fmt.Errorf("invalid admin endpoint: %w", err)
		}
		host := strings.TrimSpace(parsed.Hostname())
		if host == "" {
			return "", 0, fmt.Errorf("admin endpoint host is empty")
		}
		portRaw := strings.TrimSpace(parsed.Port())
		if portRaw == "" {
			return host, 0, nil
		}
		port, err := strconv.Atoi(portRaw)
		if err != nil || port <= 0 || port > 65535 {
			return "", 0, fmt.Errorf("admin endpoint port is invalid")
		}
		return host, port, nil
	}

	host = strings.TrimSpace(endpointHost(endpoint))
	portRaw := strings.TrimSpace(endpointPort(endpoint))
	if host == "" {
		host = strings.TrimSpace(endpoint)
	}
	if host == "" {
		return "", 0, fmt.Errorf("admin endpoint host is empty")
	}
	if portRaw == "" {
		return host, 0, nil
	}
	port, err = strconv.Atoi(portRaw)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("admin endpoint port is invalid")
	}
	return host, port, nil
}
