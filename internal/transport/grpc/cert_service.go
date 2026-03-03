package grpc

import (
	"admin/internal/service"
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	certServiceName      = "admin.transport.cert.v1.CertService"
	certUploadMethodName = "UploadCert"
	certUploadMethodPath = "/" + certServiceName + "/" + certUploadMethodName
)

type UploadCertRequest struct {
	ObjectID string `json:"object_id"`
	CertType string `json:"cert_type"`
	Content  string `json:"content"`
}

type UploadCertResponse struct {
	Key string `json:"key"`
}

type CertTransportService struct {
	certStore *service.CertStoreService
}

type certTransportServer interface {
	UploadCert(context.Context, *UploadCertRequest) (*UploadCertResponse, error)
}

func NewCertTransportService(certStore *service.CertStoreService) *CertTransportService {
	return &CertTransportService{certStore: certStore}
}

func (s *CertTransportService) UploadCert(
	ctx context.Context,
	req *UploadCertRequest,
) (*UploadCertResponse, error) {
	if s == nil || s.certStore == nil {
		return nil, status.Error(codes.Unavailable, "cert store service unavailable")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}

	objectID, err := uuid.Parse(strings.TrimSpace(req.ObjectID))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid object_id")
	}

	key, err := s.certStore.UploadCert(ctx, objectID, req.CertType, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrCertTypeInvalid),
			errors.Is(err, service.ErrObjectIDInvalid):
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, service.ErrCertStoreServiceNil):
			return nil, status.Error(codes.Unavailable, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &UploadCertResponse{Key: key}, nil
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
	in := new(UploadCertRequest)
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
		return base.UploadCert(currentCtx, req.(*UploadCertRequest))
	}
	return interceptor(ctx, in, info, handler)
}
