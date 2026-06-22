package plant

import (
	"math/rand"
	"testing"
	"time"
)

func TestEverySpeciesHasAClass(t *testing.T) {
	if len(speciesClass) != len(SpeciesList) {
		t.Fatalf("speciesClass has %d entries, SpeciesList has %d", len(speciesClass), len(SpeciesList))
	}
	for i, cid := range speciesClass {
		if _, ok := Classes[cid]; !ok {
			t.Errorf("species %d (%s) maps to unknown class %d", i, SpeciesList[i], cid)
		}
	}
}

func TestClassTraitsAreSane(t *testing.T) {
	for id, c := range Classes {
		if c.Name == "" {
			t.Errorf("class %d has empty name", id)
		}
		if c.WaterIntervalSecs <= 0 || c.DeathSecs <= c.WaterIntervalSecs {
			t.Errorf("class %s: death (%d) must exceed water interval (%d), both positive", c.Name, c.DeathSecs, c.WaterIntervalSecs)
		}
		if c.GrowthMultiplier <= 0 {
			t.Errorf("class %s: growth multiplier must be positive", c.Name)
		}
		if c.MutationRarity <= 0 {
			t.Errorf("class %s: mutation rarity must be positive", c.Name)
		}
	}
}

func TestClassBySpecies(t *testing.T) {
	cactus := indexOfSpecies("cactus")
	fern := indexOfSpecies("fern")
	p := New("/tmp/x.json", 1)
	p.Species = cactus
	if p.Class().DeathSecs <= New("/tmp/y.json", 1).classOf(fern).DeathSecs {
		t.Error("cactus should tolerate dryness longer than fern")
	}
}

func TestAbilityCarnivoreBonus(t *testing.T) {
	// Find a seed where the carnivore catches prey to confirm the bonus path.
	got := false
	for seed := int64(0); seed < 2000 && !got; seed++ {
		p := NewWithDeps("/tmp/x.json", 1, fixedNow(12*3600), rand.New(rand.NewSource(seed)))
		p.Species = indexOfSpecies("venus flytrap")
		if p.Class().Ability != AbilityCarnivore {
			t.Fatal("venus flytrap should be a carnivore")
		}
		if p.abilityBonus() > 0 {
			got = true
		}
	}
	if !got {
		t.Error("carnivore should sometimes yield a catch bonus")
	}
}

func TestAbilityDayNight(t *testing.T) {
	// Build times with an explicit location so .Hour() is timezone-independent.
	noon := func() time.Time { return time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC) }
	night := func() time.Time { return time.Date(2020, 1, 1, 2, 0, 0, 0, time.UTC) }
	rng := rand.New(rand.NewSource(1))

	// A photosynthetic class gains in daylight, not at night.
	sun := NewWithDeps("/tmp/s.json", 1, noon, rng)
	sun.Species = indexOfSpecies("sunflower")
	if sun.Class().Ability != AbilityPhotosynth {
		t.Fatal("sunflower should be photosynthetic")
	}
	if sun.abilityBonus() <= 0 {
		t.Error("photosynthetic plant should gain a bonus in daylight")
	}
	sun.SetClock(night)
	if sun.abilityBonus() != 0 {
		t.Error("photosynthetic plant should gain nothing at night")
	}
}

func TestGrowthSpeedDiffersByClass(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	// herb (fast) reaches stage 1 with fewer ticks than tree (slow)
	herb := NewWithDeps("/tmp/h.json", 1, fixedNow(12*3600), rng)
	herb.Species = indexOfSpecies("hemp")
	tree := NewWithDeps("/tmp/t.json", 1, fixedNow(12*3600), rng)
	tree.Species = indexOfSpecies("baobab")
	if herb.Class().GrowthMultiplier >= tree.Class().GrowthMultiplier {
		t.Errorf("herb growth mult (%v) should be less than tree (%v)", herb.Class().GrowthMultiplier, tree.Class().GrowthMultiplier)
	}
}

// helpers for tests
func indexOfSpecies(name string) int {
	for i, s := range SpeciesList {
		if s == name {
			return i
		}
	}
	return -1
}

func (p *Plant) classOf(species int) Class {
	return Classes[speciesClass[species]]
}
