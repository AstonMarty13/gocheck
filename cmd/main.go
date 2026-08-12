// Command gocheck monitors a list of websites and serves a status dashboard.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/AstonMarty13/gocheck/internal/monitor"
	"github.com/AstonMarty13/gocheck/internal/web"
)

func main() {
	var (
		sitesPath = flag.String("sites", "sites.txt", "path to the file listing URLs to monitor")
		addr      = flag.String("addr", ":8080", "address the dashboard listens on")
		interval  = flag.Duration("interval", 10*time.Second, "delay between two checks of the same site")
		timeout   = flag.Duration("timeout", 5*time.Second, "per-request HTTP timeout")
	)
	flag.Parse()

	urls, err := monitor.LoadSites(*sitesPath)
	if err != nil {
		log.Fatalf("reading %s: %v", *sitesPath, err)
	}
	if len(urls) == 0 {
		log.Fatalf("no URLs found in %s", *sitesPath)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	m := monitor.New(urls, *interval, *timeout)
	m.Start(ctx)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           web.Handler(m),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("monitoring %d sites, dashboard on http://localhost%s", len(urls), *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server: %v", err)
	}
	log.Println("stopped")
}
