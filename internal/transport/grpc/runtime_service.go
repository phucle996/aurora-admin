package grpc

import (
	"admin/internal/service"
	"admin/pkg/logger"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	runtimeServiceName               = "admin.transport.runtime.v1.RuntimeService"
	runtimeGetRuntimeBootstrapMethod = "GetRuntimeBootstrap"
	runtimeGetUMSBootstrapMethod     = "GetUMSBootstrap"
	runtimeBootstrapAgentMethod      = "BootstrapAgent"
	runtimeRenewAgentCertMethod      = "RenewAgentCertificate"
	runtimeReportAgentMethod         = "ReportAgentHeartbeat"
	runtimeGetAgentMetricsPolicy     = "GetAgentMetricsPolicy"
	runtimeReportAgentMetrics        = "ReportAgentMetrics"
	runtimeGetHostRoutingSnapshot    = "GetHostRoutingSnapshot"
	runtimeGetRuntimeBootstrapPath   = "/" + runtimeServiceName + "/" + runtimeGetRuntimeBootstrapMethod
	runtimeGetUMSBootstrapPath       = "/" + runtimeServiceName + "/" + runtimeGetUMSBootstrapMethod
	runtimeBootstrapAgentPath        = "/" + runtimeServiceName + "/" + runtimeBootstrapAgentMethod
	runtimeRenewAgentCertPath        = "/" + runtimeServiceName + "/" + runtimeRenewAgentCertMethod
	runtimeReportAgentPath           = "/" + runtimeServiceName + "/" + runtimeReportAgentMethod
	runtimeGetAgentPolicyPath        = "/" + runtimeServiceName + "/" + runtimeGetAgentMetricsPolicy
	runtimeReportAgentMetricsPath    = "/" + runtimeServiceName + "/" + runtimeReportAgentMetrics
	runtimeGetHostRoutingPath        = "/" + runtimeServiceName + "/" + runtimeGetHostRoutingSnapshot
)

type RuntimeTransportService struct {
	runtimeSvc *service.RuntimeBootstrapService
}

type runtimeTransportServer interface {
	GetRuntimeBootstrap(context.Context, *structpb.Struct) (*structpb.Struct, error)
	GetUMSBootstrap(context.Context, *structpb.Struct) (*structpb.Struct, error)
	BootstrapAgent(context.Context, *structpb.Struct) (*structpb.Struct, error)
	RenewAgentCertificate(context.Context, *structpb.Struct) (*structpb.Struct, error)
	ReportAgentHeartbeat(context.Context, *structpb.Struct) (*structpb.Struct, error)
	GetAgentMetricsPolicy(context.Context, *structpb.Struct) (*structpb.Struct, error)
	ReportAgentMetrics(context.Context, *structpb.Struct) (*structpb.Struct, error)
	GetHostRoutingSnapshot(context.Context, *structpb.Struct) (*structpb.Struct, error)
}

func NewRuntimeTransportService(runtimeSvc *service.RuntimeBootstrapService) *RuntimeTransportService {
	return &RuntimeTransportService{runtimeSvc: runtimeSvc}
}

func (s *RuntimeTransportService) GetRuntimeBootstrap(
	ctx context.Context,
	req *structpb.Struct,
) (*structpb.Struct, error) {
	if s == nil || s.runtimeSvc == nil {
		return nil, status.Error(codes.Unavailable, "runtime bootstrap service unavailable")
	}

	moduleName := readStructString(req, "module_name")
	appPort := readStructInt32(req, "app_port")
	clientCertDER, certErr := extractClientCertDER(ctx)
	if certErr != nil {
		return nil, status.Error(codes.Unauthenticated, certErr.Error())
	}
	if err := s.runtimeSvc.AuthorizeBootstrapClient(ctx, moduleName, clientCertDER); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	values, err := s.runtimeSvc.BuildRuntimeValues(ctx, service.RuntimeBootstrapRequest{
		ModuleName: moduleName,
		AppPort:    appPort,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to build runtime bootstrap values")
	}

	valuesAny := make(map[string]any, len(values))
	for k, v := range values {
		valuesAny[k] = v
	}

	payload := map[string]any{
		"values": valuesAny,
	}
	res, err := structpb.NewStruct(payload)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to build grpc response payload")
	}
	return res, nil
}

// GetUMSBootstrap is kept as backward-compatible alias.
func (s *RuntimeTransportService) GetUMSBootstrap(
	ctx context.Context,
	req *structpb.Struct,
) (*structpb.Struct, error) {
	return s.GetRuntimeBootstrap(ctx, req)
}

func (s *RuntimeTransportService) BootstrapAgent(
	ctx context.Context,
	req *structpb.Struct,
) (*structpb.Struct, error) {
	if s == nil || s.runtimeSvc == nil {
		return nil, status.Error(codes.Unavailable, "runtime bootstrap service unavailable")
	}

	result, err := s.runtimeSvc.BootstrapAgent(ctx, service.AgentBootstrapRequest{
		NodeID:            readStructString(req, "node_id"),
		ClusterID:         readStructString(req, "cluster_id"),
		ServiceID:         readStructString(req, "service_id"),
		Role:              readStructString(req, "role"),
		Hostname:          readStructString(req, "hostname"),
		IPAddress:         readStructString(req, "ip"),
		BootstrapToken:    readStructString(req, "bootstrap_token"),
		CSRPEM:            readStructString(req, "csr_pem"),
		ServerCSRPEM:      readStructString(req, "server_csr_pem"),
		AgentProbeAddr:    readStructString(req, "agent_probe_addr"),
		AgentGRPCEndpoint: readStructString(req, "agent_grpc_endpoint"),
		Platform:          readStructString(req, "platform"),
	}, extractPeerAddress(ctx))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAgentBootstrapTokenInvalid):
			return nil, status.Error(codes.PermissionDenied, "invalid bootstrap token")
		case errors.Is(err, service.ErrAgentCSRInvalid):
			return nil, status.Error(codes.InvalidArgument, "invalid csr")
		case isAgentBootstrapValidationError(err):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			logger.SysError("runtime.bootstrap_agent", err, "bootstrap agent failed")
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	res, encodeErr := structpb.NewStruct(map[string]any{
		"ok":                  true,
		"client_cert_pem":     result.ClientCertPEM,
		"server_cert_pem":     result.ServerCertPEM,
		"admin_server_ca_pem": result.AdminServerCAPEM,
		"ca_cert_pem":         result.AdminServerCAPEM,
		"client_serial_hex":   result.ClientSerialHex,
		"server_serial_hex":   result.ServerSerialHex,
		"client_expires_at":   result.ClientExpiresAt.UTC().Format(time.RFC3339Nano),
		"server_expires_at":   result.ServerExpiresAt.UTC().Format(time.RFC3339Nano),
	})
	if encodeErr != nil {
		return nil, status.Error(codes.Internal, "failed to build grpc response payload")
	}
	return res, nil
}

func (s *RuntimeTransportService) RenewAgentCertificate(
	ctx context.Context,
	req *structpb.Struct,
) (*structpb.Struct, error) {
	if s == nil || s.runtimeSvc == nil {
		return nil, status.Error(codes.Unavailable, "runtime bootstrap service unavailable")
	}
	_, peerInfo, certErr := extractClientPeerDetails(ctx)
	if certErr != nil {
		return nil, status.Error(codes.Unauthenticated, certErr.Error())
	}
	if err := validateAgentPeerClaims(peerInfo.Claims); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	if err := authorizeAgentRole(runtimeRenewAgentCertPath, peerInfo.Claims.Role); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	revoked, revokedErr := s.runtimeSvc.IsAgentCertificateRevoked(ctx, peerInfo.CertSerialHex)
	if revokedErr != nil {
		return nil, status.Error(codes.Internal, "failed to verify certificate status")
	}
	if revoked {
		return nil, status.Error(codes.PermissionDenied, "certificate revoked")
	}

	result, renewErr := s.runtimeSvc.RenewAgentCertificate(ctx, service.AgentRenewRequest{
		CSRPEM:       readStructString(req, "csr_pem"),
		ServerCSRPEM: readStructString(req, "server_csr_pem"),
	}, peerInfo.Claims, readStructString(req, "hostname"), readStructString(req, "ip"))
	if renewErr != nil {
		switch {
		case errors.Is(renewErr, service.ErrAgentCSRInvalid):
			return nil, status.Error(codes.InvalidArgument, "invalid csr")
		case isAgentBootstrapValidationError(renewErr):
			return nil, status.Error(codes.InvalidArgument, renewErr.Error())
		default:
			logger.SysError("runtime.renew_agent_cert", renewErr, "renew agent certificate failed")
			return nil, status.Error(codes.Internal, renewErr.Error())
		}
	}

	res, encodeErr := structpb.NewStruct(map[string]any{
		"ok":                  true,
		"client_cert_pem":     result.ClientCertPEM,
		"server_cert_pem":     result.ServerCertPEM,
		"admin_server_ca_pem": result.AdminServerCAPEM,
		"ca_cert_pem":         result.AdminServerCAPEM,
		"client_serial_hex":   result.ClientSerialHex,
		"server_serial_hex":   result.ServerSerialHex,
		"client_expires_at":   result.ClientExpiresAt.UTC().Format(time.RFC3339Nano),
		"server_expires_at":   result.ServerExpiresAt.UTC().Format(time.RFC3339Nano),
	})
	if encodeErr != nil {
		return nil, status.Error(codes.Internal, "failed to build grpc response payload")
	}
	return res, nil
}

func (s *RuntimeTransportService) ReportAgentHeartbeat(
	ctx context.Context,
	req *structpb.Struct,
) (*structpb.Struct, error) {
	if s == nil || s.runtimeSvc == nil {
		return nil, status.Error(codes.Unavailable, "runtime bootstrap service unavailable")
	}

	certDER, peerInfo, certErr := extractClientPeerDetails(ctx)
	if certErr != nil {
		return nil, status.Error(codes.Unauthenticated, certErr.Error())
	}
	if err := validateAgentPeerClaims(peerInfo.Claims); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	_ = certDER
	if err := authorizeAgentRole(runtimeReportAgentPath, peerInfo.Claims.Role); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	input := service.AgentHeartbeatInput{
		AgentID:           readStructString(req, "agent_id"),
		ServiceID:         strings.TrimSpace(peerInfo.Claims.ServiceID),
		Role:              strings.TrimSpace(peerInfo.Claims.Role),
		ClusterID:         strings.TrimSpace(peerInfo.Claims.ClusterID),
		Hostname:          readStructString(req, "hostname"),
		AgentIP:           readStructString(req, "ip"),
		AgentVersion:      readStructString(req, "agent_version"),
		AgentProbeAddr:    readStructString(req, "agent_probe_addr"),
		AgentGRPCEndpoint: readStructString(req, "agent_grpc_endpoint"),
		Platform:          readStructString(req, "platform"),
		LibvirtURI:        readStructString(req, "libvirt_uri"),
	}
	if strings.TrimSpace(input.AgentID) == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if err := validateAgentIdentityFromPeer(input.AgentID, peerInfo.Claims); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	revoked, revokedErr := s.runtimeSvc.IsAgentCertificateRevoked(ctx, peerInfo.CertSerialHex)
	if revokedErr != nil {
		return nil, status.Error(codes.Internal, "failed to verify certificate status")
	}
	if revoked {
		return nil, status.Error(codes.PermissionDenied, "certificate revoked")
	}

	if err := s.runtimeSvc.UpsertConnectedAgent(ctx, input, service.AgentPeerInfo{
		RemoteAddress: peerInfo.RemoteAddress,
		CertSHA256:    peerInfo.CertSHA256,
		CertSubject:   peerInfo.CertSubject,
		CertSerialHex: peerInfo.CertSerialHex,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to upsert agent heartbeat")
	}

	res, err := structpb.NewStruct(map[string]any{
		"ok":          true,
		"agent_id":    strings.TrimSpace(input.AgentID),
		"received_at": peerInfo.ReceivedAt,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to build grpc response payload")
	}
	return res, nil
}

func (s *RuntimeTransportService) GetAgentMetricsPolicy(
	ctx context.Context,
	req *structpb.Struct,
) (*structpb.Struct, error) {
	if s == nil || s.runtimeSvc == nil {
		return nil, status.Error(codes.Unavailable, "runtime bootstrap service unavailable")
	}
	_, peerInfo, certErr := extractClientPeerDetails(ctx)
	if certErr != nil {
		return nil, status.Error(codes.Unauthenticated, certErr.Error())
	}
	if err := validateAgentPeerClaims(peerInfo.Claims); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	if err := authorizeAgentRole(runtimeGetAgentPolicyPath, peerInfo.Claims.Role); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	agentID := readStructString(req, "agent_id")
	if strings.TrimSpace(agentID) == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if err := validateAgentIdentityFromPeer(agentID, peerInfo.Claims); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	revoked, revokedErr := s.runtimeSvc.IsAgentCertificateRevoked(ctx, peerInfo.CertSerialHex)
	if revokedErr != nil {
		return nil, status.Error(codes.Internal, "failed to verify certificate status")
	}
	if revoked {
		return nil, status.Error(codes.PermissionDenied, "certificate revoked")
	}

	policy, err := s.runtimeSvc.GetAgentMetricsPolicy(ctx, agentID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load metrics policy")
	}
	res, encodeErr := structpb.NewStruct(map[string]any{
		"stream_enabled":                 policy.StreamEnabled,
		"batch_flush_interval_seconds":   int64(policy.BatchFlushInterval / time.Second),
		"batch_sample_interval_seconds":  int64(policy.BatchSampleInterval / time.Second),
		"stream_sample_interval_seconds": int64(policy.StreamSampleInterval / time.Second),
		"max_batch_records":              int64(policy.MaxBatchRecords),
	})
	if encodeErr != nil {
		return nil, status.Error(codes.Internal, "failed to build grpc response payload")
	}
	return res, nil
}

func (s *RuntimeTransportService) GetHostRoutingSnapshot(
	ctx context.Context,
	req *structpb.Struct,
) (*structpb.Struct, error) {
	if s == nil || s.runtimeSvc == nil {
		return nil, status.Error(codes.Unavailable, "runtime bootstrap service unavailable")
	}
	_, peerInfo, certErr := extractClientPeerDetails(ctx)
	if certErr != nil {
		return nil, status.Error(codes.Unauthenticated, certErr.Error())
	}
	if err := validateAgentPeerClaims(peerInfo.Claims); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	if err := authorizeAgentRole(runtimeGetHostRoutingPath, peerInfo.Claims.Role); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	if agentID := strings.TrimSpace(readStructString(req, "agent_id")); agentID != "" {
		if err := validateAgentIdentityFromPeer(agentID, peerInfo.Claims); err != nil {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
	}

	entries, err := s.runtimeSvc.ListHostRoutingEntries(ctx)
	if err != nil {
		logger.SysError("runtime.host_routing_snapshot", err, "list host routing snapshot failed")
		return nil, status.Error(codes.Internal, "list host routing snapshot failed")
	}
	items := make([]any, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Host) == "" || strings.TrimSpace(entry.Address) == "" {
			continue
		}
		items = append(items, map[string]any{
			"host":    strings.TrimSpace(entry.Host),
			"address": strings.TrimSpace(entry.Address),
		})
	}
	res, encodeErr := structpb.NewStruct(map[string]any{
		"items": items,
		"count": len(items),
	})
	if encodeErr != nil {
		return nil, status.Error(codes.Internal, "failed to build grpc response payload")
	}
	return res, nil
}

func (s *RuntimeTransportService) ReportAgentMetrics(
	ctx context.Context,
	req *structpb.Struct,
) (*structpb.Struct, error) {
	if s == nil || s.runtimeSvc == nil {
		return nil, status.Error(codes.Unavailable, "runtime bootstrap service unavailable")
	}
	_, peerInfo, certErr := extractClientPeerDetails(ctx)
	if certErr != nil {
		return nil, status.Error(codes.Unauthenticated, certErr.Error())
	}
	if err := validateAgentPeerClaims(peerInfo.Claims); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	if err := authorizeAgentRole(runtimeReportAgentMetricsPath, peerInfo.Claims.Role); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}

	agentID := readStructString(req, "agent_id")
	if strings.TrimSpace(agentID) == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if err := validateAgentIdentityFromPeer(agentID, peerInfo.Claims); err != nil {
		return nil, status.Error(codes.PermissionDenied, err.Error())
	}
	revoked, revokedErr := s.runtimeSvc.IsAgentCertificateRevoked(ctx, peerInfo.CertSerialHex)
	if revokedErr != nil {
		return nil, status.Error(codes.Internal, "failed to verify certificate status")
	}
	if revoked {
		return nil, status.Error(codes.PermissionDenied, "certificate revoked")
	}

	records, parseErr := readMetricRecords(req, "records")
	if parseErr != nil {
		return nil, status.Error(codes.InvalidArgument, parseErr.Error())
	}
	if len(records) == 0 {
		return nil, status.Error(codes.InvalidArgument, "records is required")
	}
	mode := strings.ToLower(strings.TrimSpace(readStructString(req, "mode")))
	if mode != "stream" && mode != "batch" {
		return nil, status.Error(codes.InvalidArgument, "metrics mode is invalid")
	}

	res, encodeErr := structpb.NewStruct(map[string]any{
		"ok":                  true,
		"count":               len(records),
		"agent_id":            strings.TrimSpace(agentID),
		"mode":                mode,
		"persisted":           false,
		"status":              "under_development",
		"persistence_message": "metrics persistence is under development",
	})
	if encodeErr != nil {
		return nil, status.Error(codes.Internal, "failed to build grpc response payload")
	}
	return res, nil
}

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
	Claims         service.AgentIdentityClaims
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

func validateAgentIdentityFromPeer(agentID string, claims service.AgentIdentityClaims) error {
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

func parseAgentClaimsFromPeerCertificate(cert *x509.Certificate) service.AgentIdentityClaims {
	claims := service.AgentIdentityClaims{}
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

func validateAgentPeerClaims(claims service.AgentIdentityClaims) error {
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

func readMetricRecords(req *structpb.Struct, key string) ([]service.AgentMetricRecord, error) {
	if req == nil {
		return nil, errors.New("request payload is empty")
	}
	field, ok := req.GetFields()[key]
	if !ok || field == nil || field.GetListValue() == nil {
		return nil, errors.New("records is required")
	}
	values := field.GetListValue().GetValues()
	out := make([]service.AgentMetricRecord, 0, len(values))
	for _, item := range values {
		if item == nil || item.GetStructValue() == nil {
			continue
		}
		fields := item.GetStructValue().GetFields()

		gpu := service.AgentGPUMetricRecord{}
		if gpuField, ok := fields["gpu"]; ok && gpuField != nil && gpuField.GetStructValue() != nil {
			gpuFields := gpuField.GetStructValue().GetFields()
			gpu = service.AgentGPUMetricRecord{
				Count:            readStructUintField(gpuFields, "count"),
				UtilPercent:      readStructNumberField(gpuFields, "util_percent"),
				MemoryUsedBytes:  readStructUintField(gpuFields, "memory_used_bytes"),
				MemoryTotalBytes: readStructUintField(gpuFields, "memory_total_bytes"),
			}
		}

		record := service.AgentMetricRecord{
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

func readMetricServiceRecords(fields map[string]*structpb.Value, key string) []service.AgentServiceMetricRecord {
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

	out := make([]service.AgentServiceMetricRecord, 0, len(items))
	for _, item := range items {
		if item == nil || item.GetStructValue() == nil {
			continue
		}
		serviceFields := item.GetStructValue().GetFields()
		out = append(out, service.AgentServiceMetricRecord{
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

func RegisterRuntimeTransportServer(server *gogrpc.Server, svc *RuntimeTransportService) {
	if server == nil || svc == nil {
		return
	}

	server.RegisterService(&gogrpc.ServiceDesc{
		ServiceName: runtimeServiceName,
		HandlerType: (*runtimeTransportServer)(nil),
		Methods: []gogrpc.MethodDesc{
			{
				MethodName: runtimeGetRuntimeBootstrapMethod,
				Handler:    runtimeGetRuntimeBootstrapHandler,
			},
			{
				MethodName: runtimeGetUMSBootstrapMethod,
				Handler:    runtimeGetUMSBootstrapHandler,
			},
			{
				MethodName: runtimeBootstrapAgentMethod,
				Handler:    runtimeBootstrapAgentHandler,
			},
			{
				MethodName: runtimeRenewAgentCertMethod,
				Handler:    runtimeRenewAgentCertHandler,
			},
			{
				MethodName: runtimeReportAgentMethod,
				Handler:    runtimeReportAgentHandler,
			},
			{
				MethodName: runtimeGetAgentMetricsPolicy,
				Handler:    runtimeGetAgentMetricsPolicyHandler,
			},
			{
				MethodName: runtimeGetHostRoutingSnapshot,
				Handler:    runtimeGetHostRoutingSnapshotHandler,
			},
			{
				MethodName: runtimeReportAgentMetrics,
				Handler:    runtimeReportAgentMetricsHandler,
			},
		},
		Streams:  []gogrpc.StreamDesc{},
		Metadata: "runtime_service",
	}, svc)
}

func runtimeReportAgentHandler(
	srv interface{},
	ctx context.Context,
	dec func(interface{}) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (interface{}, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}

	base := srv.(*RuntimeTransportService)
	if interceptor == nil {
		return base.ReportAgentHeartbeat(ctx, in)
	}

	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: runtimeReportAgentPath,
	}
	handler := func(currentCtx context.Context, req interface{}) (interface{}, error) {
		return base.ReportAgentHeartbeat(currentCtx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func runtimeBootstrapAgentHandler(
	srv interface{},
	ctx context.Context,
	dec func(interface{}) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (interface{}, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}

	base := srv.(*RuntimeTransportService)
	if interceptor == nil {
		return base.BootstrapAgent(ctx, in)
	}

	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: runtimeBootstrapAgentPath,
	}
	handler := func(currentCtx context.Context, req interface{}) (interface{}, error) {
		return base.BootstrapAgent(currentCtx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func runtimeRenewAgentCertHandler(
	srv interface{},
	ctx context.Context,
	dec func(interface{}) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (interface{}, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}

	base := srv.(*RuntimeTransportService)
	if interceptor == nil {
		return base.RenewAgentCertificate(ctx, in)
	}

	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: runtimeRenewAgentCertPath,
	}
	handler := func(currentCtx context.Context, req interface{}) (interface{}, error) {
		return base.RenewAgentCertificate(currentCtx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func runtimeGetAgentMetricsPolicyHandler(
	srv interface{},
	ctx context.Context,
	dec func(interface{}) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (interface{}, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}

	base := srv.(*RuntimeTransportService)
	if interceptor == nil {
		return base.GetAgentMetricsPolicy(ctx, in)
	}

	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: runtimeGetAgentPolicyPath,
	}
	handler := func(currentCtx context.Context, req interface{}) (interface{}, error) {
		return base.GetAgentMetricsPolicy(currentCtx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func runtimeGetHostRoutingSnapshotHandler(
	srv any,
	ctx context.Context,
	dec func(any) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (any, error) {
	in := new(structpb.Struct)
	if err := dec(in); err != nil {
		return nil, err
	}
	base, ok := srv.(runtimeTransportServer)
	if !ok {
		return nil, status.Error(codes.Internal, "runtime transport server unavailable")
	}
	if interceptor == nil {
		return base.GetHostRoutingSnapshot(ctx, in)
	}
	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: runtimeGetHostRoutingPath,
	}
	handler := func(currentCtx context.Context, req any) (any, error) {
		return base.GetHostRoutingSnapshot(currentCtx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func runtimeReportAgentMetricsHandler(
	srv interface{},
	ctx context.Context,
	dec func(interface{}) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (interface{}, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}

	base := srv.(*RuntimeTransportService)
	if interceptor == nil {
		return base.ReportAgentMetrics(ctx, in)
	}

	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: runtimeReportAgentMetricsPath,
	}
	handler := func(currentCtx context.Context, req interface{}) (interface{}, error) {
		return base.ReportAgentMetrics(currentCtx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func runtimeGetUMSBootstrapHandler(
	srv interface{},
	ctx context.Context,
	dec func(interface{}) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (interface{}, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}

	base := srv.(*RuntimeTransportService)
	if interceptor == nil {
		return base.GetUMSBootstrap(ctx, in)
	}

	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: runtimeGetUMSBootstrapPath,
	}
	handler := func(currentCtx context.Context, req interface{}) (interface{}, error) {
		return base.GetUMSBootstrap(currentCtx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func runtimeGetRuntimeBootstrapHandler(
	srv interface{},
	ctx context.Context,
	dec func(interface{}) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (interface{}, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}

	base := srv.(*RuntimeTransportService)
	if interceptor == nil {
		return base.GetRuntimeBootstrap(ctx, in)
	}

	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: runtimeGetRuntimeBootstrapPath,
	}
	handler := func(currentCtx context.Context, req interface{}) (interface{}, error) {
		return base.GetRuntimeBootstrap(currentCtx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func readStructString(req *structpb.Struct, key string) string {
	if req == nil {
		return ""
	}
	field, ok := req.GetFields()[key]
	if !ok || field == nil {
		return ""
	}
	return field.GetStringValue()
}

func readStructInt32(req *structpb.Struct, key string) int32 {
	if req == nil {
		return 0
	}
	field, ok := req.GetFields()[key]
	if !ok || field == nil {
		return 0
	}
	number := field.GetNumberValue()
	if number < 0 || number > 65535 {
		return -1
	}
	return int32(number)
}

func isAgentBootstrapValidationError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, " is required") ||
		strings.Contains(msg, "claim mismatch")
}
