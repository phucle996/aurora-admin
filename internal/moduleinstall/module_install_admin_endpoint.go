package moduleinstall

import (
	"context"
	"fmt"
	"strings"
)

func (s *ModuleInstallService) resolveAdminBootstrapEndpoint(ctx context.Context) (string, error) {
	if s == nil || s.endpointRepo == nil {
		return "", fmt.Errorf("module install service is nil")
	}

	items, err := s.endpointRepo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("load admin endpoint failed: %w", err)
	}
	for _, item := range items {
		name := canonicalModuleName(strings.Trim(item.Name, "/"))
		if name != "admin" {
			continue
		}
		endpoint := resolveEndpointFromStoredValue(item.Value)
		if endpoint != "" {
			return endpoint, nil
		}
	}
	return "", fmt.Errorf("admin endpoint is empty")
}
