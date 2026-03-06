package moduleinstall

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

const (
	defaultTLSBaseDir       = "/etc/aurora/certs"
	defaultTLSCAKeyPath     = "/etc/aurora/certs/ca.key"
	defaultAdminTLSCAPath   = "/etc/aurora/certs/ca.crt"
	defaultAdminTLSCertPath = "/etc/aurora/certs/admin.crt"
	defaultAdminTLSKeyPath  = "/etc/aurora/certs/admin.key"
)

type moduleTLSPaths struct {
	CertPath string
	KeyPath  string
	CAPath   string
}

type moduleTLSBundle struct {
	CertPEM []byte
	KeyPEM  []byte
	CAPEM   []byte
}

func resolveModuleTLSPaths(moduleName string) moduleTLSPaths {
	name := strings.TrimSpace(strings.ToLower(moduleName))
	if name == "" {
		name = "service"
	}
	return moduleTLSPaths{
		CertPath: defaultTLSBaseDir + "/" + name + ".crt",
		KeyPath:  defaultTLSBaseDir + "/" + name + ".key",
		CAPath:   defaultTLSBaseDir + "/ca.crt",
	}
}

func installModuleTLSOnTarget(
	ctx context.Context,
	target moduleInstallTarget,
	moduleName string,
	appHost string,
	endpoint string,
	logFn InstallLogFn,
) error {
	paths := resolveModuleTLSPaths(moduleName)
	host := resolveTLSHost(endpoint, appHost)
	if host == "" {
		host = "localhost"
	}

	logInstall(logFn, "tls", "generate tls cert host=%s module=%s (signed by admin CA)", host, moduleName)
	bundle, err := generateModuleTLSBundle(host, moduleName)
	if err != nil {
		return err
	}

	certB64 := base64.StdEncoding.EncodeToString(bundle.CertPEM)
	keyB64 := base64.StdEncoding.EncodeToString(bundle.KeyPEM)
	caB64 := base64.StdEncoding.EncodeToString(bundle.CAPEM)

	ownerUser := "aurora"
	ownerGroup := "aurora"
	script := strings.Join([]string{
		"set -e",
		`ensure_root(){ if [ "$(id -u)" -eq 0 ]; then "$@"; return; fi; if command -v sudo >/dev/null 2>&1; then sudo "$@"; return; fi; echo "need root or sudo" >&2; exit 1; }`,
		`if ! id -u ` + shellEscape(ownerUser) + ` >/dev/null 2>&1; then owner_user="root"; owner_group="root"; else owner_user=` + shellEscape(ownerUser) + `; owner_group="$(id -gn ` + ownerUser + ` 2>/dev/null || echo ` + ownerGroup + `)"; fi`,
		"write_file(){",
		`  path="$1"`,
		`  payload="$2"`,
		`  tmp="$(mktemp)"`,
		`  printf '%s' "$payload" | base64 -d > "$tmp"`,
		`  ensure_root mkdir -p "$(dirname "$path")"`,
		`  ensure_root install -m 0400 -o "$owner_user" -g "$owner_group" "$tmp" "$path"`,
		`  rm -f "$tmp"`,
		"}",
		"write_file " + shellEscape(paths.CertPath) + " " + shellEscape(certB64),
		"write_file " + shellEscape(paths.KeyPath) + " " + shellEscape(keyB64),
		"write_file " + shellEscape(paths.CAPath) + " " + shellEscape(caB64),
		`echo "tls_materials_installed"`,
	}, "\n")

	output, exitCode, runErr := runCommandOnTarget(ctx, target, script, 60*time.Second, func(line string) {
		logInstall(logFn, "tls", "%s", line)
	}, func(line string) {
		logInstall(logFn, "tls", "%s", line)
	})
	if runErr != nil {
		return fmt.Errorf("push tls materials failed (exit_code=%d): %w", exitCode, runErr)
	}
	if strings.TrimSpace(output) == "" {
		logInstall(logFn, "tls", "tls material command completed")
	}

	logInstall(logFn, "tls", "tls installed cert=%s key=%s ca=%s", paths.CertPath, paths.KeyPath, paths.CAPath)
	return nil
}

func generateModuleTLSBundle(host string, moduleName string) (*moduleTLSBundle, error) {
	caCert, caKey, caPEM, err := loadSigningCA()
	if err != nil {
		return nil, err
	}

	serviceKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	dnsNames := []string{"localhost"}
	host = strings.TrimSpace(host)
	if host != "" && net.ParseIP(host) == nil {
		dnsNames = append(dnsNames, host)
	}

	serviceTemplate := &x509.Certificate{
		SerialNumber: randomSerial(),
		Subject: pkix.Name{
			CommonName: strings.TrimSpace(host),
		},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(825 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     dnsNames,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		SubjectKeyId: []byte(moduleName),
	}
	if ip := net.ParseIP(host); ip != nil {
		serviceTemplate.IPAddresses = append(serviceTemplate.IPAddresses, ip)
	}

	der, err := x509.CreateCertificate(
		rand.Reader,
		serviceTemplate,
		caCert,
		&serviceKey.PublicKey,
		caKey,
	)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serviceKey)})

	return &moduleTLSBundle{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		CAPEM:   caPEM,
	}, nil
}

func loadSigningCA() (*x509.Certificate, crypto.PrivateKey, []byte, error) {
	caPEM, err := os.ReadFile(defaultTLSBaseDir + "/ca.crt")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read admin ca cert failed: %w", err)
	}
	certBlock, _ := pem.Decode(caPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, nil, nil, fmt.Errorf("invalid admin ca cert pem")
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse admin ca cert failed: %w", err)
	}

	keyPEM, err := os.ReadFile(defaultTLSCAKeyPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read admin ca key failed: %w", err)
	}
	key, err := parsePrivateKeyPEM(keyPEM)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse admin ca key failed: %w", err)
	}
	return caCert, key, caPEM, nil
}

func parsePrivateKeyPEM(keyPEM []byte) (crypto.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, errors.New("invalid private key pem")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	if keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if key, ok := keyAny.(*rsa.PrivateKey); ok {
			return key, nil
		}
		return nil, errors.New("unsupported private key type")
	}
	return nil, errors.New("unsupported private key format")
}

func resolveTLSHost(endpoint, appHost string) string {
	host := endpointHost(endpoint)
	if host != "" {
		return host
	}
	return normalizeAddress(appHost)
}

func randomSerial() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	if serial.Sign() <= 0 {
		return big.NewInt(time.Now().UnixNano())
	}
	return serial
}
