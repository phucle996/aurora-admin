package moduleinstall

import (
	"admin/internal/endpointmeta"
	pkgutils "admin/pkg/utils"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	randomPortMin         = 20000
	randomPortMax         = 60000
	installCommandTimeout = 40 * time.Minute
)

func runInstallCommand(
	ctx context.Context,
	command string,
	target moduleInstallTarget,
	onStdout func(line string),
	onStderr func(line string),
) (string, int, error) {
	return runCommandOnTarget(ctx, target, command, installCommandTimeout, onStdout, onStderr)
}

func runCommandOnTarget(
	ctx context.Context,
	target moduleInstallTarget,
	command string,
	timeout time.Duration,
	onStdout func(line string),
	onStderr func(line string),
) (string, int, error) {
	if target.AgentGRPCEndpoint != "" {
		return runCommandOnAgent(ctx, target, command, timeout, onStdout, onStderr)
	}
	return "", -1, fmt.Errorf("agent endpoint is required")
}

func normalizeAgentGRPCEndpoint(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil {
			if strings.TrimSpace(parsed.Host) != "" {
				return strings.TrimSpace(parsed.Host)
			}
		}
	}
	return value
}

func encodeEndpointValue(target moduleInstallTarget, endpoint string) string {
	return fmt.Sprintf(
		"%s(%s|%s|%s):%s",
		ModuleInstallScopeRemote,
		target.AgentID,
		target.AgentGRPCEndpoint,
		target.Host,
		strings.TrimSpace(endpoint),
	)
}

func parseEndpointTargetAndEndpoint(raw string) (moduleInstallTarget, string, bool) {
	parsed := endpointmeta.Parse(raw)
	if !parsed.HasMetadata || strings.EqualFold(strings.TrimSpace(parsed.Scope), ModuleInstallScopeRemote) == false {
		return moduleInstallTarget{}, "", false
	}
	target := moduleInstallTarget{
		AgentID:           strings.TrimSpace(parsed.AgentID),
		AgentGRPCEndpoint: normalizeAgentGRPCEndpoint(parsed.AgentGRPCEndpoint),
		Host:              normalizeAddress(parsed.Host),
	}
	if target.Host == "" {
		target.Host = hostFromEndpoint(target.AgentGRPCEndpoint)
	}
	if target.Host == "" {
		target.Host = "agent-target"
	}
	if target.AgentGRPCEndpoint == "" {
		return moduleInstallTarget{}, "", false
	}
	return target, strings.TrimSpace(parsed.Endpoint), true
}

func resolveEndpointFromStoredValue(raw string) string {
	return strings.TrimSpace(endpointmeta.ExtractEndpoint(raw))
}

func normalizeAddress(raw string) string {
	return pkgutils.NormalizeAddress(raw)
}

func normalizeModuleName(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.Trim(value, "/")
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func resolveInstallEndpoint(appHost string) (string, int32, error) {
	host := normalizeAddress(appHost)
	if host == "" {
		return "", 0, fmt.Errorf("app_host is invalid")
	}
	if !pkgutils.IsValidHost(host) {
		return "", 0, fmt.Errorf("app_host is invalid")
	}

	port, err := randomInstallPort()
	if err != nil {
		return "", 0, fmt.Errorf("generate random app_port failed: %w", err)
	}

	return host, port, nil
}

func randomInstallPort() (int32, error) {
	span := big.NewInt(randomPortMax - randomPortMin + 1)
	n, err := rand.Int(rand.Reader, span)
	if err != nil {
		return 0, err
	}
	return int32(randomPortMin) + int32(n.Int64()), nil
}

func resolveInstallPortForTarget(
	ctx context.Context,
	target moduleInstallTarget,
	candidate int32,
) (int32, error) {
	if candidate <= 0 || candidate > 65535 {
		return 0, fmt.Errorf("invalid install app port")
	}

	available, checkErr := isTargetTCPPortAvailable(ctx, target, candidate)
	if checkErr != nil {
		return 0, checkErr
	}
	if available {
		return candidate, nil
	}

	const maxAttempts = 32
	for i := 0; i < maxAttempts; i++ {
		next, err := randomInstallPort()
		if err != nil {
			return 0, err
		}
		ok, err := isTargetTCPPortAvailable(ctx, target, next)
		if err != nil {
			return 0, err
		}
		if ok {
			return next, nil
		}
	}
	return 0, fmt.Errorf("cannot allocate available app port on target")
}

func isTargetTCPPortAvailable(ctx context.Context, target moduleInstallTarget, port int32) (bool, error) {
	if port <= 0 || port > 65535 {
		return false, fmt.Errorf("invalid port")
	}
	script := strings.Join([]string{
		"set -e",
		"port=" + shellEscape(strconv.Itoa(int(port))),
		`is_used=0`,
		`if command -v ss >/dev/null 2>&1; then`,
		`  if ss -ltnH "( sport = :$port )" 2>/dev/null | grep -q .; then is_used=1; fi`,
		`elif command -v netstat >/dev/null 2>&1; then`,
		`  if netstat -ltn 2>/dev/null | awk '{print $4}' | grep -E "[:.]$port$" >/dev/null; then is_used=1; fi`,
		`else`,
		`  # No socket tooling; assume available and let install fail later if conflict exists.`,
		`  is_used=0`,
		`fi`,
		`if [ "$is_used" -eq 1 ]; then exit 10; fi`,
		`exit 0`,
	}, "\n")

	output, exitCode, err := runCommandOnTarget(ctx, target, script, 15*time.Second, nil, nil)
	if err == nil && exitCode == 0 {
		return true, nil
	}
	if exitCode == 10 {
		return false, nil
	}
	detail := strings.TrimSpace(output)
	if detail == "" {
		return false, fmt.Errorf("check target port availability failed: %w", err)
	}
	return false, fmt.Errorf("check target port availability failed: %s", strings.Join(strings.Fields(detail), " "))
}

func buildDefaultUIInstallCommand(
	scriptURL string,
	appHost string,
	appPort int32,
	uiEnvPath string,
) string {
	args := []string{}
	preRunSteps := []string{}
	envAssignments := []string{}

	if strings.TrimSpace(appHost) == "" || appPort <= 0 {
		return ""
	}
	if strings.TrimSpace(uiEnvPath) != "" {
		args = append(args, "--env-file", strings.TrimSpace(uiEnvPath))
	}
	args = append(args, "--app-host", strings.TrimSpace(appHost))
	args = append(args, "--app-port", strconv.Itoa(int(appPort)))
	scriptURL = strings.TrimSpace(scriptURL)
	if scriptURL == "" {
		return ""
	}

	escapedArgs := make([]string, 0, len(args))
	for _, arg := range args {
		escapedArgs = append(escapedArgs, shellEscape(arg))
	}
	envPrefix := ""
	if len(envAssignments) > 0 {
		envPrefix = strings.Join(envAssignments, " ") + " "
	}
	joinedArgs := strings.Join(escapedArgs, " ")
	runScriptCmd := envPrefix + "\"$tmp_script\" " + joinedArgs
	sudoRunScriptCmd := "\"$tmp_script\" " + joinedArgs
	if len(envAssignments) > 0 {
		sudoRunScriptCmd = "env " + strings.Join(envAssignments, " ") + " " + sudoRunScriptCmd
	}
	installScript := strings.Join([]string{
		"set -e",
		"script_url=" + shellEscape(scriptURL),
		"tmp_script=\"$(mktemp)\"",
		"cleanup(){ rm -f \"$tmp_script\"; }",
		"trap cleanup EXIT",
		"if command -v curl >/dev/null 2>&1; then",
		"  curl -fsSL \"$script_url\" -o \"$tmp_script\"",
		"elif command -v wget >/dev/null 2>&1; then",
		"  wget -qO \"$tmp_script\" \"$script_url\"",
		"else",
		"  if command -v apt-get >/dev/null 2>&1; then apt-get update -y && apt-get install -y curl; fi",
		"  if command -v dnf >/dev/null 2>&1; then dnf install -y curl; fi",
		"  if command -v yum >/dev/null 2>&1; then yum install -y curl; fi",
		"  if command -v apk >/dev/null 2>&1; then apk add --no-cache curl; fi",
		"  if ! command -v curl >/dev/null 2>&1; then echo 'cannot install curl/wget to fetch install script' && exit 1; fi",
		"  curl -fsSL \"$script_url\" -o \"$tmp_script\"",
		"fi",
		strings.Join(preRunSteps, "\n"),
		"chmod +x \"$tmp_script\"",
		"if [ \"$(id -u)\" -eq 0 ]; then",
		"  " + runScriptCmd,
		"elif command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then",
		"  sudo -n " + sudoRunScriptCmd,
		"else",
		"  " + runScriptCmd,
		"fi",
	}, "\n")

	return installScript
}

func endpointHost(endpoint string) string {
	return pkgutils.EndpointHost(endpoint)
}

func endpointPort(endpoint string) string {
	return pkgutils.EndpointPort(endpoint)
}

func buildHealthcheckCandidates(endpoint string) []string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil
	}

	seen := map[string]struct{}{}
	add := func(urlValue string, out *[]string) {
		cleaned := strings.TrimSpace(urlValue)
		if cleaned == "" {
			return
		}
		if _, ok := seen[cleaned]; ok {
			return
		}
		seen[cleaned] = struct{}{}
		*out = append(*out, cleaned)
	}

	candidates := make([]string, 0, 12)
	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err == nil && strings.TrimSpace(parsed.Host) != "" {
			parsed.Scheme = "https"
			httpsBase := strings.TrimRight(parsed.String(), "/")
			add(httpsBase+"/health/liveness", &candidates)
			add(httpsBase+"/health/readiness", &candidates)
			add(httpsBase+"/health", &candidates)
			add(httpsBase, &candidates)
			return candidates
		}
	}

	httpsBase := "https://" + endpoint
	add(httpsBase+"/health/liveness", &candidates)
	add(httpsBase+"/health/readiness", &candidates)
	add(httpsBase+"/health", &candidates)
	add(httpsBase, &candidates)

	port := endpointPort(endpoint)
	if port != "" {
		localHTTPS := "https://127.0.0.1:" + port
		add(localHTTPS+"/health/liveness", &candidates)
		add(localHTTPS+"/health/readiness", &candidates)
		add(localHTTPS+"/health", &candidates)
		add(localHTTPS, &candidates)
	}

	return candidates
}

func ensureCurlAndCheckEndpoint(
	ctx context.Context,
	target moduleInstallTarget,
	moduleName string,
	endpoint string,
	onLine func(line string),
) (string, error) {
	candidates := buildHealthcheckCandidates(endpoint)
	if len(candidates) == 0 {
		return "", fmt.Errorf("no healthcheck candidate for endpoint %s", endpoint)
	}

	urlList := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		urlList = append(urlList, shellEscape(candidate))
	}

	tlsPaths := resolveModuleTLSPaths(moduleName)
	resolveHost := strings.TrimSpace(endpointHost(endpoint))
	resolvePort := strings.TrimSpace(endpointPort(endpoint))
	if resolvePort == "" {
		resolvePort = "443"
	}
	resolveAddr := "127.0.0.1"
	healthScript := strings.Join([]string{
		"set -e",
		"cert_path=" + shellEscape(tlsPaths.CertPath),
		"key_path=" + shellEscape(tlsPaths.KeyPath),
		"ca_path=" + shellEscape(tlsPaths.CAPath),
		"resolve_host=" + shellEscape(resolveHost),
		"resolve_port=" + shellEscape(resolvePort),
		"resolve_addr=" + shellEscape(resolveAddr),
		`resolve_opt=""`,
		`if [ -n "$resolve_host" ] && [ -n "$resolve_port" ] && [ -n "$resolve_addr" ]; then resolve_opt="--resolve ${resolve_host}:${resolve_port}:${resolve_addr}"; echo "healthcheck_resolve:${resolve_host}:${resolve_port}:${resolve_addr}"; fi`,
		"check_file(){",
		"  path=\"$1\"",
		"  if [ -f \"$path\" ]; then return 0; fi",
		"  if command -v sudo >/dev/null 2>&1 && sudo -n test -f \"$path\" >/dev/null 2>&1; then return 0; fi",
		"  return 1",
		"}",
		"check_file \"$cert_path\" || { echo \"missing tls cert: $cert_path\"; exit 1; }",
		"check_file \"$key_path\" || { echo \"missing tls key: $key_path\"; exit 1; }",
		"check_file \"$ca_path\" || { echo \"missing tls ca: $ca_path\"; exit 1; }",
		"ensure_curl(){",
		"  if command -v curl >/dev/null 2>&1; then return 0; fi",
		"  install_cmd=''",
		"  if command -v apt-get >/dev/null 2>&1; then install_cmd='apt-get update -y && apt-get install -y curl'; fi",
		"  if [ -z \"$install_cmd\" ] && command -v dnf >/dev/null 2>&1; then install_cmd='dnf install -y curl'; fi",
		"  if [ -z \"$install_cmd\" ] && command -v yum >/dev/null 2>&1; then install_cmd='yum install -y curl'; fi",
		"  if [ -z \"$install_cmd\" ] && command -v apk >/dev/null 2>&1; then install_cmd='apk add --no-cache curl'; fi",
		"  if [ -z \"$install_cmd\" ] && command -v zypper >/dev/null 2>&1; then install_cmd='zypper --non-interactive install curl'; fi",
		"  if [ -z \"$install_cmd\" ]; then echo 'cannot install curl automatically'; return 1; fi",
		"  if [ \"$(id -u)\" -eq 0 ]; then sh -lc \"$install_cmd\"; return $?; fi",
		"  if command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then sudo -n sh -lc \"$install_cmd\"; return $?; fi",
		"  echo 'need root/sudo to install curl'; return 1",
		"}",
		"ensure_curl",
		"for url in " + strings.Join(urlList, " ") + "; do",
		"  if curl --fail --silent --show-error --max-time 8 --cacert \"$ca_path\" --cert \"$cert_path\" --key \"$key_path\" $resolve_opt \"$url\" >/dev/null; then",
		"    echo \"healthcheck_ok:$url\"",
		"    exit 0",
		"  fi",
		"done",
		"echo 'healthcheck_failed'",
		"exit 1",
	}, "\n")

	output, exitCode, err := runCommandOnTarget(
		ctx,
		target,
		healthScript,
		90*time.Second,
		onLine,
		onLine,
	)
	if err != nil {
		return output, fmt.Errorf("exit_code=%d: %w", exitCode, err)
	}
	if exitCode != 0 {
		return output, fmt.Errorf("exit_code=%d", exitCode)
	}
	return output, nil
}

func logInstall(logFn InstallLogFn, stage string, format string, args ...any) {
	if logFn == nil {
		return
	}
	message := strings.TrimSpace(fmt.Sprintf(format, args...))
	if message == "" {
		return
	}
	logFn(strings.TrimSpace(stage), message)
}

func shellEscape(raw string) string {
	if raw == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(raw, "'", `'"'"'`) + "'"
}

func regexpEscape(raw string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`.`, `\.`,
		`+`, `\+`,
		`*`, `\*`,
		`?`, `\?`,
		`^`, `\^`,
		`$`, `\$`,
		`(`, `\(`,
		`)`, `\)`,
		`[`, `\[`,
		`]`, `\]`,
		`{`, `\{`,
		`}`, `\}`,
		`|`, `\|`,
	)
	return replacer.Replace(raw)
}
