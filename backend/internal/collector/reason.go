package collector

import (
	"fmt"

	"github.com/simanto4321/postgres-atlas/backend/internal/model"
)

func missingReason(m model.MissingIndex) string {
	return fmt.Sprintf(
		"%d sequential scans read %d rows (~%.0f rows/scan) on a %d-row table; an index on the filtered/joined columns would avoid full scans.",
		m.SeqScan, m.SeqTupRead, m.AvgRowsRead, m.LiveRows,
	)
}
