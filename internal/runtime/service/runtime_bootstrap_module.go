package service

import (
	keycfg "admin/internal/key"
	"context"
	"crypto/rand"
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
	"strings"
	"time"
)

const moduleBootstrapRuntimeRole = "module-runtime"

type moduleBootstrapTokenRecord struct {
	TokenHash   string `json:"token_hash"`
	ModuleScope string `json:"module_scope"`
	IssuedAt    string `json:"issued_at"`
	ExpiresAt   string `json:"expires_at"`
	UsedAt      string `json:"used_at,omitempty"`
	MaxUse      int    `json:"max_use"`
}

type moduleIdentityClaims struct {
	ModuleName string
	ServiceID  string
	Role       string
}

func (s *RuntimeBootstrapService) IssueModuleBootstrapToken(
	ctx context.Context,
	moduleName string,
) (*ModuleBootstrapTokenResult, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	name := normalizeBootstrapModuleName(moduleName)
	if name == "" {
		return nil, fmt.Errorf("module_name is required")
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate module bootstrap token failed: %w", err)
	}
	token := strings.TrimSpace(base64.StdEncoding.EncodeToString(raw))
	if token == "" {
		return nil, fmt.Errorf("generate module bootstrap token failed: empty token")
	}

	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	record := moduleBootstrapTokenRecord{
		TokenHash:   hash,
		ModuleScope: name,
		IssuedAt:    now.Format(time.RFC3339Nano),
		ExpiresAt:   now.Add(10 * time.Minute).Format(time.RFC3339Nano),
		MaxUse:      1,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("marshal module bootstrap token record failed: %w", err)
	}
	if err := s.runtimeRepo.Upsert(ctx, keycfg.RuntimeModuleBootstrapTokenKey(hash), strings.TrimSpace(string(payload))); err != nil {
		return nil, fmt.Errorf("store module bootstrap token failed: %w", err)
	}

	return &ModuleBootstrapTokenResult{
		Token: token,
	}, nil
}

func (s *RuntimeBootstrapService) BootstrapModuleClient(
	ctx context.Context,
	req ModuleClientBootstrapRequest,
) (*ModuleClientBootstrapResult, error) {
	if s == nil || s.runtimeRepo == nil || s.certStoreRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	moduleName := normalizeBootstrapModuleName(req.ModuleName)
	if moduleName == "" {
		return nil, fmt.Errorf("module_name is required")
	}
	if strings.TrimSpace(req.BootstrapToken) == "" {
		return nil, fmt.Errorf("bootstrap_token is required")
	}

	csr, claims, err := parseAndVerifyModuleCSR(req.CSRPEM)
	if err != nil {
		return nil, err
	}
	if err := compareModuleClaims(moduleName, claims); err != nil {
		return nil, ErrAgentCSRInvalid
	}

	consumeResult, consumeErr := s.runtimeRepo.ConsumeModuleBootstrapTokenTx(
		ctx,
		req.BootstrapToken,
		moduleName,
		time.Now().UTC(),
	)
	if consumeErr != nil {
		return nil, fmt.Errorf("consume module bootstrap token failed: %w", consumeErr)
	}
	if consumeResult == nil || !consumeResult.Consumed {
		return nil, ErrAgentBootstrapTokenInvalid
	}

	caCert, caKey, caErr := s.loadAdminSigningCA()
	if caErr != nil {
		return nil, caErr
	}
	adminServerCAPEM, adminCAErr := s.loadAdminServerCAPEM()
	if adminCAErr != nil {
		return nil, adminCAErr
	}

	clientCertPEM, metadata, issueErr := issueModuleBootstrapCertificate(caCert, caKey, csr, claims)
	if issueErr != nil {
		return nil, issueErr
	}

	storeKey := runtimeBootstrapClientCertStoreKey(s.certStorePrefix, moduleName)
	if err := s.certStoreRepo.Put(ctx, storeKey, clientCertPEM); err != nil {
		return nil, fmt.Errorf("store module bootstrap client cert failed: %w", err)
	}

	return &ModuleClientBootstrapResult{
		ClientCertPEM:    clientCertPEM,
		AdminServerCAPEM: adminServerCAPEM,
		SerialHex:        metadata.SerialHex,
		ExpiresAt:        metadata.ExpiresAt.UTC(),
	}, nil
}

func parseAndVerifyModuleCSR(raw string) (*x509.CertificateRequest, moduleIdentityClaims, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, moduleIdentityClaims{}, ErrAgentCSRInvalid
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, moduleIdentityClaims{}, ErrAgentCSRInvalid
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, moduleIdentityClaims{}, ErrAgentCSRInvalid
	}
	claims := parseModuleClaimsFromURIs(csr.URIs)
	if err := validateModuleClaims(claims); err != nil {
		return nil, moduleIdentityClaims{}, ErrAgentCSRInvalid
	}
	return csr, claims, nil
}

func parseModuleClaimsFromURIs(uris []*url.URL) moduleIdentityClaims {
	claims := moduleIdentityClaims{}
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
		case "module":
			claims.ModuleName = value
		case "service":
			claims.ServiceID = value
		case "role":
			claims.Role = value
		}
	}
	return claims
}

func validateModuleClaims(claims moduleIdentityClaims) error {
	if normalizeBootstrapModuleName(claims.ModuleName) == "" {
		return fmt.Errorf("module_name is required")
	}
	if strings.TrimSpace(claims.ServiceID) == "" {
		return fmt.Errorf("service_id is required")
	}
	if strings.TrimSpace(claims.Role) == "" {
		return fmt.Errorf("role is required")
	}
	return nil
}

func compareModuleClaims(expectedModule string, claims moduleIdentityClaims) error {
	expected := normalizeBootstrapModuleName(expectedModule)
	actual := normalizeBootstrapModuleName(claims.ModuleName)
	if expected == "" || actual == "" || expected != actual {
		return fmt.Errorf("module_name claim mismatch")
	}
	if !strings.EqualFold(strings.TrimSpace(claims.ServiceID), expected) {
		return fmt.Errorf("service_id claim mismatch")
	}
	if !strings.EqualFold(strings.TrimSpace(claims.Role), moduleBootstrapRuntimeRole) {
		return fmt.Errorf("role claim mismatch")
	}
	return nil
}

func (s *RuntimeBootstrapService) loadAdminSigningCA() (*x509.Certificate, any, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("runtime bootstrap service is nil")
	}
	caCertPath := strings.TrimSpace(s.adminCACertPath)
	caKeyPath := strings.TrimSpace(s.adminCAKeyPath)
	if caCertPath == "" || caKeyPath == "" {
		return nil, nil, fmt.Errorf("admin signing ca paths are not configured")
	}

	caCertPEMBytes, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read admin ca cert failed: %w", err)
	}
	caKeyPEMBytes, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read admin ca key failed: %w", err)
	}

	certBlock, _ := pem.Decode(caCertPEMBytes)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, nil, fmt.Errorf("invalid admin ca cert pem")
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse admin ca cert failed: %w", err)
	}

	keyBlock, _ := pem.Decode(caKeyPEMBytes)
	if keyBlock == nil {
		return nil, nil, fmt.Errorf("invalid admin ca key pem")
	}

	var caKey any
	switch keyBlock.Type {
	case "RSA PRIVATE KEY":
		parsedKey, parseErr := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("parse rsa admin ca key failed: %w", parseErr)
		}
		caKey = parsedKey
	case "EC PRIVATE KEY":
		parsedKey, parseErr := x509.ParseECPrivateKey(keyBlock.Bytes)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("parse ec admin ca key failed: %w", parseErr)
		}
		caKey = parsedKey
	case "PRIVATE KEY":
		parsedKey, parseErr := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("parse pkcs8 admin ca key failed: %w", parseErr)
		}
		caKey = parsedKey
	default:
		return nil, nil, fmt.Errorf("unsupported admin ca key type: %s", keyBlock.Type)
	}

	if !caPrivateKeyMatchesCertificate(caCert, caKey) {
		return nil, nil, fmt.Errorf("admin ca cert and key do not match")
	}
	return caCert, caKey, nil
}

func issueModuleBootstrapCertificate(
	caCert *x509.Certificate,
	caKey any,
	csr *x509.CertificateRequest,
	claims moduleIdentityClaims,
) (string, issuedCertMetadata, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", issuedCertMetadata{}, fmt.Errorf("generate cert serial failed: %w", err)
	}
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)

	moduleName := normalizeBootstrapModuleName(claims.ModuleName)
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "module:" + moduleName,
			Organization: []string{"Aurora"},
			OrganizationalUnit: []string{
				"service:" + strings.TrimSpace(claims.ServiceID),
				"role:" + strings.TrimSpace(claims.Role),
			},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              expiresAt,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              uniqueStrings(csr.DNSNames),
		IPAddresses:           uniqueIPs(csr.IPAddresses),
		URIs:                  buildModuleIdentityURIs(claims),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tpl, caCert, csr.PublicKey, caKey)
	if err != nil {
		return "", issuedCertMetadata{}, fmt.Errorf("issue module bootstrap cert failed: %w", err)
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

func buildModuleIdentityURIs(claims moduleIdentityClaims) []*url.URL {
	values := []string{
		fmt.Sprintf("spiffe://%s/module/%s", agentSPIFFETrustDomain, normalizeBootstrapModuleName(claims.ModuleName)),
		fmt.Sprintf("spiffe://%s/service/%s", agentSPIFFETrustDomain, strings.TrimSpace(claims.ServiceID)),
		fmt.Sprintf("spiffe://%s/role/%s", agentSPIFFETrustDomain, strings.TrimSpace(claims.Role)),
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

func uniqueIPs(items []net.IP) []net.IP {
	seen := make(map[string]struct{}, len(items))
	out := make([]net.IP, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		value := strings.TrimSpace(item.String())
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, item)
	}
	return out
}
