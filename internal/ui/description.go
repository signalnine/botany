package ui

import (
	"fmt"
	"math/rand"
	"strings"

	"botany/internal/plant"
)

var stageDescriptions = map[int][]string{
	0: {
		"You're excited about your new seed.",
		"You wonder what kind of plant your seed will grow into.",
		"You're ready for a new start with this plant.",
		"You're tired of waiting for your seed to grow.",
		"You wish your seed could tell you what it needs.",
		"You can feel the spirit inside your seed.",
		"These pretzels are making you thirsty.",
		"Way to plant, Ann!",
		"'To see things in the seed, that is genius' - Lao Tzu",
	},
	1: {
		"The seedling fills you with hope.",
		"The seedling shakes in the wind.",
		"You can make out a tiny leaf - or is that a thorn?",
		"You can feel the seedling looking back at you.",
		"You blow a kiss to your seedling.",
		"You think about all the seedlings who came before it.",
		"You and your seedling make a great team.",
		"Your seedling grows slowly and quietly.",
		"You meditate on the paths your plant's life could take.",
	},
	2: {
		"The %[1]s makes you feel relaxed.",
		"You sing a song to your %[1]s.",
		"You quietly sit with your %[1]s for a few minutes.",
		"Your %[1]s looks pretty good.",
		"You play loud techno to your %[1]s.",
		"You play piano to your %[1]s.",
		"You play rap music to your %[1]s.",
		"You whistle a tune to your %[1]s.",
		"You read a poem to your %[1]s.",
		"You tell a secret to your %[1]s.",
		"You play your favorite record for your %[1]s.",
	},
	3: {
		"Your %[1]s is growing nicely!",
		"You're proud of the dedication it took to grow your %[1]s.",
		"You take a deep breath with your %[1]s.",
		"You think of all the words that rhyme with %[1]s.",
		"The %[1]s looks full of life.",
		"The %[1]s inspires you.",
		"Your %[1]s makes you forget about your problems.",
		"Your %[1]s gives you a reason to keep going.",
		"Looking at your %[1]s helps you focus on what matters.",
		"You think about how nice this %[1]s looks here.",
		"The buds of your %[1]s might bloom soon.",
	},
	4: {
		"The %[2]s flowers look nice on your %[1]s!",
		"The %[2]s flowers have bloomed and fill you with positivity.",
		"The %[2]s flowers remind you of your childhood.",
		"The %[2]s flowers remind you of spring mornings.",
		"The %[2]s flowers remind you of a forgotten memory.",
		"The %[2]s flowers remind you of your happy place.",
		"The aroma of the %[2]s flowers energize you.",
		"The %[1]s has grown beautiful %[2]s flowers.",
		"The %[2]s petals remind you of that favorite shirt you lost.",
		"The %[2]s flowers remind you of your crush.",
		"You smell the %[2]s flowers and are filled with peace.",
	},
	5: {
		"You fondly remember the time you spent caring for your %[1]s.",
		"Seed pods have grown on your %[1]s.",
		"You feel like your %[1]s appreciates your care.",
		"The %[1]s fills you with love.",
		"You're ready for whatever comes after your %[1]s.",
		"You're excited to start growing your next plant.",
		"You reflect on when your %[1]s was just a seedling.",
		"You grow nostalgic about the early days with your %[1]s.",
	},
	99: {
		"You wish you had taken better care of your plant.",
		"If only you had watered your plant more often..",
		"Your plant is dead, there's always next time.",
		"You cry over the withered leaves of your plant.",
		"Your plant died. Maybe you need a fresh start.",
	},
}

// plantDescription builds the multi-line "look" text for a plant, including
// growth hints and species/rarity/color hints by stage. Ported from the Python
// get_plant_description.
func plantDescription(p *plant.Plant, rng *rand.Rand) string {
	species := plant.SpeciesList[p.Species]
	color := plant.ColorList[p.Color]
	stage := p.Stage
	if p.Dead {
		stage = 99
	}

	var out strings.Builder

	// growth hint for not-yet-grown plants
	if stage <= 4 {
		var lastGrowthAt int64
		if stage >= 1 {
			lastGrowthAt = plant.LifeStages[stage-1]
		}
		ticksSinceLast := p.Ticks - float64(lastGrowthAt)
		ticksBetween := float64(plant.LifeStages[stage] - lastGrowthAt)
		if ticksSinceLast >= ticksBetween*0.8 {
			out.WriteString("You notice your plant looks different.\n")
		}
	}

	choices := stageDescriptions[stage]
	line := choices[rng.Intn(len(choices))]
	if strings.Contains(line, "%") {
		line = fmt.Sprintf(line, species, color)
	}
	out.WriteString(line)
	out.WriteString("\n")

	switch stage {
	case 1:
		opts := []string{
			species,
			plant.SpeciesList[mod(p.Species+3, len(plant.SpeciesList))],
			plant.SpeciesList[mod(p.Species-3, len(plant.SpeciesList))],
		}
		rng.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
		out.WriteString(fmt.Sprintf("It could be a(n) %s, %s, or %s.\n", opts[0], opts[1], opts[2]))
	case 2:
		if p.Rarity >= 2 {
			out.WriteString("You feel like your plant is special..\n")
		}
	case 3:
		opts := []string{
			color,
			plant.ColorList[mod(p.Color+3, len(plant.ColorList))],
			plant.ColorList[mod(p.Color-3, len(plant.ColorList))],
		}
		rng.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
		out.WriteString(fmt.Sprintf("You can see the first hints of %s, %s, or %s.\n", opts[0], opts[1], opts[2]))
	}

	return out.String()
}

// mod is a Python-style modulo that always returns a non-negative result.
func mod(a, n int) int {
	return ((a % n) + n) % n
}
