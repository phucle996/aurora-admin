package grpc

import (
	runtimesvc "admin/internal/runtime/service"
	"context"
	"strings"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	runtimeServiceName               = "admin.transport.runtime.v1.RuntimeService"
	runtimeGetRuntimeBootstrapMethod = "GetRuntimeBootstrap"
	runtimeBootstrapModuleMethod     = "BootstrapModuleClient"
	runtimeBootstrapAgentMethod      = "BootstrapAgent"
	runtimeRenewAgentCertMethod      = "RenewAgentCertificate"
	runtimeReportAgentMethod         = "ReportAgentHeartbeat"
	runtimeGetAgentMetricsPolicy     = "GetAgentMetricsPolicy"
	runtimeReportAgentMetrics        = "ReportAgentMetrics"
	runtimeGetHostRoutingSnapshot    = "GetHostRoutingSnapshot"
	runtimeGetRuntimeBootstrapPath   = "/" + runtimeServiceName + "/" + runtimeGetRuntimeBootstrapMethod
	runtimeBootstrapModulePath       = "/" + runtimeServiceName + "/" + runtimeBootstrapModuleMethod
	runtimeBootstrapAgentPath        = "/" + runtimeServiceName + "/" + runtimeBootstrapAgentMethod
	runtimeRenewAgentCertPath        = "/" + runtimeServiceName + "/" + runtimeRenewAgentCertMethod
	runtimeReportAgentPath           = "/" + runtimeServiceName + "/" + runtimeReportAgentMethod
	runtimeGetAgentPolicyPath        = "/" + runtimeServiceName + "/" + runtimeGetAgentMetricsPolicy
	runtimeReportAgentMetricsPath    = "/" + runtimeServiceName + "/" + runtimeReportAgentMetrics
	runtimeGetHostRoutingPath        = "/" + runtimeServiceName + "/" + runtimeGetHostRoutingSnapshot
)

type RuntimeTransportService struct {
	runtimeSvc *runtimesvc.RuntimeBootstrapService
}

type runtimeTransportServer interface {
	GetRuntimeBootstrap(context.Context, *structpb.Struct) (*structpb.Struct, error)
	BootstrapModuleClient(context.Context, *structpb.Struct) (*structpb.Struct, error)
	BootstrapAgent(context.Context, *structpb.Struct) (*structpb.Struct, error)
	RenewAgentCertificate(context.Context, *structpb.Struct) (*structpb.Struct, error)
	ReportAgentHeartbeat(context.Context, *structpb.Struct) (*structpb.Struct, error)
	GetAgentMetricsPolicy(context.Context, *structpb.Struct) (*structpb.Struct, error)
	ReportAgentMetrics(context.Context, *structpb.Struct) (*structpb.Struct, error)
	GetHostRoutingSnapshot(context.Context, *structpb.Struct) (*structpb.Struct, error)
}

func NewRuntimeTransportService(runtimeSvc *runtimesvc.RuntimeBootstrapService) *RuntimeTransportService {
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

	config, err := s.runtimeSvc.BuildStructuredRuntimeConfig(ctx, runtimesvc.RuntimeBootstrapRequest{
		ModuleName: moduleName,
		AppPort:    appPort,
		ConfigKeys: readStructStringList(req, "config_keys"),
	})
	if err != nil {
		switch {
		case isRuntimeBootstrapRequestError(err):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case isRuntimeBootstrapStateError(err):
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		default:
			return nil, status.Error(codes.Internal, "failed to build runtime bootstrap config")
		}
	}
	res, err := structpb.NewStruct(map[string]any{
		"config": config,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to build grpc response payload")
	}
	return res, nil
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
				MethodName: runtimeBootstrapModuleMethod,
				Handler:    runtimeBootstrapModuleHandler,
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

func readStructStringList(req *structpb.Struct, key string) []string {
	if req == nil {
		return nil
	}
	field, ok := req.GetFields()[key]
	if !ok || field == nil || field.GetListValue() == nil {
		return nil
	}
	items := field.GetListValue().GetValues()
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		value := strings.TrimSpace(item.GetStringValue())
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
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

func isRuntimeBootstrapRequestError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "module_name is required") ||
		strings.Contains(msg, "config_keys is required") ||
		strings.Contains(msg, "unsupported config key") ||
		strings.Contains(msg, "unsupported config group") ||
		strings.Contains(msg, "app_port is invalid")
}

func isRuntimeBootstrapStateError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "runtime bootstrap validation failed") ||
		strings.Contains(msg, "endpoint not found") ||
		strings.Contains(msg, "resolve ") ||
		strings.Contains(msg, "has no port") ||
		strings.Contains(msg, "port is invalid")
}
