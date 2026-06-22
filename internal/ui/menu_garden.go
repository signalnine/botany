package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"botany/internal/plant"
	"botany/internal/storage"
)

// drawGarden shows the paginated community garden with sort and filter, ported
// from the Python draw_garden.
func (m *Menu) drawGarden() {
	m.infoKind = 0
	m.infoLines = nil
	garden, err := m.data.RetrieveGarden()
	if err != nil {
		return
	}
	original := FormatGardenData(garden)
	rows := make([]GardenRow, len(original))
	copy(rows, original)

	sortColumn, sortAscending := 0, true
	sortKeys := map[rune]int{'n': 0, 'a': 1, 's': 2, 'd': 3}
	SortGarden(rows, sortColumn, sortAscending)

	index := 0
	for {
		_, h := m.size()
		perPage := h - 16
		if perPage < 1 {
			perPage = 1
		}
		indexMax := index + perPage
		if indexMax > len(rows) {
			indexMax = len(rows)
		}

		m.renderBase()
		for i := index; i < indexMax; i++ {
			m.drawStr(2, 14+(i-index), FormatGardenEntry(rows[i]), tcell.StyleDefault)
		}
		status := fmt.Sprintf("(%d-%d/%d) | sp/next | bksp/prev | s <col #>/sort | f/filter | q/quit",
			index, indexMax, len(rows))
		m.drawStr(2, h-2, status, tcell.StyleDefault)
		m.screen.Show()

		ev := m.nextKeyModal()
		if ev == nil {
			return
		}

		switch {
		case ev.Key() == tcell.KeyEscape, ev.Rune() == 'q', ev.Rune() == 'x':
			return
		case ev.Key() == tcell.KeyEnter, ev.Key() == tcell.KeyPgDn, ev.Rune() == ' ':
			index += perPage
			if index >= len(rows) {
				return
			}
		case ev.Key() == tcell.KeyBackspace, ev.Key() == tcell.KeyBackspace2, ev.Key() == tcell.KeyPgUp:
			index -= perPage
			if index < 0 {
				index = 0
			}
		case ev.Rune() == 'j', ev.Key() == tcell.KeyDown:
			if index+1 <= len(rows)-1 {
				index++
			}
			if index < 0 {
				index = 0
			}
		case ev.Rune() == 'k', ev.Key() == tcell.KeyUp:
			if index > 0 {
				index--
			}
		case ev.Rune() == 's':
			next := m.nextKeyModal()
			if next == nil {
				return
			}
			column := -1
			if c, ok := sortKeys[next.Rune()]; ok {
				column = c
			} else if next.Rune() >= '1' && next.Rune() <= '4' {
				column = int(next.Rune() - '1')
			}
			if column != -1 {
				if sortColumn == column {
					sortAscending = !sortAscending
				} else {
					sortColumn = column
					sortAscending = true
				}
				SortGarden(rows, sortColumn, sortAscending)
			}
		case ev.Rune() == '/', ev.Rune() == 'f':
			m.drawStr(2, h-2, "Filter: "+strings.Repeat(" ", len(status)-8), tcell.StyleDefault)
			m.screen.Show()
			pattern := m.getUserString(10, h-2, isPrintable, nil)
			rows = FilterGarden(original, pattern)
			SortGarden(rows, sortColumn, sortAscending)
			index = 0
		}
	}
}

// visitHandler lets the player water a neighbour's plant, ported from the Python
// visit_handler.
func (m *Menu) visitHandler() {
	m.infoKind = 0
	m.infoLines = nil

	m.mu.Lock()
	visitors := append([]string(nil), m.plant.Visitors...)
	owner := m.plant.Owner
	m.mu.Unlock()

	m.renderBase()
	m.drawStr(2, 14, "whose plant would you like to visit?", tcell.StyleDefault)
	m.drawStr(2, 15, "~", tcell.StyleDefault)
	if len(visitors) > 0 {
		m.drawStr(2, 17, "since last time, you were visited by: ", tcell.StyleDefault)
		m.drawStr(2, 18, m.latestVisitorLine(visitors), tcell.StyleDefault)
		m.mu.Lock()
		m.plant.Visitors = []string{}
		m.mu.Unlock()
	}
	m.drawStr(2, 20, "this week you've been visited by: ", tcell.StyleDefault)
	m.drawStr(2, 21, m.weeklyVisitorText(owner), tcell.StyleDefault)
	m.screen.Show()

	comp := NewCompleter(m.gardenOwners())
	guest := m.getUserString(3, 15, isAlnum, comp)
	if guest == "" {
		return
	}
	if strings.EqualFold(guest, m.data.User) {
		m.drawStr(2, 16, "you're already here!", tcell.StyleDefault)
		m.screen.Show()
		m.nextKeyModal()
		return
	}

	guestJSON := filepath.Join(m.data.HomeParent, guest, ".botany", guest+"_plant_data.json")
	guestDesc := ""
	if data, err := storage.ReadPlantData(guestJSON); err == nil {
		if d, ok := data["description"].(string); ok {
			guestDesc = d
		}
		m.visitedPlant = visitedPlantFromData(data)
	}

	defer func() { m.visitedPlant = nil }()
	guestVisitorFile := filepath.Join(m.data.HomeParent, guest, ".botany", "visitors.json")
	if _, err := os.Stat(guestVisitorFile); err == nil {
		ok, _ := storage.WaterOnVisit(guestVisitorFile, m.data.User, time.Now().Unix()-1)
		if ok {
			m.renderBase()
			m.drawStr(2, 16, fmt.Sprintf("...you watered ~%s's %s...", guest, guestDesc), tcell.StyleDefault)
		} else {
			m.renderBase()
			m.drawStr(2, 16, fmt.Sprintf("%s's garden is locked, but you can see in...", guest), tcell.StyleDefault)
		}
	} else {
		m.renderBase()
		m.drawStr(2, 16, fmt.Sprintf("i can't seem to find directions to %s...", guest), tcell.StyleDefault)
	}
	m.screen.Show()
	m.nextKeyModal()
}

// gardenOwners returns the distinct owners in the community garden, for tab
// completion when visiting.
func (m *Menu) gardenOwners() []string {
	garden, err := m.data.RetrieveGarden()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, e := range garden {
		if e.Owner != "" && !seen[e.Owner] {
			seen[e.Owner] = true
			out = append(out, e.Owner)
		}
	}
	return out
}

func (m *Menu) latestVisitorLine(visitors []string) string {
	w, _ := m.size()
	var b strings.Builder
	for _, v := range visitors {
		if b.Len()+len(v) > w-10 {
			b.WriteString("and more")
			break
		}
		b.WriteString(v + " ")
	}
	return b.String()
}

func (m *Menu) weeklyVisitorText(owner string) string {
	counts, err := m.data.WeeklyVisitors(owner)
	if err != nil || len(counts) == 0 {
		return "nobody :("
	}
	w, _ := m.size()
	var block, line strings.Builder
	for _, c := range counts {
		s := fmt.Sprintf("%s(%d) ", c.Name, c.Visits)
		if line.Len()+len(s) > w-3 {
			block.WriteString("\n")
			line.Reset()
		}
		block.WriteString(s)
		line.WriteString(s)
	}
	return block.String()
}

// visitedPlantFromData builds a drawable plant from another player's exported
// JSON, ported from the Python get_visited_plant.
func visitedPlantFromData(data map[string]any) *drawablePlant {
	dead, ok := data["is_dead"].(bool)
	if !ok {
		return nil
	}
	p := &drawablePlant{Dead: dead}
	if dead {
		return p
	}
	if stage, ok := data["stage"].(string); ok {
		if idx := indexOf(plant.StageList, stage); idx >= 0 {
			p.Stage = idx
		}
	}
	if species, ok := data["species"].(string); ok {
		idx := indexOf(plant.SpeciesList, species)
		if idx < 0 {
			return nil
		}
		p.Species = idx
	} else if p.Stage > 1 {
		return nil
	}
	return p
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}
