package moduleinstall

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
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
const agentInstallModuleMethodPath = "/aurora.agent.v1.AgentService/InstallModule"
const agentInstallModuleStreamMethodPath = "/aurora.agent.v1.AgentService/InstallModuleStream"
const agentRestartModuleMethodPath = "/aurora.agent.v1.AgentService/RestartModule"
const agentUninstallModuleMethodPath = "/aurora.agent.v1.AgentService/UninstallModule"
const agentListInstalledModulesMethodPath = "/aurora.agent.v1.AgentService/ListInstalledModules"

type agentRunCommandRequest struct {
	Command        string `json:"command"`
	TimeoutSeconds int32  `json:"timeout_seconds,omitempty"`
}

type agentRunCommandResponse struct {
	OK        bool   `json:"ok"`
	ExitCode  int32  `json:"exit_code"`
	Output    string `json:"output"`
	ErrorText string `json:"error_text,omitempty"`
}

type agentInstallLogEntry struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

type agentInstallModuleRequest struct {
	APIVersion       string            `json:"api_version,omitempty"`
	RequestID        string            `json:"request_id"`
	Module           string            `json:"module"`
	Version          string            `json:"version"`
	ArtifactURL      string            `json:"artifact_url"`
	ArtifactChecksum string            `json:"artifact_checksum"`
	AppHost          string            `json:"app_host"`
	AppPort          int32             `json:"app_port"`
	Env              map[string]string `json:"env,omitempty"`
}

type agentInstallModuleResult struct {
	APIVersion            string                    `json:"api_version,omitempty"`
	Module                string                    `json:"module"`
	Version               string                    `json:"version"`
	Runtime               string                    `json:"runtime"`
	ServiceName           string                    `json:"service_name,omitempty"`
	Endpoint              string                    `json:"endpoint,omitempty"`
	Status                string                    `json:"status"`
	Health                string                    `json:"health,omitempty"`
	ManifestSchemaVersion string                    `json:"manifest_schema_version,omitempty"`
	Capabilities          agentArtifactCapabilities `json:"capabilities,omitempty"`
}

type agentInstallModuleResponse struct {
	APIVersion string                    `json:"api_version,omitempty"`
	OK         bool                      `json:"ok"`
	Result     *agentInstallModuleResult `json:"result,omitempty"`
	Logs       []agentInstallLogEntry    `json:"logs,omitempty"`
	ErrorText  string                    `json:"error_text,omitempty"`
}

type agentInstallModuleStreamEvent struct {
	APIVersion string                    `json:"api_version,omitempty"`
	Type       string                    `json:"type"`
	Stage      string                    `json:"stage,omitempty"`
	Message    string                    `json:"message,omitempty"`
	Result     *agentInstallModuleResult `json:"result,omitempty"`
	ErrorText  string                    `json:"error_text,omitempty"`
}

type agentRestartModuleRequest struct {
	APIVersion  string `json:"api_version,omitempty"`
	RequestID   string `json:"request_id"`
	Module      string `json:"module"`
	ServiceName string `json:"service_name"`
}

type agentRestartModuleResult struct {
	APIVersion  string `json:"api_version,omitempty"`
	Module      string `json:"module"`
	Runtime     string `json:"runtime"`
	ServiceName string `json:"service_name"`
	Status      string `json:"status"`
	Health      string `json:"health,omitempty"`
}

type agentRestartModuleResponse struct {
	APIVersion string                    `json:"api_version,omitempty"`
	OK         bool                      `json:"ok"`
	Result     *agentRestartModuleResult `json:"result,omitempty"`
	Logs       []agentInstallLogEntry    `json:"logs,omitempty"`
	ErrorText  string                    `json:"error_text,omitempty"`
}

type agentUninstallModuleRequest struct {
	APIVersion    string `json:"api_version,omitempty"`
	RequestID     string `json:"request_id"`
	Module        string `json:"module"`
	ServiceName   string `json:"service_name"`
	UnitPath      string `json:"unit_path,omitempty"`
	BinaryPath    string `json:"binary_path,omitempty"`
	EnvFilePath   string `json:"env_file_path,omitempty"`
	NginxSitePath string `json:"nginx_site_path,omitempty"`
}

func newAgentOperationRequestID(action string, moduleName string) string {
	return fmt.Sprintf("%s-%s-%d", strings.TrimSpace(action), canonicalModuleName(moduleName), time.Now().UTC().UnixNano())
}

type agentUninstallModuleResult struct {
	APIVersion  string `json:"api_version,omitempty"`
	Module      string `json:"module"`
	Runtime     string `json:"runtime"`
	ServiceName string `json:"service_name"`
	Status      string `json:"status"`
	Health      string `json:"health,omitempty"`
}

type agentUninstallModuleResponse struct {
	APIVersion string                      `json:"api_version,omitempty"`
	OK         bool                        `json:"ok"`
	Result     *agentUninstallModuleResult `json:"result,omitempty"`
	Logs       []agentInstallLogEntry      `json:"logs,omitempty"`
	ErrorText  string                      `json:"error_text,omitempty"`
}

type agentListInstalledModulesRequest struct {
	APIVersion string `json:"api_version,omitempty"`
}

type agentInstalledModuleRecord struct {
	APIVersion            string                    `json:"api_version,omitempty"`
	Module                string                    `json:"module"`
	Version               string                    `json:"version"`
	Runtime               string                    `json:"runtime"`
	ServiceName           string                    `json:"service_name,omitempty"`
	Endpoint              string                    `json:"endpoint,omitempty"`
	Status                string                    `json:"status"`
	Health                string                    `json:"health,omitempty"`
	ObservedAt            string                    `json:"observed_at"`
	ManifestSchemaVersion string                    `json:"manifest_schema_version,omitempty"`
	Capabilities          agentArtifactCapabilities `json:"capabilities,omitempty"`
}

type agentListInstalledModulesResponse struct {
	APIVersion string                       `json:"api_version,omitempty"`
	OK         bool                         `json:"ok"`
	Items      []agentInstalledModuleRecord `json:"items,omitempty"`
	ErrorText  string                       `json:"error_text,omitempty"`
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

	callTimeoutSeconds := int32(timeout / time.Second)
	if callTimeoutSeconds <= 0 {
		callTimeoutSeconds = 1
	}
	req := &agentRunCommandRequest{
		Command:        strings.TrimSpace(command),
		TimeoutSeconds: callTimeoutSeconds,
	}
	res := &agentRunCommandResponse{}

	callCtx, cancelCall := context.WithTimeout(ctx, timeout+10*time.Second)
	defer cancelCall()
	if err := invokeAgentRPC(callCtx, endpoint, agentRunCommandMethodPath, req, res); err != nil {
		return "", -1, classifyAgentRPCInvokeError(err)
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

func installModuleOnAgent(
	ctx context.Context,
	target moduleInstallTarget,
	req agentInstallModuleRequest,
) (*agentInstallModuleResponse, error) {
	endpoint := normalizeAgentGRPCEndpoint(target.AgentGRPCEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("agent grpc endpoint is empty")
	}
	res := &agentInstallModuleResponse{}
	if err := invokeAgentRPC(ctx, endpoint, agentInstallModuleMethodPath, &req, res); err != nil {
		return nil, classifyAgentRPCInvokeError(err)
	}
	if res.OK {
		res.APIVersion = normalizeAgentInstallerAPIVersion(res.APIVersion)
		if res.Result != nil {
			res.Result.APIVersion = normalizeAgentInstallerAPIVersion(res.Result.APIVersion)
			res.Result.ManifestSchemaVersion = normalizeAgentBundleSchemaVersion(res.Result.ManifestSchemaVersion)
			res.Result.Capabilities = normalizeAgentArtifactCapabilities(res.Result.Capabilities)
		}
		return res, nil
	}
	errText := strings.TrimSpace(res.ErrorText)
	if errText == "" {
		errText = "agent install module failed"
	}
	return res, fmt.Errorf("%s", errText)
}

func streamInstallModuleOnAgent(
	ctx context.Context,
	target moduleInstallTarget,
	req agentInstallModuleRequest,
	onEvent func(agentInstallModuleStreamEvent),
) (*agentInstallModuleResponse, error) {
	endpoint := normalizeAgentGRPCEndpoint(target.AgentGRPCEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("agent grpc endpoint is empty")
	}
	conn, err := dialAgentRPC(endpoint)
	if err != nil {
		return nil, classifyAgentRPCInvokeError(err)
	}
	defer conn.Close()

	stream, err := conn.NewStream(
		ctx,
		&gogrpc.StreamDesc{ServerStreams: true},
		agentInstallModuleStreamMethodPath,
	)
	if err != nil {
		return nil, classifyAgentRPCInvokeError(err)
	}
	if err := stream.SendMsg(&req); err != nil {
		return nil, classifyAgentRPCInvokeError(err)
	}
	if err := stream.CloseSend(); err != nil {
		return nil, classifyAgentRPCInvokeError(err)
	}

	res := &agentInstallModuleResponse{
		Logs: make([]agentInstallLogEntry, 0, 16),
	}
	for {
		var event agentInstallModuleStreamEvent
		if err := stream.RecvMsg(&event); err != nil {
			if err == io.EOF {
				break
			}
			return res, classifyAgentRPCInvokeError(err)
		}
		if onEvent != nil {
			onEvent(event)
		}
		stage := strings.TrimSpace(event.Stage)
		message := strings.TrimSpace(event.Message)
		if stage != "" || message != "" {
			res.Logs = append(res.Logs, agentInstallLogEntry{
				Stage:   stage,
				Message: message,
			})
		}
		switch strings.ToLower(strings.TrimSpace(event.Type)) {
		case "result":
			res.OK = true
			res.Result = event.Result
		case "error":
			res.OK = false
			res.ErrorText = strings.TrimSpace(event.ErrorText)
			if event.Result != nil {
				res.Result = event.Result
			}
		}
	}

	if res.OK {
		res.APIVersion = normalizeAgentInstallerAPIVersion(res.APIVersion)
		if res.Result != nil {
			res.Result.APIVersion = normalizeAgentInstallerAPIVersion(res.Result.APIVersion)
			res.Result.ManifestSchemaVersion = normalizeAgentBundleSchemaVersion(res.Result.ManifestSchemaVersion)
			res.Result.Capabilities = normalizeAgentArtifactCapabilities(res.Result.Capabilities)
		}
		return res, nil
	}
	errText := strings.TrimSpace(res.ErrorText)
	if errText == "" {
		errText = "agent install module stream failed"
	}
	return res, fmt.Errorf("%s", errText)
}

func restartModuleOnAgent(
	ctx context.Context,
	target moduleInstallTarget,
	req agentRestartModuleRequest,
) (*agentRestartModuleResponse, error) {
	endpoint := normalizeAgentGRPCEndpoint(target.AgentGRPCEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("agent grpc endpoint is empty")
	}
	res := &agentRestartModuleResponse{}
	if err := invokeAgentRPC(ctx, endpoint, agentRestartModuleMethodPath, &req, res); err != nil {
		return nil, classifyAgentRPCInvokeError(err)
	}
	if res.OK {
		res.APIVersion = normalizeAgentInstallerAPIVersion(res.APIVersion)
		if res.Result != nil {
			res.Result.APIVersion = normalizeAgentInstallerAPIVersion(res.Result.APIVersion)
		}
		return res, nil
	}
	errText := strings.TrimSpace(res.ErrorText)
	if errText == "" {
		errText = "agent restart module failed"
	}
	return res, fmt.Errorf("%s", errText)
}

func uninstallModuleOnAgent(
	ctx context.Context,
	target moduleInstallTarget,
	req agentUninstallModuleRequest,
) (*agentUninstallModuleResponse, error) {
	endpoint := normalizeAgentGRPCEndpoint(target.AgentGRPCEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("agent grpc endpoint is empty")
	}
	res := &agentUninstallModuleResponse{}
	if err := invokeAgentRPC(ctx, endpoint, agentUninstallModuleMethodPath, &req, res); err != nil {
		return nil, classifyAgentRPCInvokeError(err)
	}
	if res.OK {
		res.APIVersion = normalizeAgentInstallerAPIVersion(res.APIVersion)
		if res.Result != nil {
			res.Result.APIVersion = normalizeAgentInstallerAPIVersion(res.Result.APIVersion)
		}
		return res, nil
	}
	errText := strings.TrimSpace(res.ErrorText)
	if errText == "" {
		errText = "agent uninstall module failed"
	}
	return res, fmt.Errorf("%s", errText)
}

func listInstalledModulesOnAgent(
	ctx context.Context,
	target moduleInstallTarget,
) (*agentListInstalledModulesResponse, error) {
	endpoint := normalizeAgentGRPCEndpoint(target.AgentGRPCEndpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("agent grpc endpoint is empty")
	}
	res := &agentListInstalledModulesResponse{}
	if err := invokeAgentRPC(ctx, endpoint, agentListInstalledModulesMethodPath, &agentListInstalledModulesRequest{
		APIVersion: installerRPCVersionV1,
	}, res); err != nil {
		return nil, classifyAgentRPCInvokeError(err)
	}
	if res.OK {
		res.APIVersion = normalizeAgentInstallerAPIVersion(res.APIVersion)
		for i := range res.Items {
			res.Items[i].APIVersion = normalizeAgentInstallerAPIVersion(res.Items[i].APIVersion)
			res.Items[i].ManifestSchemaVersion = normalizeAgentBundleSchemaVersion(res.Items[i].ManifestSchemaVersion)
			res.Items[i].Capabilities = normalizeAgentArtifactCapabilities(res.Items[i].Capabilities)
		}
		return res, nil
	}
	errText := strings.TrimSpace(res.ErrorText)
	if errText == "" {
		errText = "agent list installed modules failed"
	}
	return res, fmt.Errorf("%s", errText)
}

func invokeAgentRPC(ctx context.Context, endpoint string, methodPath string, req any, res any) error {
	conn, err := dialAgentRPC(endpoint)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Invoke(ctx, methodPath, req, res); err != nil {
		return err
	}
	return nil
}

func dialAgentRPC(endpoint string) (*gogrpc.ClientConn, error) {
	tlsCfg, tlsErr := buildAgentRPCDialTLSConfig(endpoint)
	if tlsErr != nil {
		return nil, tlsErr
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
		return nil, fmt.Errorf("dial agent grpc failed (%s): %w", endpoint, err)
	}
	return conn, nil
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

func classifyAgentRPCInvokeError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "certificate signed by unknown authority") {
		return fmt.Errorf(
			"invoke agent rpc failed: mTLS verification failed between admin and agent; agent certificate is not trusted by configured agent mTLS CA (re-bootstrap/reinstall agent to rotate cert with current agent CA): %w",
			err,
		)
	}
	if strings.Contains(message, "tls: bad certificate") {
		return fmt.Errorf(
			"invoke agent rpc failed: mTLS verification failed between admin and agent (admin client certificate rejected by agent); ensure both sides use the same Aurora Agent mTLS CA: %w",
			err,
		)
	}
	return fmt.Errorf("invoke agent rpc failed: %w", err)
}
