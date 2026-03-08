package moduleinstall

import (
	"admin/internal/repository"
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

const uiRemoteEnvPath = "/tmp/aurora-ui.env"

func (s *ModuleInstallService) generateAndPushUIEnv(
	ctx context.Context,
	target moduleInstallTarget,
	endpointItems []repository.EndpointKV,
	endpointListErr error,
	logFn InstallLogFn,
) (string, error) {
	envValues, err := s.resolveUIEnvValuesFromEndpoints(endpointItems, endpointListErr)
	if err != nil {
		return "", err
	}

	content := buildDotEnvContent(envValues)
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("ui env content is empty")
	}

	payload := base64.StdEncoding.EncodeToString([]byte(content))
	script := strings.Join([]string{
		"set -e",
		"out_path=" + shellEscape(uiRemoteEnvPath),
		`tmp_path="$(mktemp)"`,
		"printf '%s' " + shellEscape(payload) + ` | base64 -d > "$tmp_path"`,
		`chmod 600 "$tmp_path"`,
		`mv "$tmp_path" "$out_path"`,
		`echo "ui_env_written:$out_path"`,
	}, "\n")

	logInstall(logFn, "env", "generate ui env from endpoint records")
	output, exitCode, runErr := runCommandOnTarget(ctx, target, script, 30*time.Second, func(line string) {
		logInstall(logFn, "env", "%s", line)
	}, func(line string) {
		logInstall(logFn, "env", "%s", line)
	})
	if runErr != nil {
		return "", fmt.Errorf("write ui env failed (exit_code=%d): %w", exitCode, runErr)
	}
	if strings.TrimSpace(output) != "" {
		logInstall(logFn, "env", "ui env prepared path=%s", uiRemoteEnvPath)
	}
	return uiRemoteEnvPath, nil
}

func (s *ModuleInstallService) resolveUIEnvValuesFromEndpoints(items []repository.EndpointKV, listErr error) (map[string]string, error) {
	if s == nil || s.endpointRepo == nil {
		return nil, fmt.Errorf("module install service is nil")
	}
	if listErr != nil {
		return nil, fmt.Errorf("load endpoint list failed: %w", listErr)
	}

	values := map[string]string{}
	for _, item := range items {
		moduleName := canonicalModuleName(item.Name)
		if moduleName == "" {
			continue
		}
		baseURL := endpointValueToHTTPSBaseURL(item.Value)
		if baseURL == "" {
			continue
		}

		switch moduleName {
		case "ums":
			values["VITE_UMS_API_URL"] = baseURL
			if strings.TrimSpace(values["VITE_API_URL"]) == "" {
				values["VITE_API_URL"] = baseURL
			}
		case "vm":
			values["VITE_VM_API_URL"] = baseURL
		case "paas":
			values["VITE_PAAS_API_URL"] = baseURL
		case "dbaas":
			values["VITE_DBAAS_API_URL"] = baseURL
		case "platform":
			values["VITE_PLATFORM_API_URL"] = baseURL
		case "mail":
			values["VITE_MAIL_API_URL"] = baseURL
		}
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("cannot build ui env: no endpoint record found in etcd")
	}
	return values, nil
}

func endpointValueToHTTPSBaseURL(raw string) string {
	endpoint := strings.TrimSpace(resolveEndpointFromStoredValue(raw))
	if endpoint == "" {
		return ""
	}
	if strings.HasPrefix(endpoint, "https://") || strings.HasPrefix(endpoint, "http://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return ""
		}
		if strings.TrimSpace(parsed.Host) == "" {
			return ""
		}
		parsed.Scheme = "https"
		parsed.Path = strings.TrimRight(parsed.Path, "/")
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return strings.TrimRight(parsed.String(), "/")
	}
	return "https://" + strings.TrimRight(endpoint, "/")
}

func buildDotEnvContent(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	builder := strings.Builder{}
	for _, key := range keys {
		value := strings.ReplaceAll(values[key], "\n", "")
		value = strings.ReplaceAll(value, "\r", "")
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(value)
		builder.WriteByte('\n')
	}
	return builder.String()
}
