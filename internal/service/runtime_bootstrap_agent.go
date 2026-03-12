package service

import (
	keycfg "admin/internal/key"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	agentRegistryLeaseTTLSeconds int64 = 30
	agentSPIFFETrustDomain             = "aurora.local"
	csrStatusPending                   = "pending"
	csrStatusApproved                  = "approved"
	csrStatusIssued                    = "issued"
	csrStatusRejected                  = "rejected"
)

type bootstrapTokenRecord struct {
	TokenHash    string `json:"token_hash"`
	ClusterScope string `json:"cluster_scope"`
	IssuedAt     string `json:"issued_at"`
	ExpiresAt    string `json:"expires_at"`
	UsedAt       string `json:"used_at,omitempty"`
	MaxUse       int    `json:"max_use"`
}

type registryAgentRecord struct {
	AgentID           string `json:"agent_id"`
	ServiceID         string `json:"service_id"`
	Role              string `json:"role"`
	ClusterID         string `json:"cluster_id"`
	Hostname          string `json:"hostname"`
	IPAddress         string `json:"ip_address"`
	AgentVersion      string `json:"agent_version"`
	AgentProbeAddr    string `json:"agent_probe_addr"`
	AgentGRPCEndpoint string `json:"agent_grpc_endpoint"`
	Platform          string `json:"platform"`
	LibvirtURI        string `json:"libvirt_uri"`
	SeenAt            string `json:"seen_at"`
}

type agentCSRRequestRecord struct {
	RequestID      string `json:"request_id"`
	AgentID        string `json:"agent_id"`
	ServiceID      string `json:"service_id"`
	Role           string `json:"role"`
	ClusterID      string `json:"cluster_id"`
	CSRHashSHA256  string `json:"csr_hash_sha256"`
	Status         string `json:"status"`
	SubmittedAt    string `json:"submitted_at"`
	ApprovedAt     string `json:"approved_at,omitempty"`
	ApprovedBy     string `json:"approved_by,omitempty"`
	IssuedAt       string `json:"issued_at,omitempty"`
	IssuedSerial   string `json:"issued_serial,omitempty"`
	IssuedExpires  string `json:"issued_expires,omitempty"`
	RejectedAt     string `json:"rejected_at,omitempty"`
	RejectedBy     string `json:"rejected_by,omitempty"`
	RejectedReason string `json:"rejected_reason,omitempty"`
}

type agentAuditEventRecord struct {
	EventID    string         `json:"event_id"`
	AgentID    string         `json:"agent_id"`
	EventType  string         `json:"event_type"`
	Status     string         `json:"status"`
	Actor      string         `json:"actor"`
	OccurredAt string         `json:"occurred_at"`
	Details    map[string]any `json:"details,omitempty"`
}

func (s *RuntimeBootstrapService) BootstrapAgent(
	ctx context.Context,
	req AgentBootstrapRequest,
	peerAddr string,
) (*AgentBootstrapResult, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	claims := AgentIdentityClaims{
		NodeID:    normalizeAgentID(req.NodeID),
		ServiceID: strings.TrimSpace(req.ServiceID),
		Role:      strings.TrimSpace(req.Role),
		ClusterID: strings.TrimSpace(req.ClusterID),
	}
	if err := validateAgentClaims(claims); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.BootstrapToken) == "" {
		return nil, fmt.Errorf("bootstrap_token is required")
	}

	consumeResult, consumeErr := s.runtimeRepo.ConsumeBootstrapTokenTx(ctx, req.BootstrapToken, time.Now().UTC())
	if consumeErr != nil {
		return nil, fmt.Errorf("consume bootstrap token failed: %w", consumeErr)
	}
	if consumeResult == nil || !consumeResult.Consumed {
		return nil, ErrAgentBootstrapTokenInvalid
	}
	clusterScope := strings.TrimSpace(consumeResult.Record.ClusterScope)
	if clusterScope != "" && clusterScope != "*" && clusterScope != claims.ClusterID {
		return nil, ErrAgentBootstrapTokenInvalid
	}

	csr, csrClaims, csrErr := parseAndVerifyCSR(req.CSRPEM)
	if csrErr != nil {
		return nil, csrErr
	}
	if err := compareAgentClaims(claims, csrClaims); err != nil {
		return nil, ErrAgentCSRInvalid
	}

	submittedAt := time.Now().UTC()
	csrHash := sha256.Sum256([]byte(strings.TrimSpace(req.CSRPEM)))
	csrRequest := agentCSRRequestRecord{
		RequestID:     randomHexID(12),
		AgentID:       claims.NodeID,
		ServiceID:     claims.ServiceID,
		Role:          claims.Role,
		ClusterID:     claims.ClusterID,
		CSRHashSHA256: hex.EncodeToString(csrHash[:]),
		Status:        csrStatusPending,
		SubmittedAt:   submittedAt.Format(time.RFC3339Nano),
	}
	if err := s.upsertCSRRequestRecord(ctx, csrRequest); err != nil {
		return nil, err
	}
	_ = s.writeAgentAuditEvent(ctx, claims.NodeID, "csr.submitted", "ok", "controller:bootstrap", map[string]any{
		"request_id":   csrRequest.RequestID,
		"cluster_id":   claims.ClusterID,
		"service_id":   claims.ServiceID,
		"role":         claims.Role,
		"submitted_at": csrRequest.SubmittedAt,
	})
	csrRequest.Status = csrStatusApproved
	csrRequest.ApprovedAt = submittedAt.Format(time.RFC3339Nano)
	csrRequest.ApprovedBy = "controller:auto-token"
	if err := s.upsertCSRRequestRecord(ctx, csrRequest); err != nil {
		return nil, err
	}
	_ = s.writeAgentAuditEvent(ctx, claims.NodeID, "csr.approved", "ok", csrRequest.ApprovedBy, map[string]any{
		"request_id": csrRequest.RequestID,
	})

	peerHost := normalizePeerRemoteHost(peerAddr)
	resolvedIP := strings.TrimSpace(req.IPAddress)
	if resolvedIP == "" {
		if ip := net.ParseIP(peerHost); ip != nil {
			resolvedIP = ip.String()
		}
	}
	resolvedGRPCEndpoint := strings.TrimSpace(req.AgentGRPCEndpoint)
	if resolvedGRPCEndpoint == "" && peerHost != "" {
		if _, port, splitErr := net.SplitHostPort(strings.TrimSpace(req.AgentProbeAddr)); splitErr == nil && strings.TrimSpace(port) != "" {
			resolvedGRPCEndpoint = net.JoinHostPort(peerHost, strings.TrimSpace(port))
		}
	}

	caCert, caKey, caCertPEM, caErr := s.loadAgentCA()
	if caErr != nil {
		return nil, caErr
	}

	issuedCertPEM, certMeta, issueErr := issueAgentClientCert(
		caCert,
		caKey,
		csr,
		claims,
		strings.TrimSpace(req.Hostname),
		resolvedIP,
		s.agentCertTTL,
	)
	if issueErr != nil {
		csrRequest.Status = csrStatusRejected
		csrRequest.RejectedAt = time.Now().UTC().Format(time.RFC3339Nano)
		csrRequest.RejectedBy = "controller:issuer"
		csrRequest.RejectedReason = strings.TrimSpace(issueErr.Error())
		_ = s.upsertCSRRequestRecord(ctx, csrRequest)
		_ = s.writeAgentAuditEvent(ctx, claims.NodeID, "csr.issued", "error", csrRequest.RejectedBy, map[string]any{
			"request_id": csrRequest.RequestID,
			"reason":     csrRequest.RejectedReason,
		})
		return nil, issueErr
	}
	csrRequest.Status = csrStatusIssued
	csrRequest.IssuedAt = time.Now().UTC().Format(time.RFC3339Nano)
	csrRequest.IssuedSerial = certMeta.SerialHex
	csrRequest.IssuedExpires = certMeta.ExpiresAt.UTC().Format(time.RFC3339Nano)
	if err := s.upsertCSRRequestRecord(ctx, csrRequest); err != nil {
		return nil, err
	}
	_ = s.writeAgentAuditEvent(ctx, claims.NodeID, "csr.issued", "ok", "controller:issuer", map[string]any{
		"request_id": csrRequest.RequestID,
		"serial":     certMeta.SerialHex,
		"expires_at": csrRequest.IssuedExpires,
	})

	kv := map[string]string{
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "agent_id"):                claims.NodeID,
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "service_id"):              claims.ServiceID,
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "role"):                    claims.Role,
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "cluster_id"):              claims.ClusterID,
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "status"):                  "bootstrapped",
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "hostname"):                strings.TrimSpace(req.Hostname),
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "ip"):                      resolvedIP,
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "platform"):                strings.TrimSpace(req.Platform),
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "probe_addr"):              strings.TrimSpace(req.AgentProbeAddr),
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "grpc_endpoint"):           resolvedGRPCEndpoint,
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "bootstrap/issued_at"):     time.Now().UTC().Format(time.RFC3339Nano),
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "bootstrap/peer_address"):  strings.TrimSpace(peerAddr),
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "bootstrap/peer_host"):     peerHost,
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "cert/serial"):             certMeta.SerialHex,
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "cert/expires_at"):         certMeta.ExpiresAt.UTC().Format(time.RFC3339Nano),
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "cert/fingerprint_sha256"): certMeta.FingerprintSHA256,
		keycfg.RuntimeAgentNodeKey(claims.NodeID, "cert/subject"):            certMeta.Subject,
	}
	for key, value := range kv {
		if upsertErr := s.runtimeRepo.Upsert(ctx, key, strings.TrimSpace(value)); upsertErr != nil {
			return nil, fmt.Errorf("upsert bootstrap metadata failed (%s): %w", key, upsertErr)
		}
	}

	return &AgentBootstrapResult{
		ClientCertPEM: issuedCertPEM,
		CACertPEM:     caCertPEM,
		SerialHex:     certMeta.SerialHex,
		ExpiresAt:     certMeta.ExpiresAt.UTC(),
	}, nil
}

func (s *RuntimeBootstrapService) RenewAgentCertificate(
	ctx context.Context,
	req AgentRenewRequest,
	peerClaims AgentIdentityClaims,
	hostname string,
	ipAddress string,
) (*AgentBootstrapResult, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}
	if err := validateAgentClaims(peerClaims); err != nil {
		return nil, err
	}

	csr, csrClaims, csrErr := parseAndVerifyCSR(req.CSRPEM)
	if csrErr != nil {
		return nil, csrErr
	}
	if err := compareAgentClaims(peerClaims, csrClaims); err != nil {
		return nil, ErrAgentCSRInvalid
	}

	caCert, caKey, caCertPEM, caErr := s.loadAgentCA()
	if caErr != nil {
		return nil, caErr
	}
	issuedCertPEM, certMeta, issueErr := issueAgentClientCert(
		caCert,
		caKey,
		csr,
		peerClaims,
		strings.TrimSpace(hostname),
		strings.TrimSpace(ipAddress),
		s.agentCertTTL,
	)
	if issueErr != nil {
		_ = s.writeAgentAuditEvent(ctx, peerClaims.NodeID, "cert.renew", "error", "agent:"+peerClaims.NodeID, map[string]any{
			"reason": strings.TrimSpace(issueErr.Error()),
		})
		return nil, issueErr
	}

	renewAt := time.Now().UTC().Format(time.RFC3339Nano)
	kv := map[string]string{
		keycfg.RuntimeAgentNodeKey(peerClaims.NodeID, "cert/serial"):             certMeta.SerialHex,
		keycfg.RuntimeAgentNodeKey(peerClaims.NodeID, "cert/expires_at"):         certMeta.ExpiresAt.UTC().Format(time.RFC3339Nano),
		keycfg.RuntimeAgentNodeKey(peerClaims.NodeID, "cert/fingerprint_sha256"): certMeta.FingerprintSHA256,
		keycfg.RuntimeAgentNodeKey(peerClaims.NodeID, "cert/subject"):            certMeta.Subject,
		keycfg.RuntimeAgentNodeKey(peerClaims.NodeID, "cert/renewed_at"):         renewAt,
	}
	for key, value := range kv {
		if upsertErr := s.runtimeRepo.Upsert(ctx, key, strings.TrimSpace(value)); upsertErr != nil {
			return nil, fmt.Errorf("upsert renew metadata failed (%s): %w", key, upsertErr)
		}
	}
	_ = s.writeAgentAuditEvent(ctx, peerClaims.NodeID, "cert.renew", "ok", "agent:"+peerClaims.NodeID, map[string]any{
		"serial":     certMeta.SerialHex,
		"expires_at": certMeta.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})

	return &AgentBootstrapResult{
		ClientCertPEM: issuedCertPEM,
		CACertPEM:     caCertPEM,
		SerialHex:     certMeta.SerialHex,
		ExpiresAt:     certMeta.ExpiresAt.UTC(),
	}, nil
}

func (s *RuntimeBootstrapService) RotateAgentBootstrapToken(ctx context.Context) (*AgentBootstrapTokenResult, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate bootstrap token failed: %w", err)
	}
	token := strings.TrimSpace(base64.StdEncoding.EncodeToString(raw))
	if token == "" {
		return nil, fmt.Errorf("generate bootstrap token failed: empty token")
	}

	existing, err := s.runtimeRepo.ListByPrefix(ctx, keycfg.RuntimeAgentBootstrapTokenPrefix)
	if err != nil {
		return nil, fmt.Errorf("list bootstrap tokens failed: %w", err)
	}
	for _, item := range existing {
		if deleteErr := s.runtimeRepo.Delete(ctx, item.Key); deleteErr != nil {
			return nil, fmt.Errorf("delete old bootstrap token failed (%s): %w", item.Key, deleteErr)
		}
	}

	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	record := bootstrapTokenRecord{
		TokenHash:    hash,
		ClusterScope: "*",
		IssuedAt:     now.Format(time.RFC3339Nano),
		ExpiresAt:    now.Add(10 * time.Minute).Format(time.RFC3339Nano),
		UsedAt:       "",
		MaxUse:       1,
	}
	payload, marshalErr := json.Marshal(record)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal bootstrap token record failed: %w", marshalErr)
	}

	tokenKey := keycfg.RuntimeAgentBootstrapTokenKey(hash)
	if err := s.runtimeRepo.Upsert(ctx, tokenKey, strings.TrimSpace(string(payload))); err != nil {
		return nil, fmt.Errorf("store bootstrap token hash failed: %w", err)
	}

	return &AgentBootstrapTokenResult{
		Token:         token,
		TokenHash:     hash,
		ClusterPolicy: record.ClusterScope,
	}, nil
}

func parseAndVerifyCSR(rawCSR string) (*x509.CertificateRequest, AgentIdentityClaims, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(rawCSR)))
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, AgentIdentityClaims{}, ErrAgentCSRInvalid
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, AgentIdentityClaims{}, ErrAgentCSRInvalid
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, AgentIdentityClaims{}, ErrAgentCSRInvalid
	}
	claims := parseClaimsFromURIs(csr.URIs)
	if err := validateAgentClaims(claims); err != nil {
		return nil, AgentIdentityClaims{}, ErrAgentCSRInvalid
	}
	return csr, claims, nil
}

func validateAgentClaims(claims AgentIdentityClaims) error {
	claims.NodeID = normalizeAgentID(claims.NodeID)
	if claims.NodeID == "" {
		return fmt.Errorf("node_id is required")
	}
	if strings.TrimSpace(claims.ServiceID) == "" {
		return fmt.Errorf("service_id is required")
	}
	if strings.TrimSpace(claims.Role) == "" {
		return fmt.Errorf("role is required")
	}
	if strings.TrimSpace(claims.ClusterID) == "" {
		return fmt.Errorf("cluster_id is required")
	}
	return nil
}

func compareAgentClaims(expected AgentIdentityClaims, actual AgentIdentityClaims) error {
	if normalizeAgentID(expected.NodeID) != normalizeAgentID(actual.NodeID) {
		return fmt.Errorf("node_id claim mismatch")
	}
	if strings.TrimSpace(expected.ServiceID) != strings.TrimSpace(actual.ServiceID) {
		return fmt.Errorf("service_id claim mismatch")
	}
	if strings.TrimSpace(expected.Role) != strings.TrimSpace(actual.Role) {
		return fmt.Errorf("role claim mismatch")
	}
	if strings.TrimSpace(expected.ClusterID) != strings.TrimSpace(actual.ClusterID) {
		return fmt.Errorf("cluster_id claim mismatch")
	}
	return nil
}

func parseClaimsFromURIs(uris []*url.URL) AgentIdentityClaims {
	claims := AgentIdentityClaims{}
	for _, uri := range uris {
		if uri == nil || strings.TrimSpace(strings.ToLower(uri.Scheme)) != "spiffe" {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(uri.Host), agentSPIFFETrustDomain) {
			continue
		}

		segments := strings.Split(strings.Trim(path.Clean(uri.Path), "/"), "/")
		if len(segments) != 2 {
			continue
		}
		kind := strings.TrimSpace(strings.ToLower(segments[0]))
		value := strings.TrimSpace(segments[1])
		if value == "" {
			continue
		}
		switch kind {
		case "node":
			claims.NodeID = value
		case "service":
			claims.ServiceID = value
		case "role":
			claims.Role = value
		case "cluster":
			claims.ClusterID = value
		}
	}
	return claims
}

type issuedCertMetadata struct {
	SerialHex         string
	ExpiresAt         time.Time
	FingerprintSHA256 string
	Subject           string
}

func (s *RuntimeBootstrapService) loadAgentCA() (*x509.Certificate, any, string, error) {
	caCertPath := strings.TrimSpace(s.agentCACertPath)
	caKeyPath := strings.TrimSpace(s.agentCAKeyPath)
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
	case "EC PRIVATE KEY":
		parsedKey, parseErr := x509.ParseECPrivateKey(keyBlock.Bytes)
		if parseErr != nil {
			return nil, nil, "", fmt.Errorf("parse ec ca key failed: %w", parseErr)
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
	if !caPrivateKeyMatchesCertificate(caCert, caKey) {
		return nil, nil, "", fmt.Errorf("ca cert and ca key do not match")
	}

	return caCert, caKey, strings.TrimSpace(string(caCertPEMBytes)), nil
}

func caPrivateKeyMatchesCertificate(cert *x509.Certificate, key any) bool {
	if cert == nil || cert.PublicKey == nil || key == nil {
		return false
	}

	switch privateKey := key.(type) {
	case *rsa.PrivateKey:
		certPub, ok := cert.PublicKey.(*rsa.PublicKey)
		return ok && certPub != nil &&
			certPub.N.Cmp(privateKey.PublicKey.N) == 0 &&
			certPub.E == privateKey.PublicKey.E
	case *ecdsa.PrivateKey:
		certPub, ok := cert.PublicKey.(*ecdsa.PublicKey)
		return ok && certPub != nil &&
			certPub.X.Cmp(privateKey.PublicKey.X) == 0 &&
			certPub.Y.Cmp(privateKey.PublicKey.Y) == 0 &&
			certPub.Curve == privateKey.PublicKey.Curve
	case ed25519.PrivateKey:
		certPub, ok := cert.PublicKey.(ed25519.PublicKey)
		if !ok || certPub == nil {
			return false
		}
		keyPub, ok := privateKey.Public().(ed25519.PublicKey)
		return ok && keyPub != nil && string(certPub) == string(keyPub)
	default:
		return false
	}
}

func issueAgentClientCert(
	caCert *x509.Certificate,
	caKey any,
	csr *x509.CertificateRequest,
	claims AgentIdentityClaims,
	hostname string,
	ipAddress string,
	ttl time.Duration,
) (string, issuedCertMetadata, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", issuedCertMetadata{}, fmt.Errorf("generate cert serial failed: %w", err)
	}
	now := time.Now().UTC()
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour
	}
	expiresAt := now.Add(ttl)

	dnsNames := uniqueStrings(append(csr.DNSNames, strings.TrimSpace(hostname), strings.TrimSpace(claims.NodeID)))
	ipAddresses := append([]net.IP{}, csr.IPAddresses...)
	if ip := net.ParseIP(strings.TrimSpace(ipAddress)); ip != nil {
		ipAddresses = append(ipAddresses, ip)
	}
	uris := buildAgentIdentityURIs(claims)

	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "agent:" + claims.NodeID,
			Organization: []string{"Aurora"},
			OrganizationalUnit: []string{
				"service:" + claims.ServiceID,
				"role:" + claims.Role,
			},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              expiresAt,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddresses,
		URIs:                  uris,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tpl, caCert, csr.PublicKey, caKey)
	if err != nil {
		return "", issuedCertMetadata{}, fmt.Errorf("issue agent cert failed: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if len(pemBytes) == 0 {
		return "", issuedCertMetadata{}, fmt.Errorf("encode issued cert pem failed")
	}
	fingerprint := sha256.Sum256(certDER)
	metadata := issuedCertMetadata{
		SerialHex:         strings.ToUpper(serial.Text(16)),
		ExpiresAt:         expiresAt.UTC(),
		FingerprintSHA256: hex.EncodeToString(fingerprint[:]),
		Subject:           strings.TrimSpace(tpl.Subject.String()),
	}
	return strings.TrimSpace(string(pemBytes)), metadata, nil
}

func buildAgentIdentityURIs(claims AgentIdentityClaims) []*url.URL {
	values := []string{
		fmt.Sprintf("spiffe://%s/node/%s", agentSPIFFETrustDomain, strings.TrimSpace(claims.NodeID)),
		fmt.Sprintf("spiffe://%s/service/%s", agentSPIFFETrustDomain, strings.TrimSpace(claims.ServiceID)),
		fmt.Sprintf("spiffe://%s/role/%s", agentSPIFFETrustDomain, strings.TrimSpace(claims.Role)),
		fmt.Sprintf("spiffe://%s/cluster/%s", agentSPIFFETrustDomain, strings.TrimSpace(claims.ClusterID)),
	}
	out := make([]*url.URL, 0, len(values))
	for _, raw := range values {
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		out = append(out, parsed)
	}
	return out
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
	if strings.TrimSpace(input.ServiceID) == "" || strings.TrimSpace(input.Role) == "" || strings.TrimSpace(input.ClusterID) == "" {
		return fmt.Errorf("service_id/role/cluster_id are required")
	}

	remoteHost := normalizePeerRemoteHost(peer.RemoteAddress)
	agentGRPCEndpoint := strings.TrimSpace(input.AgentGRPCEndpoint)
	if remoteHost != "" {
		if _, port, splitErr := net.SplitHostPort(strings.TrimSpace(input.AgentProbeAddr)); splitErr == nil && strings.TrimSpace(port) != "" {
			agentGRPCEndpoint = net.JoinHostPort(remoteHost, strings.TrimSpace(port))
		}
	}
	if strings.TrimSpace(agentGRPCEndpoint) == "" {
		return fmt.Errorf("cannot resolve agent grpc endpoint from peer/probe address")
	}

	record := registryAgentRecord{
		AgentID:           agentID,
		ServiceID:         strings.TrimSpace(input.ServiceID),
		Role:              strings.TrimSpace(input.Role),
		ClusterID:         strings.TrimSpace(input.ClusterID),
		Hostname:          strings.TrimSpace(input.Hostname),
		IPAddress:         strings.TrimSpace(input.AgentIP),
		AgentVersion:      strings.TrimSpace(input.AgentVersion),
		AgentProbeAddr:    strings.TrimSpace(input.AgentProbeAddr),
		AgentGRPCEndpoint: strings.TrimSpace(agentGRPCEndpoint),
		Platform:          strings.TrimSpace(input.Platform),
		LibvirtURI:        strings.TrimSpace(input.LibvirtURI),
		SeenAt:            time.Now().UTC().Format(time.RFC3339Nano),
	}
	payload, marshalErr := json.Marshal(record)
	if marshalErr != nil {
		return fmt.Errorf("marshal registry record failed: %w", marshalErr)
	}
	leaseID, leaseErr := s.runtimeRepo.UpsertWithLease(
		ctx,
		keycfg.RegistryAgentKey(agentID),
		strings.TrimSpace(string(payload)),
		agentRegistryLeaseTTLSeconds,
	)
	if leaseErr != nil {
		return fmt.Errorf("upsert registry record with lease failed: %w", leaseErr)
	}

	kv := map[string]string{
		keycfg.RuntimeAgentNodeKey(agentID, "agent_id"):           agentID,
		keycfg.RuntimeAgentNodeKey(agentID, "service_id"):         record.ServiceID,
		keycfg.RuntimeAgentNodeKey(agentID, "role"):               record.Role,
		keycfg.RuntimeAgentNodeKey(agentID, "cluster_id"):         record.ClusterID,
		keycfg.RuntimeAgentNodeKey(agentID, "status"):             "connected",
		keycfg.RuntimeAgentNodeKey(agentID, "last_seen_at"):       record.SeenAt,
		keycfg.RuntimeAgentNodeKey(agentID, "hostname"):           record.Hostname,
		keycfg.RuntimeAgentNodeKey(agentID, "agent_version"):      record.AgentVersion,
		keycfg.RuntimeAgentNodeKey(agentID, "probe_addr"):         record.AgentProbeAddr,
		keycfg.RuntimeAgentNodeKey(agentID, "grpc_endpoint"):      record.AgentGRPCEndpoint,
		keycfg.RuntimeAgentNodeKey(agentID, "platform"):           record.Platform,
		keycfg.RuntimeAgentNodeKey(agentID, "libvirt_uri"):        record.LibvirtURI,
		keycfg.RuntimeAgentNodeKey(agentID, "peer/address"):       strings.TrimSpace(peer.RemoteAddress),
		keycfg.RuntimeAgentNodeKey(agentID, "peer/host"):          remoteHost,
		keycfg.RuntimeAgentNodeKey(agentID, "peer/cert_sha"):      strings.TrimSpace(peer.CertSHA256),
		keycfg.RuntimeAgentNodeKey(agentID, "peer/cert_name"):     strings.TrimSpace(peer.CertSubject),
		keycfg.RuntimeAgentNodeKey(agentID, "peer/cert_serial"):   strings.TrimSpace(strings.ToUpper(peer.CertSerialHex)),
		keycfg.RuntimeAgentNodeKey(agentID, "registry/lease_id"):  strconv.FormatInt(int64(leaseID), 10),
		keycfg.RuntimeAgentNodeKey(agentID, "registry/lease_ttl"): strconv.FormatInt(agentRegistryLeaseTTLSeconds, 10),
	}

	for key, value := range kv {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if upsertErr := s.runtimeRepo.Upsert(ctx, key, strings.TrimSpace(value)); upsertErr != nil {
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

func (s *RuntimeBootstrapService) upsertCSRRequestRecord(
	ctx context.Context,
	record agentCSRRequestRecord,
) error {
	if s == nil || s.runtimeRepo == nil {
		return fmt.Errorf("runtime bootstrap service is nil")
	}
	key := keycfg.RuntimeAgentCSRRequestKey(record.AgentID, record.RequestID)
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal csr request record failed: %w", err)
	}
	if err := s.runtimeRepo.Upsert(ctx, key, strings.TrimSpace(string(payload))); err != nil {
		return fmt.Errorf("upsert csr request record failed: %w", err)
	}
	return nil
}

func (s *RuntimeBootstrapService) writeAgentAuditEvent(
	ctx context.Context,
	agentID string,
	eventType string,
	status string,
	actor string,
	details map[string]any,
) error {
	if s == nil || s.runtimeRepo == nil {
		return fmt.Errorf("runtime bootstrap service is nil")
	}
	record := agentAuditEventRecord{
		EventID:    randomHexID(16),
		AgentID:    strings.TrimSpace(agentID),
		EventType:  strings.TrimSpace(eventType),
		Status:     strings.TrimSpace(strings.ToLower(status)),
		Actor:      strings.TrimSpace(actor),
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Details:    details,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.runtimeRepo.Upsert(ctx, keycfg.RuntimeAgentAuditEventKey(record.AgentID, record.EventID), strings.TrimSpace(string(payload)))
}

func randomHexID(numBytes int) string {
	if numBytes <= 0 {
		numBytes = 8
	}
	raw := make([]byte, numBytes)
	if _, err := rand.Read(raw); err != nil {
		return strings.ToLower(strconv.FormatInt(time.Now().UTC().UnixNano(), 16))
	}
	return hex.EncodeToString(raw)
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
