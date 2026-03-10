package service

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
)

const (
	controlPlaneCertStoreTypeServerCert = "server_cert"
)

type ControlPlaneTrustSeedInput struct {
	AdminCACertPath          string
	AdminServerCertPath      string
	AgentCACertPath          string
	AgentAdminClientCertPath string
}

func (s *RuntimeBootstrapService) SeedControlPlaneTrustStore(
	ctx context.Context,
	input ControlPlaneTrustSeedInput,
) error {
	if s == nil || s.certStoreRepo == nil {
		return fmt.Errorf("runtime bootstrap service is nil")
	}

	adminCA, err := readAndValidateCertificatePEMFile(input.AdminCACertPath, "admin ca cert")
	if err != nil {
		return err
	}
	adminServerCert, err := readAndValidateCertificatePEMFile(input.AdminServerCertPath, "admin server cert")
	if err != nil {
		return err
	}
	agentCA, err := readAndValidateCertificatePEMFile(input.AgentCACertPath, "agent mtls ca cert")
	if err != nil {
		return err
	}
	adminAgentClientCert, err := readAndValidateCertificatePEMFile(input.AgentAdminClientCertPath, "admin agent client cert")
	if err != nil {
		return err
	}

	adminObjectID := controlPlaneCertStoreObjectID("admin")
	agentObjectID := controlPlaneCertStoreObjectID("agent")

	values := map[string]string{
		controlPlaneCertStoreKey(s.certStorePrefix, adminObjectID, agentCertStoreTypeCA):                adminCA,
		controlPlaneCertStoreKey(s.certStorePrefix, adminObjectID, controlPlaneCertStoreTypeServerCert): adminServerCert,
		controlPlaneCertStoreKey(s.certStorePrefix, agentObjectID, agentCertStoreTypeCA):                agentCA,
		controlPlaneCertStoreKey(s.certStorePrefix, agentObjectID, agentCertStoreTypeClientCert):        adminAgentClientCert,
	}

	for key, value := range values {
		if err := s.certStoreRepo.Put(ctx, key, value); err != nil {
			return fmt.Errorf("seed control-plane trust cert failed (%s): %w", key, err)
		}
	}
	return nil
}

func controlPlaneCertStoreObjectID(name string) uuid.UUID {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		normalized = "control-plane"
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("control-plane:"+normalized))
}

func controlPlaneCertStoreKey(prefix string, objectID uuid.UUID, certType string) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	key := objectID.String() + "-" + strings.TrimSpace(certType)
	if base == "" {
		return key
	}
	return base + "/" + key
}

func readAndValidateCertificatePEMFile(path string, label string) (string, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return "", fmt.Errorf("%s path is empty", strings.TrimSpace(label))
	}

	raw, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", fmt.Errorf("read %s failed: %w", strings.TrimSpace(label), err)
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "", fmt.Errorf("%s content is empty", strings.TrimSpace(label))
	}
	if err := validateCertificatePEM(trimmed); err != nil {
		return "", fmt.Errorf("%s is invalid pem cert: %w", strings.TrimSpace(label), err)
	}
	return trimmed, nil
}

func validateCertificatePEM(raw string) error {
	rest := []byte(strings.TrimSpace(raw))
	foundCert := false
	for len(rest) > 0 {
		block, next := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = next
		if block.Type != "CERTIFICATE" {
			continue
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return fmt.Errorf("parse cert failed: %w", err)
		}
		foundCert = true
	}
	if !foundCert {
		return fmt.Errorf("no certificate block found")
	}
	return nil
}
