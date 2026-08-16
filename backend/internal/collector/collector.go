// Package collector runs read-only introspection queries against a live
// PostgreSQL instance and assembles a raw report. All heavy analysis
// (scoring, forecasting, recommendations) happens in the analyze package on
// the returned data, so this file stays a thin, auditable query layer.
package collector

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/simanto4321/postgres-atlas/backend/internal/analyze"
	"github.com/simanto4321/postgres-atlas/backend/internal/model"
)

type Collector struct {
	pool           *pgxpool.Pool
	thresholdBytes int64
}

func New(pool *pgxpool.Pool, thresholdBytes int64) *Collector {
	return &Collector{pool: pool, thresholdBytes: thresholdBytes}
}

// Collect gathers metrics and returns a fully analyzed report.
func (c *Collector) Collect(ctx context.Context) (*model.Report, error) {
	r := &model.Report{GeneratedAt: time.Now()}

	if err := c.pool.QueryRow(ctx, qVersion).Scan(&r.Version); err != nil {
		return nil, err
	}

	var totalBytes int64
	if err := c.pool.QueryRow(ctx, qDatabase).Scan(&r.Database, &totalBytes); err != nil {
		return nil, err
	}
	r.Capacity.TotalBytes = totalBytes
	r.Capacity.TotalHuman = analyze.HumanBytes(totalBytes)

	if err := c.collectTables(ctx, r); err != nil {
		return nil, err
	}
	if err := c.collectHistory(ctx, r, totalBytes); err != nil {
		return nil, err
	}
	if err := c.collectHealth(ctx, r); err != nil {
		return nil, err
	}
	if err := c.collectIndexes(ctx, r); err != nil {
		return nil, err
	}
	worstDead, err := c.collectVacuum(ctx, r)
	if err != nil {
		return nil, err
	}

	r.Capacity.Forecast = analyze.Forecast(r.Capacity.History, c.thresholdBytes)
	analyze.ScoreHealth(&r.Health, worstDead)
	r.Recommendations = analyze.Recommend(r)
	return r, nil
}

func (c *Collector) collectTables(ctx context.Context, r *model.Report) error {
	rows, err := c.pool.Query(ctx, qTopTables)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var t model.Table
		if err := rows.Scan(&t.Schema, &t.Name, &t.TotalBytes, &t.HeapBytes,
			&t.IndexBytes, &t.ToastBytes, &t.Rows); err != nil {
			return err
		}
		t.TotalHuman = analyze.HumanBytes(t.TotalBytes)
		r.Capacity.TopTables = append(r.Capacity.TopTables, t)
	}
	return rows.Err()
}

func (c *Collector) collectHistory(ctx context.Context, r *model.Report, totalBytes int64) error {
	// Best-effort: record today's size, then read the trend. Failures here are
	// non-fatal (e.g. read-only role) — we just skip the forecast.
	_, _ = c.pool.Exec(ctx, qEnsureHistory)
	_, _ = c.pool.Exec(ctx, qRecordSize, totalBytes)

	rows, err := c.pool.Query(ctx, qLoadHistory)
	if err != nil {
		return nil // no history table access; forecast will be empty
	}
	defer rows.Close()
	for rows.Next() {
		var p model.SizePoint
		if err := rows.Scan(&p.Day, &p.Bytes); err != nil {
			return err
		}
		r.Capacity.History = append(r.Capacity.History, p)
	}
	return rows.Err()
}

func (c *Collector) collectHealth(ctx context.Context, r *model.Report) error {
	h := &r.Health
	if err := c.pool.QueryRow(ctx, qCacheHit).Scan(&h.CacheHitRatio); err != nil {
		return err
	}
	if err := c.pool.QueryRow(ctx, qConnections).Scan(&h.ConnectionsUsed, &h.ConnectionsMax); err != nil {
		return err
	}
	if err := c.pool.QueryRow(ctx, qWraparound).Scan(&h.WraparoundAge); err != nil {
		return err
	}
	// 2^31 - ~1M safety = practical wraparound budget.
	h.WraparoundPct = float64(h.WraparoundAge) / 2146483647.0
	if err := c.pool.QueryRow(ctx, qLongestQuery).Scan(&h.LongestQuerySec); err != nil {
		return err
	}
	return c.pool.QueryRow(ctx, qUptimeDays).Scan(&h.UptimeDays)
}

func (c *Collector) collectIndexes(ctx context.Context, r *model.Report) error {
	rows, err := c.pool.Query(ctx, qUnusedIndexes)
	if err != nil {
		return err
	}
	for rows.Next() {
		var u model.UnusedIndex
		if err := rows.Scan(&u.Schema, &u.Table, &u.Index, &u.SizeBytes, &u.Scans); err != nil {
			rows.Close()
			return err
		}
		u.SizeHuman = analyze.HumanBytes(u.SizeBytes)
		r.Indexes.Unused = append(r.Indexes.Unused, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	mrows, err := c.pool.Query(ctx, qMissingIndexes)
	if err != nil {
		return err
	}
	defer mrows.Close()
	for mrows.Next() {
		var m model.MissingIndex
		if err := mrows.Scan(&m.Schema, &m.Table, &m.SeqScan, &m.SeqTupRead, &m.LiveRows); err != nil {
			return err
		}
		if m.SeqScan > 0 {
			m.AvgRowsRead = float64(m.SeqTupRead) / float64(m.SeqScan)
		}
		m.Reason = missingReason(m)
		r.Indexes.Missing = append(r.Indexes.Missing, m)
	}
	return mrows.Err()
}

func (c *Collector) collectVacuum(ctx context.Context, r *model.Report) (float64, error) {
	rows, err := c.pool.Query(ctx, qVacuum)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var worst float64
	for rows.Next() {
		var v model.VacuumEntry
		if err := rows.Scan(&v.Schema, &v.Name, &v.DeadTuples, &v.LiveTuples,
			&v.LastAutovacuum, &v.LastAutoanalyze); err != nil {
			return 0, err
		}
		if total := v.DeadTuples + v.LiveTuples; total > 0 {
			v.DeadRatio = float64(v.DeadTuples) / float64(total)
		}
		if v.DeadRatio > worst {
			worst = v.DeadRatio
		}
		r.Vacuum = append(r.Vacuum, v)
	}
	return worst, rows.Err()
}
