// Command botany is a command-line, realtime, community plant buddy: a Go port
// of Jake Funke's Python botany. You're given a seed that grows into a plant;
// water it every 24h to keep it growing, and water your friends' plants too.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"

	"botany/internal/plant"
	"botany/internal/storage"
	"botany/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "botany:", err)
		os.Exit(1)
	}
}

func run() error {
	data, err := storage.Default()
	if err != nil {
		return err
	}

	var p *plant.Plant
	if data.CheckPlant() {
		p, err = data.LoadPlant()
		if err != nil {
			return fmt.Errorf("loading plant: %w", err)
		}
	} else {
		p = plant.New(data.SavefilePath, 1)
		p.Owner = data.User
		if err := data.DataWriteJSON(p); err != nil {
			return err
		}
	}
	if p.Owner == "" {
		p.Owner = data.User
	}
	applyDebugOverrides(p)

	var mu sync.Mutex
	stop := make(chan struct{})
	go runLife(p, data, &mu, stop, 2*time.Second)

	artSub, err := fs.Sub(artFS, "art")
	if err != nil {
		return err
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	// Ensure the terminal is restored even on panic.
	defer screen.Fini()
	screen.SetStyle(tcell.StyleDefault)
	screen.HideCursor()
	screen.Clear()

	useColor := screen.Colors() > 0 && ui.UseColorEnv(os.Getenv)
	menu := ui.NewMenu(screen, p, data, &mu, artSub, useColor)
	menu.Run()

	// Stop background life and finalize the screen before persisting.
	close(stop)
	screen.Fini()

	mu.Lock()
	defer mu.Unlock()
	if err := data.SavePlant(p); err != nil {
		return err
	}
	if err := data.DataWriteJSON(p); err != nil {
		return err
	}
	return data.UpdateGardenDB(p)
}

// applyDebugOverrides lets you preview a specific species/stage via the
// BOTANY_SPECIES (name or index) and BOTANY_STAGE (0-5) environment variables.
// Intended for development and previewing the art; no effect when unset.
func applyDebugOverrides(p *plant.Plant) {
	if s := os.Getenv("BOTANY_SPECIES"); s != "" {
		if idx, err := strconv.Atoi(s); err == nil && idx >= 0 && idx < len(plant.SpeciesList) {
			p.Species = idx
		} else {
			for i, name := range plant.SpeciesList {
				if strings.EqualFold(name, s) {
					p.Species = i
					break
				}
			}
		}
	}
	if s := os.Getenv("BOTANY_STAGE"); s != "" {
		if stage, err := strconv.Atoi(s); err == nil && stage >= 0 && stage < len(plant.StageList) {
			p.Stage = stage
		}
	}
}

// runLife is the background life loop, ported from Plant.life: it credits score,
// advances growth, folds in guest waterings, harvests on death, and periodically
// persists state. It ticks every two seconds.
func runLife(p *plant.Plant, data *storage.Manager, mu *sync.Mutex, stop <-chan struct{}, interval time.Duration) {
	counter := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}

		counter++
		mu.Lock()
		if !p.Dead && p.Watered24h {
			p.Tick()
		}
		_ = data.ProcessGuests(p)
		p.WaterCheck()
		if p.DeadCheck() {
			_, _ = data.HarvestPlant(p)
		}
		if counter%3 == 0 {
			_ = data.SavePlant(p)
			_ = data.DataWriteJSON(p)
			_ = data.UpdateGardenDB(p)
		}
		if counter%30 == 0 {
			_ = data.UpdateGardenJSON()
			counter = 0
		}
		mu.Unlock()
	}
}
