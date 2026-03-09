package service

import (
	keycfg "admin/internal/key"
	"admin/internal/repository"
	pkgutils "admin/pkg/utils"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RuntimeBootstrapRequest struct {
	ModuleName string
	AppPort    int32
}

type AgentHeartbeatInput struct {
	AgentID           string
	Hostname          string
	AgentVersion      string
	AgentProbeAddr    string
	AgentGRPCEndpoint string
	Platform          string
	LibvirtURI        string
}

type AgentPeerInfo struct {
	RemoteAddress string
	CertSHA256    string
	CertSubject   string
	CertSerialHex string
}

type AgentBootstrapRequest struct {
	NodeID            string
	ClusterID         string
	Hostname          string
	IPAddress         string
	BootstrapToken    string
	CSRPEM            string
	AgentProbeAddr    string
	AgentGRPCEndpoint string
	Platform          string
}

type AgentBootstrapResult struct {
	ClientCertPEM string
	CACertPEM     string
	SerialHex     string
	ExpiresAt     time.Time
}

var (
	ErrAgentBootstrapTokenInvalid = errors.New("agent bootstrap token invalid")
	ErrAgentCSRInvalid            = errors.New("agent csr invalid")
)

type RuntimeBootstrapService struct {
	runtimeRepo     repository.RuntimeConfigRepository
	endpointRepo    repository.EndpointRepository
	certStoreRepo   repository.CertStoreRepository
	certStorePrefix string
	caCertPath      string
	caKeyPath       string
	agentCertTTL    time.Duration
}

type bootstrapValueSource string

const (
	bootstrapSourceRuntime bootstrapValueSource = "runtime"
)

type bootstrapValueSpec struct {
	Source    bootstrapValueSource
	StoreKey  string
	OutputKey string
	NonEmpty  bool
}

type endpointRuntimeDependency struct {
	TargetModule  string
	BaseURLOutput string
	GRPCOutput    string
}

func NewRuntimeBootstrapService(
	runtimeRepo repository.RuntimeConfigRepository,
	endpointRepo repository.EndpointRepository,
	certStoreRepo repository.CertStoreRepository,
	certStorePrefix string,
	caCertPath string,
	caKeyPath string,
) *RuntimeBootstrapService {
	return &RuntimeBootstrapService{
		runtimeRepo:     runtimeRepo,
		endpointRepo:    endpointRepo,
		certStoreRepo:   certStoreRepo,
		certStorePrefix: strings.TrimSpace(certStorePrefix),
		caCertPath:      strings.TrimSpace(caCertPath),
		caKeyPath:       strings.TrimSpace(caKeyPath),
		agentCertTTL:    365 * 24 * time.Hour,
	}
}

func (s *RuntimeBootstrapService) BuildRuntimeValues(
	ctx context.Context,
	req RuntimeBootstrapRequest,
) (map[string]string, error) {
	if s == nil || s.runtimeRepo == nil || s.endpointRepo == nil || s.certStoreRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	moduleName := normalizeBootstrapModuleName(req.ModuleName)
	if moduleName == "" {
		return nil, fmt.Errorf("module_name is required")
	}

	schemaStoreKey := keycfg.RuntimeSchemaKey(moduleName)
	runtimeSpecs := buildRuntimeBootstrapSpecs(moduleName, schemaStoreKey)

	runtimeLoaded, err := s.loadBySpecs(ctx, s.runtimeRepo, runtimeSpecs, bootstrapSourceRuntime)
	if err != nil {
		return nil, err
	}

	values := make(map[string]string, len(runtimeSpecs)+7)
	missing := make([]string, 0)
	empty := make([]string, 0)

	m1, e1 := applySpecs(runtimeSpecs, runtimeLoaded, values)
	missing = append(missing, m1...)
	empty = append(empty, e1...)

	certValues, certErr := s.loadModuleTLSBundle(ctx, moduleName)
	if certErr != nil {
		return nil, certErr
	}
	for outputKey, certValue := range certValues {
		trimmed := strings.TrimSpace(certValue)
		if trimmed == "" {
			empty = append(empty, "cert_store/"+outputKey)
			continue
		}
		values[outputKey] = trimmed
	}

	if err := buildBootstrapValidationError(missing, empty); err != nil {
		return nil, err
	}

	endpointItems, endpointListErr := s.endpointRepo.List(ctx)
	endpointMap := buildModuleEndpointMap(endpointItems)

	appPort, err := s.resolveModulePort(ctx, moduleName, req.AppPort, endpointMap, endpointListErr)
	if err != nil {
		return nil, err
	}

	values["app/port"] = strconv.Itoa(int(appPort))
	for _, dep := range runtimeEndpointDependencies(moduleName) {
		baseURL, resolveErr := s.resolveModuleBaseURL(dep.TargetModule, endpointMap, endpointListErr)
		if resolveErr != nil {
			return nil, resolveErr
		}
		values[dep.BaseURLOutput] = baseURL
		values[dep.GRPCOutput] = toGRPCEndpoint(baseURL)
	}
	values["cors/allow_origins"] = `["https://localhost:80","https://localhost:443"]`
	values["cors/allow_methods"] = `["GET","POST","PUT","PATCH","DELETE","HEAD","OPTIONS"]`
	values["cors/allow_headers"] = `["Origin","Content-Type","Accept","Authorization"]`
	values["cors/expose_headers"] = "[]"
	values["cors/allow_credentials"] = "true"
	values["cors/max_age"] = "12h"

	return values, nil
}

func buildRuntimeBootstrapSpecs(moduleName string, schemaStoreKey string) []bootstrapValueSpec {
	specs := []bootstrapValueSpec{
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTAppTZ, OutputKey: "app/timezone", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTAppLogLevel, OutputKey: "app/log_level", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTPgURL, OutputKey: "postgresql/url", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTPgSSLMode, OutputKey: "postgresql/sslmode", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: schemaStoreKey, OutputKey: "postgresql/schema", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisAddr, OutputKey: "redis/addr", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisUser, OutputKey: "redis/username", NonEmpty: false},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisPass, OutputKey: "redis/password", NonEmpty: false},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisDB, OutputKey: "redis/db", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisTLS, OutputKey: "redis/use_tls", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisCA, OutputKey: "redis/ca", NonEmpty: false},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisKey, OutputKey: "redis/client_key", NonEmpty: false},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisCert, OutputKey: "redis/client_cert", NonEmpty: false},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTRedisInsecure, OutputKey: "redis/insecure_skip_verify", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTTTLAccess, OutputKey: "token_ttl/access_ttl", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTTTLRefresh, OutputKey: "token_ttl/refresh_ttl", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTTTLDevice, OutputKey: "token_ttl/device_ttl", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTTTLOTT, OutputKey: "token_ttl/ott_ttl", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTSecretCachePrefix, OutputKey: "token_secret/cache_prefix", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTSecretCacheChannel, OutputKey: "token_secret/cache_channel", NonEmpty: true},
		{Source: bootstrapSourceRuntime, StoreKey: keycfg.RTSecretPollEvery, OutputKey: "token_secret/poll_interval", NonEmpty: true},
	}
	if isPlatformModuleName(moduleName) {
		specs = append(specs, bootstrapValueSpec{
			Source:    bootstrapSourceRuntime,
			StoreKey:  keycfg.RTPlatformKubeconfigCipherKey,
			OutputKey: "platform/kubeconfig_cipher_key",
			NonEmpty:  true,
		})
	}
	return specs
}

func (s *RuntimeBootstrapService) loadBySpecs(
	ctx context.Context,
	repo repository.RuntimeConfigRepository,
	specs []bootstrapValueSpec,
	source bootstrapValueSource,
) (map[string]string, error) {
	keys := collectSpecStoreKeys(specs)
	values, err := repo.GetMany(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("load %s keys failed: %w", source, err)
	}
	if values == nil {
		return map[string]string{}, nil
	}
	return values, nil
}

func collectSpecStoreKeys(specs []bootstrapValueSpec) []string {
	if len(specs) == 0 {
		return nil
	}
	out := make([]string, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		key := strings.TrimSpace(spec.StoreKey)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func applySpecs(
	specs []bootstrapValueSpec,
	loaded map[string]string,
	out map[string]string,
) (missing []string, empty []string) {
	for _, spec := range specs {
		value, exists := loaded[spec.StoreKey]
		if !exists {
			missing = append(missing, formatBootstrapStoreKey(spec.Source, spec.StoreKey))
			continue
		}
		trimmed := strings.TrimSpace(value)
		if spec.NonEmpty && trimmed == "" {
			empty = append(empty, formatBootstrapStoreKey(spec.Source, spec.StoreKey))
			continue
		}
		out[spec.OutputKey] = trimmed
	}
	return missing, empty
}

func formatBootstrapStoreKey(source bootstrapValueSource, storeKey string) string {
	switch source {
	case bootstrapSourceRuntime:
		return keycfg.RuntimeStoreKey(storeKey)
	default:
		return string(source) + "/" + strings.Trim(strings.TrimSpace(storeKey), "/")
	}
}

func buildBootstrapValidationError(missing []string, empty []string) error {
	if len(missing) == 0 && len(empty) == 0 {
		return nil
	}

	sort.Strings(missing)
	sort.Strings(empty)
	parts := make([]string, 0, 2)
	if len(missing) > 0 {
		parts = append(parts, "missing=["+strings.Join(missing, ", ")+"]")
	}
	if len(empty) > 0 {
		parts = append(parts, "empty=["+strings.Join(empty, ", ")+"]")
	}
	return fmt.Errorf("runtime bootstrap validation failed: %s", strings.Join(parts, "; "))
}

func normalizeBootstrapModuleName(raw string) string {
	name := strings.ToLower(strings.Trim(strings.TrimSpace(raw), "/"))
	switch name {
	case "platform", "platform-resource", "platform_resource", "plaform-resource", "plaform_resource":
		return "platform"
	case "paas", "paas-service", "paas_service":
		return "paas"
	case "dbaas", "dbaas-service", "dbaas_service":
		return "dbaas"
	case "dbaas-module", "dbaas_module":
		return "dbaas"
	default:
		return name
	}
}

func isPlatformModuleName(name string) bool {
	return normalizeBootstrapModuleName(name) == "platform"
}

func runtimeEndpointDependencies(moduleName string) []endpointRuntimeDependency {
	switch normalizeBootstrapModuleName(moduleName) {
	case "paas", "dbaas":
		return []endpointRuntimeDependency{
			{
				TargetModule:  "platform",
				BaseURLOutput: "platform/base_url",
				GRPCOutput:    "platform/grpc_endpoint",
			},
		}
	default:
		return nil
	}
}

func (s *RuntimeBootstrapService) resolveModulePort(
	ctx context.Context,
	moduleName string,
	requestedPort int32,
	endpointMap map[string]string,
	endpointListErr error,
) (int32, error) {
	if requestedPort < 0 || requestedPort > 65535 {
		return 0, fmt.Errorf("app_port is invalid")
	}

	canonicalName := normalizeBootstrapModuleName(moduleName)
	if requestedPort > 0 {
		return requestedPort, nil
	}

	if s != nil && s.runtimeRepo != nil {
		appPortRaw, found, getErr := s.runtimeRepo.Get(ctx, keycfg.RuntimeAppPortKey(canonicalName))
		if getErr != nil {
			return 0, fmt.Errorf("resolve runtime app port for %s failed: %w", canonicalName, getErr)
		}
		if found {
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(appPortRaw))
			if parseErr != nil || parsed <= 0 || parsed > 65535 {
				return 0, fmt.Errorf("runtime app port for %s is invalid", canonicalName)
			}
			return int32(parsed), nil
		}
	}

	if endpointListErr != nil {
		return 0, fmt.Errorf("resolve %s endpoint failed: %w", canonicalName, endpointListErr)
	}

	endpoint := strings.TrimSpace(endpointMap[canonicalName])
	if endpoint == "" {
		return 0, fmt.Errorf("%s endpoint not found", canonicalName)
	}
	port := strings.TrimSpace(pkgutils.EndpointPort(endpoint))
	if port == "" {
		return 0, fmt.Errorf("%s endpoint has no port", canonicalName)
	}
	parsed, parseErr := strconv.Atoi(port)
	if parseErr != nil || parsed <= 0 || parsed > 65535 {
		return 0, fmt.Errorf("%s endpoint port is invalid", canonicalName)
	}
	return int32(parsed), nil
}

func resolveEndpointFromStoredValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if _, endpoint, ok := parseEndpointValueWithScope(value); ok {
		return strings.TrimSpace(endpoint)
	}
	if status, endpoint, ok := strings.Cut(value, ":"); ok && isLegacyEndpointStatus(status) {
		return strings.TrimSpace(endpoint)
	}
	return value
}

func isLegacyEndpointStatus(raw string) bool {
	status := strings.ToLower(strings.TrimSpace(raw))
	switch status {
	case "running",
		"installed",
		"installing",
		"stopped",
		"degraded",
		"error",
		"healthy",
		"unhealthy",
		"maintenance",
		"not_installed",
		"unknown":
		return true
	default:
		return false
	}
}

func buildModuleEndpointMap(items []repository.EndpointKV) map[string]string {
	out := make(map[string]string, len(items))
	for _, item := range items {
		moduleName := normalizeBootstrapModuleName(item.Name)
		if moduleName == "" {
			continue
		}
		if _, exists := out[moduleName]; exists {
			continue
		}
		endpoint := strings.TrimSpace(resolveEndpointFromStoredValue(item.Value))
		if endpoint == "" {
			continue
		}
		out[moduleName] = endpoint
	}
	return out
}

func (s *RuntimeBootstrapService) resolveModuleBaseURL(
	moduleName string,
	endpointMap map[string]string,
	endpointListErr error,
) (string, error) {
	targetName := normalizeBootstrapModuleName(moduleName)
	if targetName == "" {
		return "", fmt.Errorf("target module is required")
	}
	if endpointListErr != nil {
		return "", fmt.Errorf("resolve %s endpoint failed: %w", targetName, endpointListErr)
	}
	endpoint := strings.TrimSpace(endpointMap[targetName])
	if endpoint == "" {
		return "", fmt.Errorf("%s endpoint not found", targetName)
	}
	if strings.HasPrefix(endpoint, "http://") {
		return "", fmt.Errorf("%s endpoint must use https for mTLS", targetName)
	}
	if strings.HasPrefix(endpoint, "https://") {
		return strings.TrimRight(endpoint, "/"), nil
	}
	return "https://" + strings.TrimRight(endpoint, "/"), nil
}

func toGRPCEndpoint(baseURL string) string {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil {
			host := strings.TrimSpace(parsed.Host)
			if host == "" {
				return ""
			}
			if _, _, splitErr := net.SplitHostPort(host); splitErr == nil {
				return host
			}
			return net.JoinHostPort(host, "443")
		}
	}
	host := strings.Trim(raw, "/")
	if host == "" {
		return ""
	}
	if _, _, splitErr := net.SplitHostPort(host); splitErr == nil {
		return host
	}
	return net.JoinHostPort(host, "443")
}

func runtimeBootstrapTLSObjectID(moduleName string) uuid.UUID {
	name := normalizeBootstrapModuleName(moduleName)
	if name == "" {
		name = strings.Trim(strings.TrimSpace(moduleName), "/")
	}
	if name == "" {
		name = "service"
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("module-install:"+name))
}

func runtimeBootstrapTLSStoreKey(prefix string, objectID uuid.UUID, certType string) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	key := objectID.String() + "-" + strings.TrimSpace(certType)
	if base == "" {
		return key
	}
	return base + "/" + key
}

func (s *RuntimeBootstrapService) loadModuleTLSBundle(ctx context.Context, moduleName string) (map[string]string, error) {
	if s == nil || s.certStoreRepo == nil {
		return nil, fmt.Errorf("cert store repository is nil")
	}

	objectID := runtimeBootstrapTLSObjectID(moduleName)
	caKey := runtimeBootstrapTLSStoreKey(s.certStorePrefix, objectID, "ca")
	clientCertKey := runtimeBootstrapTLSStoreKey(s.certStorePrefix, objectID, "client_cert")
	clientKeyKey := runtimeBootstrapTLSStoreKey(s.certStorePrefix, objectID, "private_client")

	loaded, err := s.certStoreRepo.GetMany(ctx, []string{caKey, clientCertKey, clientKeyKey})
	if err != nil {
		return nil, fmt.Errorf("load cert store keys failed: %w", err)
	}

	values := map[string]string{
		"tls/ca_pem":          strings.TrimSpace(loaded[caKey]),
		"tls/client_cert_pem": strings.TrimSpace(loaded[clientCertKey]),
		"tls/client_key_pem":  strings.TrimSpace(loaded[clientKeyKey]),
	}
	return values, nil
}

func (s *RuntimeBootstrapService) AuthorizeBootstrapClient(
	ctx context.Context,
	moduleName string,
	presentedClientCertDER []byte,
) error {
	if s == nil || s.certStoreRepo == nil {
		return fmt.Errorf("runtime bootstrap service is nil")
	}
	name := normalizeBootstrapModuleName(moduleName)
	if name == "" {
		return fmt.Errorf("module_name is required")
	}
	if len(presentedClientCertDER) == 0 {
		return fmt.Errorf("missing client certificate")
	}

	objectID := runtimeBootstrapTLSObjectID(name)
	clientCertKey := runtimeBootstrapTLSStoreKey(s.certStorePrefix, objectID, "client_cert")
	loaded, err := s.certStoreRepo.GetMany(ctx, []string{clientCertKey})
	if err != nil {
		return fmt.Errorf("load cert store keys failed: %w", err)
	}

	expectedPEM := strings.TrimSpace(loaded[clientCertKey])
	if expectedPEM == "" {
		return fmt.Errorf("client certificate not found for module %s", name)
	}

	expectedDER, err := decodeCertificatePEM(expectedPEM)
	if err != nil {
		return fmt.Errorf("invalid stored client certificate for module %s: %w", name, err)
	}
	if !bytes.Equal(expectedDER, presentedClientCertDER) {
		return fmt.Errorf("client certificate does not match module %s", name)
	}
	return nil
}

func decodeCertificatePEM(raw string) ([]byte, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("invalid cert pem")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil, fmt.Errorf("parse cert failed: %w", err)
	}
	return block.Bytes, nil
}

func (s *RuntimeBootstrapService) BootstrapAgent(
	ctx context.Context,
	req AgentBootstrapRequest,
	peerAddr string,
) (*AgentBootstrapResult, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	nodeID := normalizeAgentID(req.NodeID)
	if nodeID == "" {
		return nil, fmt.Errorf("node_id is required")
	}
	token := strings.TrimSpace(req.BootstrapToken)
	if token == "" {
		return nil, fmt.Errorf("bootstrap_token is required")
	}

	clusterPolicy, verifyErr := s.verifyAgentBootstrapToken(ctx, token)
	if verifyErr != nil {
		return nil, verifyErr
	}

	clusterID := strings.TrimSpace(req.ClusterID)
	if clusterPolicy != "" && clusterPolicy != "*" && clusterID != clusterPolicy {
		return nil, ErrAgentBootstrapTokenInvalid
	}

	csr, csrErr := parseAndVerifyCSR(req.CSRPEM)
	if csrErr != nil {
		return nil, csrErr
	}
	peerHost := normalizePeerRemoteHost(peerAddr)
	if strings.TrimSpace(req.IPAddress) == "" {
		if ip := net.ParseIP(peerHost); ip != nil {
			req.IPAddress = ip.String()
		}
	}
	if strings.TrimSpace(req.AgentGRPCEndpoint) == "" && peerHost != "" {
		if _, port, splitErr := net.SplitHostPort(strings.TrimSpace(req.AgentProbeAddr)); splitErr == nil && strings.TrimSpace(port) != "" {
			req.AgentGRPCEndpoint = net.JoinHostPort(peerHost, strings.TrimSpace(port))
		}
	}

	caCert, caKey, caCertPEM, caErr := s.loadAgentCA()
	if caErr != nil {
		return nil, caErr
	}

	issuedCertPEM, serialHex, expiresAt, issueErr := issueAgentClientCert(caCert, caKey, csr, req, nodeID, s.agentCertTTL)
	if issueErr != nil {
		return nil, issueErr
	}

	kv := map[string]string{
		keycfg.RuntimeAgentNodeKey(nodeID, "agent_id"):               nodeID,
		keycfg.RuntimeAgentNodeKey(nodeID, "cluster_id"):             clusterID,
		keycfg.RuntimeAgentNodeKey(nodeID, "status"):                 "bootstrapped",
		keycfg.RuntimeAgentNodeKey(nodeID, "hostname"):               strings.TrimSpace(req.Hostname),
		keycfg.RuntimeAgentNodeKey(nodeID, "ip"):                     strings.TrimSpace(req.IPAddress),
		keycfg.RuntimeAgentNodeKey(nodeID, "platform"):               strings.TrimSpace(req.Platform),
		keycfg.RuntimeAgentNodeKey(nodeID, "probe_addr"):             strings.TrimSpace(req.AgentProbeAddr),
		keycfg.RuntimeAgentNodeKey(nodeID, "grpc_endpoint"):          strings.TrimSpace(req.AgentGRPCEndpoint),
		keycfg.RuntimeAgentNodeKey(nodeID, "bootstrap/issued_at"):    time.Now().UTC().Format(time.RFC3339Nano),
		keycfg.RuntimeAgentNodeKey(nodeID, "bootstrap/peer_address"): strings.TrimSpace(peerAddr),
		keycfg.RuntimeAgentNodeKey(nodeID, "bootstrap/peer_host"):    peerHost,
		keycfg.RuntimeAgentNodeKey(nodeID, "cert/serial"):            serialHex,
		keycfg.RuntimeAgentNodeKey(nodeID, "cert/expires_at"):        expiresAt.UTC().Format(time.RFC3339Nano),
	}
	for key, value := range kv {
		if upsertErr := s.runtimeRepo.Upsert(ctx, key, strings.TrimSpace(value)); upsertErr != nil {
			return nil, fmt.Errorf("upsert bootstrap metadata failed (%s): %w", key, upsertErr)
		}
	}

	return &AgentBootstrapResult{
		ClientCertPEM: issuedCertPEM,
		CACertPEM:     caCertPEM,
		SerialHex:     serialHex,
		ExpiresAt:     expiresAt.UTC(),
	}, nil
}

func (s *RuntimeBootstrapService) verifyAgentBootstrapToken(ctx context.Context, token string) (string, error) {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	hash := hex.EncodeToString(sum[:])
	key := keycfg.RuntimeAgentBootstrapTokenKey(hash)
	value, found, err := s.runtimeRepo.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("verify bootstrap token failed: %w", err)
	}
	if !found {
		return "", ErrAgentBootstrapTokenInvalid
	}
	return strings.TrimSpace(value), nil
}

func parseAndVerifyCSR(rawCSR string) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(rawCSR)))
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, ErrAgentCSRInvalid
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, ErrAgentCSRInvalid
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, ErrAgentCSRInvalid
	}
	return csr, nil
}

func (s *RuntimeBootstrapService) loadAgentCA() (*x509.Certificate, any, string, error) {
	caCertPath := strings.TrimSpace(s.caCertPath)
	caKeyPath := strings.TrimSpace(s.caKeyPath)
	if caCertPath == "" || caKeyPath == "" {
		return nil, nil, "", fmt.Errorf("agent ca paths are not configured")
	}

	caCertPEMBytes, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("read ca cert failed: %w", err)
	}
	caKeyPEMBytes, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("read ca key failed: %w", err)
	}

	certBlock, _ := pem.Decode(caCertPEMBytes)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, nil, "", fmt.Errorf("invalid ca cert pem")
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, "", fmt.Errorf("parse ca cert failed: %w", err)
	}

	keyBlock, _ := pem.Decode(caKeyPEMBytes)
	if keyBlock == nil {
		return nil, nil, "", fmt.Errorf("invalid ca key pem")
	}

	var caKey any
	switch keyBlock.Type {
	case "RSA PRIVATE KEY":
		parsedKey, parseErr := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if parseErr != nil {
			return nil, nil, "", fmt.Errorf("parse rsa ca key failed: %w", parseErr)
		}
		caKey = parsedKey
	case "PRIVATE KEY":
		parsedKey, parseErr := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if parseErr != nil {
			return nil, nil, "", fmt.Errorf("parse pkcs8 ca key failed: %w", parseErr)
		}
		caKey = parsedKey
	default:
		return nil, nil, "", fmt.Errorf("unsupported ca key type: %s", keyBlock.Type)
	}

	return caCert, caKey, strings.TrimSpace(string(caCertPEMBytes)), nil
}

func issueAgentClientCert(
	caCert *x509.Certificate,
	caKey any,
	csr *x509.CertificateRequest,
	req AgentBootstrapRequest,
	nodeID string,
	ttl time.Duration,
) (string, string, time.Time, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("generate cert serial failed: %w", err)
	}
	now := time.Now().UTC()
	if ttl <= 0 {
		ttl = 365 * 24 * time.Hour
	}
	expiresAt := now.Add(ttl)

	dnsNames := uniqueStrings(append([]string{
		nodeID,
		strings.TrimSpace(req.Hostname),
	}, csr.DNSNames...))
	ipAddresses := append([]net.IP{}, csr.IPAddresses...)
	if ip := net.ParseIP(strings.TrimSpace(req.IPAddress)); ip != nil {
		ipAddresses = append(ipAddresses, ip)
	}

	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "agent:" + nodeID,
			Organization: []string{"Aurora"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              expiresAt,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddresses,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tpl, caCert, csr.PublicKey, caKey)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("issue agent cert failed: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if len(pemBytes) == 0 {
		return "", "", time.Time{}, fmt.Errorf("encode issued cert pem failed")
	}
	return strings.TrimSpace(string(pemBytes)), strings.ToUpper(serial.Text(16)), expiresAt, nil
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func (s *RuntimeBootstrapService) UpsertConnectedAgent(
	ctx context.Context,
	input AgentHeartbeatInput,
	peer AgentPeerInfo,
) error {
	if s == nil || s.runtimeRepo == nil {
		return fmt.Errorf("runtime bootstrap service is nil")
	}

	agentID := normalizeAgentID(input.AgentID)
	if agentID == "" {
		return fmt.Errorf("agent_id is required")
	}

	remoteHost := normalizePeerRemoteHost(peer.RemoteAddress)
	agentGRPCEndpoint := ""
	if remoteHost != "" {
		if _, port, splitErr := net.SplitHostPort(strings.TrimSpace(input.AgentProbeAddr)); splitErr == nil && strings.TrimSpace(port) != "" {
			agentGRPCEndpoint = net.JoinHostPort(remoteHost, strings.TrimSpace(port))
		} else if _, port, splitErr := net.SplitHostPort(strings.TrimSpace(input.AgentGRPCEndpoint)); splitErr == nil && strings.TrimSpace(port) != "" {
			agentGRPCEndpoint = net.JoinHostPort(remoteHost, strings.TrimSpace(port))
		}
	}
	if strings.TrimSpace(agentGRPCEndpoint) == "" {
		return fmt.Errorf("cannot resolve agent grpc endpoint from peer/probe address")
	}

	kv := map[string]string{
		keycfg.RuntimeAgentNodeKey(agentID, "agent_id"):         agentID,
		keycfg.RuntimeAgentNodeKey(agentID, "status"):           "connected",
		keycfg.RuntimeAgentNodeKey(agentID, "last_seen_at"):     time.Now().UTC().Format(time.RFC3339Nano),
		keycfg.RuntimeAgentNodeKey(agentID, "hostname"):         strings.TrimSpace(input.Hostname),
		keycfg.RuntimeAgentNodeKey(agentID, "agent_version"):    strings.TrimSpace(input.AgentVersion),
		keycfg.RuntimeAgentNodeKey(agentID, "probe_addr"):       strings.TrimSpace(input.AgentProbeAddr),
		keycfg.RuntimeAgentNodeKey(agentID, "grpc_endpoint"):    strings.TrimSpace(agentGRPCEndpoint),
		keycfg.RuntimeAgentNodeKey(agentID, "platform"):         strings.TrimSpace(input.Platform),
		keycfg.RuntimeAgentNodeKey(agentID, "libvirt_uri"):      strings.TrimSpace(input.LibvirtURI),
		keycfg.RuntimeAgentNodeKey(agentID, "peer/address"):     strings.TrimSpace(peer.RemoteAddress),
		keycfg.RuntimeAgentNodeKey(agentID, "peer/host"):        remoteHost,
		keycfg.RuntimeAgentNodeKey(agentID, "peer/cert_sha"):    strings.TrimSpace(peer.CertSHA256),
		keycfg.RuntimeAgentNodeKey(agentID, "peer/cert_name"):   strings.TrimSpace(peer.CertSubject),
		keycfg.RuntimeAgentNodeKey(agentID, "peer/cert_serial"): strings.TrimSpace(strings.ToUpper(peer.CertSerialHex)),
	}

	for key, value := range kv {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if upsertErr := s.runtimeRepo.Upsert(ctx, key, value); upsertErr != nil {
			return fmt.Errorf("upsert agent runtime key failed (%s): %w", key, upsertErr)
		}
	}
	return nil
}

func (s *RuntimeBootstrapService) IsAgentCertificateRevoked(
	ctx context.Context,
	certSerialHex string,
) (bool, error) {
	if s == nil || s.runtimeRepo == nil {
		return false, fmt.Errorf("runtime bootstrap service is nil")
	}
	serial := strings.TrimSpace(strings.ToUpper(certSerialHex))
	if serial == "" {
		return false, nil
	}
	_, found, err := s.runtimeRepo.Get(ctx, keycfg.RuntimeAgentRevocationKey(serial))
	if err != nil {
		return false, fmt.Errorf("check agent cert revocation failed: %w", err)
	}
	return found, nil
}

func normalizePeerRemoteHost(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(trimmed)
	if err != nil {
		return strings.Trim(trimmed, "[]")
	}
	return strings.Trim(strings.TrimSpace(host), "[]")
}

func normalizeAgentID(raw string) string {
	id := strings.Trim(strings.TrimSpace(raw), "/")
	if id == "" {
		return ""
	}
	return strings.ReplaceAll(id, " ", "_")
}
