package moduleinstall

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func ensureTargetInstallPrivilege(
	ctx context.Context,
	target moduleInstallTarget,
	logFn InstallLogFn,
) error {
	logInstall(logFn, "preflight", "checking install privilege on target")

	script := strings.Join([]string{
		"set -e",
		`if [ "$(id -u)" -eq 0 ]; then`,
		`  echo "privilege=root"`,
		`  exit 0`,
		"fi",
		`if ! command -v sudo >/dev/null 2>&1; then`,
		`  echo "privilege=no-sudo"`,
		`  exit 12`,
		"fi",
		`if sudo -n true >/dev/null 2>&1; then`,
		`  echo "privilege=sudo-nopasswd"`,
		`  exit 0`,
		"fi",
		`echo "privilege=sudo-denied"`,
		"exit 11",
	}, "\n")

	output, exitCode, err := runCommandOnTarget(
		ctx,
		target,
		script,
		20*time.Second,
		func(line string) { logInstall(logFn, "preflight", "%s", line) },
		func(line string) { logInstall(logFn, "preflight", "%s", line) },
	)
	if err == nil {
		return nil
	}

	switch exitCode {
	case 11:
		return fmt.Errorf("target user %s has sudo but cannot run non-interactive sudo; use root user or grant NOPASSWD sudo", target.Username)
	case 12:
		return fmt.Errorf("target user %s has no sudo; use root user or install sudo + NOPASSWD policy", target.Username)
	default:
		trimmed := strings.TrimSpace(output)
		if trimmed == "" {
			return fmt.Errorf("install privilege preflight failed: %w", err)
		}
		return fmt.Errorf("install privilege preflight failed: %s", trimmed)
	}
}
