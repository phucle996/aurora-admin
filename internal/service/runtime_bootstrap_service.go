package service

import (
	"admin/internal/repository"
	"errors"
	"strings"
	"time"
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

type AgentBootstrapTokenResult struct {
	Token         string
	TokenHash     string
	ClusterPolicy string
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

const (
	agentCertStoreTypeCA         = "ca"
	agentCertStoreTypeClientCert = "client_cert"
)

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
