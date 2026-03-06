package service

import (
	"admin/internal/config"
	keycfg "admin/internal/key"
	"admin/internal/repository"
	"admin/pkg/errorvar"
	time_util "admin/pkg/logger/time"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	defaultTokenSecretPrefix = keycfg.TokenSecretLegacyPrefix

	defaultTokenSecretRotateCheckInterval = time.Hour
)

type TokenSecretKind string

const (
	TokenSecretAccessJWT  TokenSecretKind = "access_jwt"
	TokenSecretRefreshJWT TokenSecretKind = "refresh_jwt"
	TokenSecretDevice     TokenSecretKind = "device_token"
)

type TokenSecretVersion struct {
	Version       int64
	Secret        string
	RotatedAtUnix int64
	RotatedAt     string
}

type TokenSecretPair struct {
	Kind                  TokenSecretKind
	Current               TokenSecretVersion
	Previous              *TokenSecretVersion
	RotateIntervalSeconds int64
	NextRotateAtUnix      int64
}

type TokenSecretService struct {
	etcd                  *clientv3.Client
	cacheRepo             repository.TokenSecretCacheRepository
	prefix                string
	accessRotateInterval  time.Duration
	refreshRotateInterval time.Duration
	deviceRotateInterval  time.Duration
}

type tokenSecretRecord struct {
	Secret        string `json:"secret"`
	RotatedAtUnix int64  `json:"rotated_at_unix"`
	RotatedAt     string `json:"rotated_at"`
}

func NewTokenSecretService(
	etcd *clientv3.Client,
	cfg config.TokenSecretCfg,
	cacheRepo repository.TokenSecretCacheRepository,
) *TokenSecretService {
	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix == "" {
		prefix = defaultTokenSecretPrefix
	}

	accessInterval := cfg.AccessRotateInterval

	refreshInterval := cfg.RefreshRotateInterval

	deviceInterval := cfg.DeviceRotateInterval

	return &TokenSecretService{
		etcd:                  etcd,
		cacheRepo:             cacheRepo,
		prefix:                prefix,
		accessRotateInterval:  accessInterval,
		refreshRotateInterval: refreshInterval,
		deviceRotateInterval:  deviceInterval,
	}
}

func ParseTokenSecretKind(raw string) (TokenSecretKind, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "access", "access_jwt", "access_token", "jwt_access":
		return TokenSecretAccessJWT, nil
	case "refresh", "refresh_jwt", "refresh_token", "jwt_refresh":
		return TokenSecretRefreshJWT, nil
	case "device", "device_token", "device_jwt":
		return TokenSecretDevice, nil
	default:
		return "", errorvar.ErrTokenSecretKindInvalid
	}
}

func (s *TokenSecretService) Bootstrap(ctx context.Context) error {
	if s == nil || s.etcd == nil {
		return errorvar.ErrTokenSecretServiceNil
	}
	for _, kind := range s.supportedKinds() {
		if err := s.ensureInitialized(ctx, kind); err != nil {
			return err
		}
		if err := s.syncKindToCache(ctx, kind); err != nil {
			return err
		}
	}
	return nil
}

func (s *TokenSecretService) RotateDueSecrets(ctx context.Context) error {
	if s == nil || s.etcd == nil {
		return errorvar.ErrTokenSecretServiceNil
	}
	for _, kind := range s.supportedKinds() {
		if err := s.rotateIfDue(ctx, kind); err != nil {
			return err
		}
		if err := s.syncKindToCache(ctx, kind); err != nil {
			return err
		}
	}
	return nil
}

func (s *TokenSecretService) StartAutoRotate(
	ctx context.Context,
	checkInterval time.Duration,
	onError func(error),
) {
	if s == nil || s.etcd == nil {
		if onError != nil {
			onError(errorvar.ErrTokenSecretServiceNil)
		}
		return
	}
	if checkInterval <= 0 {
		checkInterval = defaultTokenSecretRotateCheckInterval
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rotateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := s.RotateDueSecrets(rotateCtx)
			cancel()
			if err != nil && onError != nil {
				onError(err)
			}
		}
	}
}

func (s *TokenSecretService) GetSecretPair(
	ctx context.Context,
	kind TokenSecretKind,
) (*TokenSecretPair, error) {
	if s == nil || s.etcd == nil {
		return nil, errorvar.ErrTokenSecretServiceNil
	}
	if !s.isSupportedKind(kind) {
		return nil, errorvar.ErrTokenSecretKindInvalid
	}

	if err := s.ensureInitialized(ctx, kind); err != nil {
		return nil, err
	}
	if err := s.rotateIfDue(ctx, kind); err != nil {
		return nil, err
	}
	if err := s.syncKindToCache(ctx, kind); err != nil {
		return nil, err
	}

	currentVersion, exists, err := s.readCurrentVersion(ctx, kind)
	if err != nil {
		return nil, err
	}
	if !exists || currentVersion <= 0 {
		return nil, errorvar.ErrTokenSecretKindInvalid
	}

	currentRecord, _, err := s.readVersionRecord(ctx, kind, currentVersion)
	if err != nil {
		return nil, err
	}

	pair := &TokenSecretPair{
		Kind: kind,
		Current: TokenSecretVersion{
			Version:       currentVersion,
			Secret:        currentRecord.Secret,
			RotatedAtUnix: currentRecord.RotatedAtUnix,
			RotatedAt:     currentRecord.RotatedAt,
		},
		RotateIntervalSeconds: int64(s.rotateIntervalFor(kind) / time.Second),
	}

	if currentRecord.RotatedAtUnix > 0 {
		nextAt := time.Unix(currentRecord.RotatedAtUnix, 0).Add(s.rotateIntervalFor(kind))
		pair.NextRotateAtUnix = nextAt.Unix()
	}

	if currentVersion > 1 {
		previousRecord, _, err := s.readVersionRecord(ctx, kind, currentVersion-1)
		if err == nil {
			pair.Previous = &TokenSecretVersion{
				Version:       currentVersion - 1,
				Secret:        previousRecord.Secret,
				RotatedAtUnix: previousRecord.RotatedAtUnix,
				RotatedAt:     previousRecord.RotatedAt,
			}
		}
	}

	return pair, nil
}

func (s *TokenSecretService) GetAllSecretPairs(ctx context.Context) ([]TokenSecretPair, error) {
	if s == nil || s.etcd == nil {
		return nil, errorvar.ErrTokenSecretServiceNil
	}
	kinds := s.supportedKinds()
	out := make([]TokenSecretPair, 0, len(kinds))

	for _, kind := range kinds {
		pair, err := s.GetSecretPair(ctx, kind)
		if err != nil {
			return nil, err
		}
		out = append(out, *pair)
	}

	return out, nil
}

func (s *TokenSecretService) ensureInitialized(ctx context.Context, kind TokenSecretKind) error {
	currentVersion, exists, err := s.readCurrentVersion(ctx, kind)
	if err != nil {
		return err
	}
	if exists && currentVersion > 0 {
		return nil
	}

	now := time.Now().In(time.Local)
	nowUnix := now.Unix()
	secret, err := generateTokenSecret()
	if err != nil {
		return err
	}
	recordRaw, err := marshalTokenSecretRecord(tokenSecretRecord{
		Secret:        secret,
		RotatedAtUnix: nowUnix,
		RotatedAt:     time_util.FormatTimeLocal(now),
	})
	if err != nil {
		return err
	}

	txnResp, err := s.etcd.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(s.currentVersionKey(kind)), "=", 0)).
		Then(
			clientv3.OpPut(s.versionKey(kind, 1), recordRaw),
			clientv3.OpPut(s.currentVersionKey(kind), "1"),
		).
		Commit()
	if err != nil {
		return err
	}
	if txnResp.Succeeded {
		return nil
	}

	_, exists, err = s.readCurrentVersion(ctx, kind)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return errors.New("bootstrap token secret failed")
}

func (s *TokenSecretService) rotateIfDue(ctx context.Context, kind TokenSecretKind) error {
	if err := s.ensureInitialized(ctx, kind); err != nil {
		return err
	}

	interval := s.rotateIntervalFor(kind)
	now := time.Now().In(time.Local)

	for range 3 {
		currentVersion, exists, err := s.readCurrentVersion(ctx, kind)
		if err != nil {
			return err
		}
		if !exists || currentVersion <= 0 {
			if err := s.ensureInitialized(ctx, kind); err != nil {
				return err
			}
			continue
		}

		currentRecord, currentRaw, err := s.readVersionRecord(ctx, kind, currentVersion)
		if err != nil {
			return err
		}

		if currentRecord.RotatedAtUnix > 0 {
			nextAllowed := time.Unix(currentRecord.RotatedAtUnix, 0).Add(interval)
			if now.Before(nextAllowed) {
				return nil
			}
		}

		newSecret, err := generateTokenSecret()
		if err != nil {
			return err
		}
		newVersion := currentVersion + 1
		newRaw, err := marshalTokenSecretRecord(tokenSecretRecord{
			Secret:        newSecret,
			RotatedAtUnix: now.Unix(),
			RotatedAt:     time_util.FormatTimeLocal(now),
		})
		if err != nil {
			return err
		}

		tx := s.etcd.Txn(ctx).
			If(
				clientv3.Compare(clientv3.Value(s.currentVersionKey(kind)), "=", strconv.FormatInt(currentVersion, 10)),
				clientv3.Compare(clientv3.Value(s.versionKey(kind, currentVersion)), "=", currentRaw),
			).
			Then(
				clientv3.OpPut(s.versionKey(kind, newVersion), newRaw),
				clientv3.OpPut(s.currentVersionKey(kind), strconv.FormatInt(newVersion, 10)),
			)
		if currentVersion > 1 {
			tx = tx.Then(clientv3.OpDelete(s.versionKey(kind, currentVersion-1)))
		}

		txnResp, err := tx.Commit()
		if err != nil {
			return err
		}
		if txnResp.Succeeded {
			return nil
		}
	}

	return errorvar.ErrTokenSecretConflict
}

func (s *TokenSecretService) readCurrentVersion(
	ctx context.Context,
	kind TokenSecretKind,
) (int64, bool, error) {
	resp, err := s.etcd.Get(ctx, s.currentVersionKey(kind))
	if err != nil {
		return 0, false, err
	}
	if len(resp.Kvs) == 0 {
		return 0, false, nil
	}

	raw := strings.TrimSpace(string(resp.Kvs[0].Value))
	if raw == "" {
		return 0, false, nil
	}

	version, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || version <= 0 {
		return 0, false, fmt.Errorf("invalid token secret version for kind %s: %q", kind, raw)
	}
	return version, true, nil
}

func (s *TokenSecretService) syncKindToCache(ctx context.Context, kind TokenSecretKind) error {
	if s == nil || s.cacheRepo == nil {
		return nil
	}

	currentVersion, exists, err := s.readCurrentVersion(ctx, kind)
	if err != nil {
		return err
	}
	if !exists || currentVersion <= 0 {
		return fmt.Errorf("missing current version for kind=%s", kind)
	}

	record, _, err := s.readVersionRecord(ctx, kind, currentVersion)
	if err != nil {
		return err
	}

	if err := s.cacheRepo.Upsert(ctx, repository.TokenSecretCacheRecord{
		Kind:          string(kind),
		Version:       currentVersion,
		Secret:        record.Secret,
		RotatedAtUnix: record.RotatedAtUnix,
	}); err != nil {
		return err
	}

	if err := s.cacheRepo.PublishInvalidate(ctx, string(kind), currentVersion); err != nil {
		return err
	}
	return nil
}

func (s *TokenSecretService) readVersionRecord(
	ctx context.Context,
	kind TokenSecretKind,
	version int64,
) (tokenSecretRecord, string, error) {
	keyResp, err := s.etcd.Get(ctx, s.versionKey(kind, version))
	if err != nil {
		return tokenSecretRecord{}, "", err
	}
	if len(keyResp.Kvs) == 0 {
		return tokenSecretRecord{}, "", fmt.Errorf("token secret value not found for kind=%s version=%d", kind, version)
	}

	raw := strings.TrimSpace(string(keyResp.Kvs[0].Value))
	if raw == "" {
		return tokenSecretRecord{}, "", fmt.Errorf("token secret value is empty for kind=%s version=%d", kind, version)
	}

	record, err := unmarshalTokenSecretRecord(raw)
	if err != nil {
		return tokenSecretRecord{}, "", err
	}
	return record, raw, nil
}

func (s *TokenSecretService) currentVersionKey(kind TokenSecretKind) string {
	return keycfg.TokenSecretCurrentVersionKey(s.prefix, string(kind))
}

func (s *TokenSecretService) versionKey(kind TokenSecretKind, version int64) string {
	return keycfg.TokenSecretVersionKey(s.prefix, string(kind), version)
}

func (s *TokenSecretService) rotateIntervalFor(kind TokenSecretKind) time.Duration {
	switch kind {
	case TokenSecretAccessJWT:
		return s.accessRotateInterval
	case TokenSecretRefreshJWT:
		return s.refreshRotateInterval
	case TokenSecretDevice:
		return s.deviceRotateInterval
	default:
		return s.accessRotateInterval
	}
}

func (s *TokenSecretService) supportedKinds() []TokenSecretKind {
	return []TokenSecretKind{
		TokenSecretAccessJWT,
		TokenSecretRefreshJWT,
		TokenSecretDevice,
	}
}

func (s *TokenSecretService) isSupportedKind(kind TokenSecretKind) bool {
	switch kind {
	case TokenSecretAccessJWT, TokenSecretRefreshJWT, TokenSecretDevice:
		return true
	default:
		return false
	}
}

func marshalTokenSecretRecord(rec tokenSecretRecord) (string, error) {
	payload, err := json.Marshal(rec)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func unmarshalTokenSecretRecord(raw string) (tokenSecretRecord, error) {
	var record tokenSecretRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return tokenSecretRecord{}, err
	}
	record.Secret = strings.TrimSpace(record.Secret)
	if record.Secret == "" {
		return tokenSecretRecord{}, errors.New("token secret is empty")
	}
	record.RotatedAt = strings.TrimSpace(record.RotatedAt)
	if record.RotatedAt == "" && record.RotatedAtUnix > 0 {
		record.RotatedAt = time_util.FormatTimeLocal(time.Unix(record.RotatedAtUnix, 0))
	}
	return record, nil
}

func generateTokenSecret() (string, error) {
	buf := make([]byte, 48)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "ts_" + base64.RawURLEncoding.EncodeToString(buf), nil
}
