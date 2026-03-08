package moduleinstall

import (
	"admin/internal/repository"
	"fmt"
	"net"
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
			if strings.TrimSpace(endpointPort(endpoint)) == "" {
				return net.JoinHostPort(endpoint, "443"), nil
			}
			return endpoint, nil
		}
	}
	return "", fmt.Errorf("admin endpoint is empty")
}
