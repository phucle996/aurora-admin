package grpc

import (
	"context"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

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

func runtimeBootstrapModuleHandler(
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
		return base.BootstrapModuleClient(ctx, in)
	}

	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: runtimeBootstrapModulePath,
	}
	handler := func(currentCtx context.Context, req interface{}) (interface{}, error) {
		return base.BootstrapModuleClient(currentCtx, req.(*structpb.Struct))
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
