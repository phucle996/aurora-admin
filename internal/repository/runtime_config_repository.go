package repository

import (
	keycfg "admin/internal/key"
	"admin/pkg/errorvar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type RuntimeConfigRepository interface {
	Get(ctx context.Context, key string) (string, bool, error)
	GetMany(ctx context.Context, keys []string) (map[string]string, error)
	ListByPrefix(ctx context.Context, prefix string) ([]RuntimeConfigKV, error)
	Upsert(ctx context.Context, key string, value string) error
	UpsertWithLease(ctx context.Context, key string, value string, ttlSeconds int64) (clientv3.LeaseID, error)
	KeepAliveLease(ctx context.Context, leaseID clientv3.LeaseID) error
	RevokeLease(ctx context.Context, leaseID clientv3.LeaseID) error
	Delete(ctx context.Context, key string) error
	ConsumeBootstrapTokenTx(ctx context.Context, rawToken string, expectedCluster string, now time.Time) (*BootstrapTokenConsumeResult, error)
}

type RuntimeConfigKV struct {
	Key   string
	Value string
}

type BootstrapTokenRecord struct {
	TokenHash    string `json:"token_hash"`
	ClusterScope string `json:"cluster_scope"`
	IssuedAt     string `json:"issued_at"`
	ExpiresAt    string `json:"expires_at"`
	UsedAt       string `json:"used_at"`
	MaxUse       int    `json:"max_use"`
}

type BootstrapTokenConsumeResult struct {
	Key      string
	Record   BootstrapTokenRecord
	Consumed bool
}

type EtcdRuntimeConfigRepository struct {
	etcd   *clientv3.Client
	prefix string
}

func NewEtcdRuntimeConfigRepository(etcd *clientv3.Client, prefix string) *EtcdRuntimeConfigRepository {
	return &EtcdRuntimeConfigRepository{
		etcd:   etcd,
		prefix: strings.TrimSpace(prefix),
	}
}

func (r *EtcdRuntimeConfigRepository) Get(ctx context.Context, key string) (string, bool, error) {
	if r == nil || r.etcd == nil {
		return "", false, errorvar.ErrRuntimeConfigRepositoryNil
	}

	cleanKey, fullKey, err := r.buildFullKey(key)
	if err != nil {
		return "", false, err
	}
	_ = cleanKey
	resp, err := r.etcd.Get(ctx, fullKey)
	if err != nil {
		return "", false, err
	}
	if len(resp.Kvs) == 0 {
		return "", false, nil
	}
	return strings.TrimSpace(string(resp.Kvs[0].Value)), true, nil
}

func (r *EtcdRuntimeConfigRepository) GetMany(ctx context.Context, keys []string) (map[string]string, error) {
	if r == nil || r.etcd == nil {
		return nil, errorvar.ErrRuntimeConfigRepositoryNil
	}
	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	cleanToFull := make(map[string]string, len(keys))
	cleanKeys := make([]string, 0, len(keys))
	ops := make([]clientv3.Op, 0, len(keys))
	for _, key := range keys {
		cleanKey, fullKey, err := r.buildFullKey(key)
		if err != nil {
			return nil, err
		}
		if _, seen := cleanToFull[cleanKey]; seen {
			continue
		}
		cleanToFull[cleanKey] = fullKey
		cleanKeys = append(cleanKeys, cleanKey)
		ops = append(ops, clientv3.OpGet(fullKey))
	}

	if len(ops) == 0 {
		return map[string]string{}, nil
	}

	txnResp, err := r.etcd.Txn(ctx).Then(ops...).Commit()
	if err != nil {
		return nil, err
	}

	out := make(map[string]string, len(cleanKeys))
	for i, cleanKey := range cleanKeys {
		if i >= len(txnResp.Responses) {
			break
		}
		resp := txnResp.Responses[i]
		if resp == nil || resp.GetResponseRange() == nil {
			continue
		}
		kvs := resp.GetResponseRange().GetKvs()
		if len(kvs) == 0 {
			continue
		}
		out[cleanKey] = strings.TrimSpace(string(kvs[0].Value))
	}
	return out, nil
}

func (r *EtcdRuntimeConfigRepository) Upsert(ctx context.Context, key string, value string) error {
	if r == nil || r.etcd == nil {
		return errorvar.ErrRuntimeConfigRepositoryNil
	}

	_, fullKey, err := r.buildFullKey(key)
	if err != nil {
		return err
	}
	_, err = r.etcd.Put(ctx, fullKey, strings.TrimSpace(value))
	return err
}

func (r *EtcdRuntimeConfigRepository) UpsertWithLease(
	ctx context.Context,
	key string,
	value string,
	ttlSeconds int64,
) (clientv3.LeaseID, error) {
	if r == nil || r.etcd == nil {
		return clientv3.NoLease, errorvar.ErrRuntimeConfigRepositoryNil
	}
	if ttlSeconds <= 0 {
		return clientv3.NoLease, errorvar.ErrRuntimeConfigKeyInvalid
	}

	_, fullKey, err := r.buildFullKey(key)
	if err != nil {
		return clientv3.NoLease, err
	}

	leaseResp, err := r.etcd.Grant(ctx, ttlSeconds)
	if err != nil {
		return clientv3.NoLease, err
	}
	if _, err := r.etcd.Put(
		ctx,
		fullKey,
		strings.TrimSpace(value),
		clientv3.WithLease(leaseResp.ID),
	); err != nil {
		return clientv3.NoLease, err
	}
	return leaseResp.ID, nil
}

func (r *EtcdRuntimeConfigRepository) KeepAliveLease(ctx context.Context, leaseID clientv3.LeaseID) error {
	if r == nil || r.etcd == nil {
		return errorvar.ErrRuntimeConfigRepositoryNil
	}
	if leaseID == clientv3.NoLease {
		return errorvar.ErrRuntimeConfigKeyInvalid
	}
	_, err := r.etcd.KeepAliveOnce(ctx, leaseID)
	return err
}

func (r *EtcdRuntimeConfigRepository) RevokeLease(ctx context.Context, leaseID clientv3.LeaseID) error {
	if r == nil || r.etcd == nil {
		return errorvar.ErrRuntimeConfigRepositoryNil
	}
	if leaseID == clientv3.NoLease {
		return errorvar.ErrRuntimeConfigKeyInvalid
	}
	_, err := r.etcd.Revoke(ctx, leaseID)
	return err
}

func (r *EtcdRuntimeConfigRepository) ListByPrefix(ctx context.Context, prefix string) ([]RuntimeConfigKV, error) {
	if r == nil || r.etcd == nil {
		return nil, errorvar.ErrRuntimeConfigRepositoryNil
	}

	cleanPrefix := strings.TrimSpace(prefix)
	if cleanPrefix == "" {
		return nil, errorvar.ErrRuntimeConfigKeyInvalid
	}

	fullPrefix := cleanPrefix
	if !strings.HasPrefix(cleanPrefix, "/") {
		trimmed := strings.Trim(cleanPrefix, "/")
		if trimmed == "" {
			return nil, errorvar.ErrRuntimeConfigKeyInvalid
		}
		prefixBase := strings.TrimRight(strings.TrimSpace(r.prefix), "/")
		if prefixBase == "" {
			return nil, errorvar.ErrRuntimeConfigKeyInvalid
		}
		fullPrefix = prefixBase + "/" + trimmed
	}

	resp, err := r.etcd.Get(
		ctx,
		fullPrefix,
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
	)
	if err != nil {
		return nil, err
	}

	items := make([]RuntimeConfigKV, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		if kv == nil {
			continue
		}
		items = append(items, RuntimeConfigKV{
			Key:   strings.TrimSpace(string(kv.Key)),
			Value: strings.TrimSpace(string(kv.Value)),
		})
	}
	return items, nil
}

func (r *EtcdRuntimeConfigRepository) Delete(ctx context.Context, key string) error {
	if r == nil || r.etcd == nil {
		return errorvar.ErrRuntimeConfigRepositoryNil
	}

	_, fullKey, err := r.buildFullKey(key)
	if err != nil {
		return err
	}
	_, err = r.etcd.Delete(ctx, fullKey)
	return err
}

func (r *EtcdRuntimeConfigRepository) ConsumeBootstrapTokenTx(
	ctx context.Context,
	rawToken string,
	expectedCluster string,
	now time.Time,
) (*BootstrapTokenConsumeResult, error) {
	if r == nil || r.etcd == nil {
		return nil, errorvar.ErrRuntimeConfigRepositoryNil
	}
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return nil, errorvar.ErrRuntimeConfigKeyInvalid
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	tokenKey := keycfg.RuntimeAgentBootstrapTokenKey(hash)
	_, fullTokenKey, err := r.buildFullKey(tokenKey)
	if err != nil {
		return nil, err
	}

	resp, err := r.etcd.Get(ctx, fullTokenKey)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	kv := resp.Kvs[0]
	if kv == nil {
		return nil, nil
	}

	var record BootstrapTokenRecord
	if err := json.Unmarshal(kv.Value, &record); err != nil {
		return nil, err
	}
	record.TokenHash = strings.TrimSpace(record.TokenHash)
	record.ClusterScope = strings.TrimSpace(record.ClusterScope)
	record.IssuedAt = strings.TrimSpace(record.IssuedAt)
	record.ExpiresAt = strings.TrimSpace(record.ExpiresAt)
	record.UsedAt = strings.TrimSpace(record.UsedAt)
	if record.TokenHash != "" && record.TokenHash != hash {
		return nil, nil
	}
	if record.MaxUse <= 0 {
		record.MaxUse = 1
	}
	clusterScope := strings.TrimSpace(record.ClusterScope)
	clusterID := strings.TrimSpace(expectedCluster)
	if clusterScope != "" && clusterScope != "*" && clusterID != "" && clusterScope != clusterID {
		return nil, nil
	}

	expiresAt, err := time.Parse(time.RFC3339Nano, record.ExpiresAt)
	if err != nil {
		return nil, err
	}
	if now.UTC().After(expiresAt.UTC()) {
		return nil, nil
	}
	if record.MaxUse <= 1 && record.UsedAt != "" {
		return nil, nil
	}

	record.UsedAt = now.UTC().Format(time.RFC3339Nano)
	updated, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}

	txnResp, err := r.etcd.Txn(ctx).
		If(
			clientv3.Compare(clientv3.ModRevision(fullTokenKey), "=", kv.ModRevision),
		).
		Then(clientv3.OpPut(fullTokenKey, strings.TrimSpace(string(updated)))).
		Commit()
	if err != nil {
		return nil, err
	}
	if !txnResp.Succeeded {
		return nil, nil
	}
	return &BootstrapTokenConsumeResult{
		Key:      tokenKey,
		Record:   record,
		Consumed: true,
	}, nil
}

func (r *EtcdRuntimeConfigRepository) buildFullKey(key string) (cleanKey string, fullKey string, err error) {
	cleanPrefix := strings.TrimRight(strings.TrimSpace(r.prefix), "/")
	trimmedKey := strings.TrimSpace(key)
	if trimmedKey == "" {
		return "", "", errorvar.ErrRuntimeConfigKeyInvalid
	}

	if strings.HasPrefix(trimmedKey, "/") {
		return trimmedKey, trimmedKey, nil
	}

	cleanKey = strings.Trim(trimmedKey, "/")
	if cleanPrefix == "" || cleanKey == "" {
		return "", "", errorvar.ErrRuntimeConfigKeyInvalid
	}
	return cleanKey, cleanPrefix + "/" + cleanKey, nil
}
