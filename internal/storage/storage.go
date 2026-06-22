// Package storage handles botany persistence: the per-user JSON savefile, the
// JSON exports read by other apps and visiting players, the shared sqlite
// community garden and its visitor log, and harvesting. The shared on-disk
// formats (exports, visitors.json, sqlite schema) match the Python original so a
// mixed Python/Go population on one host interoperates.
package storage

import (
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"time"

	"botany/internal/plant"

	_ "modernc.org/sqlite"
)

// Manager owns the filesystem and database locations for one running game.
type Manager struct {
	User              string
	BotanyDir         string // ~/.botany
	GameDir           string // holds sqlite/ and garden_file.json (shared on multiuser hosts)
	SavefilePath      string // <user>_plant.json
	GardenDBPath      string // GameDir/sqlite/garden_db.sqlite
	GardenJSONPath    string // GameDir/garden_file.json
	HarvestPath       string // ~/.botany/harvest_file.json
	PlantDataJSONPath string // ~/.botany/<user>_plant_data.json
	HomeParent        string // parent of the user's home dir, for visiting others

	now func() time.Time
}

// New builds a Manager for the given user and directories, creating BotanyDir.
func New(username, botanyDir, gameDir string) (*Manager, error) {
	if err := os.MkdirAll(botanyDir, 0o755); err != nil {
		return nil, err
	}
	return &Manager{
		User:              username,
		BotanyDir:         botanyDir,
		GameDir:           gameDir,
		SavefilePath:      filepath.Join(botanyDir, username+"_plant.json"),
		GardenDBPath:      filepath.Join(gameDir, "sqlite", "garden_db.sqlite"),
		GardenJSONPath:    filepath.Join(gameDir, "garden_file.json"),
		HarvestPath:       filepath.Join(botanyDir, "harvest_file.json"),
		PlantDataJSONPath: filepath.Join(botanyDir, username+"_plant_data.json"),
		HomeParent:        filepath.Dir(filepath.Dir(botanyDir)),
		now:               time.Now,
	}, nil
}

// Default builds a Manager using the real user, home directory ($HOME), and the
// directory containing the executable (the shared game dir).
func Default() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	gameDir := filepath.Dir(exe)
	return New(currentUsername(), filepath.Join(home, ".botany"), gameDir)
}

// currentUsername resolves the player's login name, falling back to $USER and
// then a generic name if the OS user database is unavailable.
func currentUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	return "botanist"
}

// SetClock overrides the time source (used in tests).
func (m *Manager) SetClock(now func() time.Time) { m.now = now }

func (m *Manager) nowUnix() int64 { return m.now().Unix() }

// CheckPlant reports whether a non-empty savefile exists.
func (m *Manager) CheckPlant() bool {
	info, err := os.Stat(m.SavefilePath)
	return err == nil && info.Size() > 0
}

// LoadRaw deserializes the savefile without applying offline growth.
func (m *Manager) LoadRaw() (*plant.Plant, error) {
	raw, err := os.ReadFile(m.SavefilePath)
	if err != nil {
		return nil, err
	}
	var p plant.Plant
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if p.Visitors == nil {
		p.Visitors = []string{}
	}
	p.SetClock(m.now)
	return &p, nil
}

// LoadPlant loads the savefile and credits growth that happened while the game
// was closed, matching the Python load_plant logic.
func (m *Manager) LoadPlant() (*plant.Plant, error) {
	p, err := m.LoadRaw()
	if err != nil {
		return nil, err
	}
	if err := m.ProcessGuests(p); err != nil {
		return nil, err
	}
	isWatered := p.WaterCheck()
	isDead := p.DeadCheck()
	if !isDead {
		var ticksToAdd int64
		if isWatered {
			delta := m.nowUnix() - p.LastTime
			if delta > 24*3600 {
				delta = 24 * 3600
			}
			if delta < 0 {
				delta = 0
			}
			ticksToAdd = delta
		}
		mult := math.Round((0.2*float64(p.Generation-1)+1)*10) / 10
		p.Ticks += float64(ticksToAdd) * mult
	}
	return p, nil
}

// SavePlant writes the savefile atomically.
func (m *Manager) SavePlant(p *plant.Plant) error {
	p.LastTime = m.nowUnix()
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tmp := m.SavefilePath + ".temp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.SavefilePath)
}

// DataWriteJSON writes the public <user>_plant_data.json export, revealing
// fields as the plant matures (schema matches Python data_write_json).
func (m *Manager) DataWriteJSON(p *plant.Plant) error {
	info := map[string]any{
		"owner":        p.Owner,
		"description":  p.ParsePlant(),
		"age":          p.AgeFormat(),
		"score":        p.Ticks,
		"is_dead":      p.Dead,
		"last_watered": p.WateredTimestamp,
		"file_name":    p.FileName,
		"stage":        plant.StageList[p.Stage],
		"generation":   p.Generation,
	}
	if p.Stage >= 3 {
		info["rarity"] = plant.RarityList[p.Rarity]
	}
	if p.Mutation != 0 {
		info["mutation"] = plant.MutationList[p.Mutation]
	}
	if p.Stage >= 4 {
		info["color"] = plant.ColorList[p.Color]
	}
	if p.Stage >= 2 {
		info["species"] = plant.SpeciesList[p.Species]
	}
	raw, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(m.PlantDataJSONPath, raw, 0o666)
}

// GardenEntry is one row of the community garden, as exported to JSON. The field
// tags match the Python garden_file.json keys.
type GardenEntry struct {
	Owner       string `json:"owner"`
	Description string `json:"description"`
	Age         string `json:"age"`
	Score       int    `json:"score"`
	Dead        int    `json:"dead"`
}

// VisitorCount is a visitor and their weekly visit tally.
type VisitorCount struct {
	Name   string
	Visits int
}

func (m *Manager) openDB() (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(m.GardenDBPath), 0o777); err != nil {
		return nil, err
	}
	return sql.Open("sqlite", m.GardenDBPath)
}

// InitDatabase creates the garden table and relaxes permissions so other users
// on a shared host can write to it.
func (m *Manager) InitDatabase() error {
	sqliteDir := filepath.Dir(m.GardenDBPath)
	if _, err := os.Stat(sqliteDir); os.IsNotExist(err) {
		if err := os.MkdirAll(sqliteDir, 0o777); err != nil {
			return err
		}
		os.Chmod(sqliteDir, 0o777)
	}
	db, err := m.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS garden (
		plant_id tinytext PRIMARY KEY,
		owner text,
		description text,
		age text,
		score integer,
		is_dead numeric
	)`)
	if err != nil {
		return err
	}
	// best-effort permission relaxing for multiuser hosts
	if info, statErr := os.Stat(m.GardenDBPath); statErr == nil && fileOwnedByUs(info) {
		os.Chmod(m.GardenDBPath, 0o666)
		f, _ := os.OpenFile(m.GardenJSONPath, os.O_CREATE, 0o666)
		if f != nil {
			f.Close()
		}
		os.Chmod(m.GardenJSONPath, 0o666)
	}
	return nil
}

// MigrateDatabase creates the visitors table if needed.
func (m *Manager) MigrateDatabase() error {
	db, err := m.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS visitors (
		id integer PRIMARY KEY,
		garden_name text,
		visitor_name text,
		weekly_visits integer
	)`)
	return err
}

// UpdateGardenDB inserts or updates this plant's row and marks the owner's other
// plants dead.
func (m *Manager) UpdateGardenDB(p *plant.Plant) error {
	if err := m.InitDatabase(); err != nil {
		return err
	}
	if err := m.MigrateDatabase(); err != nil {
		return err
	}
	db, err := m.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO garden (plant_id, owner, description, age, score, is_dead)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.PlantID, p.Owner, p.ParsePlant(), p.AgeFormat(), int(p.Ticks), boolToInt(p.Dead),
	); err != nil {
		return err
	}
	_, err = db.Exec(
		`UPDATE garden SET is_dead = 1 WHERE owner = ? AND plant_id <> ?`,
		p.Owner, p.PlantID,
	)
	return err
}

// RetrieveGarden returns the whole garden keyed by plant id.
func (m *Manager) RetrieveGarden() (map[string]GardenEntry, error) {
	if err := m.InitDatabase(); err != nil {
		return nil, err
	}
	db, err := m.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT plant_id, owner, description, age, score, is_dead FROM garden ORDER BY owner`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]GardenEntry{}
	for rows.Next() {
		var id string
		var e GardenEntry
		if err := rows.Scan(&id, &e.Owner, &e.Description, &e.Age, &e.Score, &e.Dead); err != nil {
			return nil, err
		}
		out[id] = e
	}
	return out, rows.Err()
}

// UpdateGardenJSON dumps the garden to garden_file.json.
func (m *Manager) UpdateGardenJSON() error {
	garden, err := m.RetrieveGarden()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(garden)
	if err != nil {
		return err
	}
	return os.WriteFile(m.GardenJSONPath, raw, 0o666)
}

// UpdateVisitorDB records visits: a new visitor starts at 1, an existing one is
// incremented, once per name occurrence.
func (m *Manager) UpdateVisitorDB(owner string, names []string) error {
	if err := m.MigrateDatabase(); err != nil {
		return err
	}
	db, err := m.openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	for _, name := range names {
		var count int
		err := db.QueryRow(
			`SELECT weekly_visits FROM visitors WHERE garden_name = ? AND visitor_name = ?`,
			owner, name,
		).Scan(&count)
		switch err {
		case sql.ErrNoRows:
			if _, err := db.Exec(
				`INSERT INTO visitors (garden_name, visitor_name, weekly_visits) VALUES (?, ?, 1)`,
				owner, name,
			); err != nil {
				return err
			}
		case nil:
			if _, err := db.Exec(
				`UPDATE visitors SET weekly_visits = weekly_visits + 1 WHERE garden_name = ? AND visitor_name = ?`,
				owner, name,
			); err != nil {
				return err
			}
		default:
			return err
		}
	}
	return nil
}

// WeeklyVisitors returns an owner's visitors ordered by visit count.
func (m *Manager) WeeklyVisitors(owner string) ([]VisitorCount, error) {
	if err := m.MigrateDatabase(); err != nil {
		return nil, err
	}
	db, err := m.openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(
		`SELECT visitor_name, weekly_visits FROM visitors WHERE garden_name = ? ORDER BY weekly_visits`,
		owner,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VisitorCount
	for rows.Next() {
		var v VisitorCount
		if err := rows.Scan(&v.Name, &v.Visits); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// HarvestPlant appends the plant to the harvest file, returning whether the
// harvest file was newly created.
func (m *Manager) HarvestPlant(p *plant.Plant) (bool, error) {
	entry := map[string]any{
		"description": p.ParsePlant(),
		"age":         p.AgeFormat(),
		"score":       p.Ticks,
	}
	harvest := map[string]any{}
	newFile := true
	if raw, err := os.ReadFile(m.HarvestPath); err == nil {
		newFile = false
		if err := json.Unmarshal(raw, &harvest); err != nil {
			return false, err
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	harvest[p.PlantID] = entry
	raw, err := json.Marshal(harvest)
	if err != nil {
		return false, err
	}
	tmp := m.HarvestPath + ".temp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, m.HarvestPath); err != nil {
		return false, err
	}
	return newFile, nil
}

// ProcessGuests folds guest waterings recorded in BotanyDir/visitors.json into
// the plant, updates the visitor database, and clears the file.
func (m *Manager) ProcessGuests(p *plant.Plant) error {
	visitorPath := filepath.Join(m.BotanyDir, "visitors.json")
	raw, err := os.ReadFile(visitorPath)
	if os.IsNotExist(err) {
		// create an empty, world-writable visitor inbox
		if werr := os.WriteFile(visitorPath, []byte("[]"), 0o666); werr != nil {
			return werr
		}
		os.Chmod(visitorPath, 0o666)
		return nil
	}
	if err != nil {
		return err
	}
	var entries []struct {
		User      string `json:"user"`
		Timestamp int64  `json:"timestamp"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return err
	}
	now := m.nowUnix()
	var guestTimestamps []int64
	var visitorsThisCheck []string
	seen := map[string]bool{}
	for _, v := range p.Visitors {
		seen[v] = true
	}
	for _, e := range entries {
		if !seen[e.User] {
			seen[e.User] = true
			p.Visitors = append(p.Visitors, e.User)
		}
		visitorsThisCheck = append(visitorsThisCheck, e.User)
		if e.Timestamp <= now && e.Timestamp >= p.WateredTimestamp {
			guestTimestamps = append(guestTimestamps, e.Timestamp)
		}
	}
	if len(visitorsThisCheck) > 0 {
		_ = m.UpdateVisitorDB(p.Owner, visitorsThisCheck) // best-effort, like Python
	}
	maxGapDays := float64(p.Class().DeathSecs) / 86400.0
	p.WateredTimestamp = plant.ResolveWatered(p.WateredTimestamp, guestTimestamps, time.Unix(now, 0), maxGapDays)
	return os.WriteFile(visitorPath, []byte("[]"), 0o666)
}

// WaterOnVisit appends a watering record to another player's visitors.json. It
// returns false (without error) if the file does not exist or is not writable.
func WaterOnVisit(visitorFile, username string, ts int64) (bool, error) {
	info, err := os.Stat(visitorFile)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_ = info
	if !isWritable(visitorFile) {
		return false, nil
	}
	raw, err := os.ReadFile(visitorFile)
	if err != nil {
		return false, err
	}
	var entries []map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &entries); err != nil {
			return false, err
		}
	}
	entries = append(entries, map[string]any{"user": username, "timestamp": ts})
	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(visitorFile, out, 0o666); err != nil {
		return false, err
	}
	return true, nil
}

// ReadPlantData reads another player's exported plant data JSON.
func ReadPlantData(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// SortedGardenIDs returns garden keys sorted by owner then id, for stable iteration.
func SortedGardenIDs(g map[string]GardenEntry) []string {
	ids := make([]string, 0, len(g))
	for id := range g {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if g[ids[i]].Owner != g[ids[j]].Owner {
			return g[ids[i]].Owner < g[ids[j]].Owner
		}
		return ids[i] < ids[j]
	})
	return ids
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
