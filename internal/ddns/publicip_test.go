package ddns

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchPublicIP(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr string
	}{
		{"plain address", http.StatusOK, "203.0.113.7", "203.0.113.7", ""},
		{"address with newline", http.StatusOK, "203.0.113.7\n", "203.0.113.7", ""},
		{"IPv6 address", http.StatusOK, "2001:db8::1", "", "not an IPv4 address"},
		{"IPv4-mapped IPv6 address", http.StatusOK, "::ffff:203.0.113.7", "", "not an IPv4 address"},
		{"HTML page", http.StatusOK, "<html>Your IP is 203.0.113.7</html>", "", "not an IPv4 address"},
		{"two addresses", http.StatusOK, "203.0.113.7\n203.0.113.8", "", "not an IPv4 address"},
		{"server error", http.StatusServiceUnavailable, "203.0.113.7", "", "HTTP 503"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer srv.Close()

			addr, err := fetchPublicIP(context.Background(), NewIPv4HTTPClient(5*time.Second), srv.URL)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("fetchPublicIP: %v", err)
			}
			if addr.String() != tt.want {
				t.Errorf("address = %s, want %s", addr, tt.want)
			}
		})
	}
}

func TestIPv4ClientRefusesIPv6Only(t *testing.T) {
	// An IPv6 loopback listener is unreachable for a tcp4-only dialer.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "203.0.113.7")
	}))
	l, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	srv.Listener = l
	srv.Start()
	defer srv.Close()

	if _, err := fetchPublicIP(context.Background(), NewIPv4HTTPClient(5*time.Second), srv.URL); err == nil {
		t.Error("expected the IPv4-only client to fail against an IPv6-only server")
	}
}
