package grpc

import (
	"admin/internal/service"
	"admin/pkg/errorvar"
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	certServiceName      = "admin.transport.cert.v1.CertService"
	certUploadMethodName = "UploadCert"
	certUploadMethodPath = "/" + certServiceName + "/" + certUploadMethodName

	certFieldObjectID = "object_id"
	certFieldCertType = "cert_type"
	certFieldContent  = "content"
	certFieldKey      = "key"
)

type CertTransportService struct {
	certStore *service.CertStoreService
}

type certTransportServer interface {
	UploadCert(context.Context, *structpb.Struct) (*structpb.Struct, error)
}

func NewCertTransportService(certStore *service.CertStoreService) *CertTransportService {
	return &CertTransportService{certStore: certStore}
}

func (s *CertTransportService) UploadCert(
	ctx context.Context,
	req *structpb.Struct,
) (*structpb.Struct, error) {
	if s == nil || s.certStore == nil {
		return nil, status.Error(codes.Unavailable, "cert store service unavailable")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}

	objectID, err := uuid.Parse(strings.TrimSpace(readStringField(req, certFieldObjectID)))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid object_id")
	}

	certType := readStringField(req, certFieldCertType)
	content := readStringField(req, certFieldContent)

	key, err := s.certStore.UploadCert(ctx, objectID, certType, content)
	if err != nil {
		switch {
		case errors.Is(err, errorvar.ErrCertTypeInvalid),
			errors.Is(err, errorvar.ErrObjectIDInvalid):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, errorvar.ErrCertStoreServiceNil):
			return nil, status.Error(codes.Unavailable, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	payload, err := structpb.NewStruct(map[string]any{
		certFieldKey: key,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to build grpc response payload")
	}

	return payload, nil
}

func RegisterCertTransportServer(server *gogrpc.Server, svc *CertTransportService) {
	if server == nil || svc == nil {
		return
	}
	server.RegisterService(&gogrpc.ServiceDesc{
		ServiceName: certServiceName,
		HandlerType: (*certTransportServer)(nil),
		Methods: []gogrpc.MethodDesc{
			{
				MethodName: certUploadMethodName,
				Handler:    certUploadHandler,
			},
		},
		Streams:  []gogrpc.StreamDesc{},
		Metadata: "cert_service",
	}, svc)
}

func certUploadHandler(
	srv interface{},
	ctx context.Context,
	dec func(interface{}) error,
	interceptor gogrpc.UnaryServerInterceptor,
) (interface{}, error) {
	in := &structpb.Struct{}
	if err := dec(in); err != nil {
		return nil, err
	}

	base := srv.(*CertTransportService)
	if interceptor == nil {
		return base.UploadCert(ctx, in)
	}

	info := &gogrpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: certUploadMethodPath,
	}
	handler := func(currentCtx context.Context, req interface{}) (interface{}, error) {
		return base.UploadCert(currentCtx, req.(*structpb.Struct))
	}
	return interceptor(ctx, in, info, handler)
}

func readStringField(req *structpb.Struct, key string) string {
	if req == nil {
		return ""
	}
	field, ok := req.GetFields()[key]
	if !ok || field == nil {
		return ""
	}
	return strings.TrimSpace(field.GetStringValue())
}
