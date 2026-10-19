// Package config reads and validates the updater configuration from the
// environment, including the Deprecated Settings kept for 1.x.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/chenwei791129/docker-godaddy-ddns/internal/godaddy"
)

const (
	// DefaultCheckURL is the IP check service used when CHECK_URL is unset.
	DefaultCheckURL      = "https://whatismyip.akamai.com/"
	defaultCheckInterval = 5 * time.Minute
	// minTTL is the lowest TTL the updater sends; smaller values are raised,
	// as the original shell script did.
	minTTL = 600
)

// Schedule decides when the next update check runs.
type Schedule interface {
	Next(time.Time) time.Time
}

type intervalSchedule time.Duration

func (s intervalSchedule) Next(t time.Time) time.Time { return t.Add(time.Duration(s)) }

// Config is the validated configuration read from the environment.
type Config struct {
	Credential     godaddy.Credential
	CredentialKind string
	Domain         string
	Name           string
	Type           string
	TTL            int
	CheckURL       string
	Schedule       Schedule
	ScheduleDesc   string
	SuccessExec    string
	FailedExec     string
}

// Load reads the configuration through getenv. It returns warnings for
// Deprecated Settings and other non-fatal problems, to be logged once.
func Load(getenv func(string) string) (Config, []string, error) {
	var cfg Config
	var warnings []string
	get := func(name string) string { return strings.TrimSpace(getenv(name)) }

	credential, kind, credentialWarnings, err := loadCredential(get)
	if err != nil {
		return cfg, nil, err
	}
	cfg.Credential, cfg.CredentialKind = credential, kind
	warnings = append(warnings, credentialWarnings...)

	cfg.Domain = get("DOMAIN")
	if cfg.Domain == "" {
		return cfg, nil, errors.New("DOMAIN is required")
	}
	cfg.Name = valueOr(get("NAME"), "@")
	cfg.Type = strings.ToUpper(valueOr(get("TYPE"), "A"))
	if cfg.Type != "A" {
		return cfg, nil, fmt.Errorf("TYPE %q is not supported: only A records (IPv4) are", cfg.Type)
	}

	cfg.TTL = minTTL
	if v := get("TTL"); v != "" {
		ttl, err := strconv.Atoi(v)
		if err != nil {
			return cfg, nil, fmt.Errorf("TTL must be a number of seconds, got %q", v)
		}
		if ttl < minTTL {
			warnings = append(warnings, fmt.Sprintf("TTL %d is below %d seconds; using %d", ttl, minTTL, minTTL))
			ttl = minTTL
		}
		cfg.TTL = ttl
	}

	cfg.CheckURL = valueOr(get("CHECK_URL"), DefaultCheckURL)
	if !strings.Contains(cfg.CheckURL, "://") {
		// curl, which the original script used, treats a bare host as http.
		cfg.CheckURL = "http://" + cfg.CheckURL
	}
	u, err := url.Parse(cfg.CheckURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return cfg, nil, fmt.Errorf("CHECK_URL must be an http or https URL, got %q", cfg.CheckURL)
	}
	if u.Scheme == "http" {
		warnings = append(warnings, "CHECK_URL uses plain HTTP, so the reported public IP can be tampered with in transit; prefer an https URL")
	}

	sched, desc, scheduleWarnings, err := loadSchedule(get)
	if err != nil {
		return cfg, nil, err
	}
	cfg.Schedule, cfg.ScheduleDesc = sched, desc
	warnings = append(warnings, scheduleWarnings...)

	// Hooks are shell commands: keep them verbatim.
	cfg.SuccessExec = getenv("SUCCESS_EXEC")
	cfg.FailedExec = getenv("FAILED_EXEC")
	if strings.Contains(cfg.SuccessExec, "PublicIP") || strings.Contains(cfg.FailedExec, "PublicIP") {
		warnings = append(warnings, "the PublicIP hook variable is deprecated and the next major version drops it; use PUBLIC_IP instead")
	}
	return cfg, warnings, nil
}

func loadCredential(get func(string) string) (godaddy.Credential, string, []string, error) {
	pat, key, secret := get("GODADDY_PAT"), get("GODADDY_KEY"), get("GODADDY_SECRET")
	switch {
	case pat != "":
		var warnings []string
		if key != "" || secret != "" {
			warnings = append(warnings, "GODADDY_PAT is set, so GODADDY_KEY and GODADDY_SECRET are ignored")
		}
		return godaddy.PersonalAccessToken(pat), "personal access token", warnings, nil
	case key != "" && secret != "":
		return godaddy.ClassicDeveloperKey{Key: key, Secret: secret}, "classic developer key",
			[]string{"GODADDY_KEY and GODADDY_SECRET are deprecated: GoDaddy is retiring this credential for the Domains API and the next major version drops it. Create a personal access token with the domains.dns:update scope and set GODADDY_PAT instead."},
			nil
	case key != "" || secret != "":
		return nil, "", nil, errors.New("GODADDY_KEY and GODADDY_SECRET must be set together (or set GODADDY_PAT instead)")
	default:
		return nil, "", nil, errors.New("set GODADDY_PAT to a GoDaddy personal access token (or the deprecated GODADDY_KEY and GODADDY_SECRET)")
	}
}

func loadSchedule(get func(string) string) (Schedule, string, []string, error) {
	interval, cronTime := get("CHECK_INTERVAL"), get("CRON_TIME")
	if interval != "" {
		d, err := time.ParseDuration(interval)
		if err != nil || d <= 0 {
			return nil, "", nil, fmt.Errorf("CHECK_INTERVAL must be a positive duration such as 5m, got %q", interval)
		}
		var warnings []string
		if cronTime != "" {
			warnings = append(warnings, "CHECK_INTERVAL is set, so CRON_TIME is ignored")
		}
		return intervalSchedule(d), "every " + d.String(), warnings, nil
	}
	if cronTime != "" {
		sched, err := cron.ParseStandard(cronTime)
		if err != nil {
			return nil, "", nil, fmt.Errorf("CRON_TIME %q is not a valid cron expression: %w", cronTime, err)
		}
		// robfig/cron returns the zero time for a schedule that never fires
		// (e.g. February 30th), which would make the updater loop without
		// pause.
		if sched.Next(time.Now()).IsZero() {
			return nil, "", nil, fmt.Errorf("CRON_TIME %q never matches a date", cronTime)
		}
		return sched, "on cron schedule " + strconv.Quote(cronTime),
			[]string{"CRON_TIME is deprecated and the next major version drops it; use CHECK_INTERVAL (for example CHECK_INTERVAL=5m) instead"},
			nil
	}
	return intervalSchedule(defaultCheckInterval), "every " + defaultCheckInterval.String(), nil, nil
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
