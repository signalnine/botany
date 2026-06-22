package main

import (
	"io/fs"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"botany/internal/plant"
	"botany/internal/storage"
	"botany/internal/ui"
)

// TestLifeLoopAndUIConcurrent runs the real background life loop alongside the
// real UI event loop, hammering them with input so the race detector can prove
// the shared plant state is correctly synchronized. It also confirms that
// selecting exit (Escape) actually terminates Run.
func TestLifeLoopAndUIConcurrent(t *testing.T) {
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
	p.Watered24h = true
	p.WateredTimestamp = time.Now().Unix()

	var mu sync.Mutex
	stop := make(chan struct{})
	lifeDone := make(chan struct{})
	go func() {
		runLife(p, data, &mu, stop, time.Millisecond) // fast ticks to stress the mutex
		close(lifeDone)
	}()

	art, err := fs.Sub(artFS, "art")
	if err != nil {
		t.Fatalf("art fs: %v", err)
	}
	menu := ui.NewMenu(screen, p, data, &mu, art, false)

	done := make(chan struct{})
	go func() {
		menu.Run() // must return when Escape selects exit
		close(done)
	}()

	go func() {
		// drive navigation to force frequent concurrent reads of the plant
		for i := 0; i < 30; i++ {
			screen.InjectKey(tcell.KeyDown, 0, tcell.ModNone)
			time.Sleep(2 * time.Millisecond)
		}
		// Escape selects exit. Retry so a momentarily full sim-screen event
		// queue can't drop it and hang the test.
		for {
			select {
			case <-done:
				return
			default:
				screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
				time.Sleep(20 * time.Millisecond)
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("menu.Run did not exit after Escape")
	}
	// Stop the life loop and wait for it to finish writing before the test's
	// temp dirs are torn down.
	close(stop)
	<-lifeDone
	screen.Fini()
}
