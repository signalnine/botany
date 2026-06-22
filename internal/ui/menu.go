// Package ui renders the botany game with tcell, porting the curses-based
// CursedMenu from the Python original. The pure presentation logic (art parsing,
// water gauge, garden sort/filter, completer) lives in sibling files and is unit
// tested; this file drives the screen and input.
package ui

import (
	"fmt"
	"io/fs"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"

	"botany/internal/plant"
	"botany/internal/storage"
)

// plantArtList maps a species index to its art file basename. The names differ
// from SpeciesList for a few species (e.g. "venus flytrap" -> "flytrap").
var plantArtList = []string{
	"poppy", "cactus", "aloe", "flytrap", "jadeplant", "fern",
	"daffodil", "sunflower", "baobab", "lithops", "hemp", "pansy",
	"iris", "agave", "ficus", "moss", "sage", "snapdragon",
	"columbine", "brugmansia", "palm", "pachypodium",
	// Go port additions (must stay parallel to plant.SpeciesList):
	"bamboo", "rose", "orchid", "mushroom", "ivy", "lavender", "maple",
	"pitcher", "sundew", "waterlily", "kelp", "tulip", "morningglory",
}

// Menu is the running terminal UI.
type Menu struct {
	screen   tcell.Screen
	plant    *plant.Plant
	data     *storage.Manager
	mu       *sync.Mutex
	art      fs.FS
	useColor bool
	rng      *rand.Rand

	options  []string
	selected int
	title    string
	subtitle string

	infoLines    []string
	infoKind     int // 0 none, 1 look, 4 instructions
	visitedPlant *drawablePlant

	exit bool
}

// drawablePlant is the minimal data needed to render a plant's art, used for
// both the live plant and visited plants.
type drawablePlant struct {
	Dead    bool
	Stage   int
	Species int
}

// tickEvent is posted once a second to drive live redraws.
type tickEvent struct{ t time.Time }

func (e *tickEvent) When() time.Time { return e.t }

// NewMenu builds a Menu. art is a filesystem rooted at the art directory.
func NewMenu(screen tcell.Screen, p *plant.Plant, data *storage.Manager, mu *sync.Mutex, art fs.FS, useColor bool) *Menu {
	return &Menu{
		screen:   screen,
		plant:    p,
		data:     data,
		mu:       mu,
		art:      art,
		useColor: useColor,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
		options:  []string{"water", "look", "garden", "visit", "instructions", "exit"},
		title:    " botany ",
		subtitle: "options",
	}
}

// Run drives the main event loop until the user exits.
func (m *Menu) Run() {
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case now := <-t.C:
				m.screen.PostEvent(&tickEvent{t: now})
			}
		}
	}()
	defer close(stop)

	m.draw()
	for !m.exit {
		switch ev := m.screen.PollEvent().(type) {
		case nil:
			return
		case *tickEvent:
			m.draw()
		case *tcell.EventResize:
			m.screen.Sync()
			m.draw()
		case *tcell.EventKey:
			req := m.handleNav(ev)
			if req == "exit" {
				m.exit = true
				continue
			}
			if req != "" {
				m.handleRequest(req)
			}
			m.draw()
		}
	}
}

func (m *Menu) size() (int, int) { return m.screen.Size() }

func (m *Menu) drawStr(x, y int, s string, style tcell.Style) {
	w, h := m.size()
	if y < 0 || y >= h {
		return
	}
	for _, r := range s {
		if x >= w {
			break
		}
		if x >= 0 {
			m.screen.SetContent(x, y, r, nil, style)
		}
		x++
	}
}

// updateOptions inserts or removes the "harvest" option based on plant state.
func (m *Menu) updateOptions(dead bool, stage int) {
	has := false
	idx := -1
	for i, o := range m.options {
		if o == "harvest" {
			has = true
			idx = i
		}
	}
	if dead || stage == 5 {
		if !has {
			// insert before the trailing "exit"
			n := len(m.options)
			m.options = append(m.options[:n-1], "harvest", m.options[n-1])
		}
	} else if has {
		m.options = append(m.options[:idx], m.options[idx+1:]...)
	}
	if m.selected >= len(m.options) {
		m.selected = len(m.options) - 1
	}
}

func (m *Menu) snapshot() (dead bool, stage, species, ticks, gen int, wts, waterInterval int64, pstr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.plant
	return p.Dead, p.Stage, p.Species, int(p.Ticks), p.Generation, p.WateredTimestamp, p.Class().WaterIntervalSecs, p.ParsePlant()
}

// renderBase clears the screen and draws the menu, plant status, water gauge, and
// plant art, without flushing.
func (m *Menu) renderBase() {
	dead, stage, species, ticks, _, wts, waterInterval, pstr := m.snapshot()
	m.updateOptions(dead, stage)
	m.screen.Clear()

	highlight := tcell.StyleDefault.Reverse(true)
	normal := tcell.StyleDefault
	bold := tcell.StyleDefault.Bold(true)
	dim := tcell.StyleDefault.Dim(true)

	m.drawStr(2, 1, m.title, tcell.StyleDefault.Reverse(true))
	m.drawStr(2, 3, m.subtitle, bold)

	for i, opt := range m.options {
		style := normal
		if i == m.selected {
			style = highlight
		}
		m.drawStr(4, 4+i, fmt.Sprintf("%d - %s", i+1, opt), style)
	}

	m.drawStr(2, 12, "plant:", dim)
	m.drawStr(9, 12, pstr, normal)
	m.drawStr(2, 13, "score:", dim)
	m.drawStr(9, 13, fmt.Sprintf("%d", ticks), normal)

	if !dead {
		m.drawStr(14, 4, WaterGauge(time.Now().Unix(), wts, waterInterval), normal)
	} else {
		m.drawStr(14, 4, "(   RIP   )", normal)
	}

	dp := &drawablePlant{Dead: dead, Stage: stage, Species: species}
	if m.visitedPlant != nil {
		dp = m.visitedPlant
	}
	m.drawPlantArt(dp)
}

// draw renders the base UI plus the info pane and flushes.
func (m *Menu) draw() {
	m.renderBase()
	for i, line := range m.infoLines {
		m.drawStr(2, 14+i, line, tcell.StyleDefault)
	}
	m.screen.Show()
}

func (m *Menu) artXPos() int {
	w, _ := m.size()
	return (w-37)/2 + 25
}

// drawPlantArt selects and renders the art for the given plant state.
func (m *Menu) drawPlantArt(p *drawablePlant) {
	ypos, xpos := 0, m.artXPos()
	now := time.Now()
	switch {
	case p.Dead:
		m.renderArt("rip", ypos, xpos)
	case now.Month() == 10 && now.Day() == 31:
		m.renderArt("jackolantern", ypos, xpos)
	case p.Stage == 0:
		m.renderArt("seed", ypos, xpos)
	case p.Stage == 1:
		m.renderArt("seedling", ypos, xpos)
	case p.Stage == 2:
		m.renderArt(plantArtList[p.Species]+"1", ypos, xpos)
	case p.Stage == 3 || p.Stage == 5:
		m.renderArt(plantArtList[p.Species]+"2", ypos, xpos)
	case p.Stage == 4:
		m.renderArt(plantArtList[p.Species]+"3", ypos, xpos)
	}
}

// renderArt draws an art file, preferring the colored .ansi version when color
// is enabled and available, otherwise the .txt version.
func (m *Menu) renderArt(name string, ypos, xpos int) {
	if m.useColor {
		if data, err := fs.ReadFile(m.art, name+".ansi"); err == nil {
			m.renderANSI(data, ypos, xpos)
			return
		}
	}
	if data, err := fs.ReadFile(m.art, name+".txt"); err == nil {
		m.renderASCII(data, ypos, xpos)
	}
}

func (m *Menu) renderASCII(data []byte, ypos, xpos int) {
	w, h := m.size()
	for i, line := range strings.Split(string(data), "\n") {
		y := ypos + i + 2
		if y >= h {
			break
		}
		line = strings.TrimRight(line, "\r")
		x := xpos
		for _, r := range line {
			if x >= w {
				break
			}
			m.screen.SetContent(x, y, r, nil, tcell.StyleDefault)
			x++
		}
	}
}

func (m *Menu) renderANSI(data []byte, ypos, xpos int) {
	w, h := m.size()
	pair := 0
	for i, line := range strings.Split(string(data), "\n") {
		y := ypos + i + 2
		if y >= h {
			break
		}
		var cells []ArtCell
		cells, pair = ParseANSILine(line, pair)
		x := xpos
		for _, c := range cells {
			if x >= w {
				break
			}
			m.screen.SetContent(x, y, c.Ch, nil, pairStyle(c.Pair))
			x++
		}
	}
}

// handleNav processes a keypress on the main menu, returning a selected option to
// act on or "" for navigation-only input.
func (m *Menu) handleNav(ev *tcell.EventKey) string {
	switch ev.Key() {
	case tcell.KeyEnter:
		return m.options[m.selected]
	case tcell.KeyEscape:
		return m.options[len(m.options)-1]
	case tcell.KeyDown, tcell.KeyCtrlN:
		m.selected = (m.selected + 1) % len(m.options)
		return ""
	case tcell.KeyUp, tcell.KeyCtrlP:
		m.selected = (m.selected - 1 + len(m.options)) % len(m.options)
		return ""
	case tcell.KeyRune:
		r := ev.Rune()
		switch r {
		case 'q':
			m.selected = len(m.options) - 1
			return ""
		case 'j':
			m.selected = (m.selected + 1) % len(m.options)
			return ""
		case 'k':
			m.selected = (m.selected - 1 + len(m.options)) % len(m.options)
			return ""
		}
		maxDigit := len(m.options)
		if maxDigit > 7 {
			maxDigit = 7
		}
		if r >= '1' && r <= rune('0'+maxDigit) {
			m.selected = int(r-'0') - 1
		}
	}
	return ""
}

func (m *Menu) handleRequest(request string) {
	switch request {
	case "water":
		m.mu.Lock()
		m.plant.Water()
		m.mu.Unlock()
	case "look":
		m.toggleLook()
	case "instructions":
		m.toggleInstructions()
	case "garden":
		m.drawGarden()
	case "visit":
		m.visitHandler()
	case "harvest":
		m.harvestConfirm()
	}
}

func (m *Menu) toggleLook() {
	if m.infoKind == 1 {
		m.infoKind = 0
		m.infoLines = nil
		return
	}
	m.mu.Lock()
	text := plantDescription(m.plant, m.rng)
	gen := m.plant.Generation
	class := m.plant.Class()
	m.mu.Unlock()
	mult := 1 + 0.2*float64(gen-1)
	text += fmt.Sprintf("Generation: %d\nGrowth rate: %.1fx\n", gen, mult)
	text += fmt.Sprintf("Class: %s", class.Name)
	if ability := class.AbilityName(); ability != "" {
		text += " - " + ability
	}
	m.infoLines = splitTruncate(text, m.lineWidth())
	m.infoKind = 1
}

func (m *Menu) toggleInstructions() {
	if m.infoKind == 4 {
		m.infoKind = 0
		m.infoLines = nil
		return
	}
	text := "welcome to botany. you've been given a seed\n" +
		"that will grow into a beautiful plant. check\n" +
		"in and water your plant every 24h to keep it\n" +
		"growing. 5 days without water = death. your\n" +
		"plant depends on you & your friends to live!\n" +
		"more info is available in the readme :)\n" +
		"https://github.com/jifunks/botany/blob/master/README.md\n" +
		"                               cheers,\n" +
		"                               curio\n"
	m.infoLines = splitTruncate(text, m.lineWidth())
	m.infoKind = 4
}

func (m *Menu) lineWidth() int {
	w, _ := m.size()
	return w - 3
}

func splitTruncate(text string, width int) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if width > 0 && len(line) > width {
			line = line[:width]
		}
		out = append(out, line)
	}
	return out
}

func (m *Menu) clearInfoPane() {
	w, h := m.size()
	blank := strings.Repeat(" ", w-3)
	for y := 14; y < h; y++ {
		m.drawStr(2, y, blank, tcell.StyleDefault)
	}
}

// nextKeyModal polls for the next key, redrawing on resize and ignoring live
// ticks (used by modal sub-screens).
func (m *Menu) nextKeyModal() *tcell.EventKey {
	for {
		switch e := m.screen.PollEvent().(type) {
		case nil:
			m.exit = true
			return nil
		case *tcell.EventResize:
			m.screen.Sync()
		case *tcell.EventKey:
			return e
		}
	}
}

func (m *Menu) harvestConfirm() {
	m.mu.Lock()
	dead := m.plant.Dead
	stage := m.plant.Stage
	maxStage := len(plant.StageList) - 1
	gen := m.plant.Generation
	m.mu.Unlock()

	var b strings.Builder
	if !dead && stage == maxStage {
		b.WriteString("Congratulations! You raised your plant to its final stage of growth.\n")
		b.WriteString(fmt.Sprintf("Your next plant will grow at a speed of: %.1fx\n", 1+0.2*float64(gen)))
	}
	b.WriteString("If you harvest your plant you'll start over from a seed.\nContinue? (Y/n)")

	m.renderBase()
	for i, line := range splitTruncate(b.String(), m.lineWidth()) {
		m.drawStr(2, 14+i, line, tcell.StyleDefault)
	}
	m.screen.Show()

	ev := m.nextKeyModal()
	if ev == nil {
		return
	}
	yes := ev.Key() == tcell.KeyEnter || ev.Rune() == 'Y' || ev.Rune() == 'y'
	if yes {
		m.mu.Lock()
		m.data.HarvestPlant(m.plant)
		m.plant.StartOver()
		m.data.SavePlant(m.plant)
		m.data.DataWriteJSON(m.plant)
		m.data.UpdateGardenDB(m.plant)
		m.mu.Unlock()
	}
	m.infoKind = 0
	m.infoLines = nil
}
