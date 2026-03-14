package service

import (
	keycfg "admin/internal/key"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type AgentMetricRecord struct {
	TimestampUnixMillis int64                      `json:"ts_ms"`
	CPUUsagePercent     float64                    `json:"cpu_usage_percent"`
	MemoryUsedBytes     uint64                     `json:"memory_used_bytes"`
	MemoryTotalBytes    uint64                     `json:"memory_total_bytes"`
	DiskReadBps         float64                    `json:"disk_read_bps"`
	DiskWriteBps        float64                    `json:"disk_write_bps"`
	NetworkRxBps        float64                    `json:"network_rx_bps"`
	NetworkTxBps        float64                    `json:"network_tx_bps"`
	GPU                 AgentGPUMetricRecord       `json:"gpu"`
	Services            []AgentServiceMetricRecord `json:"services"`
	UptimeSeconds       uint64                     `json:"uptime_seconds"`
}

type AgentGPUMetricRecord struct {
	Count            uint64  `json:"count"`
	UtilPercent      float64 `json:"util_percent"`
	MemoryUsedBytes  uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes uint64  `json:"memory_total_bytes"`
}

type AgentServiceMetricRecord struct {
	Service            string  `json:"service"`
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryUsedBytes    uint64  `json:"memory_used_bytes"`
	DiskReadBps        float64 `json:"disk_read_bps"`
	DiskWriteBps       float64 `json:"disk_write_bps"`
	NetworkRxBps       float64 `json:"network_rx_bps"`
	NetworkTxBps       float64 `json:"network_tx_bps"`
	GPUUtilPercent     float64 `json:"gpu_util_percent"`
	GPUMemoryUsedBytes uint64  `json:"gpu_memory_used_bytes"`
}

type AgentMetricsPolicy struct {
	StreamEnabled        bool
	BatchFlushInterval   time.Duration
	BatchSampleInterval  time.Duration
	StreamSampleInterval time.Duration
	MaxBatchRecords      int
}

type AgentMetricsReportInput struct {
	AgentID string
	Mode    string
	Records []AgentMetricRecord
}

func (s *RuntimeBootstrapService) GetAgentMetricsPolicy(
	ctx context.Context,
	agentID string,
) (AgentMetricsPolicy, error) {
	if s == nil || s.runtimeRepo == nil {
		return AgentMetricsPolicy{}, fmt.Errorf("runtime bootstrap service is nil")
	}
	id := normalizeAgentID(agentID)
	if id == "" {
		return AgentMetricsPolicy{}, fmt.Errorf("agent_id is required")
	}

	defaultPolicy := AgentMetricsPolicy{
		StreamEnabled:        false,
		BatchFlushInterval:   3 * time.Minute,
		BatchSampleInterval:  10 * time.Second,
		StreamSampleInterval: 3 * time.Second,
		MaxBatchRecords:      2048,
	}

	keys := []string{
		keycfg.RuntimeAgentMetricsPolicyKey(id, "stream_enabled"),
		keycfg.RuntimeAgentMetricsPolicyKey(id, "batch_flush_interval"),
		keycfg.RuntimeAgentMetricsPolicyKey(id, "batch_sample_interval"),
		keycfg.RuntimeAgentMetricsPolicyKey(id, "stream_sample_interval"),
		keycfg.RuntimeAgentMetricsPolicyKey(id, "max_batch_records"),
	}
	values, err := s.runtimeRepo.GetMany(ctx, keys)
	if err != nil {
		return AgentMetricsPolicy{}, fmt.Errorf("load agent metrics policy failed: %w", err)
	}

	policy := defaultPolicy
	policy.StreamEnabled = parseBool(values[keys[0]], defaultPolicy.StreamEnabled)
	policy.BatchFlushInterval = parseDuration(values[keys[1]], defaultPolicy.BatchFlushInterval)
	policy.BatchSampleInterval = parseDuration(values[keys[2]], defaultPolicy.BatchSampleInterval)
	policy.StreamSampleInterval = parseDuration(values[keys[3]], defaultPolicy.StreamSampleInterval)
	policy.MaxBatchRecords = parsePositiveInt(values[keys[4]], defaultPolicy.MaxBatchRecords)
	return policy, nil
}

func (s *RuntimeBootstrapService) SaveAgentMetricsReport(
	ctx context.Context,
	input AgentMetricsReportInput,
	peer AgentPeerInfo,
) error {
	// Metrics persistence is intentionally disabled for now.
	// A dedicated metrics module/storage pipeline will own this in a later phase.
	_ = ctx
	_ = input
	_ = peer
	return nil
}

func parseBool(raw string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func parsePositiveInt(raw string, fallback int) int {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
