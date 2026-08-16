// Package model defines the shared report shape returned by the API and
// consumed by the TypeScript dashboard. Keep the JSON tags stable — the
// frontend and the bundled demo snapshot depend on them.
package model

import "time"

type Report struct {
	GeneratedAt     time.Time        `json:"generated_at"`
	Database        string           `json:"database"`
	Version         string           `json:"version"`
	Capacity        Capacity         `json:"capacity"`
	Health          Health           `json:"health"`
	Bloat           []BloatEntry     `json:"bloat"`
	Indexes         IndexReport      `json:"indexes"`
	Vacuum          []VacuumEntry    `json:"vacuum"`
	Recommendations []Recommendation `json:"recommendations"`
}

type Capacity struct {
	TotalBytes int64      `json:"total_bytes"`
	TotalHuman string     `json:"total_human"`
	Forecast   Forecast   `json:"forecast"`
	TopTables  []Table    `json:"top_tables"`
	History    []SizePoint `json:"history"`
}

type SizePoint struct {
	Day   string `json:"day"`
	Bytes int64  `json:"bytes"`
}

type Forecast struct {
	DailyGrowthBytes int64   `json:"daily_growth_bytes"`
	DailyGrowthHuman string  `json:"daily_growth_human"`
	ThresholdBytes   int64   `json:"threshold_bytes"`
	DaysToThreshold  float64 `json:"days_to_threshold"`
	ProjectedFull    string  `json:"projected_full"`
}

type Table struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	TotalBytes  int64  `json:"total_bytes"`
	HeapBytes   int64  `json:"heap_bytes"`
	IndexBytes  int64  `json:"index_bytes"`
	ToastBytes  int64  `json:"toast_bytes"`
	Rows        int64  `json:"rows"`
	TotalHuman  string `json:"total_human"`
}

type Health struct {
	Score           int           `json:"score"`
	Grade           string        `json:"grade"`
	CacheHitRatio   float64       `json:"cache_hit_ratio"`
	ConnectionsUsed int           `json:"connections_used"`
	ConnectionsMax  int           `json:"connections_max"`
	WraparoundAge   int64         `json:"wraparound_age"`
	WraparoundPct   float64       `json:"wraparound_pct"`
	LongestQuerySec float64       `json:"longest_query_sec"`
	UptimeDays      float64       `json:"uptime_days"`
	Checks          []HealthCheck `json:"checks"`
}

type HealthCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | critical
	Detail string `json:"detail"`
}

type BloatEntry struct {
	Schema     string  `json:"schema"`
	Name       string  `json:"name"`
	BloatBytes int64   `json:"bloat_bytes"`
	BloatRatio float64 `json:"bloat_ratio"`
	BloatHuman string  `json:"bloat_human"`
}

type IndexReport struct {
	Unused    []UnusedIndex    `json:"unused"`
	Duplicate []DuplicateIndex `json:"duplicate"`
	Missing   []MissingIndex   `json:"missing"`
}

type UnusedIndex struct {
	Schema    string `json:"schema"`
	Table     string `json:"table"`
	Index     string `json:"index"`
	SizeBytes int64  `json:"size_bytes"`
	SizeHuman string `json:"size_human"`
	Scans     int64  `json:"scans"`
}

type DuplicateIndex struct {
	Schema  string   `json:"schema"`
	Table   string   `json:"table"`
	Indexes []string `json:"indexes"`
	Columns string   `json:"columns"`
}

type MissingIndex struct {
	Schema      string  `json:"schema"`
	Table       string  `json:"table"`
	SeqScan     int64   `json:"seq_scan"`
	SeqTupRead  int64   `json:"seq_tup_read"`
	LiveRows    int64   `json:"live_rows"`
	AvgRowsRead float64 `json:"avg_rows_read"`
	Reason      string  `json:"reason"`
}

type VacuumEntry struct {
	Schema        string  `json:"schema"`
	Name          string  `json:"name"`
	DeadTuples    int64   `json:"dead_tuples"`
	LiveTuples    int64   `json:"live_tuples"`
	DeadRatio     float64 `json:"dead_ratio"`
	LastAutovacuum string `json:"last_autovacuum"`
	LastAutoanalyze string `json:"last_autoanalyze"`
}

type Recommendation struct {
	ID        string `json:"id"`
	Severity  string `json:"severity"` // info | warning | critical
	Category  string `json:"category"`
	Title     string `json:"title"`
	Rationale string `json:"rationale"`
	ActionSQL string `json:"action_sql"`
}
