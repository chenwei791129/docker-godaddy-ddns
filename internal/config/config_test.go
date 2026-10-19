package config

import (
	"strings"
	"testing"
	"time"

	"github.com/chenwei791129/docker-godaddy-ddns/internal/godaddy"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// base returns a minimal valid environment with a personal access token.
func base(extra map[string]string) map[string]string {
	env := map[string]string{"GODADDY_PAT": "pat", "DOMAIN": "example.com"}
	for k, v := range extra {
		env[k] = v
	}
	return env
}

func hasWarning(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, warnings, err := Load(envFrom(base(nil)))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if cfg.Name != "@" || cfg.Type != "A" || cfg.TTL != 600 || cfg.CheckURL != DefaultCheckURL {
		t.Errorf("defaults = %+v", cfg)
	}
	if got := cfg.Schedule.Next(time.Unix(0, 0)).Sub(time.Unix(0, 0)); got != 5*time.Minute {
		t.Errorf("default interval = %v", got)
	}
}

func TestLoadCredential(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		want        godaddy.Credential
		wantWarning string
		wantErr     string
	}{
		{
			name: "personal access token",
			env:  map[string]string{"GODADDY_PAT": "pat"},
			want: godaddy.PersonalAccessToken("pat"),
		},
		{
			name:        "classic developer key is deprecated",
			env:         map[string]string{"GODADDY_KEY": "key", "GODADDY_SECRET": "secret"},
			want:        godaddy.ClassicDeveloperKey{Key: "key", Secret: "secret"},
			wantWarning: "deprecated",
		},
		{
			name:        "personal access token wins over the classic developer key",
			env:         map[string]string{"GODADDY_PAT": "pat", "GODADDY_KEY": "key", "GODADDY_SECRET": "secret"},
			want:        godaddy.PersonalAccessToken("pat"),
			wantWarning: "ignored",
		},
		{
			name:    "key without secret",
			env:     map[string]string{"GODADDY_KEY": "key"},
			wantErr: "must be set together",
		},
		{
			name:    "secret without key",
			env:     map[string]string{"GODADDY_SECRET": "secret"},
			wantErr: "must be set together",
		},
		{
			name:    "no credential",
			env:     map[string]string{},
			wantErr: "set GODADDY_PAT",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{"DOMAIN": "example.com"}
			for k, v := range tt.env {
				env[k] = v
			}
			cfg, warnings, err := Load(envFrom(env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Credential != tt.want {
				t.Errorf("credential = %#v, want %#v", cfg.Credential, tt.want)
			}
			if tt.wantWarning == "" && len(warnings) != 0 {
				t.Errorf("unexpected warnings: %v", warnings)
			}
			if tt.wantWarning != "" && !hasWarning(warnings, tt.wantWarning) {
				t.Errorf("warnings = %v, want one containing %q", warnings, tt.wantWarning)
			}
		})
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"missing domain", map[string]string{"GODADDY_PAT": "pat"}, "DOMAIN is required"},
		{"AAAA records", base(map[string]string{"TYPE": "AAAA"}), "only A records"},
		{"non-numeric TTL", base(map[string]string{"TTL": "ten"}), "TTL must be"},
		{"check URL with unsupported scheme", base(map[string]string{"CHECK_URL": "ftp://example.com/"}), "CHECK_URL must be"},
		{"invalid interval", base(map[string]string{"CHECK_INTERVAL": "5"}), "CHECK_INTERVAL must be"},
		{"zero interval", base(map[string]string{"CHECK_INTERVAL": "0s"}), "CHECK_INTERVAL must be"},
		{"invalid cron", base(map[string]string{"CRON_TIME": "every five minutes"}), "CRON_TIME"},
		{"cron that never fires", base(map[string]string{"CRON_TIME": "0 0 30 2 *"}), "never matches"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Load(envFrom(tt.env))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfigLowercaseType(t *testing.T) {
	cfg, _, err := Load(envFrom(base(map[string]string{"TYPE": "a"})))
	if err != nil || cfg.Type != "A" {
		t.Errorf("Type = %q, err = %v", cfg.Type, err)
	}
}

func TestLoadConfigTTLBelowMinimum(t *testing.T) {
	// As in the original script, anything below 600 is raised, including 0
	// and negative values.
	for _, ttl := range []string{"60", "0", "-1"} {
		cfg, warnings, err := Load(envFrom(base(map[string]string{"TTL": ttl})))
		if err != nil {
			t.Fatalf("TTL=%s: Load: %v", ttl, err)
		}
		if cfg.TTL != 600 || !hasWarning(warnings, "below 600") {
			t.Errorf("TTL=%s: TTL = %d, warnings = %v", ttl, cfg.TTL, warnings)
		}
	}
}

func TestLoadConfigCheckURLWithoutScheme(t *testing.T) {
	cfg, warnings, err := Load(envFrom(base(map[string]string{"CHECK_URL": "ifconfig.me/ip"})))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CheckURL != "http://ifconfig.me/ip" || !hasWarning(warnings, "plain HTTP") {
		t.Errorf("CheckURL = %q, warnings = %v", cfg.CheckURL, warnings)
	}
}

func TestLoadConfigWarnsAboutPublicIPHookVariable(t *testing.T) {
	_, warnings, err := Load(envFrom(base(map[string]string{"SUCCESS_EXEC": `echo "$PublicIP"`})))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !hasWarning(warnings, "PublicIP hook variable is deprecated") {
		t.Errorf("warnings = %v", warnings)
	}
}

func TestLoadConfigPlainHTTPCheckURL(t *testing.T) {
	cfg, warnings, err := Load(envFrom(base(map[string]string{"CHECK_URL": "http://whatismyip.akamai.com/"})))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CheckURL != "http://whatismyip.akamai.com/" || !hasWarning(warnings, "plain HTTP") {
		t.Errorf("CheckURL = %q, warnings = %v", cfg.CheckURL, warnings)
	}
}

func TestLoadSchedule(t *testing.T) {
	start := time.Date(2026, 1, 1, 10, 2, 30, 0, time.UTC)
	tests := []struct {
		name        string
		env         map[string]string
		wantNext    time.Time
		wantWarning string
	}{
		{"check interval", map[string]string{"CHECK_INTERVAL": "90s"}, start.Add(90 * time.Second), ""},
		{"deprecated cron time", map[string]string{"CRON_TIME": "*/5 * * * *"}, time.Date(2026, 1, 1, 10, 5, 0, 0, time.UTC), "CRON_TIME is deprecated"},
		{"check interval wins over cron time", map[string]string{"CHECK_INTERVAL": "1m", "CRON_TIME": "0 * * * *"}, start.Add(time.Minute), "CRON_TIME is ignored"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, warnings, err := Load(envFrom(base(tt.env)))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := cfg.Schedule.Next(start); !got.Equal(tt.wantNext) {
				t.Errorf("Next = %v, want %v", got, tt.wantNext)
			}
			if tt.wantWarning == "" && len(warnings) != 0 {
				t.Errorf("unexpected warnings: %v", warnings)
			}
			if tt.wantWarning != "" && !hasWarning(warnings, tt.wantWarning) {
				t.Errorf("warnings = %v, want one containing %q", warnings, tt.wantWarning)
			}
		})
	}
}

func TestLoadConfigKeepsHookCommandsVerbatim(t *testing.T) {
	cfg, _, err := Load(envFrom(base(map[string]string{"SUCCESS_EXEC": "  echo ok ", "FAILED_EXEC": "echo fail"})))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SuccessExec != "  echo ok " || cfg.FailedExec != "echo fail" {
		t.Errorf("hooks = %q / %q", cfg.SuccessExec, cfg.FailedExec)
	}
}
