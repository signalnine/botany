package ui

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// isAlnum mirrors Python str.isalnum for the characters we care about.
func isAlnum(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isPrintable mirrors membership in Python's string.printable for typed filters.
func isPrintable(r rune) bool {
	return r >= 32 && r <= 126
}

// getUserString reads a line of input at (xpos,ypos), accepting characters that
// pass filter (plus '_'), with optional tab completion. Ported from the Python
// get_user_string.
func (m *Menu) getUserString(xpos, ypos int, filter func(rune) bool, comp *Completer) string {
	var s string
	for {
		ev := m.nextKeyModal()
		if ev == nil {
			return s
		}
		switch ev.Key() {
		case tcell.KeyEnter:
			return s
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if len(s) > 0 {
				s = s[:len(s)-1]
				if comp != nil {
					comp.UpdateInput(s)
				}
				m.clearLine(xpos, ypos)
			}
		case tcell.KeyTab:
			if comp != nil {
				s = comp.Complete(1)
				m.clearLine(xpos, ypos)
			}
		case tcell.KeyBacktab:
			if comp != nil {
				s = comp.Complete(-1)
				m.clearLine(xpos, ypos)
			}
		case tcell.KeyRune:
			r := ev.Rune()
			if filter(r) || r == '_' {
				s += string(r)
				if comp != nil {
					comp.UpdateInput(s)
				}
			}
		}
		m.drawStr(xpos, ypos, s, tcell.StyleDefault)
		m.screen.Show()
	}
}

func (m *Menu) clearLine(xpos, ypos int) {
	w, _ := m.size()
	if w-xpos-1 > 0 {
		m.drawStr(xpos, ypos, strings.Repeat(" ", w-xpos-1), tcell.StyleDefault)
	}
}
