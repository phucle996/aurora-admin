package service

import (
	keycfg "admin/internal/key"
	"context"
	"fmt"
	"sort"
	"strings"
)

type HostRoutingEntry struct {
	Host    string
	Address string
}

func (s *RuntimeBootstrapService) ListHostRoutingEntries(ctx context.Context) ([]HostRoutingEntry, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	kvs, err := s.runtimeRepo.ListByPrefix(ctx, keycfg.RuntimeHostsPrefix)
	if err != nil {
		return nil, fmt.Errorf("list host routing entries failed: %w", err)
	}

	out := make([]HostRoutingEntry, 0, len(kvs))
	for _, kv := range kvs {
		host := parseRuntimeHostEntryKey(kv.Key)
		address := strings.TrimSpace(kv.Value)
		if host == "" || address == "" {
			continue
		}
		out = append(out, HostRoutingEntry{
			Host:    host,
			Address: address,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Host == out[j].Host {
			return out[i].Address < out[j].Address
		}
		return out[i].Host < out[j].Host
	})
	return out, nil
}

func parseRuntimeHostEntryKey(key string) string {
	prefix := strings.TrimRight(strings.TrimSpace(keycfg.RuntimeHostsPrefix), "/") + "/"
	trimmed := strings.TrimSpace(key)
	if !strings.HasPrefix(trimmed, prefix) {
		return ""
	}
	return strings.Trim(strings.TrimPrefix(trimmed, prefix), "/")
}
