package analyze

import "github.com/simanto4321/postgres-atlas/backend/internal/model"

// ScoreHealth derives a 0-100 health score and per-dimension checks from the
// raw metrics already populated on h. It is deterministic and side-effect free
// apart from mutating the passed-in Health.
func ScoreHealth(h *model.Health, worstDeadRatio float64) {
	score := 100
	var checks []model.HealthCheck

	// Cache hit ratio: below 0.99 is a smell, below 0.95 hurts.
	switch {
	case h.CacheHitRatio >= 0.99:
		checks = append(checks, chk("Cache hit ratio", "ok", pctDetail(h.CacheHitRatio)))
	case h.CacheHitRatio >= 0.95:
		score -= 8
		checks = append(checks, chk("Cache hit ratio", "warn", pctDetail(h.CacheHitRatio)))
	default:
		score -= 18
		checks = append(checks, chk("Cache hit ratio", "critical", pctDetail(h.CacheHitRatio)))
	}

	// Connection saturation.
	if h.ConnectionsMax > 0 {
		frac := float64(h.ConnectionsUsed) / float64(h.ConnectionsMax)
		switch {
		case frac < 0.7:
			checks = append(checks, chk("Connections", "ok", conDetail(h)))
		case frac < 0.9:
			score -= 8
			checks = append(checks, chk("Connections", "warn", conDetail(h)))
		default:
			score -= 16
			checks = append(checks, chk("Connections", "critical", conDetail(h)))
		}
	}

	// Transaction-ID wraparound: pct of the ~2.1B budget consumed.
	switch {
	case h.WraparoundPct < 0.5:
		checks = append(checks, chk("XID wraparound", "ok", pctDetail(h.WraparoundPct)))
	case h.WraparoundPct < 0.8:
		score -= 12
		checks = append(checks, chk("XID wraparound", "warn", pctDetail(h.WraparoundPct)))
	default:
		score -= 30
		checks = append(checks, chk("XID wraparound", "critical", pctDetail(h.WraparoundPct)))
	}

	// Dead-tuple pressure (worst table).
	switch {
	case worstDeadRatio < 0.1:
		checks = append(checks, chk("Dead tuples", "ok", pctDetail(worstDeadRatio)))
	case worstDeadRatio < 0.25:
		score -= 8
		checks = append(checks, chk("Dead tuples", "warn", pctDetail(worstDeadRatio)))
	default:
		score -= 16
		checks = append(checks, chk("Dead tuples", "critical", pctDetail(worstDeadRatio)))
	}

	// Long-running query.
	if h.LongestQuerySec >= 300 {
		score -= 10
		checks = append(checks, chk("Long-running query", "warn", secDetail(h.LongestQuerySec)))
	} else {
		checks = append(checks, chk("Long-running query", "ok", secDetail(h.LongestQuerySec)))
	}

	if score < 0 {
		score = 0
	}
	h.Score = score
	h.Grade = Grade(score)
	h.Checks = checks
}

// Grade maps a numeric score to a letter grade.
func Grade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func chk(name, status, detail string) model.HealthCheck {
	return model.HealthCheck{Name: name, Status: status, Detail: detail}
}
