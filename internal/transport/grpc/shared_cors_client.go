package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

var sharedCorsRPCMethods = []string{
	"/aurora.transport.runtime.v1.RuntimeService/ApplySharedCORS",
	"/admin.transport.runtime.v1.RuntimeService/ApplySharedCORS",
}

type SharedCORSClient interface {
	ApplySharedCORS(ctx context.Context, endpoint string, corsValues map[string]string) error
}

type sharedCORSClient struct{}

func NewSharedCORSClient() SharedCORSClient {
	return &sharedCORSClient{}
}

func (c *sharedCORSClient) ApplySharedCORS(
	ctx context.Context,
	endpoint string,
	corsValues map[string]string,
) error {
	dialAddress, serverName, err := resolveGRPCDialAddress(endpoint)
	if err != nil {
		return err
	}

	req, err := buildSharedCORSRequest(corsValues)
	if err != nil {
		return err
	}

	// Do not force mTLS here; try h2c first, then one-way TLS.
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion:         tls.VersionTLS12,
			ServerName:         strings.TrimSpace(serverName),
			InsecureSkipVerify: true,
		})),
	}

	var errs []string
	for _, dialOpt := range opts {
		callErr := invokeSharedCORS(ctx, dialAddress, dialOpt, req)
		if callErr == nil {
			return nil
		}
		errs = append(errs, callErr.Error())
	}

	return fmt.Errorf("shared cors rpc push failed: %s", strings.Join(errs, " | "))
}

func buildSharedCORSRequest(corsValues map[string]string) (*structpb.Struct, error) {
	valuesAny := make(map[string]any, len(corsValues))
	for k, v := range corsValues {
		valuesAny[k] = v
	}

	req, err := structpb.NewStruct(map[string]any{
		"values":    valuesAny,
		"pushed_at": time.Now().UTC().Format(time.RFC3339Nano),
		"source":    "admin",
	})
	if err != nil {
		return nil, fmt.Errorf("build request failed: %w", err)
	}
	return req, nil
}

func invokeSharedCORS(
	ctx context.Context,
	dialAddress string,
	dialOpt grpc.DialOption,
	req *structpb.Struct,
) error {
	conn, err := grpc.DialContext(
		ctx,
		dialAddress,
		dialOpt,
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer conn.Close()

	var lastErr error
	for _, method := range sharedCorsRPCMethods {
		res := &structpb.Struct{}
		callErr := conn.Invoke(ctx, method, req, res)
		if callErr == nil {
			return nil
		}
		st, ok := status.FromError(callErr)
		if ok && (st.Code() == codes.Unimplemented || st.Code() == codes.NotFound) {
			lastErr = callErr
			continue
		}
		return callErr
	}

	if lastErr != nil {
		return fmt.Errorf("service does not support shared cors rpc method")
	}
	return fmt.Errorf("shared cors rpc push failed")
}

func resolveGRPCDialAddress(endpoint string) (address string, serverName string, err error) {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return "", "", fmt.Errorf("endpoint is empty")
	}

	if strings.Contains(raw, "://") {
		parsed, parseErr := url.Parse(raw)
		if parseErr != nil {
			return "", "", fmt.Errorf("invalid endpoint: %w", parseErr)
		}
		host := strings.TrimSpace(parsed.Hostname())
		port := strings.TrimSpace(parsed.Port())
		if host == "" {
			return "", "", fmt.Errorf("endpoint host is empty")
		}
		if port == "" {
			switch strings.ToLower(strings.TrimSpace(parsed.Scheme)) {
			case "https", "grpcs", "tls":
				port = "443"
			default:
				port = "80"
			}
		}
		return net.JoinHostPort(host, port), host, nil
	}

	if host, port, splitErr := net.SplitHostPort(raw); splitErr == nil {
		cleanHost := strings.Trim(strings.TrimSpace(host), "[]")
		cleanPort := strings.TrimSpace(port)
		if cleanHost == "" || cleanPort == "" {
			return "", "", fmt.Errorf("endpoint host/port is invalid")
		}
		return net.JoinHostPort(cleanHost, cleanPort), cleanHost, nil
	}

	host := endpointHost(raw)
	port := endpointPort(raw)
	if host == "" || port == "" {
		return "", "", fmt.Errorf("endpoint must be host:port")
	}
	return net.JoinHostPort(host, port), host, nil
}

func endpointHost(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return ""
		}
		return strings.Trim(strings.TrimSpace(parsed.Hostname()), "[]")
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(strings.TrimSpace(host), "[]")
	}
	if idx := strings.LastIndex(value, ":"); idx > 0 {
		return strings.Trim(strings.TrimSpace(value[:idx]), "[]")
	}
	return strings.Trim(strings.TrimSpace(value), "[]")
}

func endpointPort(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(parsed.Port())
	}
	if _, port, err := net.SplitHostPort(value); err == nil {
		return strings.TrimSpace(port)
	}
	if idx := strings.LastIndex(value, ":"); idx > 0 && idx < len(value)-1 {
		return strings.TrimSpace(value[idx+1:])
	}
	return ""
}
