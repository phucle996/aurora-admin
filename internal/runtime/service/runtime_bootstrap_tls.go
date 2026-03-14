package service

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

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

func runtimeBootstrapClientCertStoreKey(prefix string, moduleName string) string {
	return runtimeBootstrapTLSStoreKey(prefix, runtimeBootstrapTLSObjectID(moduleName), "bootstrap_client_cert")
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
	bootstrapClientCertKey := runtimeBootstrapClientCertStoreKey(s.certStorePrefix, name)
	clientCertKey := runtimeBootstrapTLSStoreKey(s.certStorePrefix, objectID, "client_cert")
	loaded, err := s.certStoreRepo.GetMany(ctx, []string{bootstrapClientCertKey, clientCertKey})
	if err != nil {
		return fmt.Errorf("load cert store keys failed: %w", err)
	}

	expectedPEM := strings.TrimSpace(loaded[bootstrapClientCertKey])
	if expectedPEM == "" {
		expectedPEM = strings.TrimSpace(loaded[clientCertKey])
	}
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
