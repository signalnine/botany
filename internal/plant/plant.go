// Package plant holds the botany game logic: a plant's growth stages, species,
// rarity, mutations, watering and death, and generation bonuses. The logic is a
// faithful port of the original Python plant.py, with time and randomness made
// injectable so the behavior can be tested deterministically.
package plant

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/google/uuid"
)

// StageList names each growth stage, indexed by Plant.Stage.
var StageList = []string{
	"seed",
	"seedling",
	"young",
	"mature",
	"flowering",
	"seed-bearing",
}

// ColorList names each flower color, indexed by Plant.Color.
var ColorList = []string{
	"red", "orange", "yellow", "green", "blue", "indigo",
	"violet", "white", "black", "gold", "rainbow",
}

// RarityList names each rarity tier, indexed by Plant.Rarity.
var RarityList = []string{"common", "uncommon", "rare", "legendary", "godly"}

// SpeciesList names each species, indexed by Plant.Species. The first 22 match
// the Python original; the rest are added by the Go port and must stay parallel
// to speciesClass in class.go.
var SpeciesList = []string{
	"poppy", "cactus", "aloe", "venus flytrap", "jade plant", "fern",
	"daffodil", "sunflower", "baobab", "lithops", "hemp", "pansy",
	"iris", "agave", "ficus", "moss", "sage", "snapdragon",
	"columbine", "brugmansia", "palm", "pachypodium",
	// Go port additions:
	"bamboo", "rose", "orchid", "mushroom", "ivy", "lavender", "maple",
	"pitcher plant", "sundew", "water lily", "kelp", "tulip", "morning glory",
}

// MutationList names each mutation, indexed by Plant.Mutation. Index 0 is "none".
var MutationList = []string{
	"", "humming", "noxious", "vorpal", "glowing", "electric", "icy",
	"flaming", "psychic", "screaming", "chaotic", "hissing", "gelatinous",
	"deformed", "shaggy", "scaly", "depressed", "anxious", "metallic",
	"glossy", "psychedelic", "bonsai", "foamy", "singing", "fractal",
	"crunchy", "goth", "oozing", "stinky", "aromatic", "juicy", "smug",
	"vibrating", "lithe", "chalky", "naive", "ersatz", "disco", "levitating",
	"colossal", "luminous", "cosmic", "ethereal", "cursed", "buff", "narcotic",
	"gnu/linux", "abraxan", // rip dear friend
}

const day = int64(24 * 3600)

// LifeStages holds the tick thresholds (in seconds-equivalent) at which a plant
// advances to the next stage: seedling, young, mature, flowering, seed-bearing.
var LifeStages = [5]int64{1 * day, 3 * day, 10 * day, 20 * day, 30 * day}

// Plant is your plant. Exported fields are persisted as JSON; the on-disk schema
// is private to the Go port (the Python original used pickle).
type Plant struct {
	PlantID          string   `json:"plant_id"`
	Stage            int      `json:"stage"`
	Mutation         int      `json:"mutation"`
	Species          int      `json:"species"`
	Color            int      `json:"color"`
	Rarity           int      `json:"rarity"`
	Ticks            float64  `json:"ticks"`
	Generation       int      `json:"generation"`
	Dead             bool     `json:"dead"`
	Owner            string   `json:"owner"`
	FileName         string   `json:"file_name"`
	StartTime        int64    `json:"start_time"`
	LastTime         int64    `json:"last_time"`
	WateredTimestamp int64    `json:"watered_timestamp"`
	Watered24h       bool     `json:"watered_24h"`
	Visitors         []string `json:"visitors"`

	// Injected dependencies, never serialized. nil means use real time / a
	// lazily seeded RNG.
	now func() time.Time
	rng *rand.Rand
}

// New creates a fresh plant owned by owner-less caller; owner is set by callers
// that know the username. It uses real time and a seeded RNG.
func New(fileName string, generation int) *Plant {
	p := &Plant{FileName: fileName}
	p.init(generation)
	return p
}

// NewWithDeps is like New but injects a clock and RNG for deterministic tests.
func NewWithDeps(fileName string, generation int, now func() time.Time, rng *rand.Rand) *Plant {
	p := &Plant{FileName: fileName, now: now, rng: rng}
	p.init(generation)
	return p
}

func (p *Plant) init(generation int) {
	r := p.rand()
	nowT := p.clock().Unix()
	p.PlantID = uuid.NewString()
	p.Stage = 0
	p.Mutation = 0
	p.Species = r.Intn(len(SpeciesList))
	p.Color = r.Intn(len(ColorList))
	p.Rarity = RarityCheck(r)
	p.Ticks = 0
	p.Generation = generation
	p.Dead = false
	p.StartTime = nowT
	p.LastTime = nowT
	// Must water the plant first: set the last watering just past the plant's
	// class water interval so it starts out thirsty regardless of class.
	p.WateredTimestamp = nowT - p.Class().WaterIntervalSecs - 1
	p.Watered24h = false
	p.Visitors = []string{}
}

// SetClock installs a custom time source (used at load time and in tests).
func (p *Plant) SetClock(now func() time.Time) { p.now = now }

// SetRand installs a custom RNG (used at load time and in tests).
func (p *Plant) SetRand(rng *rand.Rand) { p.rng = rng }

func (p *Plant) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

func (p *Plant) rand() *rand.Rand {
	if p.rng == nil {
		p.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return p.rng
}

// rarityForSeed maps a roll in [1,256] to a rarity tier, matching the Python
// rarity_check band computation.
func rarityForSeed(seed int) int {
	const max = 256.0
	commonRange := math.Round((2.0 / 3) * max)
	uncommonRange := math.Round((2.0 / 3) * (max - commonRange))
	rareRange := math.Round((2.0 / 3) * (max - commonRange - uncommonRange))
	legendaryRange := math.Round((2.0 / 3) * (max - commonRange - uncommonRange - rareRange))

	commonMax := commonRange
	uncommonMax := commonMax + uncommonRange
	rareMax := uncommonMax + rareRange
	legendaryMax := rareMax + legendaryRange

	s := float64(seed)
	switch {
	case s <= commonMax:
		return 0
	case s <= uncommonMax:
		return 1
	case s <= rareMax:
		return 2
	case s <= legendaryMax:
		return 3
	default:
		return 4
	}
}

// RarityCheck rolls a new rarity tier.
func RarityCheck(rng *rand.Rand) int {
	return rarityForSeed(rng.Intn(256) + 1)
}

// ParsePlant renders the plant's traits as a human-readable string, revealing
// more detail as the plant matures (matches Python parse_plant).
func (p *Plant) ParsePlant() string {
	out := ""
	if p.Stage >= 3 {
		out += RarityList[p.Rarity] + " "
	}
	if p.Mutation != 0 {
		out += MutationList[p.Mutation] + " "
	}
	if p.Stage >= 4 {
		out += ColorList[p.Color] + " "
	}
	out += StageList[p.Stage] + " "
	if p.Stage >= 2 {
		out += SpeciesList[p.Species] + " "
	}
	// trim a single trailing space (and any stray surrounding whitespace)
	for len(out) > 0 && out[len(out)-1] == ' ' {
		out = out[:len(out)-1]
	}
	return out
}

// Grow advances the plant one stage, capped at the final stage.
func (p *Plant) Grow() {
	if p.Stage < len(StageList)-1 {
		p.Stage++
	}
}

// DeadCheck returns whether the plant is dead, killing it once its class's
// drought tolerance has been exceeded since the last watering.
func (p *Plant) DeadCheck() bool {
	if p.Dead {
		return true
	}
	if p.clock().Unix()-p.WateredTimestamp > p.Class().DeathSecs {
		p.Dead = true
	}
	return p.Dead
}

// WaterCheck returns whether the plant has been watered within its class's water
// interval (the window during which it keeps growing) and updates Watered24h.
func (p *Plant) WaterCheck() bool {
	delta := p.clock().Unix() - p.WateredTimestamp
	if delta <= p.Class().WaterIntervalSecs {
		p.Watered24h = true
		return true
	}
	p.Watered24h = false
	return false
}

// Water waters a living plant.
func (p *Plant) Water() {
	if !p.Dead {
		p.WateredTimestamp = p.clock().Unix()
		p.Watered24h = true
	}
}

// ScoreIncrement is the per-tick score gain including the generation bonus,
// rounded to one decimal place (matches Python score_inc).
func (p *Plant) ScoreIncrement() float64 {
	bonus := round1(0.2 * float64(p.Generation-1))
	return round1(1 * (1 + bonus))
}

// GrowthMultiplier is the displayed growth-rate multiplier for this generation.
func (p *Plant) GrowthMultiplier() float64 {
	return 1 + 0.2*float64(p.Generation-1)
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// setMutation applies mutation index idx if the plant has none yet. It reports
// whether a visible mutation was applied.
func (p *Plant) setMutation(idx int) bool {
	if p.Mutation != 0 {
		return false
	}
	p.Mutation = idx
	return idx != 0
}

// MutateCheck gives the plant a per-tick chance (set by its class) to gain a
// mutation. It reports whether a mutation was newly applied.
func (p *Plant) MutateCheck() bool {
	rarity := p.Class().MutationRarity
	r := p.rand()
	if r.Intn(rarity)+1 == rarity {
		return p.setMutation(r.Intn(len(MutationList)))
	}
	return false
}

// AgeFormat renders the plant's age as "Nd:Nh:Nm:Ns".
func (p *Plant) AgeFormat() string {
	ageSeconds := p.clock().Unix() - p.StartTime
	days := ageSeconds / (24 * 60 * 60)
	ageSeconds %= 24 * 60 * 60
	hours := ageSeconds / (60 * 60)
	ageSeconds %= 60 * 60
	minutes := ageSeconds / 60
	seconds := ageSeconds % 60
	return fmt.Sprintf("%dd:%dh:%dm:%ds", days, hours, minutes, seconds)
}

// StartOver resets the plant to a fresh seed after it reaches its final stage,
// advancing the generation if the plant was still alive.
func (p *Plant) StartOver() {
	next := p.Generation
	if !p.Dead {
		next++
	}
	p.Dead = true
	p.init(next)
}

// Tick performs one life iteration's state update when the plant is alive and
// watered: it adds score (plus any class ability bonus), advances growth at
// class-scaled thresholds, and rolls for mutation.
func (p *Plant) Tick() {
	if p.Dead || !p.Watered24h {
		return
	}
	p.Ticks += p.ScoreIncrement() + p.abilityBonus()
	threshold := float64(LifeStages[p.Stage]) * p.Class().GrowthMultiplier
	if p.Stage < len(StageList)-1 && p.Ticks >= threshold {
		p.Grow()
	}
	p.MutateCheck()
}

// ResolveWatered computes the effective watered timestamp given the plant's own
// last watering and a set of guest waterings, ignoring any watering that occurs
// after a gap longer than maxGapDays (the plant would already have died). The
// Python original hardcoded five days; here the caller passes the class's
// drought tolerance.
func ResolveWatered(base int64, guests []int64, now time.Time, maxGapDays float64) int64 {
	valid := make([]int64, 0, len(guests))
	for _, g := range guests {
		if g <= now.Unix() && g >= base {
			valid = append(valid, g)
		}
	}
	if len(valid) == 0 {
		return base
	}
	all := append([]int64{base}, valid...)
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	// find the first gap longer than the tolerance; keep timestamps only up to it
	for i := 1; i < len(all); i++ {
		if float64(all[i]-all[i-1])/86400.0 > maxGapDays {
			return all[i-1]
		}
	}
	return all[len(all)-1]
}
