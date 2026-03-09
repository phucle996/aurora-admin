package repository

import (
	"admin/pkg/errorvar"
	"context"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type RuntimeConfigRepository interface {
	Get(ctx context.Context, key string) (string, bool, error)
	GetMany(ctx context.Context, keys []string) (map[string]string, error)
	ListByPrefix(ctx context.Context, prefix string) ([]RuntimeConfigKV, error)
	Upsert(ctx context.Context, key string, value string) error
	Delete(ctx context.Context, key string) error
}

type RuntimeConfigKV struct {
	Key   string
	Value string
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
