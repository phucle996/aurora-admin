package service

import (
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
	defaultTokenSecretPrefix              = "/admin/token-secret"
	defaultAccessTokenRotateInterval      = 72 * time.Hour
	defaultRefreshTokenRotateInterval     = 7 * 24 * time.Hour
	defaultDeviceTokenRotateInterval      = 14 * 24 * time.Hour
	defaultTokenSecretRotateCheckInterval = time.Hour
)

var (
	ErrTokenSecretServiceNil  = errors.New("token secret service is nil")
	ErrTokenSecretKindInvalid = errors.New("token secret kind is invalid")
	ErrTokenSecretConflict    = errors.New("token secret rotate conflict")
)

type TokenSecretKind string

const (
	TokenSecretAccessJWT  TokenSecretKind = "access_jwt"
	TokenSecretRefreshJWT TokenSecretKind = "refresh_jwt"
	TokenSecretDevice     TokenSecretKind = "device_token"
)

type TokenSecretServiceConfig struct {
	Prefix                string
	AccessRotateInterval  time.Duration
	RefreshRotateInterval time.Duration
	DeviceRotateInterval  time.Duration
}

type TokenSecretVersion struct {
	Version       int64  `json:"version"`
	Secret        string `json:"secret"`
	RotatedAtUnix int64  `json:"rotated_at_unix"`
	RotatedAt     string `json:"rotated_at"`
}

type TokenSecretPair struct {
	Kind                  TokenSecretKind     `json:"kind"`
	Current               TokenSecretVersion  `json:"current"`
	Previous              *TokenSecretVersion `json:"previous,omitempty"`
	RotateIntervalSeconds int64               `json:"rotate_interval_seconds"`
	NextRotateAtUnix      int64               `json:"next_rotate_at_unix"`
}

type TokenSecretService struct {
	etcd                  *clientv3.Client
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

func NewTokenSecretService(etcd *clientv3.Client, cfg TokenSecretServiceConfig) *TokenSecretService {
	prefix := strings.TrimSpace(cfg.Prefix)
	if prefix == "" {
		prefix = defaultTokenSecretPrefix
	}

	accessInterval := cfg.AccessRotateInterval
	if accessInterval <= 0 {
		accessInterval = defaultAccessTokenRotateInterval
	}
	refreshInterval := cfg.RefreshRotateInterval
	if refreshInterval <= 0 {
		refreshInterval = defaultRefreshTokenRotateInterval
	}
	deviceInterval := cfg.DeviceRotateInterval
	if deviceInterval <= 0 {
		deviceInterval = defaultDeviceTokenRotateInterval
	}

	return &TokenSecretService{
		etcd:                  etcd,
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
		return "", ErrTokenSecretKindInvalid
	}
}

func (s *TokenSecretService) Bootstrap(ctx context.Context) error {
	if s == nil || s.etcd == nil {
		return ErrTokenSecretServiceNil
	}
	for _, kind := range s.supportedKinds() {
		if err := s.ensureInitialized(ctx, kind); err != nil {
			return err
		}
	}
	return nil
}

func (s *TokenSecretService) RotateDueSecrets(ctx context.Context) error {
	if s == nil || s.etcd == nil {
		return ErrTokenSecretServiceNil
	}
	for _, kind := range s.supportedKinds() {
		if err := s.rotateIfDue(ctx, kind); err != nil {
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
			onError(ErrTokenSecretServiceNil)
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
		return nil, ErrTokenSecretServiceNil
	}
	if !s.isSupportedKind(kind) {
		return nil, ErrTokenSecretKindInvalid
	}

	if err := s.ensureInitialized(ctx, kind); err != nil {
		return nil, err
	}
	if err := s.rotateIfDue(ctx, kind); err != nil {
		return nil, err
	}

	currentVersion, exists, err := s.readCurrentVersion(ctx, kind)
	if err != nil {
		return nil, err
	}
	if !exists || currentVersion <= 0 {
		return nil, ErrTokenSecretKindInvalid
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
		return nil, ErrTokenSecretServiceNil
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
		RotatedAt:     formatTimeLocal(now),
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
			RotatedAt:     formatTimeLocal(now),
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

	return ErrTokenSecretConflict
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
	return fmt.Sprintf("%s/%s/current_version", s.prefix, kind)
}

func (s *TokenSecretService) versionKey(kind TokenSecretKind, version int64) string {
	return fmt.Sprintf("%s/%s/v/%d", s.prefix, kind, version)
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
		return defaultAccessTokenRotateInterval
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
		record.RotatedAt = formatTimeLocal(time.Unix(record.RotatedAtUnix, 0))
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
