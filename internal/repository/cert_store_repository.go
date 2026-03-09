package repository

import (
	"admin/pkg/errorvar"
	"context"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type CertStoreRepository interface {
	Put(ctx context.Context, key string, value string) error
	Delete(ctx context.Context, key string) error
	GetMany(ctx context.Context, keys []string) (map[string]string, error)
}

type EtcdCertStoreRepository struct {
	etcd *clientv3.Client
}

func NewEtcdCertStoreRepository(etcd *clientv3.Client) *EtcdCertStoreRepository {
	return &EtcdCertStoreRepository{etcd: etcd}
}

func (r *EtcdCertStoreRepository) Put(ctx context.Context, key string, value string) error {
	if r == nil || r.etcd == nil {
		return errorvar.ErrCertStoreRepositoryNil
	}
	_, err := r.etcd.Put(ctx, key, value)
	return err
}

func (r *EtcdCertStoreRepository) Delete(ctx context.Context, key string) error {
	if r == nil || r.etcd == nil {
		return errorvar.ErrCertStoreRepositoryNil
	}
	_, err := r.etcd.Delete(ctx, key)
	return err
}

func (r *EtcdCertStoreRepository) GetMany(ctx context.Context, keys []string) (map[string]string, error) {
	if r == nil || r.etcd == nil {
		return nil, errorvar.ErrCertStoreRepositoryNil
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		res, err := r.etcd.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if len(res.Kvs) == 0 {
			continue
		}
		out[key] = string(res.Kvs[0].Value)
	}
	return out, nil
}
