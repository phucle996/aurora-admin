package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"golang.org/x/crypto/argon2"
)

const (
	defaultAPIKeyPrefix         = "/admin/apikey"
	defaultAPIKeyRotateInterval = 72 * time.Hour

	argon2Memory  = 64 * 1024
	argon2Time    = 3
	argon2Threads = 2
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

var (
	ErrAPIKeyServiceNil    = errors.New("api key service is nil")
	ErrAPIKeyInvalid       = errors.New("api key is invalid")
	ErrAPIKeyMismatch      = errors.New("provided api key does not match current api key")
	ErrAPIKeyConflict      = errors.New("api key rotate conflict")
	ErrAPIKeyRotateTooSoon = errors.New("api key rotate is allowed every configured interval")
)

type APIKeyServiceConfig struct {
	Prefix         string
	RotateInterval time.Duration
	OnBootstrap    func(ctx context.Context, result APIKeyBootstrapResult)
	OnRotate       func(ctx context.Context, result APIKeyRotationResult)
}

type APIKeyVersion struct {
	Version int64  `json:"version"`
	Key     string `json:"key"`
}

type APIKeyBootstrapResult struct {
	Created  bool           `json:"created"`
	Current  APIKeyVersion  `json:"current"`
	Previous *APIKeyVersion `json:"previous,omitempty"`
}

type APIKeyRotationResult struct {
	Old APIKeyVersion `json:"old"`
	New APIKeyVersion `json:"new"`
}

type APIKeyValidationResult struct {
	Version   APIKeyVersion `json:"version"`
	IsCurrent bool          `json:"is_current"`
}

type APIKeyService struct {
	etcd           *clientv3.Client
	prefix         string
	rotateInterval time.Duration
	onBootstrap    func(ctx context.Context, result APIKeyBootstrapResult)
	onRotate       func(ctx context.Context, result APIKeyRotationResult)
}

func NewAPIKeyService(etcd *clientv3.Client, cfg APIKeyServiceConfig) *APIKeyService {
	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix == "" {
		prefix = defaultAPIKeyPrefix
	}

	rotateInterval := cfg.RotateInterval
	if rotateInterval <= 0 {
		rotateInterval = defaultAPIKeyRotateInterval
	}

	return &APIKeyService{
		etcd:           etcd,
		prefix:         prefix,
		rotateInterval: rotateInterval,
		onBootstrap:    cfg.OnBootstrap,
		onRotate:       cfg.OnRotate,
	}
}

func (s *APIKeyService) BootstrapAPIKey(ctx context.Context) (*APIKeyBootstrapResult, error) {
	if s == nil || s.etcd == nil {
		return nil, ErrAPIKeyServiceNil
	}

	currentVersion, exists, err := s.readCurrentVersion(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return &APIKeyBootstrapResult{
			Created: false,
			Current: APIKeyVersion{
				Version: currentVersion,
				Key:     "",
			},
		}, nil
	}

	latestVersion, hasVersionKey, err := s.readLatestVersionFromKeys(ctx)
	if err != nil {
		return nil, err
	}
	if hasVersionKey {
		return &APIKeyBootstrapResult{
			Created: false,
			Current: APIKeyVersion{
				Version: latestVersion,
				Key:     "",
			},
		}, nil
	}

	plainKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	hashedKey, err := hashAPIKey(plainKey)
	if err != nil {
		return nil, err
	}

	now := time.Now().In(time.Local)
	nowUnix := now.Unix()
	txnResp, err := s.etcd.Txn(ctx).
		If(
			clientv3.Compare(clientv3.Version(s.currentVersionKey()), "=", 0),
			clientv3.Compare(clientv3.Version(s.versionKey(1)), "=", 0),
		).
		Then(
			clientv3.OpPut(s.versionKey(1), hashedKey),
			clientv3.OpPut(s.currentVersionKey(), "1"),
			clientv3.OpPut(s.currentRotatedAtKey(), strconv.FormatInt(nowUnix, 10)),
			clientv3.OpPut(s.currentRotatedAtTextKey(), formatTimeLocal(now)),
		).
		Commit()
	if err != nil {
		return nil, err
	}

	if txnResp.Succeeded {
		result := &APIKeyBootstrapResult{
			Created: true,
			Current: APIKeyVersion{
				Version: 1,
				Key:     plainKey,
			},
		}
		if s.onBootstrap != nil {
			s.onBootstrap(ctx, *result)
		}
		return result, nil
	}

	currentVersion, exists, err = s.readCurrentVersion(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		latestVersion, hasVersionKey, latestErr := s.readLatestVersionFromKeys(ctx)
		if latestErr != nil {
			return nil, latestErr
		}
		if !hasVersionKey {
			return nil, errors.New("api key bootstrap failed: key not found after bootstrap race")
		}
		return &APIKeyBootstrapResult{
			Created: false,
			Current: APIKeyVersion{
				Version: latestVersion,
				Key:     "",
			},
		}, nil
	}
	return &APIKeyBootstrapResult{
		Created: false,
		Current: APIKeyVersion{
			Version: currentVersion,
			Key:     "",
		},
	}, nil
}

func (s *APIKeyService) RotateAPIKey(ctx context.Context, oldKey string) (*APIKeyRotationResult, error) {
	if s == nil || s.etcd == nil {
		return nil, ErrAPIKeyServiceNil
	}
	oldKey = strings.TrimSpace(oldKey)
	if oldKey == "" {
		return nil, ErrAPIKeyInvalid
	}

	currentVersion, exists, err := s.readCurrentVersion(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrAPIKeyInvalid
	}

	currentStored, err := s.readVersionStored(ctx, currentVersion)
	if err != nil {
		return nil, err
	}
	if !compareStoredAPIKey(currentStored, oldKey) {
		return nil, ErrAPIKeyMismatch
	}

	lastRotateAt, err := s.readCurrentRotatedAt(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(time.Local)
	if lastRotateAt > 0 {
		nextAllowed := time.Unix(lastRotateAt, 0).Add(s.rotateInterval)
		if now.Before(nextAllowed) {
			return nil, fmt.Errorf("%w: next rotate at %s", ErrAPIKeyRotateTooSoon, formatTimeLocal(nextAllowed))
		}
	}

	newPlainKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	newHashedKey, err := hashAPIKey(newPlainKey)
	if err != nil {
		return nil, err
	}

	nextVersion := currentVersion + 1
	nextVersionStr := strconv.FormatInt(nextVersion, 10)
	nowUnix := strconv.FormatInt(now.Unix(), 10)

	tx := s.etcd.Txn(ctx).
		If(
			clientv3.Compare(clientv3.Value(s.currentVersionKey()), "=", strconv.FormatInt(currentVersion, 10)),
			clientv3.Compare(clientv3.Value(s.versionKey(currentVersion)), "=", currentStored),
		).
		Then(
			clientv3.OpPut(s.versionKey(nextVersion), newHashedKey),
			clientv3.OpPut(s.currentVersionKey(), nextVersionStr),
			clientv3.OpPut(s.currentRotatedAtKey(), nowUnix),
			clientv3.OpPut(s.currentRotatedAtTextKey(), formatTimeLocal(now)),
		)

	if currentVersion > 1 {
		tx = tx.Then(clientv3.OpDelete(s.versionKey(currentVersion - 1)))
	}

	txnResp, err := tx.Commit()
	if err != nil {
		return nil, err
	}
	if !txnResp.Succeeded {
		return nil, ErrAPIKeyConflict
	}

	result := &APIKeyRotationResult{
		Old: APIKeyVersion{
			Version: currentVersion,
			Key:     oldKey,
		},
		New: APIKeyVersion{
			Version: nextVersion,
			Key:     newPlainKey,
		},
	}
	if s.onRotate != nil {
		s.onRotate(ctx, *result)
	}
	return result, nil
}

func (s *APIKeyService) ValidateAPIKey(ctx context.Context, apiKey string) (*APIKeyValidationResult, error) {
	if s == nil || s.etcd == nil {
		return nil, ErrAPIKeyServiceNil
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, ErrAPIKeyInvalid
	}

	currentVersion, exists, err := s.readCurrentVersion(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrAPIKeyInvalid
	}

	currentStored, err := s.readVersionStored(ctx, currentVersion)
	if err != nil {
		return nil, err
	}
	if compareStoredAPIKey(currentStored, apiKey) {
		return &APIKeyValidationResult{
			Version: APIKeyVersion{
				Version: currentVersion,
			},
			IsCurrent: true,
		}, nil
	}

	if currentVersion > 1 {
		previousStored, err := s.readVersionStored(ctx, currentVersion-1)
		if err == nil && compareStoredAPIKey(previousStored, apiKey) {
			return &APIKeyValidationResult{
				Version: APIKeyVersion{
					Version: currentVersion - 1,
				},
				IsCurrent: false,
			}, nil
		}
	}

	return nil, ErrAPIKeyMismatch
}

func (s *APIKeyService) readCurrentVersion(ctx context.Context) (int64, bool, error) {
	resp, err := s.etcd.Get(ctx, s.currentVersionKey())
	if err != nil {
		return 0, false, err
	}
	if len(resp.Kvs) == 0 {
		return 0, false, nil
	}

	versionStr := strings.TrimSpace(string(resp.Kvs[0].Value))
	version, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil || version <= 0 {
		return 0, false, fmt.Errorf("invalid api key version in etcd: %q", versionStr)
	}
	return version, true, nil
}

func (s *APIKeyService) readCurrentRotatedAt(ctx context.Context) (int64, error) {
	resp, err := s.etcd.Get(ctx, s.currentRotatedAtKey())
	if err != nil {
		return 0, err
	}
	if len(resp.Kvs) == 0 {
		return 0, nil
	}
	raw := strings.TrimSpace(string(resp.Kvs[0].Value))
	if raw == "" {
		return 0, nil
	}
	unix, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid current rotate timestamp in etcd: %q", raw)
	}
	return unix, nil
}

func (s *APIKeyService) readVersionStored(ctx context.Context, version int64) (string, error) {
	keyResp, err := s.etcd.Get(ctx, s.versionKey(version))
	if err != nil {
		return "", err
	}
	if len(keyResp.Kvs) == 0 {
		return "", fmt.Errorf("api key value not found for version %d", version)
	}

	value := strings.TrimSpace(string(keyResp.Kvs[0].Value))
	if value == "" {
		return "", fmt.Errorf("api key value is empty for version %d", version)
	}
	return value, nil
}

func (s *APIKeyService) readLatestVersionFromKeys(ctx context.Context) (int64, bool, error) {
	resp, err := s.etcd.Get(
		ctx,
		s.versionPrefixKey(),
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend),
		clientv3.WithLimit(1),
	)
	if err != nil {
		return 0, false, err
	}
	if len(resp.Kvs) == 0 {
		return 0, false, nil
	}

	rawKey := strings.TrimSpace(string(resp.Kvs[0].Key))
	versionStr := strings.TrimPrefix(rawKey, s.versionPrefixKey())
	version, err := strconv.ParseInt(strings.TrimSpace(versionStr), 10, 64)
	if err != nil || version <= 0 {
		return 0, false, fmt.Errorf("invalid api key version key in etcd: %q", rawKey)
	}
	return version, true, nil
}

func (s *APIKeyService) currentVersionKey() string {
	return s.prefix + "/current_version"
}

func (s *APIKeyService) currentRotatedAtKey() string {
	return s.prefix + "/current_rotated_at"
}

func (s *APIKeyService) currentRotatedAtTextKey() string {
	return s.prefix + "/current_rotated_at_text"
}

func (s *APIKeyService) versionKey(version int64) string {
	return fmt.Sprintf("%s/v/%d", s.prefix, version)
}

func (s *APIKeyService) versionPrefixKey() string {
	return s.prefix + "/v/"
}

func generateAPIKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "ak_" + base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashAPIKey(apiKey string) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", ErrAPIKeyInvalid
	}

	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(apiKey),
		salt,
		argon2Time,
		argon2Memory,
		argon2Threads,
		argon2KeyLen,
	)

	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argon2Memory, argon2Time, argon2Threads, saltB64, hashB64), nil
}

func compareStoredAPIKey(stored, provided string) bool {
	stored = strings.TrimSpace(stored)
	provided = strings.TrimSpace(provided)
	if stored == "" || provided == "" {
		return false
	}

	// Backward compatible with old plain storage.
	if !strings.HasPrefix(stored, "$argon2id$") {
		return subtle.ConstantTimeCompare([]byte(stored), []byte(provided)) == 1
	}

	parts := strings.Split(stored, "$")
	if len(parts) != 6 {
		return false
	}
	if parts[1] != "argon2id" {
		return false
	}

	var memory uint32
	var timeCost uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	decodedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	computed := argon2.IDKey([]byte(provided), salt, timeCost, memory, threads, uint32(len(decodedHash)))
	return subtle.ConstantTimeCompare(computed, decodedHash) == 1
}
