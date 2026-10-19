package ddns

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// NewIPv4HTTPClient returns an HTTP client that only connects over IPv4, so a
// dual-stack host reports the address the A record needs rather than an IPv6
// one.
func NewIPv4HTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp4", addr)
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

// fetchPublicIP asks the IP check service at checkURL for the Public IP. The
// response body must be exactly one IPv4 address, optionally surrounded by
// whitespace.
func fetchPublicIP(ctx context.Context, client *http.Client, checkURL string) (netip.Addr, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return netip.Addr{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return netip.Addr{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return netip.Addr{}, fmt.Errorf("IP check service returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return netip.Addr{}, err
	}
	text := strings.TrimSpace(string(body))
	addr, err := netip.ParseAddr(text)
	if err != nil || !addr.Is4() {
		return netip.Addr{}, fmt.Errorf("IP check service returned %q, which is not an IPv4 address", truncate(text, 64))
	}
	return addr, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
