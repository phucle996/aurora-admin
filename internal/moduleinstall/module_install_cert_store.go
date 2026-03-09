package moduleinstall

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	certStoreTypeCA          = "ca"
	certStoreTypeClientCert  = "client_cert"
	certStoreTypePrivateCert = "private_client"
)

func moduleCertStoreObjectID(moduleName string) uuid.UUID {
	name := canonicalModuleName(moduleName)
	if name == "" {
		name = normalizeModuleName(moduleName)
	}
	if name == "" {
		name = "service"
	}
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("module-install:"+name))
}

func buildCertStoreKey(prefix string, objectID uuid.UUID, certType string) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	key := objectID.String() + "-" + strings.TrimSpace(certType)
	if base == "" {
		return key
	}
	return base + "/" + key
}

func moduleTLSCertStoreKeys(prefix string, moduleName string) []string {
	objectID := moduleCertStoreObjectID(moduleName)
	return []string{
		buildCertStoreKey(prefix, objectID, certStoreTypeCA),
		buildCertStoreKey(prefix, objectID, certStoreTypeClientCert),
		buildCertStoreKey(prefix, objectID, certStoreTypePrivateCert),
	}
}

func (s *ModuleInstallService) seedModuleTLSBundle(
	ctx context.Context,
	moduleName string,
	bundle *moduleTLSBundle,
) error {
	if s == nil || s.certStoreRepo == nil {
		return fmt.Errorf("cert store repository is nil")
	}
	if bundle == nil {
		return fmt.Errorf("tls bundle is nil")
	}

	objectID := moduleCertStoreObjectID(moduleName)
	values := map[string]string{
		buildCertStoreKey(s.certStorePrefix, objectID, certStoreTypeCA):          strings.TrimSpace(string(bundle.CAPEM)),
		buildCertStoreKey(s.certStorePrefix, objectID, certStoreTypeClientCert):  strings.TrimSpace(string(bundle.CertPEM)),
		buildCertStoreKey(s.certStorePrefix, objectID, certStoreTypePrivateCert): strings.TrimSpace(string(bundle.KeyPEM)),
	}

	for key, value := range values {
		if value == "" {
			return fmt.Errorf("tls bundle content is empty for key %s", key)
		}
		if err := s.certStoreRepo.Put(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (s *ModuleInstallService) addModuleTLSRollbackStep(stack *rollbackStack, moduleName string) {
	if stack == nil || s == nil || s.certStoreRepo == nil {
		return
	}
	keys := moduleTLSCertStoreKeys(s.certStorePrefix, moduleName)
	stack.Add("cert-store", func(rollbackCtx context.Context) error {
		for _, key := range keys {
			if strings.TrimSpace(key) == "" {
				continue
			}
			cleanupCtx, cancel := context.WithTimeout(rollbackCtx, 10*time.Second)
			err := s.certStoreRepo.Delete(cleanupCtx, key)
			cancel()
			if err != nil {
				return fmt.Errorf("cert store cleanup failed (%s): %w", key, err)
			}
		}
		return nil
	})
}
