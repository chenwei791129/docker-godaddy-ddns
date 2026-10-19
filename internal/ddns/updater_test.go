package ddns

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chenwei791129/docker-godaddy-ddns/internal/config"
	"github.com/chenwei791129/docker-godaddy-ddns/internal/godaddy"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// everyHour schedules checks an hour apart.
type everyHour struct{}

func (everyHour) Next(t time.Time) time.Time { return t.Add(time.Hour) }

// fakeDNS records calls and returns canned results.
type fakeDNS struct {
	records    []godaddy.Record
	getErr     error
	replaceErr error
	onGet      func() // runs inside GetRecords, e.g. to simulate a shutdown

	gets     int
	replaced [][]godaddy.Record
}

func (f *fakeDNS) GetRecords(context.Context, string, string, string) ([]godaddy.Record, error) {
	f.gets++
	if f.onGet != nil {
		f.onGet()
	}
	return f.records, f.getErr
}

func (f *fakeDNS) ReplaceRecords(_ context.Context, _, _, _ string, records []godaddy.Record) error {
	f.replaced = append(f.replaced, records)
	return f.replaceErr
}

type hookCall struct {
	name    string
	command string
	event   hookEvent
}

// ipServer serves whatever address is currently stored in ip.
type ipServer struct {
	*httptest.Server
	mu sync.Mutex
	ip string
}

func newIPServer(t *testing.T, ip string) *ipServer {
	s := &ipServer{ip: ip}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.ip == "" {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, s.ip+"\n")
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *ipServer) set(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ip = ip
}

func newTestUpdater(t *testing.T, dns *fakeDNS, checkURL string) (*Updater, *[]hookCall) {
	t.Helper()
	var calls []hookCall
	u := &Updater{
		cfg: config.Config{
			Domain:      "example.com",
			Name:        "home",
			Type:        "A",
			TTL:         600,
			CheckURL:    checkURL,
			SuccessExec: "notify success",
			FailedExec:  "notify failure",
		},
		dns:      dns,
		ipClient: NewIPv4HTTPClient(5 * time.Second),
		runHook: func(_ context.Context, name, command string, event hookEvent) {
			calls = append(calls, hookCall{name, command, event})
		},
	}
	return u, &calls
}

func TestRunOnceUpdatesChangedRecord(t *testing.T) {
	ips := newIPServer(t, "198.51.100.4")
	dns := &fakeDNS{records: []godaddy.Record{{Data: "203.0.113.7", TTL: 600}}}
	u, calls := newTestUpdater(t, dns, ips.URL)

	u.RunOnce(context.Background())

	if len(dns.replaced) != 1 || dns.replaced[0][0] != (godaddy.Record{Data: "198.51.100.4", TTL: 600}) {
		t.Fatalf("replaced = %+v", dns.replaced)
	}
	want := []hookCall{{"success hook", "notify success", hookEvent{PublicIP: "198.51.100.4", PreviousIP: "203.0.113.7"}}}
	if len(*calls) != 1 || (*calls)[0] != want[0] {
		t.Errorf("hooks = %+v, want %+v", *calls, want)
	}
}

func TestRunOnceSkipsUpToDateRecord(t *testing.T) {
	ips := newIPServer(t, "203.0.113.7")
	dns := &fakeDNS{records: []godaddy.Record{{Data: "203.0.113.7"}}}
	u, calls := newTestUpdater(t, dns, ips.URL)

	u.RunOnce(context.Background())

	if len(dns.replaced) != 0 || len(*calls) != 0 {
		t.Errorf("replaced = %+v, hooks = %+v; want no update and no hook", dns.replaced, *calls)
	}
}

func TestRunOnceCachesConfirmedIP(t *testing.T) {
	ips := newIPServer(t, "203.0.113.7")
	dns := &fakeDNS{records: []godaddy.Record{{Data: "203.0.113.7"}}}
	u, _ := newTestUpdater(t, dns, ips.URL)

	u.RunOnce(context.Background())
	u.RunOnce(context.Background())
	if dns.gets != 1 {
		t.Errorf("GoDaddy queried %d times, want once while the public IP is unchanged", dns.gets)
	}

	ips.set("198.51.100.4")
	u.RunOnce(context.Background())
	if dns.gets != 2 || len(dns.replaced) != 1 {
		t.Errorf("after an IP change: gets = %d, replaced = %d; want 2 and 1", dns.gets, len(dns.replaced))
	}
}

func TestRunOnceReplacesMultipleRecords(t *testing.T) {
	ips := newIPServer(t, "203.0.113.7")
	dns := &fakeDNS{records: []godaddy.Record{{Data: "203.0.113.7"}, {Data: "203.0.113.8"}}}
	u, _ := newTestUpdater(t, dns, ips.URL)

	u.RunOnce(context.Background())

	if len(dns.replaced) != 1 || len(dns.replaced[0]) != 1 {
		t.Errorf("replaced = %+v, want the records replaced by a single one", dns.replaced)
	}
}

func TestRunOnceCreatesMissingRecord(t *testing.T) {
	ips := newIPServer(t, "203.0.113.7")
	dns := &fakeDNS{}
	u, calls := newTestUpdater(t, dns, ips.URL)

	u.RunOnce(context.Background())

	if len(dns.replaced) != 1 || len(*calls) != 1 || (*calls)[0].event.PreviousIP != "" {
		t.Errorf("replaced = %+v, hooks = %+v", dns.replaced, *calls)
	}
}

func TestRunOnceUpdatesWhenReadFails(t *testing.T) {
	ips := newIPServer(t, "203.0.113.7")
	dns := &fakeDNS{getErr: errors.New("timeout")}
	u, calls := newTestUpdater(t, dns, ips.URL)

	u.RunOnce(context.Background())

	if len(dns.replaced) != 1 || len(*calls) != 1 || (*calls)[0].name != "success hook" {
		t.Errorf("replaced = %+v, hooks = %+v", dns.replaced, *calls)
	}
}

func TestRunOnceUpdateFailure(t *testing.T) {
	ips := newIPServer(t, "198.51.100.4")
	dns := &fakeDNS{
		records:    []godaddy.Record{{Data: "203.0.113.7"}},
		replaceErr: &godaddy.APIError{StatusCode: 401, Code: "UNABLE_TO_AUTHENTICATE"},
	}
	u, calls := newTestUpdater(t, dns, ips.URL)

	u.RunOnce(context.Background())

	if len(*calls) != 1 {
		t.Fatalf("hooks = %+v, want one failure hook", *calls)
	}
	call := (*calls)[0]
	if call.name != "failure hook" || call.command != "notify failure" ||
		call.event.PublicIP != "198.51.100.4" || call.event.PreviousIP != "203.0.113.7" ||
		!strings.Contains(call.event.FailureReason, "UNABLE_TO_AUTHENTICATE") {
		t.Errorf("hook = %+v", call)
	}
	if u.lastIP != "" {
		t.Errorf("lastIP = %q, want it unset so the next check retries", u.lastIP)
	}
}

func TestRunOncePublicIPFailure(t *testing.T) {
	ips := newIPServer(t, "")
	dns := &fakeDNS{}
	u, calls := newTestUpdater(t, dns, ips.URL)

	u.RunOnce(context.Background())

	if dns.gets != 0 || len(dns.replaced) != 0 {
		t.Errorf("GoDaddy called without a public IP: gets = %d, replaced = %d", dns.gets, len(dns.replaced))
	}
	if len(*calls) != 1 || (*calls)[0].name != "failure hook" ||
		(*calls)[0].event.PublicIP != "" || !strings.Contains((*calls)[0].event.FailureReason, "public IP") {
		t.Errorf("hooks = %+v", *calls)
	}
}

func TestRunStopsWhenContextIsCancelled(t *testing.T) {
	ips := newIPServer(t, "203.0.113.7")
	dns := &fakeDNS{records: []godaddy.Record{{Data: "203.0.113.7"}}}
	u, _ := newTestUpdater(t, dns, ips.URL)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		u.Run(ctx, everyHour{})
		close(done)
	}()
	time.Sleep(200 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after the context was cancelled")
	}
	if dns.gets != 1 {
		t.Errorf("gets = %d, want the immediate first check only", dns.gets)
	}
}

// TestEndToEnd wires the real GoDaddy client and real shell hooks to fake
// GoDaddy and IP check servers.
func TestEndToEnd(t *testing.T) {
	ips := newIPServer(t, "198.51.100.4")

	var mu sync.Mutex
	current := "203.0.113.7"
	var requests []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, r.Method+" "+r.URL.EscapedPath()+" "+r.Header.Get("Authorization")+" "+string(body))
		switch r.Method {
		case http.MethodGet:
			_, _ = io.WriteString(w, `[{"data":"`+current+`","name":"home","ttl":600,"type":"A"}]`)
		case http.MethodPut:
			current = "198.51.100.4"
		}
	}))
	defer api.Close()

	out := filepath.Join(t.TempDir(), "hook")
	t.Setenv("HOOK_OUT", out)
	cfg, _, err := config.Load(envFrom(map[string]string{
		"GODADDY_PAT":  "pat-123",
		"DOMAIN":       "example.com",
		"NAME":         "home",
		"CHECK_URL":    ips.URL,
		"SUCCESS_EXEC": `printf '%s->%s' "$PREVIOUS_IP" "$PUBLIC_IP" > "$HOOK_OUT"`,
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	u := &Updater{
		cfg:      cfg,
		dns:      &godaddy.Client{BaseURL: api.URL, Credential: cfg.Credential},
		ipClient: NewIPv4HTTPClient(5 * time.Second),
		runHook:  runHook,
	}

	u.RunOnce(context.Background())
	u.RunOnce(context.Background()) // unchanged public IP: no GoDaddy calls

	want := []string{
		"GET /v1/domains/example.com/records/A/home Bearer pat-123 ",
		`PUT /v1/domains/example.com/records/A/home Bearer pat-123 [{"data":"198.51.100.4","ttl":600}]`,
	}
	if strings.Join(requests, "\n") != strings.Join(want, "\n") {
		t.Errorf("requests:\n%s\nwant:\n%s", strings.Join(requests, "\n"), strings.Join(want, "\n"))
	}
	if got, _ := os.ReadFile(out); string(got) != "203.0.113.7->198.51.100.4" {
		t.Errorf("success hook wrote %q", got)
	}
}

func TestRunOnceShutdownIsNotAFailure(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(blocked)
	}))
	defer srv.Close()
	dns := &fakeDNS{}
	u, calls := newTestUpdater(t, dns, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	u.RunOnce(ctx)
	<-blocked

	if len(*calls) != 0 {
		t.Errorf("hooks = %+v, want none when the check is interrupted by shutdown", *calls)
	}
}

func TestRunOnceShutdownWhileReadingRecord(t *testing.T) {
	ips := newIPServer(t, "198.51.100.4")
	ctx, cancel := context.WithCancel(context.Background())
	dns := &fakeDNS{getErr: context.Canceled, onGet: cancel}
	u, calls := newTestUpdater(t, dns, ips.URL)

	u.RunOnce(ctx)

	if len(dns.replaced) != 0 || len(*calls) != 0 {
		t.Errorf("replaced = %+v, hooks = %+v; want no update attempt and no hook during shutdown", dns.replaced, *calls)
	}
}
