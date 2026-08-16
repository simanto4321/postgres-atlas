package analyze

import (
	"math"
	"testing"

	"github.com/simanto4321/postgres-atlas/backend/internal/model"
)

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		512:                 "512 bytes",
		2048:                "2.0 kB",
		5 * 1024 * 1024:     "5.0 MB",
		3 * 1024 * 1024 * 1024: "3.0 GB",
	}
	for in, want := range cases {
		if got := HumanBytes(in); got != want {
			t.Errorf("HumanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestForecastLinearGrowth(t *testing.T) {
	// +1 GB per day, current 10 GB, threshold 20 GB -> ~10 days.
	gb := int64(1 << 30)
	var history []model.SizePoint
	for i := 0; i <= 10; i++ {
		history = append(history, model.SizePoint{Bytes: int64(i) * gb})
	}
	f := Forecast(history, 20*gb)
	if math.Abs(float64(f.DailyGrowthBytes)-float64(gb)) > float64(gb)*0.02 {
		t.Errorf("daily growth = %d, want ~%d", f.DailyGrowthBytes, gb)
	}
	if math.Abs(f.DaysToThreshold-10) > 0.5 {
		t.Errorf("days to threshold = %.2f, want ~10", f.DaysToThreshold)
	}
}

func TestForecastFlatIsInfinite(t *testing.T) {
	history := []model.SizePoint{{Bytes: 100}, {Bytes: 100}, {Bytes: 100}}
	f := Forecast(history, 1000)
	if !math.IsInf(f.DaysToThreshold, 1) {
		t.Errorf("flat history should never reach threshold, got %.2f", f.DaysToThreshold)
	}
}

func TestScoreHealthGrades(t *testing.T) {
	healthy := &model.Health{CacheHitRatio: 0.995, ConnectionsUsed: 10, ConnectionsMax: 100, WraparoundPct: 0.1, LongestQuerySec: 2}
	ScoreHealth(healthy, 0.02)
	if healthy.Score < 90 || healthy.Grade != "A" {
		t.Errorf("healthy DB should grade A, got %d/%s", healthy.Score, healthy.Grade)
	}

	sick := &model.Health{CacheHitRatio: 0.90, ConnectionsUsed: 95, ConnectionsMax: 100, WraparoundPct: 0.85, LongestQuerySec: 900}
	ScoreHealth(sick, 0.5)
	if sick.Score > 60 {
		t.Errorf("unhealthy DB should score low, got %d", sick.Score)
	}
	if sick.Score >= healthy.Score {
		t.Errorf("sick score %d should be below healthy %d", sick.Score, healthy.Score)
	}
}

func TestRecommendPrioritizesCritical(t *testing.T) {
	r := &model.Report{
		Health: model.Health{WraparoundPct: 0.85, CacheHitRatio: 0.9},
		Indexes: model.IndexReport{
			Unused: []model.UnusedIndex{{Schema: "public", Table: "orders", Index: "idx_old", SizeBytes: 200 << 20, SizeHuman: "200.0 MB", Scans: 0}},
		},
	}
	recs := Recommend(r)
	if len(recs) == 0 {
		t.Fatal("expected recommendations")
	}
	if recs[0].Severity != "critical" {
		t.Errorf("first recommendation should be critical, got %q", recs[0].Severity)
	}
	// Ensure the unused-index drop suggestion is present and uses CONCURRENTLY.
	found := false
	for _, rec := range recs {
		if rec.ID == "unused-index-idx_old" {
			found = true
			if want := "DROP INDEX CONCURRENTLY public.idx_old;"; rec.ActionSQL != want {
				t.Errorf("action = %q, want %q", rec.ActionSQL, want)
			}
		}
	}
	if !found {
		t.Error("expected an unused-index recommendation")
	}
}

func TestRecommendIgnoresTinyIndexes(t *testing.T) {
	r := &model.Report{
		Indexes: model.IndexReport{
			Unused: []model.UnusedIndex{{Index: "idx_tiny", SizeBytes: 4096, Scans: 0}},
		},
	}
	for _, rec := range Recommend(r) {
		if rec.ID == "unused-index-idx_tiny" {
			t.Error("tiny indexes should not generate recommendations")
		}
	}
}
