// Command gensnapshot builds a realistic demo report using the same analysis
// engine that powers the live collector. The resulting JSON is served by
// `atlas --snapshot` so the dashboard is fully runnable without PostgreSQL.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/simanto4321/postgres-atlas/backend/internal/analyze"
	"github.com/simanto4321/postgres-atlas/backend/internal/model"
)

func main() {
	out := "docs/sample-report.json"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}

	gb := int64(1 << 30)
	mb := int64(1 << 20)

	history := make([]model.SizePoint, 0, 30)
	base := 42 * gb
	day := time.Now().AddDate(0, 0, -29)
	for i := 0; i < 30; i++ {
		// ~480 MB/day growth with a little noise.
		bytes := base + int64(i)*480*mb + int64((i%5)*12)*mb
		history = append(history, model.SizePoint{Day: day.AddDate(0, 0, i).Format("2006-01-02"), Bytes: bytes})
	}
	current := history[len(history)-1].Bytes

	tables := []model.Table{
		{Schema: "public", Name: "orders", TotalBytes: 18 * gb, HeapBytes: 14 * gb, IndexBytes: 3 * gb, ToastBytes: 1 * gb, Rows: 48_200_000},
		{Schema: "public", Name: "order_items", TotalBytes: 12 * gb, HeapBytes: 10 * gb, IndexBytes: 2 * gb, ToastBytes: 0, Rows: 162_000_000},
		{Schema: "public", Name: "events", TotalBytes: 9 * gb, HeapBytes: 8 * gb, IndexBytes: 900 * mb, ToastBytes: 100 * mb, Rows: 410_000_000},
		{Schema: "public", Name: "customers", TotalBytes: 2 * gb, HeapBytes: 1400 * mb, IndexBytes: 500 * mb, ToastBytes: 100 * mb, Rows: 4_800_000},
		{Schema: "public", Name: "products", TotalBytes: 420 * mb, HeapBytes: 280 * mb, IndexBytes: 120 * mb, ToastBytes: 20 * mb, Rows: 180_000},
		{Schema: "analytics", Name: "daily_revenue", TotalBytes: 180 * mb, HeapBytes: 150 * mb, IndexBytes: 30 * mb, ToastBytes: 0, Rows: 2_200_000},
	}
	for i := range tables {
		tables[i].TotalHuman = analyze.HumanBytes(tables[i].TotalBytes)
	}

	r := &model.Report{
		GeneratedAt: time.Now().UTC(),
		Database:    "commerce",
		Version:     "PostgreSQL 16.4 on x86_64-pc-linux-gnu",
		Capacity: model.Capacity{
			TotalBytes: current,
			TotalHuman: analyze.HumanBytes(current),
			TopTables:  tables,
			History:    history,
		},
		Health: model.Health{
			CacheHitRatio:   0.972,
			ConnectionsUsed: 78,
			ConnectionsMax:  100,
			WraparoundAge:   620_000_000,
			WraparoundPct:   620_000_000.0 / 2146483647.0,
			LongestQuerySec: 42,
			UptimeDays:      47.3,
		},
		Bloat: []model.BloatEntry{
			{Schema: "public", Name: "orders", BloatBytes: 3 * gb, BloatRatio: 0.42},
			{Schema: "public", Name: "events", BloatBytes: 1800 * mb, BloatRatio: 0.36},
		},
		Indexes: model.IndexReport{
			Unused: []model.UnusedIndex{
				{Schema: "public", Table: "orders", Index: "idx_orders_legacy_status", SizeBytes: 820 * mb, Scans: 0},
				{Schema: "public", Table: "customers", Index: "idx_customers_old_email", SizeBytes: 210 * mb, Scans: 0},
				{Schema: "public", Table: "products", Index: "idx_products_sku_lower", SizeBytes: 18 * mb, Scans: 0},
			},
			Duplicate: []model.DuplicateIndex{
				{Schema: "public", Table: "customers", Indexes: []string{"customers_email_key", "idx_customers_email"}, Columns: "email"},
			},
			Missing: []model.MissingIndex{
				{Schema: "public", Table: "order_items", SeqScan: 18420, SeqTupRead: 2_900_000_000, LiveRows: 162_000_000, AvgRowsRead: 157_400},
			},
		},
		Vacuum: []model.VacuumEntry{
			{Schema: "public", Name: "orders", DeadTuples: 6_400_000, LiveTuples: 48_200_000, LastAutovacuum: "2026-08-14 03:12", LastAutoanalyze: "2026-08-14 03:40"},
			{Schema: "public", Name: "events", DeadTuples: 38_000_000, LiveTuples: 410_000_000, LastAutovacuum: "2026-08-13 21:05", LastAutoanalyze: "2026-08-13 21:40"},
			{Schema: "public", Name: "customers", DeadTuples: 90_000, LiveTuples: 4_800_000, LastAutovacuum: "2026-08-15 01:10", LastAutoanalyze: "2026-08-15 01:12"},
		},
	}

	for i := range r.Bloat {
		r.Bloat[i].BloatHuman = analyze.HumanBytes(r.Bloat[i].BloatBytes)
	}
	for i := range r.Indexes.Unused {
		r.Indexes.Unused[i].SizeHuman = analyze.HumanBytes(r.Indexes.Unused[i].SizeBytes)
	}
	for i := range r.Indexes.Missing {
		r.Indexes.Missing[i].Reason = fmt.Sprintf(
			"%d sequential scans read %d rows (~%.0f rows/scan) on a %d-row table; an index on the filtered/joined columns would avoid full scans.",
			r.Indexes.Missing[i].SeqScan, r.Indexes.Missing[i].SeqTupRead,
			r.Indexes.Missing[i].AvgRowsRead, r.Indexes.Missing[i].LiveRows,
		)
	}
	for i := range r.Vacuum {
		if total := r.Vacuum[i].DeadTuples + r.Vacuum[i].LiveTuples; total > 0 {
			r.Vacuum[i].DeadRatio = float64(r.Vacuum[i].DeadTuples) / float64(total)
		}
	}

	r.Capacity.Forecast = analyze.Forecast(r.Capacity.History, 100*gb)
	worstDead := 0.0
	for _, v := range r.Vacuum {
		if v.DeadRatio > worstDead {
			worstDead = v.DeadRatio
		}
	}
	analyze.ScoreHealth(&r.Health, worstDead)
	r.Recommendations = analyze.Recommend(r)

	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(out, b, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s (score %d/%s, %d recommendations)\n", out, r.Health.Score, r.Health.Grade, len(r.Recommendations))
}
