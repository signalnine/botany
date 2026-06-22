package ui

import (
	"testing"
)

func TestWaterGauge(t *testing.T) {
	cases := []struct {
		delta int64
		want  string
	}{
		{0, "())))))))))) 100% "},
		{8640, "())))))))).) 90% "},
		{43200, "())))).....) 50% "},
		{86399, "().........) 0% "},
		{86400, "(..........) 0% "},
		{200000, "(..........) 0% "},
	}
	for _, c := range cases {
		got := WaterGauge(1_000_000, 1_000_000-c.delta, 86400)
		if got != c.want {
			t.Errorf("WaterGauge(delta=%d) = %q, want %q", c.delta, got, c.want)
		}
	}
	// A longer class interval drains the gauge more slowly: half a day into a
	// 2-day interval is still 75% full.
	if got := WaterGauge(1_000_000, 1_000_000-43200, 2*86400); got != "())))))))..) 75% " {
		t.Errorf("WaterGauge with 2-day interval = %q, want %q", got, "())))))))..) 75% ")
	}
}

func TestEscPair(t *testing.T) {
	cases := map[string]int{
		"":        0,
		"0":       0,
		"30":      1, // black
		"31":      7, // red
		"32":      3, // green
		"33":      6, // yellow
		"34":      4, // blue
		"35":      5, // magenta
		"36":      8, // cyan
		"37":      2, // white
		"38;5;2":  3, // green
		"38;5;16": 1, // black
		"38;5;9":  7, // red
		"unknown": 0, // default normal
	}
	for code, want := range cases {
		if got := escPair(code); got != want {
			t.Errorf("escPair(%q) = %d, want %d", code, got, want)
		}
	}
}

func TestParseANSILine(t *testing.T) {
	// two glyphs, green then red
	line := "\x1b[38;5;2m/\x1b[38;5;9m\\\r\n"
	cells, endPair := ParseANSILine(line, 0)
	if len(cells) != 2 {
		t.Fatalf("got %d cells, want 2: %+v", len(cells), cells)
	}
	if cells[0].Ch != '/' || cells[0].Pair != 3 {
		t.Errorf("cell0 = %+v, want {/ 3}", cells[0])
	}
	if cells[1].Ch != '\\' || cells[1].Pair != 7 {
		t.Errorf("cell1 = %+v, want {\\ 7}", cells[1])
	}
	if endPair != 7 {
		t.Errorf("endPair = %d, want 7", endPair)
	}
}

func TestParseANSILineThreadsColor(t *testing.T) {
	// a leading plain token inherits the carried-in pair
	cells, _ := ParseANSILine("ab", 3)
	for i, c := range cells {
		if c.Pair != 3 {
			t.Errorf("cell %d pair = %d, want carried 3", i, c.Pair)
		}
	}
}

func TestFormatGardenEntry(t *testing.T) {
	cases := []struct {
		row  GardenRow
		want string
	}{
		{GardenRow{"curiouser", "2d:3h:4m:5s", 955337, "common jade plant"},
			"curiouser      -      2d:3h:4m:5s -   955337p - common jade plant"},
		{GardenRow{"averylongusername123", "0d:0h:0m:0s", 5, "seed"},
			"averylongusern -      0d:0h:0m:0s -        5p - seed"},
	}
	for _, c := range cases {
		if got := FormatGardenEntry(c.row); got != c.want {
			t.Errorf("FormatGardenEntry = %q\n want %q", got, c.want)
		}
	}
}

func TestAgeToSeconds(t *testing.T) {
	if got := ageToSeconds("2d:3h:4m:5s"); got != 183845 {
		t.Errorf("ageToSeconds = %d, want 183845", got)
	}
	if got := ageToSeconds("garbage"); got != 0 {
		t.Errorf("ageToSeconds(garbage) = %d, want 0", got)
	}
}

func TestSortGarden(t *testing.T) {
	rows := []GardenRow{
		{"bob", "1d:0h:0m:0s", 50, "z"},
		{"alice", "2d:0h:0m:0s", 10, "a"},
		{"carol", "0d:1h:0m:0s", 99, "m"},
	}
	// by name ascending
	SortGarden(rows, 0, true)
	if rows[0].Owner != "alice" || rows[2].Owner != "carol" {
		t.Errorf("name asc order wrong: %v", names(rows))
	}
	// by score descending
	SortGarden(rows, 2, false)
	if rows[0].Score != 99 || rows[2].Score != 10 {
		t.Errorf("score desc order wrong: %v", names(rows))
	}
	// by age ascending (carol 1h < bob 1d < alice 2d)
	SortGarden(rows, 1, true)
	if rows[0].Owner != "carol" || rows[1].Owner != "bob" || rows[2].Owner != "alice" {
		t.Errorf("age asc order wrong: %v", names(rows))
	}
}

func names(rows []GardenRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Owner
	}
	return out
}

func TestFilterGarden(t *testing.T) {
	rows := []GardenRow{
		{"alice", "1d:0h:0m:0s", 10, "common fern"},
		{"bob", "1d:0h:0m:0s", 20, "rare cactus"},
	}
	if got := FilterGarden(rows, ""); len(got) != 2 {
		t.Errorf("empty pattern should keep all, got %d", len(got))
	}
	if got := FilterGarden(rows, "cactus"); len(got) != 1 || got[0].Owner != "bob" {
		t.Errorf("filter cactus = %v", names(got))
	}
	if got := FilterGarden(rows, "["); len(got) != 0 {
		t.Errorf("invalid regex should match nothing, got %d", len(got))
	}
}

func TestCompleter(t *testing.T) {
	c := NewCompleter([]string{"alice", "alan", "bob"})
	c.UpdateInput("al")
	// forward cycles through alice, alan (sorted), then back to base "al"
	got := []string{c.Complete(1), c.Complete(1), c.Complete(1)}
	want := []string{"alan", "alice", "al"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("complete step %d = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestUseColorEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	cases := []struct {
		name string
		m    map[string]string
		want bool
	}{
		{"default", map[string]string{}, true},
		{"python_colors_0", map[string]string{"PYTHON_COLORS": "0"}, false},
		{"python_colors_1_beats_no_color", map[string]string{"PYTHON_COLORS": "1", "NO_COLOR": "1"}, true},
		{"no_color", map[string]string{"NO_COLOR": "1"}, false},
		{"no_color_beats_force", map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1"}, false},
		{"force_color", map[string]string{"FORCE_COLOR": "1"}, true},
	}
	for _, c := range cases {
		if got := useColorEnv(env(c.m)); got != c.want {
			t.Errorf("%s: useColorEnv = %v, want %v", c.name, got, c.want)
		}
	}
}
