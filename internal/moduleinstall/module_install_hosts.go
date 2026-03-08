package moduleinstall

import (
	"admin/internal/repository"
	sshpkg "admin/pkg/ssh"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *ModuleInstallService) resolveHostSyncTargets(current moduleInstallTarget, items []repository.EndpointKV, listErr error) ([]moduleInstallTarget, error) {
	if listErr != nil {
		return nil, listErr
	}

	out := make([]moduleInstallTarget, 0, len(items)+2)
	out = append(out, current)
	out = append(out, moduleInstallTarget{
		Scope:    ModuleInstallScopeLocal,
		Username: "aurora",
		Host:     "127.0.0.1",
		Port:     22,
	})

	for _, item := range items {
		target, ok := parseEndpointTarget(item.Value)
		if !ok {
			continue
		}
		out = append(out, target)
	}
	return dedupeTargets(out), nil
}

func (s *ModuleInstallService) buildHostsEntries(
	currentHost string,
	currentAddress string,
	items []repository.EndpointKV,
	listErr error,
) ([]hostsEntry, error) {
	entries := map[string]string{}
	adminAddress := detectLocalIPv4()
	normalizedCurrentHost := strings.TrimSpace(currentHost)
	normalizedCurrentAddress := normalizeAddress(currentAddress)
	if normalizedCurrentHost != "" && !isValidHost(normalizedCurrentHost) {
		normalizedCurrentHost = ""
	}
	if normalizedCurrentHost != "" && normalizedCurrentAddress != "" {
		entries[normalizedCurrentHost] = normalizedCurrentAddress
	}

	if listErr != nil {
		return mapToHostsEntries(entries), listErr
	}

	for _, item := range items {
		name := strings.Trim(strings.TrimSpace(item.Name), "/")
		if strings.EqualFold(name, "admin") {
			adminHost := endpointHostFromAnyValue(item.Value)
			if adminHost != "" && isValidHost(adminHost) && adminAddress != "" {
				entries[adminHost] = adminAddress
			}
		}

		target, endpoint, ok := parseEndpointTargetAndEndpoint(item.Value)
		if !ok {
			continue
		}
		host := endpointHost(endpoint)
		address := normalizeAddress(target.Host)
		if host == "" || !isValidHost(host) || address == "" {
			continue
		}
		old, exists := entries[host]
		if exists && isLoopbackAddress(old) && !isLoopbackAddress(address) {
			entries[host] = address
			continue
		}
		if !exists {
			entries[host] = address
		}
	}

	return mapToHostsEntries(entries), nil
}

func (s *ModuleInstallService) resolveAdminHostsEntry(items []repository.EndpointKV, listErr error) (hostsEntry, bool) {
	if s == nil || s.endpointRepo == nil {
		return hostsEntry{}, false
	}
	adminAddress := normalizeAddress(detectLocalIPv4())
	if adminAddress == "" {
		return hostsEntry{}, false
	}
	if listErr != nil {
		return hostsEntry{}, false
	}
	for _, item := range items {
		name := strings.Trim(strings.TrimSpace(item.Name), "/")
		if !strings.EqualFold(name, "admin") {
			continue
		}
		adminHost := endpointHostFromAnyValue(item.Value)
		if adminHost == "" || !isValidHost(adminHost) {
			return hostsEntry{}, false
		}
		return hostsEntry{
			Address: adminAddress,
			Host:    adminHost,
		}, true
	}
	return hostsEntry{}, false
}

func dedupeTargets(items []moduleInstallTarget) []moduleInstallTarget {
	out := make([]moduleInstallTarget, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := strings.Join([]string{
			item.Scope,
			item.Username,
			item.Host,
			strconv.Itoa(int(item.Port)),
			deref(item.Password),
			deref(item.PrivateKey),
			deref(item.HostKeyFingerprint),
		}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Host < out[j].Host
	})
	return out
}

func syncHostsForTargets(
	ctx context.Context,
	entries []hostsEntry,
	targets []moduleInstallTarget,
) ([]string, []string) {
	hostsUpdated := make([]string, 0, len(targets))
	warnings := make([]string, 0)
	if len(entries) == 0 {
		return hostsUpdated, append(warnings, "skip /etc/hosts sync: empty hosts entries")
	}

	for _, target := range targets {
		switch target.Scope {
		case ModuleInstallScopeLocal:
			hasErr := false
			for _, entry := range entries {
				if err := upsertLocalHosts(entry.Address, entry.Host); err != nil {
					hasErr = true
					warnings = append(warnings, fmt.Sprintf("local hosts update failed (%s): %v", entry.Host, err))
					continue
				}
			}
			if hasErr {
				continue
			}
			hostsUpdated = append(hostsUpdated, "local")
		case ModuleInstallScopeRemote:
			hasErr := false
			if target.HostKeyFingerprint == nil {
				warnings = append(warnings, fmt.Sprintf("remote hosts update skipped (%s): missing ssh host key fingerprint", target.Host))
				continue
			}
			for _, entry := range entries {
				cmd := buildHostsUpdateCommand(entry.Address, entry.Host)
				runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				_, err := sshpkg.Run(runCtx, sshpkg.RunInput{
					Host:               target.Host,
					Port:               target.Port,
					Username:           target.Username,
					Password:           target.Password,
					PrivateKey:         target.PrivateKey,
					HostKeyFingerprint: target.HostKeyFingerprint,
					Timeout:            15 * time.Second,
					Command:            cmd,
				})
				cancel()
				if err != nil {
					hasErr = true
					warnings = append(warnings, fmt.Sprintf("remote hosts update failed (%s/%s): %v", target.Host, entry.Host, err))
				}
			}
			if hasErr {
				continue
			}
			hostsUpdated = append(hostsUpdated, target.Host)
		}
	}

	return hostsUpdated, warnings
}

func upsertLocalHosts(address, host string) error {
	script := buildHostsUpdateScript(address, host)
	cmd := exec.Command("bash", "-lc", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func buildHostsUpdateCommand(address, host string) string {
	return "bash -lc " + strconv.Quote(buildHostsUpdateScript(address, host))
}

func buildHostsUpdateScript(address, host string) string {
	return fmt.Sprintf(
		`tmp="$(mktemp)"; grep -v -E "(^|[[:space:]])%s([[:space:]]|$)" /etc/hosts > "$tmp" || true; printf "%%s %%s\n" %s %s >> "$tmp"; if [ "$(id -u)" -eq 0 ]; then cat "$tmp" > /etc/hosts; else sudo sh -c "cat \"$tmp\" > /etc/hosts"; fi; rm -f "$tmp"`,
		regexpEscape(host),
		shellEscape(address),
		shellEscape(host),
	)
}

func endpointHostFromAnyValue(raw string) string {
	if _, endpoint, ok := parseEndpointTargetAndEndpoint(raw); ok {
		return endpointHost(endpoint)
	}

	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	_, endpoint, ok := strings.Cut(value, ":")
	if !ok {
		return ""
	}
	return endpointHost(endpoint)
}
