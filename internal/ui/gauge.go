package ui

import (
	"math"
	"strconv"
	"strings"
)

// WaterGauge renders the water level as a bracketed bar plus a percentage. The
// gauge empties over intervalSecs, which varies by plant class (a cactus stays
// wet far longer than a fern). Formatting matches the Python water_gauge.
func WaterGauge(now, wateredTS, intervalSecs int64) string {
	if intervalSecs <= 0 {
		intervalSecs = 86400
	}
	pct := 1 - float64(now-wateredTS)/float64(intervalSecs)
	if pct < 0 {
		pct = 0
	}
	waterLeft := int(math.Ceil(pct * 10))
	if waterLeft > 10 {
		waterLeft = 10
	}
	return "(" + strings.Repeat(")", waterLeft) + strings.Repeat(".", 10-waterLeft) +
		") " + strconv.Itoa(int(pct*100)) + "% "
}
