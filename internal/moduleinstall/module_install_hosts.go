package moduleinstall

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
		case ModuleInstallScopeRemote:
			hasErr := false
			if target.AgentGRPCEndpoint == "" {
				warnings = append(warnings, fmt.Sprintf("remote hosts update skipped (%s): missing agent endpoint", target.Host))
				continue
			}
			for _, entry := range entries {
				cmd := buildHostsUpdateCommand(entry.Address, entry.Host, target.SudoPassword)
				runResultOutput, _, err := runCommandOnTarget(ctx, target, cmd, 20*time.Second, nil, nil)
				if err != nil {
					hasErr = true
					detail := strings.TrimSpace(runResultOutput)
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

func buildHostsUpdateCommand(address, host string, sudoPassword *string) string {
	return "bash -lc " + strconv.Quote(buildHostsUpdateScript(address, host, sudoPassword))
}

func buildHostsDeleteCommand(host string, sudoPassword *string) string {
	return "bash -lc " + strconv.Quote(buildHostsDeleteScript(host, sudoPassword))
}

func buildHostsUpdateScript(address, host string, sudoPassword *string) string {
	sudoPasswordValue := ""
	if sudoPassword != nil {
		sudoPasswordValue = *sudoPassword
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
		`set -e; sudo_pw=%s; update_cmd=%s; if [ "$(id -u)" -eq 0 ]; then sh -lc "$update_cmd"; exit $?; fi; if command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then sudo -n sh -lc "$update_cmd"; exit $?; fi; if command -v sudo >/dev/null 2>&1 && [ -n "$sudo_pw" ]; then printf '%%s\n' "$sudo_pw" | sudo -S -k sh -lc "$update_cmd"; exit $?; fi; echo "need sudo privilege to write /etc/hosts" >&2; exit 1`,
		shellEscape(sudoPasswordValue),
		shellEscape(updateCmd),
	)
}

func buildHostsDeleteScript(host string, sudoPassword *string) string {
	sudoPasswordValue := ""
	if sudoPassword != nil {
		sudoPasswordValue = *sudoPassword
	}
	hostPattern := fmt.Sprintf("(^|[[:space:]])%s([[:space:]]|$)", regexpEscape(host))
	deleteCmd := fmt.Sprintf(
		`if grep -Eq %s /etc/hosts; then sed -i -E '/%s/d' /etc/hosts; fi`,
		shellEscape(hostPattern),
		hostPattern,
	)
	return fmt.Sprintf(
		`set -e; sudo_pw=%s; delete_cmd=%s; if [ "$(id -u)" -eq 0 ]; then sh -lc "$delete_cmd"; exit $?; fi; if command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then sudo -n sh -lc "$delete_cmd"; exit $?; fi; if command -v sudo >/dev/null 2>&1 && [ -n "$sudo_pw" ]; then printf '%%s\n' "$sudo_pw" | sudo -S -k sh -lc "$delete_cmd"; exit $?; fi; echo "need sudo privilege to write /etc/hosts" >&2; exit 1`,
		shellEscape(sudoPasswordValue),
		shellEscape(deleteCmd),
	)
}
