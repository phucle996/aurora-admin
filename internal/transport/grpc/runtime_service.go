package grpc

import (
	"admin/internal/service"
	"context"
	"errors"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	runtimeServiceName           = "admin.transport.runtime.v1.RuntimeService"
	runtimeGetUMSBootstrapMethod = "GetUMSBootstrap"
	runtimeGetUMSBootstrapPath   = "/" + runtimeServiceName + "/" + runtimeGetUMSBootstrapMethod
)

type RuntimeTransportService struct {
	runtimeSvc *service.RuntimeBootstrapService
}

type runtimeTransportServer interface {
	GetUMSBootstrap(context.Context, *structpb.Struct) (*structpb.Struct, error)
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

func extractClientCertDER(ctx context.Context) ([]byte, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.AuthInfo == nil {
		return nil, errors.New("missing peer auth info")
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, errors.New("invalid peer auth type")
	}
	if len(tlsInfo.State.PeerCertificates) == 0 || tlsInfo.State.PeerCertificates[0] == nil {
		return nil, errors.New("missing peer certificate")
	}
	return tlsInfo.State.PeerCertificates[0].Raw, nil
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
		},
		Streams:  []gogrpc.StreamDesc{},
		Metadata: "runtime_service",
	}, svc)
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
