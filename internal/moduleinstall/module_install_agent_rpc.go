package moduleinstall

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	gogrpc "google.golang.org/grpc"
	ggrpccreds "google.golang.org/grpc/credentials"
	ggrpcencoding "google.golang.org/grpc/encoding"
)

const agentRunCommandMethodPath = "/aurora.agent.v1.AgentService/RunCommand"

type agentRunCommandRequest struct {
	Command        string            `json:"command"`
	TimeoutSeconds int32             `json:"timeout_seconds,omitempty"`
	InstallRuntime string            `json:"install_runtime,omitempty"`
	Kubeconfig     string            `json:"kubeconfig,omitempty"`
	KubeconfigPath string            `json:"kubeconfig_path,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
}

type agentRunCommandResponse struct {
	OK        bool   `json:"ok"`
	ExitCode  int32  `json:"exit_code"`
	Output    string `json:"output"`
	ErrorText string `json:"error_text,omitempty"`
}

type moduleInstallAgentJSONCodec struct{}

func (moduleInstallAgentJSONCodec) Name() string { return "json" }

func (moduleInstallAgentJSONCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (moduleInstallAgentJSONCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

var registerModuleInstallAgentCodecOnce sync.Once

type agentRPCDialTLSConfig struct {
	caPath         string
	clientCertPath string
	clientKeyPath  string
}

var (
	agentRPCDialTLSMu  sync.RWMutex
	agentRPCDialTLSCfg agentRPCDialTLSConfig
)

func configureAgentRPCDialTLS(caPath, clientCertPath, clientKeyPath string) {
	agentRPCDialTLSMu.Lock()
	agentRPCDialTLSCfg = agentRPCDialTLSConfig{
		caPath:         strings.TrimSpace(caPath),
		clientCertPath: strings.TrimSpace(clientCertPath),
		clientKeyPath:  strings.TrimSpace(clientKeyPath),
	}
	agentRPCDialTLSMu.Unlock()
}

func loadAgentRPCDialTLSConfig() agentRPCDialTLSConfig {
	agentRPCDialTLSMu.RLock()
	defer agentRPCDialTLSMu.RUnlock()
	return agentRPCDialTLSCfg
}

func registerModuleInstallAgentCodec() {
	registerModuleInstallAgentCodecOnce.Do(func() {
		ggrpcencoding.RegisterCodec(moduleInstallAgentJSONCodec{})
	})
}

func runCommandOnAgent(
	ctx context.Context,
	target moduleInstallTarget,
	command string,
	timeout time.Duration,
	onStdout func(line string),
	onStderr func(line string),
) (string, int, error) {
	registerModuleInstallAgentCodec()

	endpoint := normalizeAgentGRPCEndpoint(target.AgentGRPCEndpoint)
	if endpoint == "" {
		return "", -1, fmt.Errorf("agent grpc endpoint is empty")
	}
	if timeout <= 0 {
		timeout = 40 * time.Minute
	}

	tlsCfg, tlsErr := buildAgentRPCDialTLSConfig(endpoint)
	if tlsErr != nil {
		return "", -1, tlsErr
	}

	conn, err := gogrpc.NewClient(
		endpoint,
		gogrpc.WithTransportCredentials(ggrpccreds.NewTLS(tlsCfg)),
		gogrpc.WithDefaultCallOptions(
			gogrpc.ForceCodec(moduleInstallAgentJSONCodec{}),
			gogrpc.CallContentSubtype("json"),
		),
	)
	if err != nil {
		return "", -1, fmt.Errorf("dial agent grpc failed (%s): %w", endpoint, err)
	}
	defer conn.Close()

	callTimeoutSeconds := int32(timeout / time.Second)
	if callTimeoutSeconds <= 0 {
		callTimeoutSeconds = 1
	}
	req := &agentRunCommandRequest{
		Command:        strings.TrimSpace(command),
		TimeoutSeconds: callTimeoutSeconds,
		InstallRuntime: normalizeInstallRuntime(target.InstallRuntime),
		Kubeconfig:     strings.TrimSpace(target.Kubeconfig),
		KubeconfigPath: strings.TrimSpace(target.KubeconfigPath),
	}
	res := &agentRunCommandResponse{}

	callCtx, cancelCall := context.WithTimeout(ctx, timeout+10*time.Second)
	defer cancelCall()
	if err := conn.Invoke(callCtx, agentRunCommandMethodPath, req, res); err != nil {
		return "", -1, fmt.Errorf("invoke agent run command failed: %w", err)
	}

	output := strings.TrimRight(strings.ReplaceAll(res.Output, "\r\n", "\n"), "\n")
	if output != "" {
		scanner := bufio.NewScanner(strings.NewReader(output))
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if onStdout != nil {
				onStdout(line)
			}
		}
		if scanErr := scanner.Err(); scanErr != nil && onStderr != nil {
			onStderr(fmt.Sprintf("scan agent output failed: %v", scanErr))
		}
	}

	exitCode := int(res.ExitCode)
	if res.OK && exitCode == 0 {
		return output, exitCode, nil
	}

	errText := strings.TrimSpace(res.ErrorText)
	if errText == "" {
		errText = fmt.Sprintf("agent command failed (exit_code=%d)", exitCode)
	}
	if onStderr != nil && errText != "" {
		onStderr(errText)
	}
	return output, exitCode, fmt.Errorf("%s", errText)
}

func buildAgentRPCDialTLSConfig(endpoint string) (*tls.Config, error) {
	cfg := loadAgentRPCDialTLSConfig()
	if cfg.caPath == "" || cfg.clientCertPath == "" || cfg.clientKeyPath == "" {
		return nil, fmt.Errorf("agent rpc tls config is incomplete")
	}

	caPEM, err := os.ReadFile(cfg.caPath)
	if err != nil {
		return nil, fmt.Errorf("read agent rpc ca failed: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("invalid agent rpc ca pem")
	}

	clientCert, err := tls.LoadX509KeyPair(cfg.clientCertPath, cfg.clientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load agent rpc client cert/key failed: %w", err)
	}

	serverName := ""
	if host, _, splitErr := net.SplitHostPort(strings.TrimSpace(endpoint)); splitErr == nil {
		serverName = strings.Trim(strings.TrimSpace(host), "[]")
	}

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      pool,
		Certificates: []tls.Certificate{clientCert},
	}
	if serverName != "" && net.ParseIP(serverName) == nil {
		tlsCfg.ServerName = serverName
	}
	return tlsCfg, nil
}
