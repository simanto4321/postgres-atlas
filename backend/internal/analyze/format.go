package analyze

import (
	"fmt"

	"github.com/simanto4321/postgres-atlas/backend/internal/model"
)

func pctDetail(frac float64) string {
	return fmt.Sprintf("%.1f%%", frac*100)
}

func secDetail(s float64) string {
	if s <= 0 {
		return "none"
	}
	return fmt.Sprintf("%.0fs", s)
}

func conDetail(h *model.Health) string {
	return fmt.Sprintf("%d / %d", h.ConnectionsUsed, h.ConnectionsMax)
}
