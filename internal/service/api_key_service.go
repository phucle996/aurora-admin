package service

import (
	"admin/internal/config"
	"admin/pkg/apikeyhash"
	"admin/pkg/errorvar"
	time_util "admin/pkg/logger/time"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type APIKeyVersion struct {
	Version int64
	Key     string
}

type APIKeyBootstrapResult struct {
	Created  bool
	Current  APIKeyVersion
	Previous *APIKeyVersion
}

type APIKeyRotationResult struct {
	Old APIKeyVersion
	New APIKeyVersion
}

type APIKeyValidationResult struct {
	Version   APIKeyVersion
	IsCurrent bool
}

type APIKeyService struct {
	etcd           *clientv3.Client
	prefix         string
	rotateInterval time.Duration
	onBootstrap    func(context.Context, APIKeyBootstrapResult)
	onRotate       func(context.Context, APIKeyRotationResult)
}

func NewAPIKeyService(
	etcd *clientv3.Client,
	cfg config.APIKeyCfg,
	onBootstrap func(context.Context, APIKeyBootstrapResult),
	onRotate func(context.Context, APIKeyRotationResult),
) *APIKeyService {

	return &APIKeyService{
		etcd:           etcd,
		prefix:         cfg.Prefix,
		rotateInterval: cfg.RotateInterval,
		onBootstrap:    onBootstrap,
		onRotate:       onRotate,
	}
}

func (s *APIKeyService) BootstrapAPIKey(ctx context.Context) (*APIKeyBootstrapResult, error) {
	if s == nil || s.etcd == nil {
		return nil, errorvar.ErrAPIKeyServiceNil
	}

	currentVersion, exists, err := s.readCurrentVersion(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return &APIKeyBootstrapResult{
			Created: false,
			Current: APIKeyVersion{Version: currentVersion},
		}, nil
	}

	latestVersion, hasVersionKey, err := s.readLatestVersionFromKeys(ctx)
	if err != nil {
		return nil, err
	}
	if hasVersionKey {
		return &APIKeyBootstrapResult{
			Created: false,
			Current: APIKeyVersion{Version: latestVersion},
		}, nil
	}

	plainKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	hashedKey, err := apikeyhash.Hash(plainKey)
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
			clientv3.OpPut(s.currentRotatedAtTextKey(), time_util.FormatTimeLocal(now)),
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
	if exists {
		return &APIKeyBootstrapResult{
			Created: false,
			Current: APIKeyVersion{Version: currentVersion},
		}, nil
	}

	latestVersion, hasVersionKey, err = s.readLatestVersionFromKeys(ctx)
	if err != nil {
		return nil, err
	}
	if !hasVersionKey {
		return nil, errors.New("api key bootstrap failed: key not found after bootstrap race")
	}
	return &APIKeyBootstrapResult{
		Created: false,
		Current: APIKeyVersion{Version: latestVersion},
	}, nil
}

func (s *APIKeyService) RotateAPIKey(ctx context.Context, oldKey string) (*APIKeyRotationResult, error) {
	if s == nil || s.etcd == nil {
		return nil, errorvar.ErrAPIKeyServiceNil
	}
	oldKey = strings.TrimSpace(oldKey)
	if oldKey == "" {
		return nil, errorvar.ErrAPIKeyInvalid
	}

	currentVersion, exists, err := s.readCurrentVersion(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errorvar.ErrAPIKeyInvalid
	}

	currentStored, err := s.readVersionStored(ctx, currentVersion)
	if err != nil {
		return nil, err
	}
	if !apikeyhash.Compare(currentStored, oldKey) {
		return nil, errorvar.ErrAPIKeyMismatch
	}

	lastRotateAtUnix, err := s.readCurrentRotatedAt(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(time.Local)
	if lastRotateAtUnix > 0 {
		nextAllowed := time.Unix(lastRotateAtUnix, 0).Add(s.rotateInterval)
		if now.Before(nextAllowed) {
			return nil, fmt.Errorf("%w: next rotate at %s", errorvar.ErrAPIKeyRotateTooSoon, time_util.FormatTimeLocal(nextAllowed))
		}
	}

	newPlainKey, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	newHashedKey, err := apikeyhash.Hash(newPlainKey)
	if err != nil {
		return nil, err
	}

	nextVersion := currentVersion + 1
	nowUnix := strconv.FormatInt(now.Unix(), 10)

	tx := s.etcd.Txn(ctx).
		If(
			clientv3.Compare(clientv3.Value(s.currentVersionKey()), "=", strconv.FormatInt(currentVersion, 10)),
			clientv3.Compare(clientv3.Value(s.versionKey(currentVersion)), "=", currentStored),
		).
		Then(
			clientv3.OpPut(s.versionKey(nextVersion), newHashedKey),
			clientv3.OpPut(s.currentVersionKey(), strconv.FormatInt(nextVersion, 10)),
			clientv3.OpPut(s.currentRotatedAtKey(), nowUnix),
			clientv3.OpPut(s.currentRotatedAtTextKey(), time_util.FormatTimeLocal(now)),
		)

	// Keep only 2 latest versions (current + previous) for validation grace period.
	if currentVersion > 1 {
		tx = tx.Then(clientv3.OpDelete(s.versionKey(currentVersion - 1)))
	}

	txnResp, err := tx.Commit()
	if err != nil {
		return nil, err
	}
	if !txnResp.Succeeded {
		return nil, errorvar.ErrAPIKeyConflict
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
		return nil, errorvar.ErrAPIKeyServiceNil
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errorvar.ErrAPIKeyInvalid
	}

	currentVersion, exists, err := s.readCurrentVersion(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errorvar.ErrAPIKeyInvalid
	}

	currentStored, err := s.readVersionStored(ctx, currentVersion)
	if err != nil {
		return nil, err
	}
	if apikeyhash.Compare(currentStored, apiKey) {
		return &APIKeyValidationResult{
			Version:   APIKeyVersion{Version: currentVersion},
			IsCurrent: true,
		}, nil
	}

	if currentVersion > 1 {
		previousStored, err := s.readVersionStored(ctx, currentVersion-1)
		if err == nil && apikeyhash.Compare(previousStored, apiKey) {
			return &APIKeyValidationResult{
				Version:   APIKeyVersion{Version: currentVersion - 1},
				IsCurrent: false,
			}, nil
		}
	}

	return nil, errorvar.ErrAPIKeyMismatch
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
	resp, err := s.etcd.Get(ctx, s.versionKey(version))
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("api key value not found for version %d", version)
	}

	value := strings.TrimSpace(string(resp.Kvs[0].Value))
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
