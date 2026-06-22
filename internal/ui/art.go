package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// escCodeToPair maps the SGR color codes used in the .ansi art files to one of
// the eight curses-style color pairs the original used. Mirrors the Python
// CursedMenu.ESC_CODE_TO_PAIR table.
var escCodeToPair = map[string]int{
	"":        0, // normal
	"0":       0, // normal
	"30":      1, // black
	"31":      7, // red
	"32":      3, // green
	"33":      6, // yellow
	"34":      4, // blue
	"35":      5, // magenta
	"36":      8, // cyan
	"37":      2, // white
	"39":      0, // normal
	"38;5;1":  7, // red
	"38;5;2":  3, // green
	"38;5;3":  6, // yellow
	"38;5;4":  4, // blue
	"38;5;5":  5, // magenta
	"38;5;6":  8, // cyan
	"38;5;7":  2, // white
	"38;5;8":  0, // normal
	"38;5;9":  7, // red
	"38;5;10": 3, // green
	"38;5;11": 6, // yellow
	"38;5;12": 4, // blue
	"38;5;13": 5, // magenta
	"38;5;14": 8, // cyan
	"38;5;15": 2, // white
	"38;5;16": 1, // black
}

// escPair returns the color pair for an SGR code, defaulting to 0 (normal).
func escPair(code string) int {
	return escCodeToPair[code]
}

// ArtCell is a single rendered glyph with its color pair (0-8).
type ArtCell struct {
	Ch   rune
	Pair int
}

// ParseANSILine parses one line of .ansi art into colored cells, carrying the
// previous line's trailing color pair in via startPair. It returns the cells and
// the pair in effect at the end of the line. Mirrors the Python ansi_render
// token-splitting logic.
func ParseANSILine(line string, startPair int) ([]ArtCell, int) {
	line = strings.TrimRight(line, "\r\n")
	pair := startPair
	var cells []ArtCell
	for _, tok := range strings.Split(line, "\x1b[") {
		text := tok
		if i := strings.IndexByte(tok, 'm'); i >= 0 {
			pair = escPair(tok[:i])
			text = tok[i+1:]
		}
		for _, r := range text {
			cells = append(cells, ArtCell{Ch: r, Pair: pair})
		}
	}
	return cells, pair
}

// pairStyle maps a curses-style color pair to a tcell style. The pairs mirror
// CursedMenu.define_colors.
func pairStyle(pair int) tcell.Style {
	s := tcell.StyleDefault
	switch pair {
	case 1:
		return s.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
	case 2:
		return s.Foreground(tcell.ColorWhite)
	case 3:
		return s.Foreground(tcell.ColorGreen)
	case 4:
		return s.Foreground(tcell.ColorBlue)
	case 5:
		return s.Foreground(tcell.ColorDarkMagenta)
	case 6:
		return s.Foreground(tcell.ColorYellow)
	case 7:
		return s.Foreground(tcell.ColorRed)
	case 8:
		return s.Foreground(tcell.ColorTeal)
	default:
		return s
	}
}

// UseColorEnv reports whether color output is wanted based on environment
// variables only (terminal capability is checked separately by the caller).
func UseColorEnv(getenv func(string) string) bool { return useColorEnv(getenv) }

// useColorEnv applies the PYTHON_COLORS > NO_COLOR > FORCE_COLOR precedence using
// the supplied getenv function. It does not check terminal capability.
func useColorEnv(getenv func(string) string) bool {
	if getenv("PYTHON_COLORS") == "0" {
		return false
	}
	if getenv("PYTHON_COLORS") == "1" {
		return true
	}
	if getenv("NO_COLOR") != "" {
		return false
	}
	if getenv("FORCE_COLOR") != "" {
		return true
	}
	return true
}
