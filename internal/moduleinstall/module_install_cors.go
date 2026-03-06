package moduleinstall

import (
	keycfg "admin/internal/key"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func (s *ModuleInstallService) upsertSharedCorsAllowOrigins(
	ctx context.Context,
	appHost string,
	endpoint string,
	logFn InstallLogFn,
) error {
	origin := buildHTTPSOrigin(appHost, endpoint)
	if origin == "" {
		logInstall(logFn, "cors", "[warn] skip update allow_origins: cannot resolve origin from app_host=%s endpoint=%s", appHost, endpoint)
		return nil
	}

	currentRaw, exists, err := s.sharedCorsRepo.Get(ctx, keycfg.SharedCORSAllowOrigins)
	if err != nil {
		return fmt.Errorf("load shared cors allow_origins failed: %w", err)
	}

	origins := parseStringList(currentRaw)
	if !exists {
		origins = []string{}
	}
	origins = appendUniqueNormalized(origins, origin)

	payload, marshalErr := json.Marshal(origins)
	if marshalErr != nil {
		return fmt.Errorf("marshal shared cors allow_origins failed: %w", marshalErr)
	}

	if err := s.sharedCorsRepo.Upsert(ctx, keycfg.SharedCORSAllowOrigins, string(payload)); err != nil {
		return fmt.Errorf("upsert shared cors allow_origins failed: %w", err)
	}
	logInstall(logFn, "cors", "allow_origins updated value=%s", string(payload))
	return nil
}

func buildHTTPSOrigin(appHost string, endpoint string) string {
	host := normalizeAddress(appHost)
	if host == "" {
		host = endpointHost(endpoint)
	}
	if host == "" {
		return ""
	}
	port := endpointPort(endpoint)
	if strings.TrimSpace(port) == "" {
		return "https://" + host
	}
	p, err := strconv.Atoi(strings.TrimSpace(port))
	if err != nil || p <= 0 || p > 65535 {
		return "https://" + host
	}
	if p == 443 {
		return "https://" + host
	}
	return "https://" + net.JoinHostPort(host, port)
}

func parseStringList(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(trimmed), &arr); err == nil {
			out := make([]string, 0, len(arr))
			for _, item := range arr {
				item = strings.TrimSpace(item)
				if item != "" {
					out = append(out, item)
				}
			}
			return out
		}
	}

	parts := strings.Split(trimmed, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.Trim(strings.TrimSpace(part), `"'[]`)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func appendUniqueNormalized(items []string, value string) []string {
	cleanValue := strings.TrimSpace(value)
	if cleanValue == "" {
		return items
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), cleanValue) {
			return items
		}
	}
	return append(items, cleanValue)
}
