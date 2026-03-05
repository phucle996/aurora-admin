package service

import (
	"admin/internal/repository"
	"admin/pkg/errorvar"
	"context"
	"sort"
	"strings"
)

type EnabledModule struct {
	Name      string
	Status    string
	Endpoint  string
	Installed bool
}

type EnabledModuleService struct {
	repo repository.EndpointRepository
}

func NewEnabledModuleService(repo repository.EndpointRepository) *EnabledModuleService {
	return &EnabledModuleService{
		repo: repo,
	}
}

func (s *EnabledModuleService) List(ctx context.Context) ([]EnabledModule, error) {
	if s == nil || s.repo == nil {
		return nil, errorvar.ErrEnabledModuleServiceNil
	}

	kvItems, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]EnabledModule, 0, len(kvItems))
	for _, item := range kvItems {
		moduleName := strings.TrimSpace(item.Name)
		if moduleName == "" {
			continue
		}

		status, endpoint, installed := parseModuleValue(item.Value)
		items = append(items, EnabledModule{
			Name:      moduleName,
			Status:    status,
			Endpoint:  endpoint,
			Installed: installed,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return items, nil
}

func parseModuleValue(raw string) (status string, endpoint string, installed bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "not_installed", "", false
	}

	left, right, ok := strings.Cut(value, ":")
	if ok {
		maybeStatus := normalizeStatus(left)
		if isKnownModuleStatus(maybeStatus) {
			parsedEndpoint := strings.TrimSpace(right)
			if parsedEndpoint == "" {
				return maybeStatus, "", false
			}
			return maybeStatus, parsedEndpoint, true
		}
	}

	// Backward compatibility with legacy values that store endpoint only.
	return "installed", value, true
}

func normalizeStatus(raw string) string {
	replacer := strings.NewReplacer(" ", "_", "-", "_")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(raw)))
}

func isKnownModuleStatus(status string) bool {
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
