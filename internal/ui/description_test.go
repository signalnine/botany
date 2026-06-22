package ui

import (
	"math/rand"
	"strings"
	"testing"

	"botany/internal/plant"
)

func TestPlantDescriptionNoFormatLeak(t *testing.T) {
	// Stage 0/1/99 templates have no format verbs; formatting must not leak
	// "%!(EXTRA ...)" into the text.
	for _, stage := range []int{0, 1, 5} {
		p := plant.New("/tmp/x.json", 1)
		p.Stage = stage
		for seed := int64(0); seed < 50; seed++ {
			got := plantDescription(p, rand.New(rand.NewSource(seed)))
			if strings.Contains(got, "%!") || strings.Contains(got, "EXTRA") {
				t.Fatalf("stage %d description leaked format args: %q", stage, got)
			}
		}
	}
}

func TestPlantDescriptionFillsSpecies(t *testing.T) {
	p := plant.New("/tmp/x.json", 1)
	p.Stage = 2
	p.Species = 1 // cactus
	got := plantDescription(p, rand.New(rand.NewSource(1)))
	if !strings.Contains(got, "cactus") {
		t.Errorf("stage 2 description should mention species: %q", got)
	}
}

func TestPlantDescriptionDeadStage(t *testing.T) {
	p := plant.New("/tmp/x.json", 1)
	p.Dead = true
	got := plantDescription(p, rand.New(rand.NewSource(1)))
	if strings.Contains(got, "%") {
		t.Errorf("dead description should have no format leftovers: %q", got)
	}
	if got == "" {
		t.Error("dead plant should still have a description")
	}
}
