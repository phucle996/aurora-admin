package moduleinstall

import (
	"admin/pkg/errorvar"
	sshpkg "admin/pkg/ssh"
	pkgutils "admin/pkg/utils"
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	randomPortMin            = 20000
	randomPortMax            = 60000
	installSSHCommandTimeout = 40 * time.Minute
)

func runInstallCommand(
	ctx context.Context,
	command string,
	target moduleInstallTarget,
	onStdout func(line string),
	onStderr func(line string),
) (string, int, error) {
	return runCommandOnTarget(ctx, target, command, installSSHCommandTimeout, onStdout, onStderr)
}

func runCommandOnTarget(
	ctx context.Context,
	target moduleInstallTarget,
	command string,
	timeout time.Duration,
	onStdout func(line string),
	onStderr func(line string),
) (string, int, error) {
	if target.Scope == ModuleInstallScopeLocal {
		if onStdout == nil && onStderr == nil {
			cmd := exec.CommandContext(ctx, "bash", "-lc", command)
			out, err := cmd.CombinedOutput()
			exitCode := 0
			if cmd.ProcessState != nil {
				exitCode = cmd.ProcessState.ExitCode()
			}
			return string(out), exitCode, err
		}

		cmd := exec.CommandContext(ctx, "bash", "-lc", command)
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return "", -1, err
		}
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return "", -1, err
		}

		var outputMu sync.Mutex
		var outputBuilder strings.Builder
		appendLine := func(line string) {
			outputMu.Lock()
			outputBuilder.WriteString(line)
			outputBuilder.WriteByte('\n')
			outputMu.Unlock()
		}

		var wg sync.WaitGroup
		var readErrMu sync.Mutex
		var readErr error
		setReadErr := func(err error) {
			if err == nil {
				return
			}
			readErrMu.Lock()
			if readErr == nil {
				readErr = err
			}
			readErrMu.Unlock()
		}

		consumePipe := func(pipe io.Reader, onLine func(line string)) {
			defer wg.Done()
			scanner := bufio.NewScanner(pipe)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				appendLine(line)
				if onLine != nil {
					onLine(line)
				}
			}
			setReadErr(scanner.Err())
		}

		wg.Add(2)
		go consumePipe(stdoutPipe, onStdout)
		go consumePipe(stderrPipe, onStderr)

		if err := cmd.Start(); err != nil {
			return "", -1, err
		}
		waitErr := cmd.Wait()
		wg.Wait()

		readErrMu.Lock()
		scannerErr := readErr
		readErrMu.Unlock()
		if scannerErr != nil && waitErr == nil {
			waitErr = scannerErr
		}

		exitCode := 0
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return outputBuilder.String(), exitCode, waitErr
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := sshpkg.Run(runCtx, sshpkg.RunInput{
		Host:               target.Host,
		Port:               target.Port,
		Username:           target.Username,
		Password:           target.Password,
		PrivateKey:         target.PrivateKey,
		HostKeyFingerprint: target.HostKeyFingerprint,
		Timeout:            timeout,
		Command:            command,
		OnStdout:           onStdout,
		OnStderr:           onStderr,
	})
	if result == nil {
		return "", -1, err
	}
	return result.Output, result.ExitCode, err
}

func buildInstallTarget(scope string, req ModuleInstallRequest) (moduleInstallTarget, error) {
	target := moduleInstallTarget{
		Scope: scope,
		Port:  22,
	}

	switch scope {
	case ModuleInstallScopeRemote:
		target.Username = strings.TrimSpace(req.SSHUsername)
		target.Host = normalizeAddress(req.SSHHost)
		if !isValidHost(target.Host) {
			return target, fmt.Errorf("ssh_host is invalid")
		}
		target.Port = normalizePort(req.SSHPort)
		target.Password = normalizeOptionalSecret(req.SSHPassword)
		target.PrivateKey = normalizeOptionalSecret(req.SSHPrivateKey)
		target.HostKeyFingerprint = normalizeOptionalSecret(req.SSHHostKeyFingerprint)
		if target.Username == "" || target.Host == "" {
			return target, fmt.Errorf("ssh_username and ssh_host are required for remote install")
		}
		if target.HostKeyFingerprint == nil {
			return target, fmt.Errorf("ssh_host_key_fingerprint is required for remote install")
		}
		return target, nil
	default:
		return target, errorvar.ErrModuleInstallScope
	}
}

func encodeEndpointValue(target moduleInstallTarget, endpoint string) string {
	secret := ""
	if target.PrivateKey != nil {
		secret = "keyb64:" + base64.RawStdEncoding.EncodeToString([]byte(*target.PrivateKey))
	} else if target.Password != nil {
		secret = "passb64:" + base64.RawStdEncoding.EncodeToString([]byte(*target.Password))
	}
	fingerprint := strings.TrimSpace(deref(target.HostKeyFingerprint))

	return fmt.Sprintf(
		"%s(%s|%s|%s|%d|%s):%s",
		target.Scope,
		target.Username,
		target.Host,
		secret,
		target.Port,
		fingerprint,
		strings.TrimSpace(endpoint),
	)
}

func parseEndpointTarget(raw string) (moduleInstallTarget, bool) {
	target, _, ok := parseEndpointTargetAndEndpoint(raw)
	return target, ok
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
	if scope != ModuleInstallScopeLocal && scope != ModuleInstallScopeRemote {
		return out, "", false
	}

	meta, endpoint, ok := strings.Cut(remainder, "):")
	if !ok {
		return out, "", false
	}
	parts := strings.Split(meta, "|")
	if len(parts) < 3 {
		return out, "", false
	}

	out.Scope = scope
	out.Username = strings.TrimSpace(parts[0])
	out.Host = normalizeAddress(parts[1])
	secret := strings.TrimSpace(parts[2])
	out.Port = 22
	if len(parts) >= 4 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(parts[3])); err == nil && parsed > 0 && parsed <= 65535 {
			out.Port = int32(parsed)
		}
	}
	if len(parts) >= 5 {
		fp := strings.TrimSpace(parts[4])
		if fp != "" {
			out.HostKeyFingerprint = &fp
		}
	}

	if strings.HasPrefix(secret, "keyb64:") {
		encoded := strings.TrimPrefix(secret, "keyb64:")
		if decoded, ok := decodeBase64Secret(encoded); ok {
			val := string(decoded)
			out.PrivateKey = &val
		}
	} else if strings.HasPrefix(secret, "passb64:") {
		encoded := strings.TrimPrefix(secret, "passb64:")
		if decoded, ok := decodeBase64Secret(encoded); ok {
			val := string(decoded)
			out.Password = &val
		}
	} else if strings.HasPrefix(secret, "key:") {
		// Backward compatibility for old records.
		encoded := strings.TrimPrefix(secret, "key:")
		if decoded, ok := decodeBase64Secret(encoded); ok {
			val := string(decoded)
			out.PrivateKey = &val
		}
	} else if secret != "" {
		// Backward compatibility for old plain password records.
		val := secret
		out.Password = &val
	}

	if out.Host == "" {
		return moduleInstallTarget{}, "", false
	}
	return out, strings.TrimSpace(endpoint), true
}

func resolveEndpointFromStoredValue(raw string) string {
	if _, endpoint, ok := parseEndpointTargetAndEndpoint(raw); ok {
		return strings.TrimSpace(endpoint)
	}
	return strings.TrimSpace(parseLegacyEndpointValue(raw))
}

func decodeBase64Secret(encoded string) ([]byte, bool) {
	value := strings.TrimSpace(encoded)
	if value == "" {
		return nil, false
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil {
		return decoded, true
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, true
	}
	return nil, false
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

func isValidHost(host string) bool {
	return pkgutils.IsValidHost(host)
}

func normalizeScope(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
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
	if !isValidHost(host) {
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

func randomAvailableLocalPort() (int32, error) {
	const maxAttempts = 64
	for i := 0; i < maxAttempts; i++ {
		candidate, err := randomInstallPort()
		if err != nil {
			return 0, err
		}
		if isLocalTCPPortAvailable(candidate) {
			return candidate, nil
		}
	}
	return pkgutils.RandomAvailableLocalPort()
}

func isLocalTCPPortAvailable(port int32) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
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

	switch canonicalModuleName(moduleName) {
	case "ums":
		args = append(args, "-r", "phucle996/aurora-ums")
		if strings.TrimSpace(appHost) != "" {
			args = append(args, "--app-host", strings.TrimSpace(appHost))
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
	runScriptCmd := "\"$tmp_script\" " + strings.Join(escapedArgs, " ")
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
		"  " + strings.ReplaceAll(runScriptCmd, `"$tmp_script"`, `sudo -n "$tmp_script"`),
		"elif command -v sudo >/dev/null 2>&1 && [ -n \"$sudo_pw\" ]; then",
		"  printf '%s\\n' \"$sudo_pw\" | sudo -S -k -p '' \"$tmp_script\" " + strings.Join(escapedArgs, " "),
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

func mapToHostsEntries(raw map[string]string) []hostsEntry {
	out := make([]hostsEntry, 0, len(raw))
	for host, address := range raw {
		cleanHost := strings.TrimSpace(host)
		cleanAddress := normalizeAddress(address)
		if cleanHost == "" || cleanAddress == "" {
			continue
		}
		out = append(out, hostsEntry{
			Address: cleanAddress,
			Host:    cleanHost,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Host < out[j].Host
	})
	return out
}

func isLoopbackAddress(raw string) bool {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return false
	}
	if clean == "localhost" {
		return true
	}
	ip := net.ParseIP(clean)
	return ip != nil && ip.IsLoopback()
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
	if target.Password != nil {
		sudoPasswordB64 = base64.StdEncoding.EncodeToString([]byte(*target.Password))
	}
	healthScript := strings.Join([]string{
		"set -e",
		"sudo_pw_b64=" + shellEscape(sudoPasswordB64),
		`sudo_pw=""`,
		`if [ -n "$sudo_pw_b64" ]; then sudo_pw="$(printf '%s' "$sudo_pw_b64" | base64 -d 2>/dev/null || true)"; fi`,
		"cert_path=" + shellEscape(tlsPaths.CertPath),
		"key_path=" + shellEscape(tlsPaths.KeyPath),
		"ca_path=" + shellEscape(tlsPaths.CAPath),
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
		"  if curl --fail --silent --show-error --max-time 8 --cacert \"$ca_path\" --cert \"$cert_path\" --key \"$key_path\" \"$url\" >/dev/null; then",
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
