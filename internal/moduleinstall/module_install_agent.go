package moduleinstall

import (
	keycfg "admin/internal/key"
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
)

type InstallAgent struct {
	AgentID           string `json:"agent_id"`
	Status            string `json:"status"`
	Hostname          string `json:"hostname"`
	IPAddress         string `json:"ip_address"`
	AgentGRPCEndpoint string `json:"agent_grpc_endpoint"`
	LastSeenAt        string `json:"last_seen_at"`
	Host              string `json:"host"`
	Port              int32  `json:"port"`
	Username          string `json:"username"`
}

func (s *ModuleInstallService) ListInstallAgents(ctx context.Context) ([]InstallAgent, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, fmt.Errorf("module install service is nil")
	}

	kvs, err := s.runtimeRepo.ListByPrefix(ctx, keycfg.RuntimeAgentPrefix)
	if err != nil {
		return nil, fmt.Errorf("list runtime agents failed: %w", err)
	}

	type draft struct {
		InstallAgent
		peerHost    string
		peerAddress string
		probeAddr   string
	}

	drafts := map[string]*draft{}
	for _, kv := range kvs {
		agentID, field, ok := parseAgentNodeField(kv.Key)
		if !ok {
			continue
		}
		item, exists := drafts[agentID]
		if !exists {
			item = &draft{
				InstallAgent: InstallAgent{
					AgentID: agentID,
					Port:    22,
				},
			}
			drafts[agentID] = item
		}
		value := strings.TrimSpace(kv.Value)
		switch field {
		case "agent_id":
			if value != "" {
				item.AgentID = value
			}
		case "status":
			item.Status = value
		case "hostname":
			item.Hostname = value
		case "ip":
			item.IPAddress = value
		case "grpc_endpoint":
			item.AgentGRPCEndpoint = value
		case "probe_addr":
			item.probeAddr = value
		case "last_seen_at":
			item.LastSeenAt = value
		case "peer/host":
			item.peerHost = value
		case "peer/address":
			item.peerAddress = value
		case "username":
			item.Username = value
		case "port":
			if parsed, parseErr := strconv.Atoi(value); parseErr == nil && parsed > 0 && parsed <= 65535 {
				item.Port = int32(parsed)
			}
		}
	}

	out := make([]InstallAgent, 0, len(drafts))
	for _, item := range drafts {
		item.AgentGRPCEndpoint = firstNonEmpty(
			item.AgentGRPCEndpoint,
			resolveGRPCEndpointFromProbe(item.peerHost, item.IPAddress, item.probeAddr),
		)
		item.Host = firstNonEmpty(
			item.peerHost,
			item.IPAddress,
			hostFromEndpoint(item.AgentGRPCEndpoint),
			hostFromAddress(item.peerAddress),
		)
		if item.Username == "" {
			item.Username = "aurora"
		}
		if item.Port <= 0 || item.Port > 65535 {
			item.Port = 22
		}
		if item.AgentID == "" {
			continue
		}
		out = append(out, item.InstallAgent)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return out[i].Status == "connected"
		}
		return out[i].AgentID < out[j].AgentID
	})
	return out, nil
}

func (s *ModuleInstallService) hydrateInstallTargetFromAgent(
	ctx context.Context,
	req *ModuleInstallRequest,
	logFn InstallLogFn,
) error {
	if s == nil || s.runtimeRepo == nil || req == nil {
		return nil
	}

	agentID := normalizeInstallAgentID(req.AgentID)
	if agentID == "" {
		return nil
	}

	keys := []string{
		keycfg.RuntimeAgentNodeKey(agentID, "agent_id"),
		keycfg.RuntimeAgentNodeKey(agentID, "status"),
		keycfg.RuntimeAgentNodeKey(agentID, "hostname"),
		keycfg.RuntimeAgentNodeKey(agentID, "ip"),
		keycfg.RuntimeAgentNodeKey(agentID, "probe_addr"),
		keycfg.RuntimeAgentNodeKey(agentID, "grpc_endpoint"),
		keycfg.RuntimeAgentNodeKey(agentID, "peer/host"),
		keycfg.RuntimeAgentNodeKey(agentID, "peer/address"),
		keycfg.RuntimeAgentNodeKey(agentID, "username"),
		keycfg.RuntimeAgentNodeKey(agentID, "port"),
	}
	values, err := s.runtimeRepo.GetMany(ctx, keys)
	if err != nil {
		return fmt.Errorf("load agent runtime keys failed: %w", err)
	}
	if len(values) == 0 {
		return fmt.Errorf("agent_id %s not found", agentID)
	}

	status := strings.TrimSpace(values[keycfg.RuntimeAgentNodeKey(agentID, "status")])
	if status != "" && status != "connected" {
		return fmt.Errorf("agent_id %s is not connectable (status=%s)", agentID, status)
	}

	peerHost := strings.TrimSpace(values[keycfg.RuntimeAgentNodeKey(agentID, "peer/host")])
	ipAddress := strings.TrimSpace(values[keycfg.RuntimeAgentNodeKey(agentID, "ip")])
	probeAddr := strings.TrimSpace(values[keycfg.RuntimeAgentNodeKey(agentID, "probe_addr")])
	grpcEndpoint := strings.TrimSpace(values[keycfg.RuntimeAgentNodeKey(agentID, "grpc_endpoint")])
	peerAddress := strings.TrimSpace(values[keycfg.RuntimeAgentNodeKey(agentID, "peer/address")])

	req.AgentGRPCEndpoint = firstNonEmpty(
		req.AgentGRPCEndpoint,
		grpcEndpoint,
		resolveGRPCEndpointFromProbe(peerHost, ipAddress, probeAddr),
	)
	if req.AgentGRPCEndpoint == "" {
		return fmt.Errorf("cannot resolve agent grpc endpoint from agent_id %s", agentID)
	}

	resolvedHost := firstNonEmpty(
		normalizeAddress(req.TargetHost),
		peerHost,
		ipAddress,
		hostFromEndpoint(grpcEndpoint),
		hostFromAddress(peerAddress),
	)
	if resolvedHost == "" {
		return fmt.Errorf("cannot resolve target host from agent_id %s", agentID)
	}
	req.TargetHost = resolvedHost

	if req.TargetPort <= 0 || req.TargetPort > 65535 {
		storedPortRaw := strings.TrimSpace(values[keycfg.RuntimeAgentNodeKey(agentID, "port")])
		if storedPortRaw != "" {
			if parsed, parseErr := strconv.Atoi(storedPortRaw); parseErr == nil && parsed > 0 && parsed <= 65535 {
				req.TargetPort = int32(parsed)
			}
		}
	}
	if req.TargetPort <= 0 || req.TargetPort > 65535 {
		req.TargetPort = 22
	}

	if strings.TrimSpace(req.TargetUser) == "" {
		req.TargetUser = strings.TrimSpace(values[keycfg.RuntimeAgentNodeKey(agentID, "username")])
	}
	if strings.TrimSpace(req.TargetUser) == "" {
		req.TargetUser = "aurora"
	}

	logInstall(
		logFn,
		"target",
		"resolved target from agent_id=%s host=%s port=%d user=%s",
		agentID,
		req.TargetHost,
		req.TargetPort,
		req.TargetUser,
	)
	return nil
}

func normalizeInstallAgentID(raw string) string {
	return strings.Trim(strings.TrimSpace(raw), "/")
}

func parseAgentNodeField(fullKey string) (agentID string, field string, ok bool) {
	base := strings.TrimRight(strings.TrimSpace(keycfg.RuntimeAgentPrefix), "/") + "/"
	key := strings.TrimSpace(fullKey)
	if !strings.HasPrefix(key, base) {
		return "", "", false
	}

	rest := strings.TrimPrefix(key, base)
	if rest == "" {
		return "", "", false
	}
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	id := normalizeInstallAgentID(parts[0])
	suffix := strings.Trim(strings.TrimSpace(parts[1]), "/")
	if id == "" || suffix == "" {
		return "", "", false
	}
	return id, suffix, true
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

func hostFromAddress(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return normalizeAddress(host)
	}
	return normalizeAddress(value)
}

func resolveGRPCEndpointFromProbe(peerHost, ipAddress, probeAddr string) string {
	probe := strings.TrimSpace(probeAddr)
	if probe == "" {
		return ""
	}
	_, port, splitErr := net.SplitHostPort(probe)
	if splitErr != nil {
		return ""
	}
	host := firstNonEmpty(peerHost, ipAddress)
	if host == "" {
		return ""
	}
	return net.JoinHostPort(host, strings.TrimSpace(port))
}
