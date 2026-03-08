package moduleinstall

import (
	"context"
	"encoding/base64"
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
	sudoPasswordB64 := ""
	if target.Password != nil {
		sudoPasswordB64 = base64.StdEncoding.EncodeToString([]byte(*target.Password))
	}

	script := strings.Join([]string{
		"set -e",
		"sudo_pw_b64=" + shellEscape(sudoPasswordB64),
		`sudo_pw=""`,
		`if [ -n "$sudo_pw_b64" ]; then sudo_pw="$(printf '%s' "$sudo_pw_b64" | base64 -d 2>/dev/null || true)"; fi`,
		`run_sudo_true(){`,
		`  if ! command -v sudo >/dev/null 2>&1; then return 1; fi`,
		`  if sudo -n true >/dev/null 2>&1; then return 0; fi`,
		`  if [ -n "$sudo_pw" ]; then printf '%s\n' "$sudo_pw" | sudo -S -k true >/dev/null 2>&1; return $?; fi`,
		`  return 1`,
		`}`,
		`if [ "$(id -u)" -eq 0 ]; then`,
		`  echo "privilege=root"`,
		`  exit 0`,
		"fi",
		`if ! command -v sudo >/dev/null 2>&1; then`,
		`  echo "privilege=no-sudo"`,
		`  exit 12`,
		"fi",
		`if run_sudo_true; then`,
		`  if sudo -n true >/dev/null 2>&1; then`,
		`    echo "privilege=sudo-nopasswd"`,
		`  else`,
		`    echo "privilege=sudo-password"`,
		`  fi`,
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
