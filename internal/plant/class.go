package plant

// This file adds plant classes (families). Each species belongs to a class that
// determines how it behaves: how long a watering lasts, how long it survives
// without water, how fast it grows, how readily it mutates, and any special
// ability that runs each life tick. Class is derived from the species index, so
// existing savefiles load unchanged.

// Ability identifies a class's special per-tick behavior.
type Ability int

const (
	AbilityNone       Ability = iota
	AbilityCarnivore          // catches prey for occasional bonus score
	AbilityVining             // spreads for occasional bonus score
	AbilityNocturnal          // gains a little extra at night
	AbilityPhotosynth         // gains a little extra in daylight
	AbilityHardy              // passive: exceptional drought endurance (no per-tick bonus)
)

// abilityNames are the human-readable ability descriptions shown in "look".
var abilityNames = map[Ability]string{
	AbilityNone:       "",
	AbilityCarnivore:  "catches insects for extra growth",
	AbilityVining:     "spreads vigorously for extra growth",
	AbilityNocturnal:  "thrives at night",
	AbilityPhotosynth: "soaks up daylight",
	AbilityHardy:      "stores water and endures drought",
}

// Class describes a family of plants that share survival and growth behavior.
type Class struct {
	Name              string
	WaterIntervalSecs int64   // how long a watering keeps the plant growing (and how long the gauge takes to empty)
	DeathSecs         int64   // seconds without water before death
	GrowthMultiplier  float64 // scales stage thresholds; <1 grows faster, >1 slower
	MutationRarity    int     // 1-in-N chance of a mutation per tick
	Ability           Ability
}

// AbilityName returns the flavor text for the class's ability, or "" if none.
func (c Class) AbilityName() string { return abilityNames[c.Ability] }

// ClassID identifies a plant class.
type ClassID int

const (
	ClassFlower ClassID = iota
	ClassSucculent
	ClassCactus
	ClassTree
	ClassHerb
	ClassFern
	ClassCarnivore
	ClassVine
	ClassFungus
	ClassAquatic
	ClassGrass
)

const (
	hour = int64(3600)
	dayS = int64(24 * 3600)
)

// Classes holds the trait table for every plant class.
var Classes = map[ClassID]Class{
	ClassFlower:    {"flower", dayS, 5 * dayS, 1.0, 9000, AbilityPhotosynth},
	ClassSucculent: {"succulent", 2 * dayS, 10 * dayS, 1.3, 12000, AbilityHardy},
	ClassCactus:    {"cactus", 3 * dayS, 14 * dayS, 1.5, 15000, AbilityNocturnal},
	ClassTree:      {"tree", 36 * hour, 7 * dayS, 1.8, 20000, AbilityPhotosynth},
	ClassHerb:      {"herb", 20 * hour, 4 * dayS, 0.7, 8000, AbilityNone},
	ClassFern:      {"fern", 12 * hour, 3 * dayS, 0.9, 11000, AbilityNocturnal},
	ClassCarnivore: {"carnivore", dayS, 5 * dayS, 1.0, 5000, AbilityCarnivore},
	ClassVine:      {"vine", dayS, 6 * dayS, 0.8, 9000, AbilityVining},
	ClassFungus:    {"fungus", 18 * hour, 4 * dayS, 0.85, 4000, AbilityNocturnal},
	ClassAquatic:   {"aquatic", 10 * hour, 2 * dayS, 0.9, 10000, AbilityPhotosynth},
	ClassGrass:     {"grass", dayS, 5 * dayS, 0.6, 9000, AbilityNone},
}

// speciesClass maps each species index to its class. It must stay parallel to
// SpeciesList.
var speciesClass = []ClassID{
	ClassFlower,    // 0  poppy
	ClassCactus,    // 1  cactus
	ClassSucculent, // 2  aloe
	ClassCarnivore, // 3  venus flytrap
	ClassSucculent, // 4  jade plant
	ClassFern,      // 5  fern
	ClassFlower,    // 6  daffodil
	ClassFlower,    // 7  sunflower
	ClassTree,      // 8  baobab
	ClassSucculent, // 9  lithops
	ClassHerb,      // 10 hemp
	ClassFlower,    // 11 pansy
	ClassFlower,    // 12 iris
	ClassSucculent, // 13 agave
	ClassTree,      // 14 ficus
	ClassFern,      // 15 moss
	ClassHerb,      // 16 sage
	ClassFlower,    // 17 snapdragon
	ClassFlower,    // 18 columbine
	ClassFlower,    // 19 brugmansia
	ClassTree,      // 20 palm
	ClassSucculent, // 21 pachypodium
	ClassGrass,     // 22 bamboo
	ClassFlower,    // 23 rose
	ClassFlower,    // 24 orchid
	ClassFungus,    // 25 mushroom
	ClassVine,      // 26 ivy
	ClassHerb,      // 27 lavender
	ClassTree,      // 28 maple
	ClassCarnivore, // 29 pitcher plant
	ClassCarnivore, // 30 sundew
	ClassAquatic,   // 31 water lily
	ClassAquatic,   // 32 kelp
	ClassFlower,    // 33 tulip
	ClassVine,      // 34 morning glory
}

// Class returns the plant's class traits.
func (p *Plant) Class() Class {
	if p.Species < 0 || p.Species >= len(speciesClass) {
		return Classes[ClassFlower]
	}
	return Classes[speciesClass[p.Species]]
}

// ability bonus tuning
const (
	carnivoreCatchChance = 300 // 1-in-N ticks
	carnivoreCatchBonus  = 10.0
	vineSpreadChance     = 300
	vineSpreadBonus      = 6.0
	dayNightBonus        = 0.25
)

func isNight(hour int) bool { return hour < 6 || hour >= 20 }

// abilityBonus returns extra ticks this life iteration from the class ability.
func (p *Plant) abilityBonus() float64 {
	switch p.Class().Ability {
	case AbilityCarnivore:
		if p.rand().Intn(carnivoreCatchChance) == 0 {
			return carnivoreCatchBonus
		}
	case AbilityVining:
		if p.rand().Intn(vineSpreadChance) == 0 {
			return vineSpreadBonus
		}
	case AbilityNocturnal:
		if isNight(p.clock().Hour()) {
			return dayNightBonus
		}
	case AbilityPhotosynth:
		if !isNight(p.clock().Hour()) {
			return dayNightBonus
		}
	}
	return 0
}
