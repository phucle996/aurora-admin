package grpc

import (
	runtimesvc "admin/internal/runtime/service"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/structpb"
)

func extractClientCertDER(ctx context.Context) ([]byte, error) {
	certDER, _, err := extractClientPeerDetails(ctx)
	return certDER, err
}

type runtimePeerInfo struct {
	RemoteAddress  string
	CertSHA256     string
	CertSubject    string
	CertCommonName string
	CertSerialHex  string
	ReceivedAt     string
	Claims         runtimesvc.AgentIdentityClaims
}

func extractClientPeerDetails(ctx context.Context) ([]byte, runtimePeerInfo, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.AuthInfo == nil {
		return nil, runtimePeerInfo{}, errors.New("missing peer auth info")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, runtimePeerInfo{}, errors.New("invalid peer auth type")
	}
	if len(tlsInfo.State.PeerCertificates) == 0 || tlsInfo.State.PeerCertificates[0] == nil {
		return nil, runtimePeerInfo{}, errors.New("missing peer certificate")
	}
	leaf := tlsInfo.State.PeerCertificates[0]
	remoteAddress := ""
	if p.Addr != nil {
		remoteAddress = strings.TrimSpace(p.Addr.String())
	}
	sum := sha256.Sum256(leaf.Raw)
	claims := parseAgentClaimsFromPeerCertificate(leaf)
	return leaf.Raw, runtimePeerInfo{
		RemoteAddress:  remoteAddress,
		CertSHA256:     hex.EncodeToString(sum[:]),
		CertSubject:    strings.TrimSpace(leaf.Subject.String()),
		CertCommonName: strings.TrimSpace(leaf.Subject.CommonName),
		CertSerialHex:  strings.ToUpper(leaf.SerialNumber.Text(16)),
		ReceivedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Claims:         claims,
	}, nil
}

func extractPeerAddress(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.Addr == nil {
		return ""
	}
	return strings.TrimSpace(p.Addr.String())
}

func validateAgentIdentityFromPeer(agentID string, claims runtimesvc.AgentIdentityClaims) error {
	id := strings.TrimSpace(agentID)
	if id == "" {
		return errors.New("agent_id is required")
	}
	certAgentID := strings.TrimSpace(claims.NodeID)
	if certAgentID == "" {
		return errors.New("missing node_id claim in certificate")
	}
	if certAgentID != id {
		return errors.New("agent_id does not match certificate")
	}
	return nil
}

func parseAgentClaimsFromPeerCertificate(cert *x509.Certificate) runtimesvc.AgentIdentityClaims {
	claims := runtimesvc.AgentIdentityClaims{}
	if cert == nil {
		return claims
	}
	for _, uri := range cert.URIs {
		if uri == nil || !strings.EqualFold(strings.TrimSpace(uri.Scheme), "spiffe") {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(uri.Host), "aurora.local") {
			continue
		}
		parts := strings.Split(strings.Trim(strings.TrimSpace(uri.Path), "/"), "/")
		if len(parts) != 2 {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if value == "" {
			continue
		}
		switch kind {
		case "node":
			claims.NodeID = value
		case "service":
			claims.ServiceID = value
		case "role":
			claims.Role = value
		case "cluster":
			claims.ClusterID = value
		}
	}
	return claims
}

func validateAgentPeerClaims(claims runtimesvc.AgentIdentityClaims) error {
	if strings.TrimSpace(claims.NodeID) == "" {
		return errors.New("missing node_id claim in certificate")
	}
	if strings.TrimSpace(claims.ServiceID) == "" {
		return errors.New("missing service_id claim in certificate")
	}
	if strings.TrimSpace(claims.Role) == "" {
		return errors.New("missing role claim in certificate")
	}
	if strings.TrimSpace(claims.ClusterID) == "" {
		return errors.New("missing cluster_id claim in certificate")
	}
	return nil
}

func authorizeAgentRole(methodPath string, role string) error {
	requiredRoles := map[string]map[string]struct{}{
		runtimeReportAgentPath: {
			"agent": {},
		},
		runtimeGetAgentPolicyPath: {
			"agent": {},
		},
		runtimeReportAgentMetricsPath: {
			"agent": {},
		},
		runtimeGetHostRoutingPath: {
			"agent": {},
		},
		runtimeRenewAgentCertPath: {
			"agent": {},
		},
	}
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return errors.New("missing role claim in certificate")
	}
	allowed, ok := requiredRoles[methodPath]
	if !ok {
		return errors.New("method is not authorized for agent role")
	}
	if _, exists := allowed[role]; !exists {
		return errors.New("role is not allowed for method")
	}
	return nil
}

func readMetricRecords(req *structpb.Struct, key string) ([]runtimesvc.AgentMetricRecord, error) {
	if req == nil {
		return nil, errors.New("request payload is empty")
	}
	field, ok := req.GetFields()[key]
	if !ok || field == nil || field.GetListValue() == nil {
		return nil, errors.New("records is required")
	}
	values := field.GetListValue().GetValues()
	out := make([]runtimesvc.AgentMetricRecord, 0, len(values))
	for _, item := range values {
		if item == nil || item.GetStructValue() == nil {
			continue
		}
		fields := item.GetStructValue().GetFields()

		gpu := runtimesvc.AgentGPUMetricRecord{}
		if gpuField, ok := fields["gpu"]; ok && gpuField != nil && gpuField.GetStructValue() != nil {
			gpuFields := gpuField.GetStructValue().GetFields()
			gpu = runtimesvc.AgentGPUMetricRecord{
				Count:            readStructUintField(gpuFields, "count"),
				UtilPercent:      readStructNumberField(gpuFields, "util_percent"),
				MemoryUsedBytes:  readStructUintField(gpuFields, "memory_used_bytes"),
				MemoryTotalBytes: readStructUintField(gpuFields, "memory_total_bytes"),
			}
		}

		record := runtimesvc.AgentMetricRecord{
			TimestampUnixMillis: int64(readStructNumberField(fields, "ts_ms")),
			CPUUsagePercent:     readStructNumberField(fields, "cpu_usage_percent"),
			MemoryUsedBytes:     readStructUintField(fields, "memory_used_bytes"),
			MemoryTotalBytes:    readStructUintField(fields, "memory_total_bytes"),
			DiskReadBps:         readStructNumberField(fields, "disk_read_bps"),
			DiskWriteBps:        readStructNumberField(fields, "disk_write_bps"),
			NetworkRxBps:        readStructNumberField(fields, "network_rx_bps"),
			NetworkTxBps:        readStructNumberField(fields, "network_tx_bps"),
			GPU:                 gpu,
			Services:            readMetricServiceRecords(fields, "services"),
			UptimeSeconds:       uint64(readStructNumberField(fields, "uptime_seconds")),
		}
		if record.MemoryUsedBytes == 0 {
			legacyPercent := readStructNumberField(fields, "memory_used_percent")
			if legacyPercent > 0 && record.MemoryTotalBytes > 0 {
				record.MemoryUsedBytes = uint64((legacyPercent / 100) * float64(record.MemoryTotalBytes))
			}
		}
		out = append(out, record)
	}
	return out, nil
}

func readMetricServiceRecords(fields map[string]*structpb.Value, key string) []runtimesvc.AgentServiceMetricRecord {
	if len(fields) == 0 {
		return nil
	}
	v, ok := fields[key]
	if !ok || v == nil || v.GetListValue() == nil {
		return nil
	}
	items := v.GetListValue().GetValues()
	if len(items) == 0 {
		return nil
	}

	out := make([]runtimesvc.AgentServiceMetricRecord, 0, len(items))
	for _, item := range items {
		if item == nil || item.GetStructValue() == nil {
			continue
		}
		serviceFields := item.GetStructValue().GetFields()
		out = append(out, runtimesvc.AgentServiceMetricRecord{
			Service:            readStructStringField(serviceFields, "service"),
			CPUUsagePercent:    readStructNumberField(serviceFields, "cpu_usage_percent"),
			MemoryUsedBytes:    readStructUintField(serviceFields, "memory_used_bytes"),
			DiskReadBps:        readStructNumberField(serviceFields, "disk_read_bps"),
			DiskWriteBps:       readStructNumberField(serviceFields, "disk_write_bps"),
			NetworkRxBps:       readStructNumberField(serviceFields, "network_rx_bps"),
			NetworkTxBps:       readStructNumberField(serviceFields, "network_tx_bps"),
			GPUUtilPercent:     readStructNumberField(serviceFields, "gpu_util_percent"),
			GPUMemoryUsedBytes: readStructUintField(serviceFields, "gpu_memory_used_bytes"),
		})
	}
	return out
}

func readStructNumberField(fields map[string]*structpb.Value, key string) float64 {
	if len(fields) == 0 {
		return 0
	}
	value, ok := fields[key]
	if !ok || value == nil {
		return 0
	}
	return value.GetNumberValue()
}

func readStructUintField(fields map[string]*structpb.Value, key string) uint64 {
	n := readStructNumberField(fields, key)
	if n <= 0 {
		return 0
	}
	return uint64(n)
}

func readStructStringField(fields map[string]*structpb.Value, key string) string {
	if len(fields) == 0 {
		return ""
	}
	value, ok := fields[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(value.GetStringValue())
}
