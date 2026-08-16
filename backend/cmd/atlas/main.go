// Command atlas is the Postgres Atlas server.
//
//	atlas --dsn postgres://user:pass@host:5432/db   # live mode
//	atlas --snapshot docs/sample-report.json        # demo mode (no DB)
//
// In live mode it periodically introspects the target database and serves an
// analyzed report; in demo mode it serves a bundled snapshot so the dashboard
// runs with zero infrastructure.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/simanto4321/postgres-atlas/backend/internal/api"
	"github.com/simanto4321/postgres-atlas/backend/internal/collector"
	"github.com/simanto4321/postgres-atlas/backend/internal/model"
)

func main() {
	addr := flag.String("addr", envOr("ATLAS_ADDR", ":8000"), "listen address")
	dsn := flag.String("dsn", os.Getenv("ATLAS_DSN"), "PostgreSQL DSN (live mode)")
	snapshot := flag.String("snapshot", os.Getenv("ATLAS_SNAPSHOT"), "path to a JSON snapshot (demo mode)")
	thresholdGB := flag.Float64("threshold-gb", envFloat("ATLAS_THRESHOLD_GB", 100), "capacity threshold for the forecast, in GB")
	flag.Parse()

	source, cleanup, err := buildSource(*dsn, *snapshot, int64(*thresholdGB*(1<<30)))
	if err != nil {
		log.Fatalf("startup: %v", err)
	}
	defer cleanup()

	srv := api.NewServer(source, 30*time.Second)
	httpSrv := &http.Server{
		Addr:         *addr,
		Handler:      srv.Routes(),
		ReadTimeout:  20 * time.Second,
		WriteTimeout: 20 * time.Second,
	}

	go func() {
		log.Printf("postgres-atlas listening on %s", *addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	log.Println("atlas stopped")
}

func buildSource(dsn, snapshot string, threshold int64) (api.Source, func(), error) {
	if snapshot != "" {
		report, err := loadSnapshot(snapshot)
		if err != nil {
			return nil, func() {}, err
		}
		log.Printf("serving snapshot %s (demo mode)", snapshot)
		return func(context.Context) (*model.Report, error) { return report, nil }, func() {}, nil
	}
	if dsn == "" {
		return nil, func() {}, errNoSource
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, func() {}, err
	}
	col := collector.New(pool, threshold)
	return col.Collect, pool.Close, nil
}

func loadSnapshot(path string) (*model.Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r model.Report
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

var errNoSource = errString("no source: pass --dsn (live) or --snapshot (demo)")

type errString string

func (e errString) Error() string { return string(e) }

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
