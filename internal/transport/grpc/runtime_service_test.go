package grpc

import (
	"admin/internal/service"
	"crypto/x509"
	"crypto/x509/pkix"
	"net/url"
	"testing"
)

func TestParseAgentClaimsFromPeerCertificate_UsesOnlySPIFEEURIs(t *testing.T) {
	t.Parallel()

	cert := &x509.Certificate{
		Subject: pkix.Name{CommonName: "agent:legacy-node"},
		URIs: []*url.URL{
			mustParseURL(t, "spiffe://aurora.local/node/node-a"),
			mustParseURL(t, "spiffe://aurora.local/service/aurora-agent"),
			mustParseURL(t, "spiffe://aurora.local/role/agent"),
			mustParseURL(t, "spiffe://aurora.local/cluster/dev"),
		},
	}

	claims := parseAgentClaimsFromPeerCertificate(cert)
	if claims.NodeID != "node-a" || claims.ServiceID != "aurora-agent" || claims.Role != "agent" || claims.ClusterID != "dev" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestParseAgentClaimsFromPeerCertificate_NoCNFallback(t *testing.T) {
	t.Parallel()

	cert := &x509.Certificate{
		Subject: pkix.Name{CommonName: "agent:legacy-node"},
	}
	claims := parseAgentClaimsFromPeerCertificate(cert)
	if claims.NodeID != "" || claims.ServiceID != "" || claims.Role != "" || claims.ClusterID != "" {
		t.Fatalf("expected empty claims when SAN URIs are missing, got: %+v", claims)
	}
}

func TestValidateAgentPeerClaims(t *testing.T) {
	t.Parallel()

	valid := service.AgentIdentityClaims{
		NodeID:    "node-a",
		ServiceID: "aurora-agent",
		Role:      "agent",
		ClusterID: "dev",
	}
	if err := validateAgentPeerClaims(valid); err != nil {
		t.Fatalf("valid claims rejected: %v", err)
	}

	invalid := valid
	invalid.Role = ""
	if err := validateAgentPeerClaims(invalid); err == nil {
		t.Fatalf("expected missing role claim error")
	}
}

func TestAuthorizeAgentRole(t *testing.T) {
	t.Parallel()

	if err := authorizeAgentRole(runtimeReportAgentPath, "agent"); err != nil {
		t.Fatalf("expected agent role allowed: %v", err)
	}
	if err := authorizeAgentRole(runtimeReportAgentPath, "admin"); err == nil {
		t.Fatalf("expected admin role denied for agent method")
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	value, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url failed: %v", err)
	}
	return value
}
