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
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

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
	if seedErr := s.seedAgentCertStore(ctx, nodeID, issuedCertPEM, caCertPEM); seedErr != nil {
		return nil, seedErr
	}

	return &AgentBootstrapResult{
		ClientCertPEM: issuedCertPEM,
		CACertPEM:     caCertPEM,
		SerialHex:     serialHex,
		ExpiresAt:     expiresAt.UTC(),
	}, nil
}

func (s *RuntimeBootstrapService) seedAgentCertStore(
	ctx context.Context,
	agentID string,
	certPEM string,
	caPEM string,
) error {
	if s == nil || s.certStoreRepo == nil {
		return fmt.Errorf("cert store repository is nil")
	}

	normalizedAgentID := normalizeAgentID(agentID)
	if normalizedAgentID == "" {
		return fmt.Errorf("agent_id is required")
	}

	trimmedCert := strings.TrimSpace(certPEM)
	trimmedCA := strings.TrimSpace(caPEM)
	if trimmedCert == "" || trimmedCA == "" {
		return fmt.Errorf("agent certificate bundle is empty")
	}

	objectID := agentCertStoreObjectID(normalizedAgentID)
	values := map[string]string{
		buildAgentCertStoreKey(s.certStorePrefix, objectID, agentCertStoreTypeClientCert): trimmedCert,
		buildAgentCertStoreKey(s.certStorePrefix, objectID, agentCertStoreTypeCA):         trimmedCA,
	}
	for key, value := range values {
		if err := s.certStoreRepo.Put(ctx, key, value); err != nil {
			return fmt.Errorf("seed agent cert store failed (%s): %w", key, err)
		}
	}
	return nil
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

func (s *RuntimeBootstrapService) RotateAgentBootstrapToken(ctx context.Context) (*AgentBootstrapTokenResult, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate bootstrap token failed: %w", err)
	}

	encoded := strings.TrimSpace(base64.StdEncoding.EncodeToString(raw))
	if encoded == "" {
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

	sum := sha256.Sum256([]byte(encoded))
	hash := hex.EncodeToString(sum[:])
	clusterPolicy := "*"

	if err := s.runtimeRepo.Upsert(ctx, keycfg.RuntimeAgentBootstrapTokenKey(hash), clusterPolicy); err != nil {
		return nil, fmt.Errorf("store bootstrap token hash failed: %w", err)
	}
	if err := s.runtimeRepo.Upsert(ctx, keycfg.RuntimeAgentBootstrapTokenActive, encoded); err != nil {
		return nil, fmt.Errorf("store active bootstrap token failed: %w", err)
	}

	return &AgentBootstrapTokenResult{
		Token:         encoded,
		TokenHash:     hash,
		ClusterPolicy: clusterPolicy,
	}, nil
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
		if !ok || certPub == nil {
			return false
		}
		return certPub.N.Cmp(privateKey.PublicKey.N) == 0 && certPub.E == privateKey.PublicKey.E
	case *ecdsa.PrivateKey:
		certPub, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok || certPub == nil {
			return false
		}
		return certPub.X.Cmp(privateKey.PublicKey.X) == 0 &&
			certPub.Y.Cmp(privateKey.PublicKey.Y) == 0 &&
			certPub.Curve == privateKey.PublicKey.Curve
	case ed25519.PrivateKey:
		certPub, ok := cert.PublicKey.(ed25519.PublicKey)
		if !ok || certPub == nil {
			return false
		}
		keyPub, ok := privateKey.Public().(ed25519.PublicKey)
		if !ok || keyPub == nil {
			return false
		}
		return string(certPub) == string(keyPub)
	case *ed25519.PrivateKey:
		if privateKey == nil {
			return false
		}
		certPub, ok := cert.PublicKey.(ed25519.PublicKey)
		if !ok || certPub == nil {
			return false
		}
		keyPub, ok := privateKey.Public().(ed25519.PublicKey)
		if !ok || keyPub == nil {
			return false
		}
		return string(certPub) == string(keyPub)
	default:
		return false
	}
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

func agentCertStoreObjectID(agentID string) uuid.UUID {
	normalized := normalizeAgentID(agentID)
	if normalized == "" {
		normalized = "agent"
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("agent-cert:"+normalized))
}

func buildAgentCertStoreKey(prefix string, objectID uuid.UUID, certType string) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	key := objectID.String() + "-" + strings.TrimSpace(certType)
	if base == "" {
		return key
	}
	return base + "/" + key
}
