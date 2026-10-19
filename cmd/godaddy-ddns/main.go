// Command godaddy-ddns keeps one GoDaddy DNS A record pointing at the public
// IPv4 address of the machine it runs on.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Embedded zone data, so TZ applies to CRON_TIME schedules without tzdata
	// installed in the image.
	_ "time/tzdata"

	"github.com/chenwei791129/docker-godaddy-ddns/internal/config"
	"github.com/chenwei791129/docker-godaddy-ddns/internal/ddns"
	"github.com/chenwei791129/docker-godaddy-ddns/internal/godaddy"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(version)
		return
	}

	cfg, warnings, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	u := ddns.New(cfg, &godaddy.Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		Credential: cfg.Credential,
	})

	log.Printf("godaddy-ddns %s: keeping the %s record of %s up to date %s using a %s",
		version, cfg.Type, u.RecordLabel(), cfg.ScheduleDesc, cfg.CredentialKind)
	for _, w := range warnings {
		log.Printf("WARNING: %s", w)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	u.Run(ctx, cfg.Schedule)
	log.Print("stopped")
}
