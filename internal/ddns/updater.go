// Package ddns runs the update check: it determines the Public IP, keeps the
// Managed Record pointing at it and runs the Success and Failure Hooks.
package ddns

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/chenwei791129/docker-godaddy-ddns/internal/config"
	"github.com/chenwei791129/docker-godaddy-ddns/internal/godaddy"
)

// RecordAPI is the part of the GoDaddy client the updater needs.
type RecordAPI interface {
	GetRecords(ctx context.Context, domain, recordType, name string) ([]godaddy.Record, error)
	ReplaceRecords(ctx context.Context, domain, recordType, name string, records []godaddy.Record) error
}

// Updater keeps the Managed Record pointing at the Public IP.
type Updater struct {
	cfg      config.Config
	dns      RecordAPI
	ipClient *http.Client
	runHook  func(ctx context.Context, name, command string, event hookEvent)

	// lastIP is the Public IP the Managed Record was last confirmed or
	// updated to; while it is unchanged GoDaddy is not queried at all.
	lastIP string
}

// New returns an Updater for cfg that manages the record through dns and looks
// up the Public IP over IPv4.
func New(cfg config.Config, dns RecordAPI) *Updater {
	return &Updater{
		cfg:      cfg,
		dns:      dns,
		ipClient: NewIPv4HTTPClient(30 * time.Second),
		runHook:  runHook,
	}
}

// RecordLabel names the Managed Record in log messages.
func (u *Updater) RecordLabel() string {
	if u.cfg.Name == "@" {
		return u.cfg.Domain
	}
	return u.cfg.Name + "." + u.cfg.Domain
}

// apiTimeout bounds the IP check and GoDaddy calls of one update check. Hooks
// have their own limit.
const apiTimeout = 2 * time.Minute

// RunOnce performs one update check.
func (u *Updater) RunOnce(ctx context.Context) {
	apiCtx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	addr, err := fetchPublicIP(apiCtx, u.ipClient, u.cfg.CheckURL)
	if ctx.Err() != nil {
		return // shutting down: not a failure worth reporting
	}
	if err != nil {
		u.fail(ctx, hookEvent{FailureReason: fmt.Sprintf("could not determine the public IP from %s: %v", u.cfg.CheckURL, err)})
		return
	}
	ip := addr.String()
	if ip == u.lastIP {
		log.Printf("public IP %s unchanged, no update required", ip)
		return
	}

	previousIP := ""
	records, err := u.dns.GetRecords(apiCtx, u.cfg.Domain, u.cfg.Type, u.cfg.Name)
	if ctx.Err() != nil {
		return
	}
	switch {
	case err != nil:
		// Same as the original script: still try the update, which reports
		// its own error if GoDaddy is unreachable.
		log.Printf("could not read the current %s record of %s, updating anyway: %v", u.cfg.Type, u.RecordLabel(), err)
	case len(records) == 1 && records[0].Data == ip:
		u.lastIP = ip
		log.Printf("%s already points to %s, no update required", u.RecordLabel(), ip)
		return
	case len(records) > 0:
		previousIP = records[0].Data
	}

	err = u.dns.ReplaceRecords(apiCtx, u.cfg.Domain, u.cfg.Type, u.cfg.Name,
		[]godaddy.Record{{Data: ip, TTL: u.cfg.TTL}})
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		u.fail(ctx, hookEvent{
			PublicIP:      ip,
			PreviousIP:    previousIP,
			FailureReason: fmt.Sprintf("could not update %s to %s: %v", u.RecordLabel(), ip, err),
		})
		return
	}
	u.lastIP = ip
	if previousIP == "" {
		log.Printf("updated %s to %s", u.RecordLabel(), ip)
	} else {
		log.Printf("updated %s from %s to %s", u.RecordLabel(), previousIP, ip)
	}
	u.runHook(ctx, "success hook", u.cfg.SuccessExec, hookEvent{PublicIP: ip, PreviousIP: previousIP})
}

func (u *Updater) fail(ctx context.Context, event hookEvent) {
	log.Print(event.FailureReason)
	u.runHook(ctx, "failure hook", u.cfg.FailedExec, event)
}

// Run checks immediately, then on every tick of sched until ctx is done.
func (u *Updater) Run(ctx context.Context, sched config.Schedule) {
	for {
		u.RunOnce(ctx)
		timer := time.NewTimer(time.Until(sched.Next(time.Now())))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
