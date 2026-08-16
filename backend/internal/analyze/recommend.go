package analyze

import (
	"fmt"
	"sort"

	"github.com/simanto4321/postgres-atlas/backend/internal/model"
)

var severityRank = map[string]int{"critical": 0, "warning": 1, "info": 2}

// Recommend turns the analyzed report into a prioritized, actionable list.
// Rules are ordered from most to least urgent and each carries a ready-to-run
// SQL suggestion. The result is sorted by severity.
func Recommend(r *model.Report) []model.Recommendation {
	var recs []model.Recommendation

	// Capacity: projected to fill soon.
	if r.Capacity.Forecast.DaysToThreshold > 0 && r.Capacity.Forecast.DaysToThreshold < 60 {
		recs = append(recs, model.Recommendation{
			ID:       "capacity-forecast",
			Severity: severityForDays(r.Capacity.Forecast.DaysToThreshold),
			Category: "Capacity",
			Title:    fmt.Sprintf("Storage projected to hit threshold in %.0f days", r.Capacity.Forecast.DaysToThreshold),
			Rationale: fmt.Sprintf("Growing ~%s/day; at this rate the %s threshold is reached on %s.",
				r.Capacity.Forecast.DailyGrowthHuman, HumanBytes(r.Capacity.Forecast.ThresholdBytes),
				r.Capacity.Forecast.ProjectedFull),
			ActionSQL: "-- Plan storage/partitioning; archive cold data or expand the volume.",
		})
	}

	// XID wraparound is an existential threat — always first if elevated.
	if r.Health.WraparoundPct >= 0.5 {
		recs = append(recs, model.Recommendation{
			ID:        "wraparound",
			Severity:  ternary(r.Health.WraparoundPct >= 0.8, "critical", "warning"),
			Category:  "Reliability",
			Title:     "Transaction-ID wraparound risk is elevated",
			Rationale: fmt.Sprintf("Oldest unfrozen XID is %.1f%% of the wraparound budget. Autovacuum may not be keeping up.", r.Health.WraparoundPct*100),
			ActionSQL: "VACUUM (FREEZE, VERBOSE);  -- and review autovacuum_freeze_max_age",
		})
	}

	// Unused indexes waste space and slow writes.
	for _, u := range r.Indexes.Unused {
		if u.SizeBytes < 1<<20 { // ignore tiny indexes
			continue
		}
		recs = append(recs, model.Recommendation{
			ID:        "unused-index-" + u.Index,
			Severity:  "warning",
			Category:  "Indexes",
			Title:     fmt.Sprintf("Unused index %q (%s)", u.Index, u.SizeHuman),
			Rationale: fmt.Sprintf("%d scans since stats reset on %s.%s. It still costs write amplification and disk.", u.Scans, u.Schema, u.Table),
			ActionSQL: fmt.Sprintf("DROP INDEX CONCURRENTLY %s.%s;", u.Schema, u.Index),
		})
	}

	// Missing-index candidates: heavy sequential scanning.
	for _, m := range r.Indexes.Missing {
		recs = append(recs, model.Recommendation{
			ID:        "missing-index-" + m.Table,
			Severity:  "warning",
			Category:  "Indexes",
			Title:     fmt.Sprintf("Table %q is heavily sequentially scanned", m.Table),
			Rationale: m.Reason,
			ActionSQL: fmt.Sprintf("-- Review predicates on %s.%s and add a targeted index (e.g. on filter/join columns).", m.Schema, m.Table),
		})
	}

	// Duplicate indexes.
	for _, d := range r.Indexes.Duplicate {
		recs = append(recs, model.Recommendation{
			ID:        "duplicate-index-" + d.Table,
			Severity:  "info",
			Category:  "Indexes",
			Title:     fmt.Sprintf("Duplicate indexes on %q (%s)", d.Table, d.Columns),
			Rationale: fmt.Sprintf("Indexes %v cover the same columns; keep one.", d.Indexes),
			ActionSQL: fmt.Sprintf("DROP INDEX CONCURRENTLY %s.%s;", d.Schema, d.Indexes[len(d.Indexes)-1]),
		})
	}

	// Bloat.
	for _, b := range r.Bloat {
		if b.BloatRatio < 0.3 || b.BloatBytes < 50<<20 {
			continue
		}
		recs = append(recs, model.Recommendation{
			ID:        "bloat-" + b.Name,
			Severity:  ternary(b.BloatRatio >= 0.5, "warning", "info"),
			Category:  "Bloat",
			Title:     fmt.Sprintf("Table %q is ~%.0f%% bloat (%s reclaimable)", b.Name, b.BloatRatio*100, b.BloatHuman),
			Rationale: "Dead space inflates scans and backups. A rewrite reclaims it (locks briefly; prefer pg_repack online).",
			ActionSQL: fmt.Sprintf("VACUUM (FULL, ANALYZE) %s.%s;  -- or: pg_repack -t %s", b.Schema, b.Name, b.Name),
		})
	}

	// Vacuum lag.
	for _, v := range r.Vacuum {
		if v.DeadRatio < 0.2 {
			continue
		}
		recs = append(recs, model.Recommendation{
			ID:        "vacuum-" + v.Name,
			Severity:  ternary(v.DeadRatio >= 0.4, "warning", "info"),
			Category:  "Vacuum",
			Title:     fmt.Sprintf("Table %q has %.0f%% dead tuples", v.Name, v.DeadRatio*100),
			Rationale: fmt.Sprintf("%d dead vs %d live tuples; last autovacuum %s. Consider a tighter autovacuum scale factor.", v.DeadTuples, v.LiveTuples, orNone(v.LastAutovacuum)),
			ActionSQL: fmt.Sprintf("VACUUM (ANALYZE) %s.%s;", v.Schema, v.Name),
		})
	}

	// Cache hit ratio.
	if r.Health.CacheHitRatio < 0.95 {
		recs = append(recs, model.Recommendation{
			ID:        "cache-hit",
			Severity:  "warning",
			Category:  "Performance",
			Title:     fmt.Sprintf("Buffer cache hit ratio is low (%.1f%%)", r.Health.CacheHitRatio*100),
			Rationale: "A hit ratio under 95% often means shared_buffers is undersized for the working set.",
			ActionSQL: "-- Consider increasing shared_buffers / effective_cache_size, or add RAM.",
		})
	}

	sort.SliceStable(recs, func(i, j int) bool {
		return severityRank[recs[i].Severity] < severityRank[recs[j].Severity]
	})
	return recs
}

func severityForDays(days float64) string {
	if days < 21 {
		return "critical"
	}
	return "warning"
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func orNone(s string) string {
	if s == "" {
		return "never"
	}
	return s
}
