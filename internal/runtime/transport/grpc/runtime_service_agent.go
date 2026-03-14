package grpc

import (
	runtimesvc "admin/internal/runtime/service"
	"admin/pkg/logger"
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

func (s *RuntimeTransportService) BootstrapAgent(
	ctx context.Context,
	req *structpb.Struct,
) (*structpb.Struct, error) {
	if s == nil || s.runtimeSvc == nil {
		return nil, status.Error(codes.Unavailable, "runtime bootstrap service unavailable")
	}

	result, err := s.runtimeSvc.BootstrapAgent(ctx, runtimesvc.AgentBootstrapRequest{
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
		case errors.Is(err, runtimesvc.ErrAgentBootstrapTokenInvalid):
			return nil, status.Error(codes.PermissionDenied, "invalid bootstrap token")
		case errors.Is(err, runtimesvc.ErrAgentCSRInvalid):
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

	result, renewErr := s.runtimeSvc.RenewAgentCertificate(ctx, runtimesvc.AgentRenewRequest{
		CSRPEM:       readStructString(req, "csr_pem"),
		ServerCSRPEM: readStructString(req, "server_csr_pem"),
	}, peerInfo.Claims, readStructString(req, "hostname"), readStructString(req, "ip"))
	if renewErr != nil {
		switch {
		case errors.Is(renewErr, runtimesvc.ErrAgentCSRInvalid):
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

	input := runtimesvc.AgentHeartbeatInput{
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
		Architecture:      readStructString(req, "architecture"),
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

	if err := s.runtimeSvc.UpsertConnectedAgent(ctx, input, runtimesvc.AgentPeerInfo{
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
