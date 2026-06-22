package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"botany/internal/plant"
)

func testManager(t *testing.T, now int64) *Manager {
	t.Helper()
	botanyDir := filepath.Join(t.TempDir(), ".botany")
	gameDir := t.TempDir()
	m, err := New("tester", botanyDir, gameDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m.SetClock(func() time.Time { return time.Unix(now, 0) })
	return m
}

func TestSaveLoadRoundTrip(t *testing.T) {
	m := testManager(t, 1_000_000)
	p := plant.New(m.SavefilePath, 1)
	p.Owner = "tester"
	p.Stage = 3
	p.Species = 4
	p.Ticks = 123.4
	if err := m.SavePlant(p); err != nil {
		t.Fatalf("SavePlant: %v", err)
	}
	got, err := m.LoadRaw()
	if err != nil {
		t.Fatalf("LoadRaw: %v", err)
	}
	if got.PlantID != p.PlantID || got.Stage != 3 || got.Species != 4 || got.Ticks != 123.4 {
		t.Errorf("round trip mismatch: %+v vs %+v", got, p)
	}
}

func TestCheckPlant(t *testing.T) {
	m := testManager(t, 1_000_000)
	if m.CheckPlant() {
		t.Error("CheckPlant should be false before any save")
	}
	if err := m.SavePlant(plant.New(m.SavefilePath, 1)); err != nil {
		t.Fatal(err)
	}
	if !m.CheckPlant() {
		t.Error("CheckPlant should be true after save")
	}
}

func TestDataWriteJSONSchema(t *testing.T) {
	m := testManager(t, 1_000_000)
	p := plant.New(m.SavefilePath, 2)
	p.Owner = "tester"
	p.Stage = 4 // flowering -> rarity, color, species all present
	p.Species = 1
	p.Color = 3
	p.Rarity = 2
	p.Mutation = 1
	p.Ticks = 555
	p.WateredTimestamp = 999
	if err := m.DataWriteJSON(p); err != nil {
		t.Fatalf("DataWriteJSON: %v", err)
	}
	raw, err := os.ReadFile(m.PlantDataJSONPath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := map[string]any{
		"owner":        "tester",
		"description":  p.ParsePlant(),
		"is_dead":      false,
		"last_watered": float64(999),
		"file_name":    m.SavefilePath,
		"stage":        "flowering",
		"generation":   float64(2),
		"rarity":       "rare",
		"mutation":     "humming",
		"color":        "green",
		"species":      "cactus",
		"score":        float64(555),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("export[%q] = %v (%T), want %v (%T)", k, got[k], got[k], v, v)
		}
	}
	if _, ok := got["age"]; !ok {
		t.Error("export missing age")
	}
}

func TestDataWriteJSONOmitsImmatureFields(t *testing.T) {
	m := testManager(t, 1_000_000)
	p := plant.New(m.SavefilePath, 1)
	p.Stage = 0
	p.Mutation = 0
	if err := m.DataWriteJSON(p); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(m.PlantDataJSONPath)
	var got map[string]any
	json.Unmarshal(raw, &got)
	for _, k := range []string{"rarity", "color", "species", "mutation"} {
		if _, ok := got[k]; ok {
			t.Errorf("seed export should omit %q", k)
		}
	}
}

func TestGardenUpsertRetrieveClean(t *testing.T) {
	m := testManager(t, 1_000_000)
	p1 := plant.New(m.SavefilePath, 1)
	p1.Owner = "tester"
	p1.Stage = 3
	if err := m.UpdateGardenDB(p1); err != nil {
		t.Fatalf("UpdateGardenDB p1: %v", err)
	}
	garden, err := m.RetrieveGarden()
	if err != nil {
		t.Fatalf("RetrieveGarden: %v", err)
	}
	if len(garden) != 1 {
		t.Fatalf("garden size = %d, want 1", len(garden))
	}
	// second plant, same owner, different id -> the old one is marked dead
	p2 := plant.New(m.SavefilePath, 1)
	p2.Owner = "tester"
	p2.Stage = 3
	if err := m.UpdateGardenDB(p2); err != nil {
		t.Fatalf("UpdateGardenDB p2: %v", err)
	}
	garden, _ = m.RetrieveGarden()
	if garden[p1.PlantID].Dead == 0 {
		t.Error("old plant of same owner should be marked dead")
	}
	if garden[p2.PlantID].Dead != 0 {
		t.Error("current plant should be alive")
	}
}

func TestVisitorDB(t *testing.T) {
	m := testManager(t, 1_000_000)
	if err := m.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	if err := m.MigrateDatabase(); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateVisitorDB("tester", []string{"alice", "alice", "bob"}); err != nil {
		t.Fatalf("UpdateVisitorDB: %v", err)
	}
	counts, err := m.WeeklyVisitors("tester")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, c := range counts {
		got[c.Name] = c.Visits
	}
	if got["alice"] != 2 || got["bob"] != 1 {
		t.Errorf("visitor counts = %v, want alice:2 bob:1", got)
	}
}

func TestHarvest(t *testing.T) {
	m := testManager(t, 1_000_000)
	p1 := plant.New(m.SavefilePath, 1)
	p1.Stage = 3
	isNew, err := m.HarvestPlant(p1)
	if err != nil {
		t.Fatalf("HarvestPlant: %v", err)
	}
	if !isNew {
		t.Error("first harvest should report new file")
	}
	p2 := plant.New(m.SavefilePath, 1)
	p2.Stage = 3
	isNew, err = m.HarvestPlant(p2)
	if err != nil {
		t.Fatal(err)
	}
	if isNew {
		t.Error("second harvest should not report new file")
	}
	raw, _ := os.ReadFile(m.HarvestPath)
	var harvest map[string]any
	json.Unmarshal(raw, &harvest)
	if len(harvest) != 2 {
		t.Errorf("harvest entries = %d, want 2", len(harvest))
	}
}

func TestWaterOnVisit(t *testing.T) {
	dir := t.TempDir()
	visitorFile := filepath.Join(dir, "visitors.json")
	if err := os.WriteFile(visitorFile, []byte("[]"), 0o666); err != nil {
		t.Fatal(err)
	}
	ok, err := WaterOnVisit(visitorFile, "guest", 12345)
	if err != nil {
		t.Fatalf("WaterOnVisit: %v", err)
	}
	if !ok {
		t.Fatal("watering a writable garden should succeed")
	}
	raw, _ := os.ReadFile(visitorFile)
	var entries []map[string]any
	json.Unmarshal(raw, &entries)
	if len(entries) != 1 || entries[0]["user"] != "guest" || entries[0]["timestamp"] != float64(12345) {
		t.Errorf("visitor entry = %v", entries)
	}
	// missing file -> no-op, false
	ok, err = WaterOnVisit(filepath.Join(dir, "nope.json"), "guest", 1)
	if err != nil || ok {
		t.Errorf("watering a missing garden: ok=%v err=%v, want false,nil", ok, err)
	}
}

func TestProcessGuestsAdvancesWatering(t *testing.T) {
	now := int64(100 * 24 * 3600)
	m := testManager(t, now)
	p := plant.New(m.SavefilePath, 1)
	p.SetClock(func() time.Time { return time.Unix(now, 0) })
	p.WateredTimestamp = now - 3*24*3600 // last self-watered 3 days ago
	// a guest watered 1 day ago
	guestTS := now - 1*24*3600
	entries := []map[string]any{{"user": "alice", "timestamp": guestTS}}
	raw, _ := json.Marshal(entries)
	os.WriteFile(filepath.Join(m.BotanyDir, "visitors.json"), raw, 0o666)

	if err := m.ProcessGuests(p); err != nil {
		t.Fatalf("ProcessGuests: %v", err)
	}
	if p.WateredTimestamp != guestTS {
		t.Errorf("watered timestamp = %d, want %d (advanced by guest)", p.WateredTimestamp, guestTS)
	}
	found := false
	for _, v := range p.Visitors {
		if v == "alice" {
			found = true
		}
	}
	if !found {
		t.Error("alice should be recorded as a visitor")
	}
	// visitors.json should be cleared
	cleared, _ := os.ReadFile(filepath.Join(m.BotanyDir, "visitors.json"))
	if string(cleared) != "[]" {
		t.Errorf("visitors.json = %q, want []", cleared)
	}
}
