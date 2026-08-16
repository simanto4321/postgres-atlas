package analyze

import (
	"math"
	"time"

	"github.com/simanto4321/postgres-atlas/backend/internal/model"
)

// Forecast fits a least-squares line to the size history and projects when the
// database will reach the given threshold. History points are assumed to be one
// day apart and in chronological order.
func Forecast(history []model.SizePoint, thresholdBytes int64) model.Forecast {
	f := model.Forecast{ThresholdBytes: thresholdBytes, DaysToThreshold: math.Inf(1)}
	if len(history) < 2 {
		return f
	}

	n := float64(len(history))
	var sumX, sumY, sumXY, sumXX float64
	for i, p := range history {
		x := float64(i)
		y := float64(p.Bytes)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return f
	}
	slope := (n*sumXY - sumX*sumY) / denom // bytes/day
	if slope < 0 {
		slope = 0
	}
	f.DailyGrowthBytes = int64(slope)
	f.DailyGrowthHuman = HumanBytes(f.DailyGrowthBytes)

	current := float64(history[len(history)-1].Bytes)
	if slope > 0 && float64(thresholdBytes) > current {
		days := (float64(thresholdBytes) - current) / slope
		f.DaysToThreshold = math.Round(days*10) / 10
		f.ProjectedFull = time.Now().AddDate(0, 0, int(days)).Format("2006-01-02")
	}
	return f
}
