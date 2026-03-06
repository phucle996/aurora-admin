package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func getEnv(env string, defaultVal string) string {
	e := os.Getenv(env)
	if e == "" {
		e = defaultVal
	}
	return e
}

func getEnvAsInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

func getEnvAsBool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}
	return d
}

func getEnvAsSlice(key string, defaultVal []string) []string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return defaultVal
	}
	return out
}

func loadPrefixedValues(ctx context.Context, cli *clientv3.Client, prefix string) (map[string]string, error) {
	if cli == nil {
		return nil, errors.New("etcd client is nil")
	}

	cleanPrefix := strings.TrimRight(strings.TrimSpace(prefix), "/")
	if cleanPrefix == "" {
		return nil, errors.New("prefix is empty")
	}

	resp, err := cli.Get(ctx, cleanPrefix+"/", clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	out := make(map[string]string, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		rawKey := strings.TrimSpace(string(kv.Key))
		if rawKey == "" {
			continue
		}
		out[rawKey] = strings.TrimSpace(string(kv.Value))
	}
	return out, nil
}

func readRequiredString(values map[string]string, key string) (string, error) {
	v, ok := values[key]
	if !ok {
		return "", fmt.Errorf("missing key %q", key)
	}
	return strings.TrimSpace(v), nil
}

func readOptionalString(values map[string]string, key string) string {
	v, ok := values[key]
	if !ok {
		return ""
	}

	trimmed := strings.TrimSpace(v)
	if strings.EqualFold(trimmed, "null") {
		return ""
	}
	return trimmed
}

func readRequiredInt(values map[string]string, key string) (int, error) {
	v, ok := values[key]
	if !ok {
		return 0, fmt.Errorf("missing key %q", key)
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("invalid int for key %q: %w", key, err)
	}
	return n, nil
}

func readRequiredBool(values map[string]string, key string) (bool, error) {
	v, ok := values[key]
	if !ok {
		return false, fmt.Errorf("missing key %q", key)
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return false, fmt.Errorf("invalid bool for key %q: %w", key, err)
	}
	return b, nil
}

func readRequiredDuration(values map[string]string, key string) (time.Duration, error) {
	v, ok := values[key]
	if !ok {
		return 0, fmt.Errorf("missing key %q", key)
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("invalid duration for key %q: %w", key, err)
	}
	return d, nil
}

func readRequiredSlice(values map[string]string, key string) ([]string, error) {
	v, ok := values[key]
	if !ok {
		return nil, fmt.Errorf("missing key %q", key)
	}
	return parseStringSlice(strings.TrimSpace(v), key)
}

func parseStringSlice(raw string, key string) ([]string, error) {
	if raw == "" {
		return []string{}, nil
	}

	if strings.HasPrefix(raw, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err != nil {
			return nil, fmt.Errorf("invalid slice(json) for key %q: %w", key, err)
		}
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			out = append(out, strings.TrimSpace(item))
		}
		return out, nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out, nil
}
