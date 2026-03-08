package ssh

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	xssh "golang.org/x/crypto/ssh"
)

type RunInput struct {
	Host       string
	Port       int32
	Username   string
	Password   *string
	PrivateKey *string
	// SHA256 fingerprint, example: SHA256:abc...
	HostKeyFingerprint *string
	Timeout            time.Duration
	Command            string
	OnStdout           func(line string)
	OnStderr           func(line string)
}

type RunResult struct {
	Output    string
	ExitCode  int
	CheckedAt time.Time
}

func Run(ctx context.Context, input RunInput) (*RunResult, error) {
	host := strings.TrimSpace(input.Host)
	username := strings.TrimSpace(input.Username)
	command := strings.TrimSpace(input.Command)
	if host == "" || username == "" || command == "" {
		return nil, errors.New("host, username and command are required")
	}

	port := input.Port
	if port <= 0 || port > 65535 {
		port = 22
	}
	timeout := input.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	password := normalizePasswordPtr(input.Password)
	privateKey := trimStringPtr(input.PrivateKey)
	expectedFingerprint := normalizeFingerprintPtr(input.HostKeyFingerprint)
	if expectedFingerprint == nil {
		return nil, errors.New("ssh host key fingerprint is required")
	}

	authMethods := make([]xssh.AuthMethod, 0, 3)
	if privateKey != nil {
		signer, err := xssh.ParsePrivateKey([]byte(*privateKey))
		if err != nil && password == nil {
			return nil, fmt.Errorf("invalid private key: %w", err)
		}
		if err == nil {
			authMethods = append(authMethods, xssh.PublicKeys(signer))
		}
	}
	if len(authMethods) == 0 {
		authMethods = append(authMethods, loadDefaultKeyAuthMethods()...)
	}
	if password != nil {
		authMethods = append(authMethods, xssh.Password(*password))
		authMethods = append(authMethods, xssh.KeyboardInteractive(
			func(_ string, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = *password
				}
				return answers, nil
			},
		))
	}

	addr := net.JoinHostPort(host, strconv.Itoa(int(port)))
	clientConfig := &xssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyVerifyCallback(*expectedFingerprint),
		Timeout:         timeout,
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}

	sshConn, chans, reqs, err := xssh.NewClientConn(conn, addr, clientConfig)
	if err != nil {
		return nil, err
	}
	client := xssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	if input.OnStdout == nil && input.OnStderr == nil {
		out, runErr := session.CombinedOutput(command)
		result := &RunResult{
			Output:    string(out),
			ExitCode:  0,
			CheckedAt: time.Now().UTC(),
		}
		if runErr == nil {
			return result, nil
		}
		var exitErr *xssh.ExitError
		if errors.As(runErr, &exitErr) {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			result.ExitCode = -1
		}
		return result, runErr
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return nil, err
	}

	var outputMu sync.Mutex
	var outputBuilder strings.Builder
	appendOutput := func(line string) {
		outputMu.Lock()
		outputBuilder.WriteString(line)
		outputBuilder.WriteByte('\n')
		outputMu.Unlock()
	}

	var wg sync.WaitGroup
	var scanErrMu sync.Mutex
	var scanErr error
	setScanErr := func(err error) {
		if err == nil {
			return
		}
		scanErrMu.Lock()
		if scanErr == nil {
			scanErr = err
		}
		scanErrMu.Unlock()
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := scanPipeLines(stdoutPipe, input.OnStdout, appendOutput); err != nil {
			setScanErr(err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := scanPipeLines(stderrPipe, input.OnStderr, appendOutput); err != nil {
			setScanErr(err)
		}
	}()

	if err := session.Start(command); err != nil {
		return nil, err
	}
	runErr := session.Wait()
	wg.Wait()

	scanErrMu.Lock()
	readErr := scanErr
	scanErrMu.Unlock()
	if readErr != nil {
		return nil, readErr
	}

	result := &RunResult{
		Output:    outputBuilder.String(),
		ExitCode:  0,
		CheckedAt: time.Now().UTC(),
	}
	if runErr == nil {
		return result, nil
	}

	var exitErr *xssh.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitStatus()
	} else {
		result.ExitCode = -1
	}
	return result, runErr
}

func scanPipeLines(r io.Reader, onLine func(line string), appendOutput func(line string)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		appendOutput(line)
		if onLine != nil {
			onLine(line)
		}
	}
	return scanner.Err()
}

func loadDefaultKeyAuthMethods() []xssh.AuthMethod {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}

	candidates := []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
		filepath.Join(home, ".ssh", "id_dsa"),
	}

	methods := make([]xssh.AuthMethod, 0, len(candidates))
	for _, path := range candidates {
		key, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		signer, parseErr := xssh.ParsePrivateKey(key)
		if parseErr != nil {
			continue
		}
		methods = append(methods, xssh.PublicKeys(signer))
	}
	return methods
}

func trimStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizePasswordPtr(v *string) *string {
	if v == nil {
		return nil
	}
	if len(*v) == 0 {
		return nil
	}
	value := *v
	return &value
}

func normalizeFingerprintPtr(v *string) *string {
	if v == nil {
		return nil
	}
	value := strings.TrimSpace(*v)
	if value == "" {
		return nil
	}
	return &value
}

func hostKeyVerifyCallback(expected string) xssh.HostKeyCallback {
	normalizedExpected := strings.TrimSpace(expected)
	return func(hostname string, remote net.Addr, key xssh.PublicKey) error {
		gotSHA := strings.TrimSpace(xssh.FingerprintSHA256(key))
		if strings.EqualFold(gotSHA, normalizedExpected) {
			return nil
		}

		gotMD5 := strings.TrimSpace(xssh.FingerprintLegacyMD5(key))
		if strings.EqualFold(gotMD5, normalizedExpected) {
			return nil
		}

		return fmt.Errorf("ssh host key mismatch host=%s remote=%s expected=%s got_sha256=%s got_md5=%s", hostname, remote.String(), normalizedExpected, gotSHA, gotMD5)
	}
}
