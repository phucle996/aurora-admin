package service

import (
	"admin/internal/repository"
	"admin/pkg/errorvar"
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var certTypePattern = regexp.MustCompile(`^[a-z0-9_]{2,64}$`)

type CertStoreServiceConfig struct {
	Prefix string
}

type CertStoreService struct {
	repo   repository.CertStoreRepository
	prefix string
}

func NewCertStoreService(repo repository.CertStoreRepository, cfg CertStoreServiceConfig) *CertStoreService {
	return &CertStoreService{
		repo:   repo,
		prefix: strings.TrimSpace(cfg.Prefix),
	}
}

func (s *CertStoreService) UploadCert(
	ctx context.Context,
	objectID uuid.UUID,
	certType string,
	content string,
) (string, error) {
	if s == nil || s.repo == nil {
		return "", errorvar.ErrCertStoreServiceNil
	}
	if objectID == uuid.Nil {
		return "", errorvar.ErrObjectIDInvalid
	}

	normalizedType, err := normalizeCertType(certType)
	if err != nil {
		return "", err
	}

	trimmedContent := strings.TrimSpace(content)
	rawKey := fmt.Sprintf("%s-%s", objectID.String(), normalizedType)
	key := rawKey
	if s.prefix != "" {
		key = strings.TrimRight(s.prefix, "/") + "/" + rawKey
	}

	// Empty content is treated as delete operation to support rollback.
	if trimmedContent == "" {
		if err := s.repo.Delete(ctx, key); err != nil {
			return "", err
		}
		return key, nil
	}

	if err := s.repo.Put(ctx, key, trimmedContent); err != nil {
		return "", err
	}
	return key, nil
}

func normalizeCertType(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	switch normalized {
	case "ca", "ca_cert", "ca_certificate", "certificate_authority":
		normalized = "ca"
	case "client_cert", "client_certificate", "cert_client", "public_client":
		normalized = "client_cert"
	case "private_client", "client_key", "private_key", "key_client":
		normalized = "private_client"
	}

	if !certTypePattern.MatchString(normalized) {
		return "", errorvar.ErrCertTypeInvalid
	}
	return normalized, nil
}
