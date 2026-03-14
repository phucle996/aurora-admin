package endpointmeta

import (
	pkgutils "admin/pkg/utils"
	"strings"
)

type Parsed struct {
	Scope             string
	Endpoint          string
	AgentID           string
	AgentGRPCEndpoint string
	Host              string
	HasMetadata       bool
}

func Parse(raw string) Parsed {
	value := strings.TrimSpace(raw)
	if value == "" {
		return Parsed{}
	}

	scopeRaw, remainder, hasScope := strings.Cut(value, "(")
	if !hasScope {
		return Parsed{}
	}

	scope := normalizeToken(scopeRaw)
	if scope != "local" && scope != "remote" {
		return Parsed{}
	}

	meta, endpoint, hasEndpoint := strings.Cut(remainder, "):")
	if !hasEndpoint {
		return Parsed{}
	}

	parsed := Parsed{
		Scope:       scope,
		Endpoint:    strings.TrimSpace(endpoint),
		HasMetadata: true,
	}

	parts := strings.Split(meta, "|")
	switch {
	case len(parts) >= 3:
		parsed.AgentID = strings.TrimSpace(parts[0])
		parsed.AgentGRPCEndpoint = strings.TrimSpace(parts[1])
		parsed.Host = strings.TrimSpace(parts[2])
	case len(parts) >= 1:
		parsed.AgentID = strings.TrimSpace(parts[0])
	}

	// Backward compatibility for older remote metadata:
	// remote(agent_id|grpc_endpoint|username|host|port):endpoint
	if len(parts) >= 4 {
		parsed.Host = strings.TrimSpace(parts[3])
	}

	return parsed
}

func ExtractEndpoint(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	parsed := Parse(value)
	if parsed.Endpoint != "" {
		return parsed.Endpoint
	}

	if status, endpoint, ok := strings.Cut(value, ":"); ok && IsKnownStatus(status) {
		return strings.TrimSpace(endpoint)
	}

	if strings.Contains(value, "://") {
		return value
	}
	if pkgutils.EndpointHost(value) != "" {
		return value
	}
	return ""
}

func IsKnownStatus(raw string) bool {
	switch normalizeToken(raw) {
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

func normalizeToken(raw string) string {
	replacer := strings.NewReplacer(" ", "_", "-", "_")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(raw)))
}
