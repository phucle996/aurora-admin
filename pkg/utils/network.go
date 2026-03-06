package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func NormalizeAddress(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" {
		return ""
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return strings.TrimSpace(parsedHost)
	}
	if strings.Contains(host, "://") {
		host = strings.SplitN(host, "://", 2)[1]
	}
	if cut, _, found := strings.Cut(host, "/"); found {
		host = cut
	}
	if h, _, found := strings.Cut(host, ":"); found {
		host = h
	}
	return strings.Trim(host, "[]")
}

func IsValidHost(host string) bool {
	clean := strings.TrimSpace(host)
	if clean == "" {
		return false
	}
	if ip := net.ParseIP(clean); ip != nil {
		return true
	}
	if len(clean) > 253 {
		return false
	}
	labels := strings.Split(clean, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func EndpointHost(endpoint string) string {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return ""
	}

	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil {
			return strings.TrimSpace(parsed.Hostname())
		}
	}

	working := raw
	if cut, _, found := strings.Cut(working, "/"); found {
		working = cut
	}

	if strings.Count(working, ":") == 1 {
		if host, _, err := net.SplitHostPort(working); err == nil {
			return NormalizeAddress(host)
		}
	}

	return NormalizeAddress(working)
}

func EndpointPort(endpoint string) string {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil {
			return parsed.Port()
		}
	}
	working := raw
	if cut, _, found := strings.Cut(working, "/"); found {
		working = cut
	}
	if host, port, err := net.SplitHostPort(working); err == nil && NormalizeAddress(host) != "" {
		return strings.TrimSpace(port)
	}
	return ""
}

func RandomAvailableLocalPort() (int32, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("cannot allocate random app port: %w", err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 || addr.Port > 65535 {
		return 0, fmt.Errorf("cannot resolve random app port")
	}
	return int32(addr.Port), nil
}
