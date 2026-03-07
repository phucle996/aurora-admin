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

type moduleDefinition struct {
	Name    string
	Aliases []string
}

var runtimeModuleDefinitions = []moduleDefinition{
	{Name: "vm", Aliases: []string{"vm", "vm-service", "kvm", "hypervisor", "libvirt"}},
	{Name: "docker", Aliases: []string{"docker"}},
	{Name: "k8s", Aliases: []string{"k8s", "kubernetes"}},
	{Name: "ums", Aliases: []string{"ums", "user", "user-management", "usermanagment"}},
	{Name: "mail", Aliases: []string{"mail", "smtp"}},
	{Name: "gateway", Aliases: []string{"gateway", "nginx", "proxy"}},
	{Name: "monitoring", Aliases: []string{"monitoring", "monitor", "metrics", "victoria", "prometheus"}},
	{Name: "platform", Aliases: []string{"platform", "platform-resource"}},
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

	parsedItems := make([]EnabledModule, 0, len(kvItems))
	for _, item := range kvItems {
		moduleName := strings.TrimSpace(item.Name)
		if moduleName == "" {
			continue
		}

		status, endpoint, installed := parseModuleValue(item.Value)
		parsed := EnabledModule{
			Name:      moduleName,
			Status:    status,
			Endpoint:  endpoint,
			Installed: installed,
		}
		if isAdminModule(parsed) {
			continue
		}
		parsedItems = append(parsedItems, parsed)
	}

	items := make([]EnabledModule, 0, len(runtimeModuleDefinitions)+len(parsedItems))
	used := make([]bool, len(parsedItems))

	for _, moduleDef := range runtimeModuleDefinitions {
		matchedIndex := findMatchedModuleIndex(parsedItems, used, moduleDef.Aliases)
		if matchedIndex >= 0 {
			used[matchedIndex] = true
			matched := parsedItems[matchedIndex]
			matched.Name = moduleDef.Name
			items = append(items, matched)
			continue
		}

		items = append(items, EnabledModule{
			Name:      moduleDef.Name,
			Status:    "not_installed",
			Endpoint:  "",
			Installed: false,
		})
	}

	for idx, item := range parsedItems {
		if used[idx] {
			continue
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return items, nil
}

func findMatchedModuleIndex(items []EnabledModule, used []bool, aliases []string) int {
	for idx, item := range items {
		if used[idx] {
			continue
		}
		text := normalizeStatus(item.Name + " " + item.Endpoint)
		for _, alias := range aliases {
			aliasNorm := normalizeStatus(alias)
			if aliasNorm == "" {
				continue
			}
			if strings.Contains(text, aliasNorm) {
				return idx
			}
		}
	}
	return -1
}

func isAdminModule(item EnabledModule) bool {
	text := normalizeStatus(item.Name + " " + item.Endpoint)
	return strings.Contains(text, "admin")
}

func parseModuleValue(raw string) (status string, endpoint string, installed bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "not_installed", "", false
	}

	if metaScope, endpointValue, ok := parseEndpointValueWithScope(value); ok {
		if endpointValue == "" {
			return "not_installed", "", false
		}
		if metaScope == "remote" {
			return "installed", endpointValue, true
		}
		return "running", endpointValue, true
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

func parseEndpointValueWithScope(value string) (scope string, endpoint string, ok bool) {
	scopeRaw, rest, hasScope := strings.Cut(value, "(")
	if !hasScope {
		return "", "", false
	}
	scopeNorm := normalizeStatus(scopeRaw)
	if scopeNorm != "local" && scopeNorm != "remote" {
		return "", "", false
	}
	_, endpointPart, hasEndpoint := strings.Cut(rest, "):")
	if !hasEndpoint {
		return "", "", false
	}
	return scopeNorm, strings.TrimSpace(endpointPart), true
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
