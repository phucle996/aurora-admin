package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net/url"
	"testing"
)

func TestParseAndVerifyCSR_ParsesSPIFFEClaims(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	uris := []*url.URL{
		mustParseURL(t, "spiffe://aurora.local/node/node-a"),
		mustParseURL(t, "spiffe://aurora.local/service/aurora-agent"),
		mustParseURL(t, "spiffe://aurora.local/role/agent"),
		mustParseURL(t, "spiffe://aurora.local/cluster/dev-cluster"),
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "agent:node-a"},
		URIs:    uris,
	}, key)
	if err != nil {
		t.Fatalf("create csr failed: %v", err)
	}
	csrPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))

	_, claims, err := parseAndVerifyCSR(csrPEM)
	if err != nil {
		t.Fatalf("parseAndVerifyCSR failed: %v", err)
	}
	if claims.NodeID != "node-a" {
		t.Fatalf("unexpected node_id: %s", claims.NodeID)
	}
	if claims.ServiceID != "aurora-agent" {
		t.Fatalf("unexpected service_id: %s", claims.ServiceID)
	}
	if claims.Role != "agent" {
		t.Fatalf("unexpected role: %s", claims.Role)
	}
	if claims.ClusterID != "dev-cluster" {
		t.Fatalf("unexpected cluster_id: %s", claims.ClusterID)
	}
}

func TestParseAndVerifyCSR_RejectsMissingClaims(t *testing.T) {
	t.Parallel()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "agent:node-a"},
	}, key)
	if err != nil {
		t.Fatalf("create csr failed: %v", err)
	}
	csrPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))

	_, _, err = parseAndVerifyCSR(csrPEM)
	if err == nil {
		t.Fatalf("expected parseAndVerifyCSR to fail for missing claims")
	}
	if err != ErrAgentCSRInvalid {
		t.Fatalf("expected ErrAgentCSRInvalid, got: %v", err)
	}
}

func TestCompareAgentClaims_Mismatch(t *testing.T) {
	t.Parallel()

	expected := AgentIdentityClaims{
		NodeID:    "node-a",
		ServiceID: "aurora-agent",
		Role:      "agent",
		ClusterID: "dev",
	}
	actual := AgentIdentityClaims{
		NodeID:    "node-a",
		ServiceID: "aurora-agent",
		Role:      "agent",
		ClusterID: "prod",
	}
	if err := compareAgentClaims(expected, actual); err == nil {
		t.Fatalf("expected claim mismatch error")
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
