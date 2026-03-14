package moduleinstall

import (
	keycfg "admin/internal/key"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
)

type InstallAgent struct {
	AgentID           string `json:"agent_id"`
	Status            string `json:"status"`
	Hostname          string `json:"hostname"`
	AgentGRPCEndpoint string `json:"agent_grpc_endpoint"`
}

type installAgentRuntimeDetails struct {
	InstallAgent
	Status       string
	Host         string
	Architecture string
}

type registryAgentPayload struct {
	AgentID           string `json:"agent_id"`
	ServiceID         string `json:"service_id"`
	Role              string `json:"role"`
	ClusterID         string `json:"cluster_id"`
	Hostname          string `json:"hostname"`
	IPAddress         string `json:"ip_address"`
	AgentVersion      string `json:"agent_version"`
	AgentProbeAddr    string `json:"agent_probe_addr"`
	AgentGRPCEndpoint string `json:"agent_grpc_endpoint"`
	Platform          string `json:"platform"`
	Architecture      string `json:"architecture"`
	LibvirtURI        string `json:"libvirt_uri"`
	SeenAt            string `json:"seen_at"`
}

func (s *ModuleInstallService) ListInstallAgents(ctx context.Context) ([]InstallAgent, error) {
	details, err := s.listInstallAgentsRuntime(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]InstallAgent, 0, len(details))
	for _, item := range details {
		if item.AgentID == "" {
			continue
		}
		out = append(out, InstallAgent{
			AgentID:           item.AgentID,
			Status:            strings.TrimSpace(item.Status),
			Hostname:          item.Hostname,
			AgentGRPCEndpoint: item.AgentGRPCEndpoint,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].AgentID < out[j].AgentID
	})
	return out, nil
}

func (s *ModuleInstallService) listInstallAgentsRuntime(ctx context.Context) ([]installAgentRuntimeDetails, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, fmt.Errorf("module install service is nil")
	}

	kvs, err := s.runtimeRepo.ListByPrefix(ctx, keycfg.RegistryAgentPrefix)
	if err != nil {
		return nil, fmt.Errorf("list registry agents failed: %w", err)
	}

	out := make([]installAgentRuntimeDetails, 0, len(kvs))
	for _, kv := range kvs {
		agentID := parseRegistryAgentID(kv.Key)
		if agentID == "" {
			continue
		}
		payload, parseErr := parseRegistryAgentPayload(kv.Value)
		if parseErr != nil {
			continue
		}
		if payload.AgentID == "" {
			payload.AgentID = agentID
		}
		resolvedEndpoint := strings.TrimSpace(payload.AgentGRPCEndpoint)
		if resolvedEndpoint == "" {
			resolvedEndpoint = resolveGRPCEndpointFromProbe(payload.IPAddress, payload.AgentProbeAddr)
		}
		host := firstNonEmpty(payload.IPAddress, hostFromEndpoint(resolvedEndpoint))

		out = append(out, installAgentRuntimeDetails{
			InstallAgent: InstallAgent{
				AgentID:           payload.AgentID,
				Status:            "connected",
				Hostname:          strings.TrimSpace(payload.Hostname),
				AgentGRPCEndpoint: resolvedEndpoint,
			},
			Status:       "connected",
			Host:         host,
			Architecture: strings.TrimSpace(payload.Architecture),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].AgentID < out[j].AgentID
	})
	return out, nil
}

func (s *ModuleInstallService) resolveInstallTargetFromAgent(
	ctx context.Context,
	agentID string,
	logFn InstallLogFn,
) (moduleInstallTarget, error) {
	if s == nil || s.runtimeRepo == nil {
		return moduleInstallTarget{}, fmt.Errorf("module install service is nil")
	}

	agentID = normalizeInstallAgentID(agentID)
	if agentID == "" {
		return moduleInstallTarget{}, fmt.Errorf("agent_id is required")
	}

	value, found, err := s.runtimeRepo.Get(ctx, keycfg.RegistryAgentKey(agentID))
	if err != nil {
		return moduleInstallTarget{}, fmt.Errorf("load registry agent failed: %w", err)
	}
	if !found || strings.TrimSpace(value) == "" {
		return moduleInstallTarget{}, fmt.Errorf("agent_id %s not found or not connected", agentID)
	}
	payload, parseErr := parseRegistryAgentPayload(value)
	if parseErr != nil {
		return moduleInstallTarget{}, fmt.Errorf("invalid registry payload for agent_id %s: %w", agentID, parseErr)
	}

	target := moduleInstallTarget{
		AgentID: agentID,
		AgentGRPCEndpoint: firstNonEmpty(
			strings.TrimSpace(payload.AgentGRPCEndpoint),
			resolveGRPCEndpointFromProbe(payload.IPAddress, payload.AgentProbeAddr),
		),
		Architecture: strings.TrimSpace(payload.Architecture),
	}
	if target.AgentGRPCEndpoint == "" {
		return moduleInstallTarget{}, fmt.Errorf("cannot resolve agent grpc endpoint from agent_id %s", agentID)
	}
	target.Host = firstNonEmpty(
		strings.TrimSpace(payload.IPAddress),
		hostFromEndpoint(target.AgentGRPCEndpoint),
	)
	if target.Host == "" {
		return moduleInstallTarget{}, fmt.Errorf("cannot resolve target host from agent_id %s", agentID)
	}

	logInstall(
		logFn,
		"target",
		"resolved target from agent_id=%s host=%s arch=%s",
		agentID,
		target.Host,
		firstNonEmpty(target.Architecture, "unknown"),
	)
	return target, nil
}

func normalizeInstallAgentID(raw string) string {
	return strings.Trim(strings.TrimSpace(raw), "/")
}

func parseRegistryAgentID(key string) string {
	prefix := strings.TrimRight(strings.TrimSpace(keycfg.RegistryAgentPrefix), "/") + "/"
	trimmed := strings.TrimSpace(key)
	if !strings.HasPrefix(trimmed, prefix) {
		return ""
	}
	return normalizeInstallAgentID(strings.TrimPrefix(trimmed, prefix))
}

func parseRegistryAgentPayload(raw string) (registryAgentPayload, error) {
	payload := registryAgentPayload{}
	if strings.TrimSpace(raw) == "" {
		return payload, fmt.Errorf("empty registry payload")
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func hostFromEndpoint(endpoint string) string {
	value := strings.TrimSpace(endpoint)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return normalizeAddress(host)
	}
	return normalizeAddress(value)
}

func resolveGRPCEndpointFromProbe(ipAddress, probeAddr string) string {
	probe := strings.TrimSpace(probeAddr)
	if probe == "" {
		return ""
	}
	_, port, splitErr := net.SplitHostPort(probe)
	if splitErr != nil {
		return ""
	}
	host := strings.TrimSpace(ipAddress)
	if host == "" {
		return ""
	}
	return net.JoinHostPort(host, strings.TrimSpace(port))
}
