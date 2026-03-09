package moduleinstall

import (
	sshpkg "admin/pkg/ssh"
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
				cmd := buildHostsUpdateCommand(entry.Address, entry.Host, target.Password)
				runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				runResult, err := sshpkg.Run(runCtx, sshpkg.RunInput{
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
					detail := ""
					if runResult != nil {
						detail = strings.TrimSpace(runResult.Output)
					}
					if detail != "" {
						detail = strings.Join(strings.Fields(detail), " ")
						warnings = append(warnings, fmt.Sprintf("remote hosts update failed (%s/%s): %v (%s)", target.Host, entry.Host, err, detail))
						continue
					}
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
	script := buildHostsUpdateScript(address, host, nil)
	cmd := exec.Command("bash", "-lc", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func buildHostsUpdateCommand(address, host string, sudoPassword *string) string {
	return "bash -lc " + strconv.Quote(buildHostsUpdateScript(address, host, sudoPassword))
}

func buildHostsUpdateScript(address, host string, sudoPassword *string) string {
	sudoPasswordB64 := ""
	if sudoPassword != nil {
		sudoPasswordB64 = base64.StdEncoding.EncodeToString([]byte(*sudoPassword))
	}
	hostPattern := fmt.Sprintf("(^|[[:space:]])%s([[:space:]]|$)", regexpEscape(host))
	updateCmd := fmt.Sprintf(
		`if grep -Eq %s /etc/hosts; then sed -i -E '/%s/d' /etc/hosts; fi; printf '%%s %%s\n' %s %s >> /etc/hosts`,
		shellEscape(hostPattern),
		hostPattern,
		shellEscape(address),
		shellEscape(host),
	)
	return fmt.Sprintf(
		`set -e; sudo_pw_b64=%s; sudo_pw=""; if [ -n "$sudo_pw_b64" ]; then sudo_pw="$(printf '%%s' "$sudo_pw_b64" | base64 -d 2>/dev/null || true)"; fi; update_cmd=%s; if [ "$(id -u)" -eq 0 ]; then sh -lc "$update_cmd"; exit $?; fi; if command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then sudo -n sh -lc "$update_cmd"; exit $?; fi; if command -v sudo >/dev/null 2>&1 && [ -n "$sudo_pw" ]; then printf '%%s\n' "$sudo_pw" | sudo -S -k sh -lc "$update_cmd"; exit $?; fi; echo "need sudo privilege to write /etc/hosts" >&2; exit 1`,
		shellEscape(sudoPasswordB64),
		shellEscape(updateCmd),
	)
}
