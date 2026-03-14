package app

import (
	sharedrepo "admin/internal/repository"
	runtimerepo "admin/internal/runtime/repository"
	"context"
)

type runtimeEndpointRepositoryAdapter struct {
	repo sharedrepo.EndpointRepository
}

func newRuntimeEndpointRepository(repo sharedrepo.EndpointRepository) runtimerepo.EndpointRepository {
	if repo == nil {
		return nil
	}
	return &runtimeEndpointRepositoryAdapter{repo: repo}
}

func (a *runtimeEndpointRepositoryAdapter) List(ctx context.Context) ([]runtimerepo.EndpointKV, error) {
	if a == nil || a.repo == nil {
		return nil, nil
	}
	items, err := a.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]runtimerepo.EndpointKV, 0, len(items))
	for _, item := range items {
		out = append(out, runtimerepo.EndpointKV{
			Name:  item.Name,
			Value: item.Value,
		})
	}
	return out, nil
}

type runtimeCertStoreRepositoryAdapter struct {
	repo sharedrepo.CertStoreRepository
}

func newRuntimeCertStoreRepository(repo sharedrepo.CertStoreRepository) runtimerepo.CertStoreRepository {
	if repo == nil {
		return nil
	}
	return &runtimeCertStoreRepositoryAdapter{repo: repo}
}

func (a *runtimeCertStoreRepositoryAdapter) Put(ctx context.Context, key string, value string) error {
	if a == nil || a.repo == nil {
		return nil
	}
	return a.repo.Put(ctx, key, value)
}

func (a *runtimeCertStoreRepositoryAdapter) GetMany(ctx context.Context, keys []string) (map[string]string, error) {
	if a == nil || a.repo == nil {
		return map[string]string{}, nil
	}
	return a.repo.GetMany(ctx, keys)
}
