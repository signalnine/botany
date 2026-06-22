package ui

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"botany/internal/plant"
	"botany/internal/storage"
)

func newTestMenu(t *testing.T) (*Menu, tcell.SimulationScreen, *plant.Plant) {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen init: %v", err)
	}
	screen.SetSize(80, 24)

	botanyDir := filepath.Join(t.TempDir(), ".botany")
	gameDir := t.TempDir()
	data, err := storage.New("tester", botanyDir, gameDir)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	p := plant.New(data.SavefilePath, 1)
	p.Owner = "tester"

	art := os.DirFS(filepath.Join("..", "..", "art"))
	var mu sync.Mutex
	return NewMenu(screen, p, data, &mu, art, false), screen, p
}

func screenText(screen tcell.SimulationScreen) string {
	cells, w, h := screen.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			runes := cells[y*w+x].Runes
			if len(runes) == 0 || runes[0] == 0 {
				b.WriteByte(' ')
			} else {
				b.WriteRune(runes[0])
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestMenuRendersBaseUI(t *testing.T) {
	m, screen, _ := newTestMenu(t)
	m.draw()
	out := screenText(screen)
	for _, want := range []string{"botany", "options", "1 - water", "plant:", "score:", "seed", "%"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered UI missing %q\n---\n%s", want, out)
		}
	}
}

func TestMenuWaterAction(t *testing.T) {
	m, _, p := newTestMenu(t)
	p.WateredTimestamp = time.Now().Unix() - 2*24*3600 // dry
	m.handleRequest("water")
	if !p.Watered24h {
		t.Error("watering via menu should set Watered24h")
	}
	if time.Now().Unix()-p.WateredTimestamp > 5 {
		t.Error("watering should set a recent watered timestamp")
	}
}

func TestMenuNavigation(t *testing.T) {
	m, _, _ := newTestMenu(t)
	start := m.selected
	m.handleNav(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if m.selected != (start+1)%len(m.options) {
		t.Errorf("down moved selection to %d, want %d", m.selected, start+1)
	}
	// number key jumps to that option
	m.handleNav(tcell.NewEventKey(tcell.KeyRune, '3', tcell.ModNone))
	if m.selected != 2 {
		t.Errorf("'3' selected %d, want index 2", m.selected)
	}
	// enter returns the selected option
	if req := m.handleNav(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); req != m.options[2] {
		t.Errorf("enter returned %q, want %q", req, m.options[2])
	}
}

func TestMenuLookShowsDescription(t *testing.T) {
	m, screen, _ := newTestMenu(t)
	m.handleRequest("look")
	m.draw()
	out := screenText(screen)
	if !strings.Contains(out, "Generation:") || !strings.Contains(out, "Growth rate:") {
		t.Errorf("look pane missing generation info\n---\n%s", out)
	}
	// toggling look again hides it
	m.handleRequest("look")
	if m.infoKind != 0 || m.infoLines != nil {
		t.Error("second look should hide the info pane")
	}
}

func TestMenuHarvestOptionAppearsAtFinalStage(t *testing.T) {
	m, _, p := newTestMenu(t)
	p.Stage = 5
	m.updateOptions(p.Dead, p.Stage)
	found := false
	for _, o := range m.options {
		if o == "harvest" {
			found = true
		}
	}
	if !found {
		t.Errorf("harvest option should appear at final stage; options=%v", m.options)
	}
	if m.options[len(m.options)-1] != "exit" {
		t.Errorf("exit must remain last; options=%v", m.options)
	}
}

func TestMenuRendersColorArt(t *testing.T) {
	m, screen, p := newTestMenu(t)
	m.useColor = true
	p.Stage = 2
	p.Species = 1 // cactus -> cactus1.ansi exists
	m.draw()
	// ensure some cell carries a non-default style (color was applied)
	cells, w, h := screen.GetContents()
	colored := false
	for i := 0; i < w*h; i++ {
		fg, _, _ := cells[i].Style.Decompose()
		if fg != tcell.ColorDefault {
			colored = true
			break
		}
	}
	if !colored {
		t.Error("colored art should produce styled cells")
	}
}
