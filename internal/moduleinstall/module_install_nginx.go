package moduleinstall

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

func ensureModuleNginxProxyOnTarget(
	ctx context.Context,
	target moduleInstallTarget,
	moduleName string,
	serverName string,
	backendPort int32,
	logFn InstallLogFn,
) error {
	cleanServerName := strings.TrimSpace(serverName)
	if cleanServerName == "" {
		return fmt.Errorf("server name is required for nginx proxy")
	}
	if backendPort <= 0 || backendPort > 65535 {
		return fmt.Errorf("backend port is invalid for nginx proxy")
	}

	paths := resolveModuleTLSPaths(moduleName)
	confPath := fmt.Sprintf("/etc/nginx/conf.d/aurora-%s.conf", strings.TrimSpace(moduleName))
	conf := renderModuleNginxConfig(cleanServerName, backendPort, paths)
	confB64 := base64.StdEncoding.EncodeToString([]byte(conf))

	logInstall(logFn, "nginx", "configure nginx reverse proxy module=%s host=%s backend_port=%d", moduleName, cleanServerName, backendPort)
	script := strings.Join([]string{
		"set -e",
		`ensure_root(){`,
		`  if [ "$(id -u)" -eq 0 ]; then "$@"; return; fi`,
		`  if command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then sudo -n "$@"; return; fi`,
		`  "$@"`,
		`}`,
		`if ! command -v nginx >/dev/null 2>&1; then`,
		`  if command -v apt-get >/dev/null 2>&1; then ensure_root apt-get install -y nginx;`,
		`  elif command -v dnf >/dev/null 2>&1; then ensure_root dnf install -y nginx;`,
		`  elif command -v yum >/dev/null 2>&1; then ensure_root yum install -y nginx;`,
		`  elif command -v apk >/dev/null 2>&1; then ensure_root apk add --no-cache nginx;`,
		`  else echo "nginx not found and cannot install automatically" >&2; exit 1; fi`,
		`fi`,
		`tmp_conf="$(mktemp)"`,
		`printf '%s' ` + shellEscape(confB64) + ` | base64 -d > "$tmp_conf"`,
		`ensure_root install -m 0644 "$tmp_conf" ` + shellEscape(confPath),
		`rm -f "$tmp_conf"`,
		`ensure_root nginx -t`,
		`if ensure_root systemctl is-active --quiet nginx; then ensure_root systemctl reload nginx || ensure_root systemctl restart nginx; else ensure_root systemctl enable nginx >/dev/null 2>&1 || true; ensure_root systemctl start nginx; fi`,
		`echo "nginx_proxy_ready:` + confPath + `"`,
	}, "\n")

	output, exitCode, err := runCommandOnTarget(ctx, target, script, 90*time.Second, func(line string) {
		logInstall(logFn, "nginx", "%s", line)
	}, func(line string) {
		logInstall(logFn, "nginx", "%s", line)
	})
	if err != nil {
		return fmt.Errorf("configure nginx proxy failed (exit_code=%d): %w", exitCode, err)
	}
	if strings.TrimSpace(output) == "" {
		logInstall(logFn, "nginx", "nginx proxy configured at %s", confPath)
	}
	return nil
}

func renderModuleNginxConfig(serverName string, backendPort int32, paths moduleTLSPaths) string {
	return fmt.Sprintf(`server {
    listen 80;
    listen [::]:80;
    server_name %s;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
    server_name %s;

    ssl_certificate %s;
    ssl_certificate_key %s;
    ssl_client_certificate %s;
    ssl_verify_client optional_no_ca;
    ssl_session_timeout 1d;
    ssl_session_cache shared:SSL:10m;

    location / {
        proxy_pass http://127.0.0.1:%d;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_buffering off;
        proxy_request_buffering off;
    }
}
`, serverName, serverName, paths.CertPath, paths.KeyPath, paths.CAPath, backendPort)
}
