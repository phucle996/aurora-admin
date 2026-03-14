package service

import (
	keycfg "admin/internal/key"
	"context"
	"fmt"
	"strings"
)

type structuredBootstrapSource string

const (
	structuredBootstrapSourceRuntime    structuredBootstrapSource = "runtime"
	structuredBootstrapSourceTLS        structuredBootstrapSource = "tls"
	structuredBootstrapSourceAppPort    structuredBootstrapSource = "app_port"
	structuredBootstrapSourceModuleGRPC structuredBootstrapSource = "module_grpc_endpoint"
)

type structuredBootstrapField struct {
	OutputPath string
	Source     structuredBootstrapSource
	StoreKey   string
	Required   bool
}

var structuredBootstrapFieldRegistry = map[string]structuredBootstrapField{
	"app.timezone": {
		OutputPath: "app.timezone",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTAppTZ,
		Required:   true,
	},
	"app.log_level": {
		OutputPath: "app.log_level",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTAppLogLevel,
		Required:   true,
	},
	"app.port": {
		OutputPath: "app.port",
		Source:     structuredBootstrapSourceAppPort,
		Required:   true,
	},
	"psql.url": {
		OutputPath: "psql.url",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTPgURL,
		Required:   true,
	},
	"psql.ssl_mode": {
		OutputPath: "psql.ssl_mode",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTPgSSLMode,
		Required:   true,
	},
	"psql.schema": {
		OutputPath: "psql.schema",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RuntimeSchemaPrefix + "/{module}",
		Required:   true,
	},
	"redis.addr": {
		OutputPath: "redis.addr",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTRedisAddr,
		Required:   true,
	},
	"redis.username": {
		OutputPath: "redis.username",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTRedisUser,
		Required:   false,
	},
	"redis.password": {
		OutputPath: "redis.password",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTRedisPass,
		Required:   false,
	},
	"redis.db": {
		OutputPath: "redis.db",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTRedisDB,
		Required:   true,
	},
	"redis.use_tls": {
		OutputPath: "redis.use_tls",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTRedisTLS,
		Required:   true,
	},
	"redis.ca": {
		OutputPath: "redis.ca",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTRedisCA,
		Required:   false,
	},
	"redis.client_key": {
		OutputPath: "redis.client_key",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTRedisKey,
		Required:   false,
	},
	"redis.client_cert": {
		OutputPath: "redis.client_cert",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTRedisCert,
		Required:   false,
	},
	"redis.insecure_skip_verify": {
		OutputPath: "redis.insecure_skip_verify",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTRedisInsecure,
		Required:   true,
	},
	"token.access_ttl": {
		OutputPath: "token.access_ttl",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTTTLAccess,
		Required:   true,
	},
	"token.refresh_ttl": {
		OutputPath: "token.refresh_ttl",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTTTLRefresh,
		Required:   true,
	},
	"token.device_ttl": {
		OutputPath: "token.device_ttl",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTTTLDevice,
		Required:   true,
	},
	"token.ott_ttl": {
		OutputPath: "token.ott_ttl",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTTTLOTT,
		Required:   true,
	},
	"platform.kubeconfig_cipher_key": {
		OutputPath: "platform.kubeconfig_cipher_key",
		Source:     structuredBootstrapSourceRuntime,
		StoreKey:   keycfg.RTPlatformKubeconfigCipherKey,
		Required:   true,
	},
	"platform.grpc_endpoint": {
		OutputPath: "platform.grpc_endpoint",
		Source:     structuredBootstrapSourceModuleGRPC,
		StoreKey:   "platform",
		Required:   true,
	},
	"tls.ca_pem": {
		OutputPath: "tls.ca_pem",
		Source:     structuredBootstrapSourceTLS,
		StoreKey:   "tls/ca_pem",
		Required:   true,
	},
	"tls.client_cert_pem": {
		OutputPath: "tls.client_cert_pem",
		Source:     structuredBootstrapSourceTLS,
		StoreKey:   "tls/client_cert_pem",
		Required:   true,
	},
	"tls.client_key_pem": {
		OutputPath: "tls.client_key_pem",
		Source:     structuredBootstrapSourceTLS,
		StoreKey:   "tls/client_key_pem",
		Required:   true,
	},
}

func (s *RuntimeBootstrapService) BuildStructuredRuntimeConfig(
	ctx context.Context,
	req RuntimeBootstrapRequest,
) (map[string]any, error) {
	if s == nil || s.runtimeRepo == nil || s.endpointRepo == nil || s.certStoreRepo == nil {
		return nil, fmt.Errorf("runtime bootstrap service is nil")
	}

	moduleName := normalizeBootstrapModuleName(req.ModuleName)
	if moduleName == "" {
		return nil, fmt.Errorf("module_name is required")
	}
	if len(req.ConfigKeys) == 0 {
		return nil, fmt.Errorf("config_keys is required")
	}

	fields := make([]structuredBootstrapField, 0, len(req.ConfigKeys))
	seenKeys := make(map[string]struct{}, len(req.ConfigKeys))
	runtimeKeys := make([]string, 0, len(req.ConfigKeys))
	seenRuntimeKeys := make(map[string]struct{}, len(req.ConfigKeys))
	endpointModules := make(map[string]struct{})
	needsTLS := false
	needsAppPort := false

	for _, rawKey := range req.ConfigKeys {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		if _, exists := seenKeys[key]; exists {
			continue
		}
		seenKeys[key] = struct{}{}

		field, ok := structuredBootstrapFieldRegistry[key]
		if !ok {
			return nil, fmt.Errorf("unsupported config key: %s", key)
		}
		field.StoreKey = resolveBootstrapStoreKey(field.StoreKey, moduleName)
		fields = append(fields, field)

		switch field.Source {
		case structuredBootstrapSourceRuntime:
			if _, exists := seenRuntimeKeys[field.StoreKey]; exists {
				continue
			}
			seenRuntimeKeys[field.StoreKey] = struct{}{}
			runtimeKeys = append(runtimeKeys, field.StoreKey)
		case structuredBootstrapSourceTLS:
			needsTLS = true
		case structuredBootstrapSourceAppPort:
			needsAppPort = true
		case structuredBootstrapSourceModuleGRPC:
			target := normalizeBootstrapModuleName(field.StoreKey)
			if target != "" {
				endpointModules[target] = struct{}{}
			}
		}
	}

	runtimeValues, err := s.runtimeRepo.GetMany(ctx, runtimeKeys)
	if err != nil {
		return nil, fmt.Errorf("load runtime keys failed: %w", err)
	}

	tlsBundle := map[string]string{}
	if needsTLS {
		tlsBundle, err = s.loadModuleTLSBundle(ctx, moduleName)
		if err != nil {
			return nil, err
		}
	}

	endpointMap := map[string]string{}
	var endpointListErr error
	if needsAppPort || len(endpointModules) > 0 {
		endpointItems, listErr := s.endpointRepo.List(ctx)
		endpointListErr = listErr
		endpointMap = buildModuleEndpointMap(endpointItems)
	}

	var appPort int32
	if needsAppPort {
		appPort, err = s.resolveModulePort(ctx, moduleName, req.AppPort, endpointMap, endpointListErr)
		if err != nil {
			return nil, err
		}
	}

	config := make(map[string]any)
	missing := make([]string, 0)
	empty := make([]string, 0)

	for _, field := range fields {
		switch field.Source {
		case structuredBootstrapSourceRuntime:
			value, exists := runtimeValues[field.StoreKey]
			if !exists {
				if field.Required {
					missing = append(missing, keycfg.RuntimeStoreKey(field.StoreKey))
				}
				continue
			}
			trimmed := strings.TrimSpace(value)
			if field.Required && trimmed == "" {
				empty = append(empty, keycfg.RuntimeStoreKey(field.StoreKey))
				continue
			}
			setStructuredRuntimeValue(config, field.OutputPath, trimmed)
		case structuredBootstrapSourceTLS:
			value := strings.TrimSpace(tlsBundle[field.StoreKey])
			if field.Required && value == "" {
				empty = append(empty, "cert_store/"+field.StoreKey)
				continue
			}
			setStructuredRuntimeValue(config, field.OutputPath, value)
		case structuredBootstrapSourceAppPort:
			if field.Required && appPort <= 0 {
				empty = append(empty, keycfg.RuntimeAppPortKey(moduleName))
				continue
			}
			setStructuredRuntimeValue(config, field.OutputPath, appPort)
		case structuredBootstrapSourceModuleGRPC:
			baseURL, resolveErr := s.resolveModuleBaseURL(field.StoreKey, endpointMap, endpointListErr)
			if resolveErr != nil {
				if field.Required {
					empty = append(empty, "endpoint/"+normalizeBootstrapModuleName(field.StoreKey))
				}
				continue
			}
			grpcEndpoint := strings.TrimSpace(toGRPCEndpoint(baseURL))
			if field.Required && grpcEndpoint == "" {
				empty = append(empty, "endpoint/"+normalizeBootstrapModuleName(field.StoreKey))
				continue
			}
			setStructuredRuntimeValue(config, field.OutputPath, grpcEndpoint)
		}
	}

	if err := buildBootstrapValidationError(missing, empty); err != nil {
		return nil, err
	}
	return config, nil
}

func resolveBootstrapStoreKey(raw string, moduleName string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	return strings.ReplaceAll(value, "{module}", normalizeBootstrapModuleName(moduleName))
}

func setStructuredRuntimeValue(target map[string]any, path string, value any) {
	parts := strings.Split(strings.Trim(strings.TrimSpace(path), "."), ".")
	if len(parts) == 0 {
		return
	}

	current := target
	for _, part := range parts[:len(parts)-1] {
		part = strings.TrimSpace(part)
		if part == "" {
			return
		}
		next, ok := current[part]
		if !ok {
			child := make(map[string]any)
			current[part] = child
			current = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			child = make(map[string]any)
			current[part] = child
		}
		current = child
	}

	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return
	}
	current[last] = value
}
