package repository

import (
	"admin/pkg/errorvar"
	"context"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type RuntimeConfigRepository interface {
	Upsert(ctx context.Context, key string, value string) error
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

func (r *EtcdRuntimeConfigRepository) Upsert(ctx context.Context, key string, value string) error {
	if r == nil || r.etcd == nil {
		return errorvar.ErrRuntimeConfigRepositoryNil
	}

	cleanPrefix := strings.TrimRight(strings.TrimSpace(r.prefix), "/")
	cleanKey := strings.Trim(strings.TrimSpace(key), "/")
	if cleanPrefix == "" || cleanKey == "" {
		return errorvar.ErrRuntimeConfigKeyInvalid
	}

	fullKey := cleanPrefix + "/" + cleanKey
	_, err := r.etcd.Put(ctx, fullKey, strings.TrimSpace(value))
	return err
}
