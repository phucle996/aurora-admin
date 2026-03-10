package moduleinstall

import (
	"admin/pkg/errorvar"
	pkgutils "admin/pkg/utils"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
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

func buildInstallTarget(scope string, req ModuleInstallRequest) (moduleInstallTarget, error) {
	target := moduleInstallTarget{
		Scope:          scope,
		InstallRuntime: normalizeInstallRuntime(req.InstallRuntime),
		Port:           22,
	}

	switch scope {
	case ModuleInstallScopeRemote:
		target.AgentID = normalizeInstallAgentID(req.AgentID)
		target.AgentGRPCEndpoint = normalizeAgentGRPCEndpoint(req.AgentGRPCEndpoint)
		target.Kubeconfig = strings.TrimSpace(req.Kubeconfig)
		target.KubeconfigPath = strings.TrimSpace(req.KubeconfigPath)
		target.Username = strings.TrimSpace(req.TargetUser)
		target.Host = normalizeAddress(req.TargetHost)
		target.SudoPassword = normalizeOptionalSecret(req.SudoPassword)
		if target.AgentGRPCEndpoint == "" {
			return target, fmt.Errorf("agent endpoint is required for remote install")
		}
		if target.Host == "" {
			target.Host = hostFromEndpoint(target.AgentGRPCEndpoint)
		}
		if target.Host == "" {
			target.Host = "agent-target"
		}
		target.Port = normalizePort(req.TargetPort)
		if target.Username == "" {
			target.Username = "aurora"
		}
		if target.Port <= 0 || target.Port > 65535 {
			target.Port = 22
		}
		if target.InstallRuntime == ModuleInstallRuntimeK8s {
			target.Port = 0
			target.Username = ""
		}
		return target, nil
	default:
		return target, errorvar.ErrModuleInstallScope
	}
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
		"%s(%s|%s|%s|%s|%d):%s",
		target.Scope,
		target.AgentID,
		target.AgentGRPCEndpoint,
		target.Username,
		target.Host,
		target.Port,
		strings.TrimSpace(endpoint),
	)
}

func parseEndpointTargetAndEndpoint(raw string) (moduleInstallTarget, string, bool) {
	value := strings.TrimSpace(raw)
	var out moduleInstallTarget
	if value == "" {
		return out, "", false
	}

	scope, remainder, ok := strings.Cut(value, "(")
	if !ok {
		return out, "", false
	}
	scope = normalizeScope(scope)
	if scope != ModuleInstallScopeRemote {
		return out, "", false
	}

	meta, endpoint, ok := strings.Cut(remainder, "):")
	if !ok {
		return out, "", false
	}
	parts := strings.Split(meta, "|")
	if len(parts) >= 4 {
		out.Scope = scope
		out.AgentID = strings.TrimSpace(parts[0])
		out.AgentGRPCEndpoint = normalizeAgentGRPCEndpoint(parts[1])
		out.Username = strings.TrimSpace(parts[2])
		out.Host = normalizeAddress(parts[3])
		out.Port = 22
		if len(parts) >= 5 {
			if parsed, err := strconv.Atoi(strings.TrimSpace(parts[4])); err == nil && parsed > 0 && parsed <= 65535 {
				out.Port = int32(parsed)
			}
		}
		if out.Host == "" {
			out.Host = hostFromEndpoint(out.AgentGRPCEndpoint)
		}
		if out.Host == "" {
			out.Host = "agent-target"
		}
		if out.Username == "" {
			out.Username = "aurora"
		}
		if out.AgentGRPCEndpoint == "" {
			return moduleInstallTarget{}, "", false
		}
		return out, strings.TrimSpace(endpoint), true
	}

	if len(parts) < 3 {
		return out, "", false
	}

	// Old endpoint format does not contain agent endpoint, skip as unsupported.
	return moduleInstallTarget{}, "", false
}

func resolveEndpointFromStoredValue(raw string) string {
	if _, endpoint, ok := parseEndpointTargetAndEndpoint(raw); ok {
		return strings.TrimSpace(endpoint)
	}
	return strings.TrimSpace(parseLegacyEndpointValue(raw))
}

func normalizeOptionalSecret(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizePort(port int32) int32 {
	if port <= 0 || port > 65535 {
		return 22
	}
	return port
}

func normalizeAddress(raw string) string {
	return pkgutils.NormalizeAddress(raw)
}

func normalizeScope(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeInstallRuntime(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", ModuleInstallRuntimeLinux:
		return ModuleInstallRuntimeLinux
	case ModuleInstallRuntimeK8s:
		return ModuleInstallRuntimeK8s
	default:
		return ModuleInstallRuntimeLinux
	}
}

func normalizeModuleName(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.Trim(value, "/")
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func resolveInstallEndpoint(scope string, appHost string, appPort int32, fallbackEndpoint string) (string, int32, error) {
	if normalizeScope(scope) != ModuleInstallScopeRemote {
		return "", 0, errorvar.ErrModuleInstallScope
	}
	host := normalizeAddress(appHost)
	if host == "" {
		return "", 0, fmt.Errorf("app_host is invalid")
	}
	if !pkgutils.IsValidHost(host) {
		return "", 0, fmt.Errorf("app_host is invalid")
	}

	port := int32(0)
	switch {
	case appPort > 0:
		if appPort > 65535 {
			return "", 0, fmt.Errorf("app_port is invalid")
		}
		port = appPort
	case appPort < 0:
		return "", 0, fmt.Errorf("app_port is invalid")
	default:
		if parsed := parsePortFromEndpoint(fallbackEndpoint); parsed > 0 {
			port = parsed
		}
	}

	if port == 0 {
		randomPort, err := randomInstallPort()
		if err != nil {
			return "", 0, fmt.Errorf("generate random app_port failed: %w", err)
		}
		port = randomPort
	}

	return host, port, nil
}

func parsePortFromEndpoint(raw string) int32 {
	portRaw := strings.TrimSpace(endpointPort(raw))
	if portRaw == "" {
		return 0
	}
	portNum, err := strconv.Atoi(portRaw)
	if err != nil || portNum <= 0 || portNum > 65535 {
		return 0
	}
	return int32(portNum)
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
	explicit bool,
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
	if explicit {
		return 0, fmt.Errorf("requested app_port %d is already in use on target", candidate)
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

func buildDefaultModuleInstallCommand(
	moduleName string,
	scriptURL string,
	appHost string,
	appPort int32,
	adminRPCEndpoint string,
	uiEnvPath string,
	sudoPassword *string,
) string {
	args := []string{}
	preRunSteps := []string{}
	envAssignments := []string{}

	switch canonicalModuleName(moduleName) {
	case "ums":
		args = append(args, "-r", "phucle996/aurora-ums")
		if strings.TrimSpace(appHost) != "" {
			args = append(args, "--app-host", strings.TrimSpace(appHost))
		}
		if appPort > 0 {
			envAssignments = append(envAssignments, "AURORA_UMS_BACKEND_PORT="+shellEscape(strconv.Itoa(int(appPort))))
		}
		if strings.TrimSpace(adminRPCEndpoint) != "" {
			args = append(args, "--admin-rpc-endpoint", strings.TrimSpace(adminRPCEndpoint))
		}
		preRunSteps = append(preRunSteps, `sed -i '/trap .* RETURN/d' "$tmp_script" || true`)
	case "platform":
		if strings.TrimSpace(adminRPCEndpoint) == "" {
			return ""
		}
		args = append(args, "--admin-rpc-endpoint", strings.TrimSpace(adminRPCEndpoint))
	case "paas":
		if strings.TrimSpace(adminRPCEndpoint) == "" {
			return ""
		}
		args = append(args, "--admin-rpc-endpoint", strings.TrimSpace(adminRPCEndpoint))
	case "dbaas":
		if strings.TrimSpace(adminRPCEndpoint) == "" {
			return ""
		}
		args = append(args, "--admin-rpc-endpoint", strings.TrimSpace(adminRPCEndpoint))
	case "ui":
		if strings.TrimSpace(appHost) == "" || appPort <= 0 {
			return ""
		}
		if strings.TrimSpace(uiEnvPath) != "" {
			args = append(args, "--env-file", strings.TrimSpace(uiEnvPath))
		}
		args = append(args, "--app-host", strings.TrimSpace(appHost))
		args = append(args, "--app-port", strconv.Itoa(int(appPort)))
	default:
		return ""
	}
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
	sudoPasswordB64 := ""
	if sudoPassword != nil {
		sudoPasswordB64 = base64.StdEncoding.EncodeToString([]byte(*sudoPassword))
	}

	installScript := strings.Join([]string{
		"set -e",
		"script_url=" + shellEscape(scriptURL),
		"sudo_pw_b64=" + shellEscape(sudoPasswordB64),
		`sudo_pw=""`,
		`if [ -n "$sudo_pw_b64" ]; then sudo_pw="$(printf '%s' "$sudo_pw_b64" | base64 -d 2>/dev/null || true)"; fi`,
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
		"elif command -v sudo >/dev/null 2>&1 && [ -n \"$sudo_pw\" ]; then",
		"  printf '%s\\n' \"$sudo_pw\" | sudo -S -k -p '' " + sudoRunScriptCmd,
		"else",
		"  " + runScriptCmd,
		"fi",
	}, "\n")

	return installScript
}

func detectLocalIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if udpAddress, ok := conn.LocalAddr().(*net.UDPAddr); ok && udpAddress.IP != nil {
			if ipv4 := udpAddress.IP.To4(); ipv4 != nil {
				return ipv4.String()
			}
		}
	}
	return "127.0.0.1"
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
	sudoPasswordB64 := ""
	if target.SudoPassword != nil {
		sudoPasswordB64 = base64.StdEncoding.EncodeToString([]byte(*target.SudoPassword))
	}
	resolveHost := strings.TrimSpace(endpointHost(endpoint))
	resolvePort := strings.TrimSpace(endpointPort(endpoint))
	if resolvePort == "" {
		resolvePort = "443"
	}
	resolveAddr := "127.0.0.1"
	healthScript := strings.Join([]string{
		"set -e",
		"sudo_pw_b64=" + shellEscape(sudoPasswordB64),
		`sudo_pw=""`,
		`if [ -n "$sudo_pw_b64" ]; then sudo_pw="$(printf '%s' "$sudo_pw_b64" | base64 -d 2>/dev/null || true)"; fi`,
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
		"  if command -v sudo >/dev/null 2>&1 && [ -n \"$sudo_pw\" ]; then printf '%s\\n' \"$sudo_pw\" | sudo -S -k test -f \"$path\" >/dev/null 2>&1; return $?; fi",
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
		"  if command -v sudo >/dev/null 2>&1 && [ -n \"$sudo_pw\" ]; then printf '%s\\n' \"$sudo_pw\" | sudo -S -k sh -lc \"$install_cmd\"; return $?; fi",
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

func deref(v *string) string {
	return pkgutils.DerefString(v)
}
