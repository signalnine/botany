package ui

import (
	"sort"
	"strings"
)

// Completer is a loop-based tab completer for login names, ported from the
// Python LoginCompleter. completionID is -1 for the raw user input and
// 0..len-1 for cycling through matches.
type Completer struct {
	s              string
	logins         []string
	completions    []string
	completionID   int
	completionBase string
}

// NewCompleter builds a completer over a set of login names.
func NewCompleter(logins []string) *Completer {
	sorted := make([]string, len(logins))
	copy(sorted, logins)
	sort.Strings(sorted)
	return &Completer{logins: sorted, completionID: -1}
}

// UpdateInput records freshly typed input and resets the completion cursor.
func (c *Completer) UpdateInput(s string) {
	c.s = s
	c.completionBase = s
	c.completionID = -1
}

// Complete returns the next (direction>0) or previous (direction<0) completion,
// looping back to the raw input at the ends.
func (c *Completer) Complete(direction int) string {
	if c.completionID == -1 {
		c.completionBase = c.s
		c.completions = c.completions[:0]
		for _, x := range c.logins {
			if strings.HasPrefix(x, c.s) && x != c.s {
				c.completions = append(c.completions, x)
			}
		}
	}
	c.completionID += direction
	if c.completionID == -2 {
		c.completionID = len(c.completions) - 1
	}
	if c.completionID == -1 || c.completionID == len(c.completions) {
		c.completionID = -1
		return c.completionBase
	}
	return c.completions[c.completionID]
}
