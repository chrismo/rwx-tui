package ui

import "testing"

// Canvas tests run under the Ascii color profile (TestMain in snapshot_test.go),
// so styling collapses to plain text and String output is the raw grid.

func TestCanvasTextAndTrim(t *testing.T) {
	c := newCanvas(10, 2)
	c.text(1, 0, "hi", cellStyle{})
	c.text(0, 1, "abc", cellStyle{})
	got := c.String()
	want := " hi\nabc"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestCanvasLines(t *testing.T) {
	c := newCanvas(5, 3)
	c.hline(0, 4, 0, '-', cellStyle{})
	c.vline(2, 0, 2, '|', cellStyle{})
	if got := c.runeAt(4, 0); got != '-' {
		t.Errorf("hline end = %q, want '-'", got)
	}
	if got := c.runeAt(2, 2); got != '|' {
		t.Errorf("vline end = %q, want '|'", got)
	}
	// hline and vline cross at (2,0); vline was drawn second so it wins there.
	if got := c.runeAt(2, 0); got != '|' {
		t.Errorf("cross cell = %q, want '|'", got)
	}
}

func TestCanvasDoubleWidthRune(t *testing.T) {
	// ⚡ (U+26A1) is a double-width rune: it must consume two columns so the
	// following text stays aligned.
	c := newCanvas(10, 1)
	end := c.set(0, 0, '⚡', cellStyle{})
	if end != 2 {
		t.Fatalf("set(⚡) advanced %d cols, want 2", end)
	}
	c.text(2, 0, "x", cellStyle{})
	// Continuation cell at (1,0) reads as blank and is skipped in output.
	if got := c.runeAt(1, 0); got != ' ' {
		t.Errorf("continuation cell = %q, want ' '", got)
	}
	if got := c.String(); got != "⚡x" {
		t.Errorf("String() = %q, want %q", got, "⚡x")
	}
}

func TestCanvasOutOfBoundsNoPanic(t *testing.T) {
	c := newCanvas(3, 3)
	c.set(-1, 0, 'a', cellStyle{})
	c.set(0, -1, 'a', cellStyle{})
	c.set(5, 5, 'a', cellStyle{})
	c.text(2, 0, "overflow", cellStyle{}) // runs past the edge
	if got := c.runeAt(2, 0); got != 'o' {
		t.Errorf("in-bounds head = %q, want 'o'", got)
	}
}
