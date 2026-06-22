package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"botany/internal/storage"
)

// GardenRow is one displayable community-garden entry.
type GardenRow struct {
	Owner       string
	Age         string
	Score       int
	Description string
}

// FormatGardenData turns the raw garden map into rows for living plants only,
// ordered by owner for stable display before any user sort.
func FormatGardenData(garden map[string]storage.GardenEntry) []GardenRow {
	var rows []GardenRow
	for _, id := range storage.SortedGardenIDs(garden) {
		e := garden[id]
		if e.Dead != 0 {
			continue
		}
		rows = append(rows, GardenRow{
			Owner:       e.Owner,
			Age:         e.Age,
			Score:       e.Score,
			Description: e.Description,
		})
	}
	return rows
}

// FormatGardenEntry renders a row as a fixed-width line (matches the Python
// "{:14.14} - {:>16} - {:>8}p - {}" format).
func FormatGardenEntry(r GardenRow) string {
	owner := r.Owner
	if len(owner) > 14 {
		owner = owner[:14]
	}
	return fmt.Sprintf("%-14s - %16s - %8dp - %s", owner, r.Age, r.Score, r.Description)
}

// ageToSeconds converts an "Nd:Nh:Nm:Ns" age into seconds; malformed input is 0.
func ageToSeconds(age string) int64 {
	parts := strings.Split(age, ":")
	if len(parts) != 4 {
		return 0
	}
	coeffs := []int64{86400, 3600, 60, 1}
	var total int64
	for i, p := range parts {
		if len(p) < 1 {
			return 0
		}
		n, err := strconv.Atoi(p[:len(p)-1])
		if err != nil {
			return 0
		}
		total += int64(n) * coeffs[i]
	}
	return total
}

// SortGarden sorts rows in place by column (0 name, 1 age, 2 score, 3 desc),
// ascending or descending.
func SortGarden(rows []GardenRow, column int, ascending bool) {
	less := func(i, j int) bool {
		a, b := rows[i], rows[j]
		switch column {
		case 1:
			return ageToSeconds(a.Age) < ageToSeconds(b.Age)
		case 2:
			return a.Score < b.Score
		case 3:
			return a.Description < b.Description
		default:
			return a.Owner < b.Owner
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if ascending {
			return less(i, j)
		}
		return less(j, i)
	})
}

// FilterGarden returns rows whose formatted entry matches the regex pattern. An
// empty pattern keeps everything; an invalid pattern matches nothing.
func FilterGarden(rows []GardenRow, pattern string) []GardenRow {
	if pattern == "" {
		out := make([]GardenRow, len(rows))
		copy(out, rows)
		return out
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}
	var out []GardenRow
	for _, r := range rows {
		if re.MatchString(FormatGardenEntry(r)) {
			out = append(out, r)
		}
	}
	return out
}
