package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// canvas is a fixed-size grid of styled cells used to composite the graph:
// node boxes are placed at computed positions and connectors are routed
// between them. It replaces the old lipgloss.JoinHorizontal approach, which
// could not draw edges.
//
// Cells carry an optional foreground color plus bold/reverse so the live TUI
// keeps its state coloring; under the Ascii color profile (the snapshot/--print
// surface) the styling collapses to plain text, keeping output deterministic.
type canvas struct {
	w, h  int
	cells []cell
}

type cell struct {
	r       rune
	fg      lipgloss.TerminalColor
	bold    bool
	reverse bool
}

// cellStyle is the styling applied when writing to the canvas.
type cellStyle struct {
	fg      lipgloss.TerminalColor
	bold    bool
	reverse bool
}

func newCanvas(w, h int) *canvas {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	cells := make([]cell, w*h)
	for i := range cells {
		cells[i].r = ' '
	}
	return &canvas{w: w, h: h, cells: cells}
}

func (c *canvas) inBounds(x, y int) bool {
	return x >= 0 && x < c.w && y >= 0 && y < c.h
}

// set writes a rune at (x,y). A double-width rune also claims the cell to its
// right as a continuation cell (rune 0), which String skips. Returns the number
// of columns consumed (1 or 2).
func (c *canvas) set(x, y int, r rune, st cellStyle) int {
	width := runewidth.RuneWidth(r)
	if width < 1 {
		width = 1
	}
	if c.inBounds(x, y) {
		c.cells[y*c.w+x] = cell{r: r, fg: st.fg, bold: st.bold, reverse: st.reverse}
	}
	if width == 2 && c.inBounds(x+1, y) {
		c.cells[y*c.w+x+1] = cell{r: 0} // continuation
	}
	return width
}

// runeAt returns the rune at (x,y), or ' ' out of bounds / on continuation.
func (c *canvas) runeAt(x, y int) rune {
	if !c.inBounds(x, y) {
		return ' '
	}
	r := c.cells[y*c.w+x].r
	if r == 0 {
		return ' '
	}
	return r
}

// text writes a string left-to-right starting at (x,y), advancing by each
// rune's display width. Returns the ending x (one past the last column).
func (c *canvas) text(x, y int, s string, st cellStyle) int {
	for _, r := range s {
		x += c.set(x, y, r, st)
	}
	return x
}

// hline draws a horizontal run of r from x1 to x2 (inclusive) at row y.
func (c *canvas) hline(x1, x2, y int, r rune, st cellStyle) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		c.set(x, y, r, st)
	}
}

// vline draws a vertical run of r from y1 to y2 (inclusive) at column x.
func (c *canvas) vline(x, y1, y2 int, r rune, st cellStyle) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	for y := y1; y <= y2; y++ {
		c.set(x, y, r, st)
	}
}

// String renders the canvas to a styled multi-line string. Trailing blank
// columns on each row are trimmed. Consecutive cells sharing a style are
// grouped into one styled span to keep escape output compact.
func (c *canvas) String() string {
	var b strings.Builder
	for y := 0; y < c.h; y++ {
		last := -1
		for x := 0; x < c.w; x++ {
			cl := c.cells[y*c.w+x]
			if cl.r != 0 && cl.r != ' ' {
				last = x
			}
		}
		x := 0
		for x <= last {
			cl := c.cells[y*c.w+x]
			if cl.r == 0 { // continuation cell
				x++
				continue
			}
			// Gather a run of same-styled cells.
			var run strings.Builder
			fg, bold, rev := cl.fg, cl.bold, cl.reverse
			for x <= last {
				nc := c.cells[y*c.w+x]
				if nc.r == 0 {
					x++
					continue
				}
				if nc.fg != fg || nc.bold != bold || nc.reverse != rev {
					break
				}
				run.WriteRune(nc.r)
				x++
			}
			b.WriteString(styleRun(run.String(), fg, bold, rev))
		}
		if y < c.h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// styleRun applies a style to a text run, or returns it raw when unstyled (the
// common case for blank/plain cells), avoiding needless lipgloss wrapping.
func styleRun(s string, fg lipgloss.TerminalColor, bold, reverse bool) string {
	if fg == nil && !bold && !reverse {
		return s
	}
	st := lipgloss.NewStyle()
	if fg != nil {
		st = st.Foreground(fg)
	}
	if bold {
		st = st.Bold(true)
	}
	if reverse {
		st = st.Reverse(true)
	}
	return st.Render(s)
}
