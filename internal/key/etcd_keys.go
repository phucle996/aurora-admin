package appctxKey

import (
	"strconv"
	"strings"
)

const (
	EndpointPrefix   = "/endpoint"
	RuntimePrefix    = "/runtime"
	RegistryPrefix   = "/registry"

	EndpointAdminKey = "/endpoint/admin"

	APIKeyPrefix            = "/apikey"
	TokenSecretPrefix       = "/token-secret"
	TokenSecretLegacyPrefix = "/aurora/token-secret"
	CertStorePrefix         = "/cert-store"

	RuntimeSchemaPrefix              = "/runtime/postgresql/schema"
	RuntimeAppPortPrefix             = "/runtime/app/port"
	RuntimeHostsPrefix               = "/runtime/hosts"
	RuntimeAgentPrefix               = "/runtime/agent/nodes"
	RuntimeAgentBootstrapTokenPrefix = "/runtime/agent/bootstrap_tokens"
	RuntimeAgentCSRPrefix            = "/runtime/agent/csr_requests"
	RuntimeAgentAuditPrefix          = "/runtime/agent/audit"
	RuntimeAgentRevocationPrefix     = "/runtime/agent/revocations"
	RegistryAgentPrefix              = "/registry/agents"

	APIKeyCurrentVersionPath       = "/apikey/current_version"
	APIKeyCurrentRotatedAtPath     = "/apikey/current_rotated_at"
	APIKeyCurrentRotatedAtTextPath = "/apikey/current_rotated_at_text"
	APIKeyVersionPrefixPath        = "/apikey/v"
)

func EndpointKey(moduleName string) string {
	name := strings.Trim(strings.TrimSpace(moduleName), "/")
	if name == "" {
		return EndpointPrefix
	}
	return EndpointPrefix + "/" + name
}

func RuntimeSchemaKey(moduleName string) string {
	name := strings.Trim(strings.TrimSpace(moduleName), "/")
	if name == "" {
		return RuntimeSchemaPrefix
	}
	return RuntimeSchemaPrefix + "/" + name
}

func RuntimeAppPortKey(moduleName string) string {
	name := strings.Trim(strings.TrimSpace(moduleName), "/")
	if name == "" {
		return RuntimeAppPortPrefix
	}
	return RuntimeAppPortPrefix + "/" + name
}

func RuntimeHostEntryKey(hostname string) string {
	host := strings.Trim(strings.TrimSpace(hostname), "/")
	if host == "" {
		return RuntimeHostsPrefix
	}
	return RuntimeHostsPrefix + "/" + host
}

func RuntimeStoreKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return RuntimePrefix
	}
	if strings.HasPrefix(trimmed, RuntimePrefix+"/") || trimmed == RuntimePrefix {
		return trimmed
	}
	return RuntimePrefix + "/" + strings.Trim(trimmed, "/")
}

func RuntimeAgentNodeKey(agentID string, suffix string) string {
	id := strings.Trim(strings.TrimSpace(agentID), "/")
	base := RuntimeAgentPrefix
	if id != "" {
		base = base + "/" + id
	}
	trimmedSuffix := strings.Trim(strings.TrimSpace(suffix), "/")
	if trimmedSuffix == "" {
		return base
	}
	return base + "/" + trimmedSuffix
}

func RuntimeAgentBootstrapTokenKey(tokenHash string) string {
	hash := strings.Trim(strings.TrimSpace(tokenHash), "/")
	if hash == "" {
		return RuntimeAgentBootstrapTokenPrefix
	}
	return RuntimeAgentBootstrapTokenPrefix + "/" + hash
}

func RuntimeAgentRevocationKey(serialHex string) string {
	serial := strings.Trim(strings.TrimSpace(strings.ToUpper(serialHex)), "/")
	if serial == "" {
		return RuntimeAgentRevocationPrefix
	}
	return RuntimeAgentRevocationPrefix + "/" + serial
}

func RuntimeAgentCSRRequestKey(agentID string, requestID string) string {
	id := strings.Trim(strings.TrimSpace(agentID), "/")
	request := strings.Trim(strings.TrimSpace(requestID), "/")
	base := RuntimeAgentCSRPrefix
	if id == "" {
		return base
	}
	base = base + "/" + id
	if request == "" {
		return base
	}
	return base + "/" + request
}

func RuntimeAgentAuditEventKey(agentID string, eventID string) string {
	id := strings.Trim(strings.TrimSpace(agentID), "/")
	event := strings.Trim(strings.TrimSpace(eventID), "/")
	base := RuntimeAgentAuditPrefix
	if id == "" {
		return base
	}
	base = base + "/" + id
	if event == "" {
		return base
	}
	return base + "/" + event
}

func RegistryAgentKey(agentID string) string {
	id := strings.Trim(strings.TrimSpace(agentID), "/")
	if id == "" {
		return RegistryAgentPrefix
	}
	return RegistryAgentPrefix + "/" + id
}

func RuntimeAgentMetricsPolicyKey(agentID string, suffix string) string {
	return RuntimeAgentNodeKey(agentID, "metrics/policy/"+strings.Trim(strings.TrimSpace(suffix), "/"))
}

func RuntimeAgentMetricsDataKey(agentID string, suffix string) string {
	return RuntimeAgentNodeKey(agentID, "metrics/data/"+strings.Trim(strings.TrimSpace(suffix), "/"))
}

func APIKeyCurrentVersionKey(prefix string) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	if base == "" {
		return APIKeyCurrentVersionPath
	}
	return base + "/current_version"
}

func APIKeyCurrentRotatedAtKey(prefix string) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	if base == "" {
		return APIKeyCurrentRotatedAtPath
	}
	return base + "/current_rotated_at"
}

func APIKeyCurrentRotatedAtTextKey(prefix string) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	if base == "" {
		return APIKeyCurrentRotatedAtTextPath
	}
	return base + "/current_rotated_at_text"
}

func APIKeyVersionKey(prefix string, version int64) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	if base == "" {
		return APIKeyVersionPrefixPath + "/" + strconv.FormatInt(version, 10)
	}
	return base + "/v/" + strconv.FormatInt(version, 10)
}

func APIKeyVersionPrefixKey(prefix string) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	if base == "" {
		return APIKeyVersionPrefixPath + "/"
	}
	return base + "/v/"
}

func TokenSecretCurrentVersionKey(prefix string, kind string) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	k := strings.Trim(strings.TrimSpace(kind), "/")
	if base == "" {
		base = TokenSecretPrefix
	}
	if k == "" {
		return base + "/current_version"
	}
	return base + "/" + k + "/current_version"
}

func TokenSecretVersionKey(prefix string, kind string, version int64) string {
	base := strings.TrimRight(strings.TrimSpace(prefix), "/")
	k := strings.Trim(strings.TrimSpace(kind), "/")
	v := strconv.FormatInt(version, 10)
	if base == "" {
		base = TokenSecretPrefix
	}
	if k == "" {
		return base + "/v/" + v
	}
	return base + "/" + k + "/v/" + v
}

const (
	RTAppTZ       = "/runtime/app/timezone"
	RTAppLogLevel = "/runtime/app/log_level"

	RTPgURL     = "/runtime/postgresql/url"
	RTPgSSLMode = "/runtime/postgresql/sslmode"

	RTAPIKeyRotateEvery = "/runtime/apikey/rotate_interval"

	RTRedisAddr     = "/runtime/redis/addr"
	RTRedisUser     = "/runtime/redis/username"
	RTRedisPass     = "/runtime/redis/password"
	RTRedisDB       = "/runtime/redis/db"
	RTRedisTLS      = "/runtime/redis/use_tls"
	RTRedisCA       = "/runtime/redis/ca"
	RTRedisKey      = "/runtime/redis/client_key"
	RTRedisCert     = "/runtime/redis/client_cert"
	RTRedisInsecure = "/runtime/redis/insecure_skip_verify"

	RTEtcdEndpoints          = "/runtime/etcd/endpoints"
	RTEtcdAutoSync           = "/runtime/etcd/auto_sync_interval"
	RTEtcdDialTimeout        = "/runtime/etcd/dial_timeout"
	RTEtcdKeepAliveTime      = "/runtime/etcd/dial_keepalive_time"
	RTEtcdKeepAliveTimeout   = "/runtime/etcd/dial_keepalive_timeout"
	RTEtcdUser               = "/runtime/etcd/username"
	RTEtcdPass               = "/runtime/etcd/password"
	RTEtcdTLS                = "/runtime/etcd/use_tls"
	RTEtcdCA                 = "/runtime/etcd/ca"
	RTEtcdKey                = "/runtime/etcd/client_key"
	RTEtcdCert               = "/runtime/etcd/client_cert"
	RTEtcdServerName         = "/runtime/etcd/server_name"
	RTEtcdInsecure           = "/runtime/etcd/insecure_skip_verify"
	RTEtcdPermitNoStream     = "/runtime/etcd/permit_without_stream"
	RTEtcdRejectOldCluster   = "/runtime/etcd/reject_old_cluster"
	RTEtcdMaxCallSendMsgSize = "/runtime/etcd/max_call_send_msg_size"
	RTEtcdMaxCallRecvMsgSize = "/runtime/etcd/max_call_recv_msg_size"

	RTTelegramBotToken = "/runtime/telegram/bot_token"
	RTTelegramChatID   = "/runtime/telegram/chat_id"

	RTSecretRotateAccess  = "/runtime/token_secret/access_rotate_interval"
	RTSecretRotateRefresh = "/runtime/token_secret/refresh_rotate_interval"
	RTSecretRotateDevice  = "/runtime/token_secret/device_rotate_interval"

	RTTTLAccess  = "/runtime/token_ttl/access_ttl"
	RTTTLRefresh = "/runtime/token_ttl/refresh_ttl"
	RTTTLDevice  = "/runtime/token_ttl/device_ttl"
	RTTTLOTT     = "/runtime/token_ttl/ott_ttl"

	RTSecretCachePrefix  = "/runtime/token_secret/cache_prefix"
	RTSecretCacheChannel = "/runtime/token_secret/cache_channel"
	RTSecretPollEvery    = "/runtime/token_secret/poll_interval"

	RTPlatformKubeconfigCipherKey = "/runtime/platform/kubeconfig_cipher_key"
)
