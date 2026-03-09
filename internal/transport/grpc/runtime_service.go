package grpc

import (
	"admin/internal/service"
	"context"
	"crypto/sha256"
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
	runtimeServiceName            = "admin.transport.runtime.v1.RuntimeService"
	runtimeGetUMSBootstrapMethod  = "GetUMSBootstrap"
	runtimeBootstrapAgentMethod   = "BootstrapAgent"
	runtimeReportAgentMethod      = "ReportAgentHeartbeat"
	runtimeGetAgentMetricsPolicy  = "GetAgentMetricsPolicy"
	runtimeReportAgentMetrics     = "ReportAgentMetrics"
	runtimeGetUMSBootstrapPath    = "/" + runtimeServiceName + "/" + runtimeGetUMSBootstrapMethod
	runtimeBootstrapAgentPath     = "/" + runtimeServiceName + "/" + runtimeBootstrapAgentMethod
	runtimeReportAgentPath        = "/" + runtimeServiceName + "/" + runtimeReportAgentMethod
	runtimeGetAgentPolicyPath     = "/" + runtimeServiceName + "/" + runtimeGetAgentMetricsPolicy
	runtimeReportAgentMetricsPath = "/" + runtimeServiceName + "/" + runtimeReportAgentMetrics
)

type RuntimeTransportService struct {
	runtimeSvc *service.RuntimeBootstrapService
}

type runtimeTransportServer interface {
	GetUMSBootstrap(context.Context, *structpb.Struct) (*structpb.Struct, error)
	BootstrapAgent(context.Context, *structpb.Struct) (*structpb.Struct, error)
	ReportAgentHeartbeat(context.Context, *structpb.Struct) (*structpb.Struct, error)
	GetAgentMetricsPolicy(context.Context, *structpb.Struct) (*structpb.Struct, error)
	ReportAgentMetrics(context.Context, *structpb.Struct) (*structpb.Struct, error)
}

func NewRuntimeTransportService(runtimeSvc *service.RuntimeBootstrapService) *RuntimeTransportService {
	return &RuntimeTransportService{runtimeSvc: runtimeSvc}
}

func (s *RuntimeTransportService) GetUMSBootstrap(
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
		Hostname:          readStructString(req, "hostname"),
		IPAddress:         readStructString(req, "ip"),
		BootstrapToken:    readStructString(req, "bootstrap_token"),
		CSRPEM:            readStructString(req, "csr_pem"),
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
		default:
			return nil, status.Error(codes.Internal, "agent bootstrap failed")
		}
	}

	res, encodeErr := structpb.NewStruct(map[string]any{
		"ok":              true,
		"client_cert_pem": result.ClientCertPEM,
		"ca_cert_pem":     result.CACertPEM,
		"serial_hex":      result.SerialHex,
		"expires_at":      result.ExpiresAt.UTC().Format(time.RFC3339Nano),
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
	_ = certDER

	input := service.AgentHeartbeatInput{
		AgentID:           readStructString(req, "agent_id"),
		Hostname:          readStructString(req, "hostname"),
		AgentVersion:      readStructString(req, "agent_version"),
		AgentProbeAddr:    readStructString(req, "agent_probe_addr"),
		AgentGRPCEndpoint: readStructString(req, "agent_grpc_endpoint"),
		Platform:          readStructString(req, "platform"),
		LibvirtURI:        readStructString(req, "libvirt_uri"),
	}
	if strings.TrimSpace(input.AgentID) == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if err := validateAgentIdentityFromPeer(input.AgentID, peerInfo); err != nil {
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
	agentID := readStructString(req, "agent_id")
	if strings.TrimSpace(agentID) == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if err := validateAgentIdentityFromPeer(agentID, peerInfo); err != nil {
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

	agentID := readStructString(req, "agent_id")
	if strings.TrimSpace(agentID) == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if err := validateAgentIdentityFromPeer(agentID, peerInfo); err != nil {
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
	return leaf.Raw, runtimePeerInfo{
		RemoteAddress:  remoteAddress,
		CertSHA256:     hex.EncodeToString(sum[:]),
		CertSubject:    strings.TrimSpace(leaf.Subject.String()),
		CertCommonName: strings.TrimSpace(leaf.Subject.CommonName),
		CertSerialHex:  strings.ToUpper(leaf.SerialNumber.Text(16)),
		ReceivedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func extractPeerAddress(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.Addr == nil {
		return ""
	}
	return strings.TrimSpace(p.Addr.String())
}

func validateAgentIdentityFromPeer(agentID string, peer runtimePeerInfo) error {
	id := strings.TrimSpace(agentID)
	if id == "" {
		return errors.New("agent_id is required")
	}
	cn := strings.TrimSpace(peer.CertCommonName)
	if cn == "" {
		return nil
	}
	if !strings.HasPrefix(cn, "agent:") {
		return nil
	}
	certAgentID := strings.TrimSpace(strings.TrimPrefix(cn, "agent:"))
	if certAgentID == "" {
		return errors.New("invalid certificate common name")
	}
	if certAgentID != id {
		return errors.New("agent_id does not match certificate")
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
		record := service.AgentMetricRecord{
			TimestampUnixMillis: int64(readStructNumberField(fields, "ts_ms")),
			CPUUsagePercent:     readStructNumberField(fields, "cpu_usage_percent"),
			MemoryUsedPercent:   readStructNumberField(fields, "memory_used_percent"),
			DiskReadBps:         readStructNumberField(fields, "disk_read_bps"),
			DiskWriteBps:        readStructNumberField(fields, "disk_write_bps"),
			NetworkRxBps:        readStructNumberField(fields, "network_rx_bps"),
			NetworkTxBps:        readStructNumberField(fields, "network_tx_bps"),
			UptimeSeconds:       uint64(readStructNumberField(fields, "uptime_seconds")),
		}
		out = append(out, record)
	}
	return out, nil
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

func RegisterRuntimeTransportServer(server *gogrpc.Server, svc *RuntimeTransportService) {
	if server == nil || svc == nil {
		return
	}

	server.RegisterService(&gogrpc.ServiceDesc{
		ServiceName: runtimeServiceName,
		HandlerType: (*runtimeTransportServer)(nil),
		Methods: []gogrpc.MethodDesc{
			{
				MethodName: runtimeGetUMSBootstrapMethod,
				Handler:    runtimeGetUMSBootstrapHandler,
			},
			{
				MethodName: runtimeBootstrapAgentMethod,
				Handler:    runtimeBootstrapAgentHandler,
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
