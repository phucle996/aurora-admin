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
	ServiceID         string
	Role              string
	ClusterID         string
	Hostname          string
	AgentIP           string
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
	ServiceID         string
	Role              string
	Hostname          string
	IPAddress         string
	BootstrapToken    string
	CSRPEM            string
	ServerCSRPEM      string
	AgentProbeAddr    string
	AgentGRPCEndpoint string
	Platform          string
}

type AgentBootstrapResult struct {
	ClientCertPEM    string
	ServerCertPEM    string
	AdminServerCAPEM string
	ClientSerialHex  string
	ServerSerialHex  string
	ClientExpiresAt  time.Time
	ServerExpiresAt  time.Time
}

type AgentIdentityClaims struct {
	NodeID    string
	ServiceID string
	Role      string
	ClusterID string
}

type AgentRenewRequest struct {
	CSRPEM       string
	ServerCSRPEM string
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
	adminCACertPath string
	agentCACertPath string
	agentCAKeyPath  string
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
	adminCACertPath string,
	agentCACertPath string,
	agentCAKeyPath string,
) *RuntimeBootstrapService {
	return &RuntimeBootstrapService{
		runtimeRepo:     runtimeRepo,
		endpointRepo:    endpointRepo,
		certStoreRepo:   certStoreRepo,
		certStorePrefix: strings.TrimSpace(certStorePrefix),
		adminCACertPath: strings.TrimSpace(adminCACertPath),
		agentCACertPath: strings.TrimSpace(agentCACertPath),
		agentCAKeyPath:  strings.TrimSpace(agentCAKeyPath),
		agentCertTTL:    30 * 24 * time.Hour,
	}
}
