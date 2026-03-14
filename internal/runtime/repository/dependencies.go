package repository

import "context"

type EndpointKV struct {
	Name  string
	Value string
}

type EndpointRepository interface {
	List(ctx context.Context) ([]EndpointKV, error)
}

type CertStoreRepository interface {
	Put(ctx context.Context, key string, value string) error
	GetMany(ctx context.Context, keys []string) (map[string]string, error)
}
