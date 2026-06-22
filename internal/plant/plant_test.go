package plant

import (
	"math/rand"
	"testing"
	"time"
)

func fixedNow(t int64) func() time.Time {
	return func() time.Time { return time.Unix(t, 0) }
}

func TestNewPlantDefaults(t *testing.T) {
	p := New("/tmp/x_plant.json", 1)
	if p.Stage != 0 {
		t.Errorf("new plant stage = %d, want 0 (seed)", p.Stage)
	}
	if p.Generation != 1 {
		t.Errorf("generation = %d, want 1", p.Generation)
	}
	if p.Dead {
		t.Error("new plant should not be dead")
	}
	if p.PlantID == "" {
		t.Error("plant id should be set")
	}
	if p.Species < 0 || p.Species >= len(SpeciesList) {
		t.Errorf("species %d out of range", p.Species)
	}
	if p.Color < 0 || p.Color >= len(ColorList) {
		t.Errorf("color %d out of range", p.Color)
	}
	if p.Rarity < 0 || p.Rarity > 4 {
		t.Errorf("rarity %d out of range", p.Rarity)
	}
	// must water first day: watered_timestamp is in the past beyond 24h
	if p.WateredTimestamp >= p.StartTime {
		t.Error("new plant should start needing water")
	}
}

func TestLifeStages(t *testing.T) {
	want := [5]int64{1 * day, 3 * day, 10 * day, 20 * day, 30 * day}
	if LifeStages != want {
		t.Errorf("LifeStages = %v, want %v", LifeStages, want)
	}
}

func TestRarityBands(t *testing.T) {
	cases := []struct {
		seed int
		want int
	}{
		{1, 0}, {171, 0}, {172, 1}, {228, 1}, {229, 2},
		{247, 2}, {248, 3}, {253, 3}, {254, 4}, {256, 4},
	}
	for _, c := range cases {
		if got := rarityForSeed(c.seed); got != c.want {
			t.Errorf("rarityForSeed(%d) = %d, want %d", c.seed, got, c.want)
		}
	}
}

func TestRarityDistribution(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	counts := make([]int, 5)
	for i := 0; i < 100000; i++ {
		counts[RarityCheck(rng)]++
	}
	// common most frequent, godly least; strictly descending
	for i := 0; i < 4; i++ {
		if counts[i] <= counts[i+1] {
			t.Errorf("rarity counts not descending: %v", counts)
			break
		}
	}
}

func TestParsePlant(t *testing.T) {
	p := New("/tmp/x.json", 1)
	p.Species = 1 // cactus
	p.Color = 3   // green
	p.Rarity = 2  // rare
	p.Mutation = 0
	p.Stage = 0
	if got := p.ParsePlant(); got != "seed" {
		t.Errorf("stage0 = %q, want %q", got, "seed")
	}
	p.Mutation = 1 // humming shows at any stage
	if got := p.ParsePlant(); got != "humming seed" {
		t.Errorf("stage0+mutation = %q, want %q", got, "humming seed")
	}
	p.Mutation = 0
	p.Stage = 2
	if got := p.ParsePlant(); got != "young cactus" {
		t.Errorf("stage2 = %q, want %q", got, "young cactus")
	}
	p.Stage = 3
	if got := p.ParsePlant(); got != "rare mature cactus" {
		t.Errorf("stage3 = %q, want %q", got, "rare mature cactus")
	}
	p.Stage = 4
	p.Mutation = 1 // humming
	if got := p.ParsePlant(); got != "rare humming green flowering cactus" {
		t.Errorf("stage4 = %q, want %q", got, "rare humming green flowering cactus")
	}
}

func TestGrow(t *testing.T) {
	p := New("/tmp/x.json", 1)
	for i := 0; i < 10; i++ {
		p.Grow()
	}
	max := len(StageList) - 1
	if p.Stage != max {
		t.Errorf("stage after many grows = %d, want capped at %d", p.Stage, max)
	}
}

func TestDeadCheck(t *testing.T) {
	now := int64(10_000_000)
	p := New("/tmp/x.json", 1)
	p.Species = indexOfSpecies("poppy") // flower: 5-day drought tolerance
	p.now = fixedNow(now)
	// watered just under 5 days ago -> alive
	p.WateredTimestamp = now - 5*day + 1
	if p.DeadCheck() {
		t.Error("plant watered <5d ago should be alive")
	}
	// watered just over 5 days ago -> dead
	p.WateredTimestamp = now - 5*day - 1
	if !p.DeadCheck() {
		t.Error("plant unwatered >5d should be dead")
	}
	// already dead stays dead
	p2 := New("/tmp/y.json", 1)
	p2.now = fixedNow(now)
	p2.Dead = true
	p2.WateredTimestamp = now
	if !p2.DeadCheck() {
		t.Error("dead plant stays dead")
	}
}

func TestWaterCheck(t *testing.T) {
	now := int64(10_000_000)
	p := New("/tmp/x.json", 1)
	p.Species = indexOfSpecies("poppy") // flower: 24h water interval
	p.now = fixedNow(now)
	p.WateredTimestamp = now - day + 1 // within 24h
	if !p.WaterCheck() {
		t.Error("within 24h should be watered")
	}
	if !p.Watered24h {
		t.Error("Watered24h flag should be set")
	}
	p.WateredTimestamp = now - day - 1 // beyond 24h
	if p.WaterCheck() {
		t.Error("beyond 24h should not be watered")
	}
	if p.Watered24h {
		t.Error("Watered24h flag should be cleared")
	}
}

func TestWater(t *testing.T) {
	now := int64(10_000_000)
	p := New("/tmp/x.json", 1)
	p.now = fixedNow(now)
	p.Water()
	if p.WateredTimestamp != now {
		t.Errorf("watered timestamp = %d, want %d", p.WateredTimestamp, now)
	}
	if !p.Watered24h {
		t.Error("watering should set Watered24h")
	}
	// dead plant cannot be watered
	p.Dead = true
	p.WateredTimestamp = 0
	p.Water()
	if p.WateredTimestamp == now {
		t.Error("dead plant should not update watered timestamp")
	}
}

func TestGenerationBonus(t *testing.T) {
	cases := []struct {
		gen     int
		scoreIn float64
		mult    float64
	}{
		{1, 1.0, 1.0},
		{2, 1.2, 1.2},
		{3, 1.4, 1.4},
	}
	for _, c := range cases {
		p := New("/tmp/x.json", c.gen)
		if got := p.ScoreIncrement(); got != c.scoreIn {
			t.Errorf("gen %d ScoreIncrement = %v, want %v", c.gen, got, c.scoreIn)
		}
		if got := p.GrowthMultiplier(); got != c.mult {
			t.Errorf("gen %d GrowthMultiplier = %v, want %v", c.gen, got, c.mult)
		}
	}
}

func TestAgeFormat(t *testing.T) {
	start := int64(1_000_000)
	p := New("/tmp/x.json", 1)
	p.StartTime = start
	p.now = fixedNow(start + 2*day + 3*3600 + 4*60 + 5)
	if got := p.AgeFormat(); got != "2d:3h:4m:5s" {
		t.Errorf("AgeFormat = %q, want %q", got, "2d:3h:4m:5s")
	}
}

func TestSetMutation(t *testing.T) {
	p := New("/tmp/x.json", 1)
	p.Mutation = 0
	if !p.setMutation(5) {
		t.Error("fresh plant should accept mutation 5")
	}
	if p.Mutation != 5 {
		t.Errorf("mutation = %d, want 5", p.Mutation)
	}
	// already mutated: no change
	if p.setMutation(7) {
		t.Error("already-mutated plant should reject new mutation")
	}
	if p.Mutation != 5 {
		t.Errorf("mutation changed to %d, want 5", p.Mutation)
	}
}

func TestStartOver(t *testing.T) {
	p := New("/tmp/x.json", 2)
	p.Stage = 5
	p.Dead = false
	oldID := p.PlantID
	p.StartOver()
	if p.Generation != 3 {
		t.Errorf("alive start-over generation = %d, want 3", p.Generation)
	}
	if p.Stage != 0 || p.Dead {
		t.Error("start-over should reset to a fresh living seed")
	}
	if p.PlantID == oldID {
		t.Error("start-over should mint a new plant id")
	}
	// dead start-over keeps generation
	p2 := New("/tmp/y.json", 4)
	p2.Dead = true
	p2.StartOver()
	if p2.Generation != 4 {
		t.Errorf("dead start-over generation = %d, want 4", p2.Generation)
	}
}

func TestTick(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	p := NewWithDeps("/tmp/x.json", 1, fixedNow(10*day), rng)
	p.Species = indexOfSpecies("hemp") // herb: no ability, deterministic base tick
	// dead: no change
	p.Dead = true
	p.Watered24h = true
	p.Tick()
	if p.Ticks != 0 {
		t.Errorf("dead plant ticked to %v, want 0", p.Ticks)
	}
	// alive but not watered: no change
	p.Dead = false
	p.Watered24h = false
	p.Tick()
	if p.Ticks != 0 {
		t.Errorf("unwatered plant ticked to %v, want 0", p.Ticks)
	}
	// alive + watered: gains base score (no ability bonus for herbs)
	p.Watered24h = true
	p.Tick()
	if p.Ticks != 1.0 {
		t.Errorf("watered plant ticks = %v, want 1.0", p.Ticks)
	}
	// crossing the class-scaled stage-0 threshold grows the plant
	threshold := float64(LifeStages[0]) * p.Class().GrowthMultiplier
	p.Ticks = threshold - 0.5
	p.Tick()
	if p.Stage != 1 {
		t.Errorf("stage after crossing threshold = %d, want 1", p.Stage)
	}
}

func TestResolveWatered(t *testing.T) {
	now := int64(100 * day)
	base := now - 3*day // last watered 3 days ago by owner
	// guest watered 1 day ago -> should advance watered timestamp
	guest := now - 1*day
	got := ResolveWatered(base, []int64{guest}, time.Unix(now, 0), 5)
	if got != guest {
		t.Errorf("ResolveWatered = %d, want %d (latest within 5d window)", got, guest)
	}
	// a guest timestamp after a >5 day gap is excluded
	gapBase := now - 20*day
	near := now - 19*day // 1 day after base, within window
	far := now - 1*day   // 18 days later -> gap >5d, excluded
	got2 := ResolveWatered(gapBase, []int64{near, far}, time.Unix(now, 0), 5)
	if got2 != near {
		t.Errorf("ResolveWatered with gap = %d, want %d", got2, near)
	}
	// no guests -> unchanged
	if got3 := ResolveWatered(base, nil, time.Unix(now, 0), 5); got3 != base {
		t.Errorf("ResolveWatered no guests = %d, want %d", got3, base)
	}
}
