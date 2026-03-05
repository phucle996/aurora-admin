package repository

import (
	"admin/pkg/errorvar"
	"context"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EndpointKV struct {
	Name  string
	Value string
}

type EndpointRepository interface {
	List(ctx context.Context) ([]EndpointKV, error)
}

type EtcdEndpointRepository struct {
	etcd   *clientv3.Client
	prefix string
}

func NewEtcdEndpointRepository(etcd *clientv3.Client, prefix string) *EtcdEndpointRepository {
	return &EtcdEndpointRepository{
		etcd:   etcd,
		prefix: strings.TrimSpace(prefix),
	}
}

func (r *EtcdEndpointRepository) List(ctx context.Context) ([]EndpointKV, error) {
	if r == nil || r.etcd == nil {
		return nil, errorvar.ErrEndpointRepositoryNil
	}

	resp, err := r.etcd.Get(ctx, r.prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	items := make([]EndpointKV, 0, len(resp.Kvs))
	trimmedPrefix := strings.TrimSpace(r.prefix)
	for _, kv := range resp.Kvs {
		key := strings.TrimSpace(string(kv.Key))
		name := strings.TrimSpace(strings.TrimPrefix(key, trimmedPrefix))
		if name == "" {
			continue
		}
		items = append(items, EndpointKV{
			Name:  name,
			Value: strings.TrimSpace(string(kv.Value)),
		})
	}
	return items, nil
}
