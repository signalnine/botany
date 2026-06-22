package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"botany/internal/plant"
)

// TestEverySpeciesHasArt ensures every species has .txt and .ansi art for each
// of its three displayed stages, and that plantArtList stays parallel to the
// species list.
func TestEverySpeciesHasArt(t *testing.T) {
	if len(plantArtList) != len(plant.SpeciesList) {
		t.Fatalf("plantArtList has %d entries, SpeciesList has %d", len(plantArtList), len(plant.SpeciesList))
	}
	artDir := filepath.Join("..", "..", "art")
	for i, base := range plantArtList {
		for _, stage := range []string{"1", "2", "3"} {
			for _, ext := range []string{".txt", ".ansi"} {
				path := filepath.Join(artDir, base+stage+ext)
				if _, err := os.Stat(path); err != nil {
					t.Errorf("species %d (%s) missing art file %s", i, plant.SpeciesList[i], base+stage+ext)
				}
			}
		}
	}
}

// TestNewSpeciesArtRenders renders a new species at its flowering stage and
// confirms art appears on screen.
func TestNewSpeciesArtRenders(t *testing.T) {
	m, screen, p := newTestMenu(t)
	m.useColor = true
	rose := indexOfSpeciesUI("rose")
	if rose < 0 {
		t.Fatal("rose species not found")
	}
	p.Species = rose
	p.Stage = 4 // flowering -> rose3 art
	m.draw()
	out := screenText(screen)
	// the rose art uses '@' blooms; ensure non-blank art was drawn on the right
	if !strings.ContainsRune(out, '@') {
		t.Errorf("rose flowering art did not render:\n%s", out)
	}
}

func indexOfSpeciesUI(name string) int {
	for i, s := range plant.SpeciesList {
		if s == name {
			return i
		}
	}
	return -1
}
