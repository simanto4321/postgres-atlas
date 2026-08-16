package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/simanto4321/postgres-atlas/backend/internal/model"
)

func TestReportEndpoint(t *testing.T) {
	src := func(_ context.Context) (*model.Report, error) {
		return &model.Report{Database: "demo", Health: model.Health{Score: 91, Grade: "A"}}, nil
	}
	srv := NewServer(src, time.Minute)
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/report")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	var r model.Report
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	if r.Database != "demo" || r.Health.Grade != "A" {
		t.Fatalf("unexpected report: %+v", r)
	}
}

func TestReportIsCached(t *testing.T) {
	var calls int32
	src := func(_ context.Context) (*model.Report, error) {
		atomic.AddInt32(&calls, 1)
		return &model.Report{Database: "demo"}, nil
	}
	srv := NewServer(src, time.Minute)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := srv.report(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("source called %d times, want 1 (cached)", got)
	}
}
