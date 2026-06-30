package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The footer keybar is generated from the keymap, so labels can't drift from
// behavior. Verify the mode-aware short help and the ? full overlay.
func TestFooterKeybarByMode(t *testing.T) {
	a := NewApp(nil, AppConfig{})

	a.mode = modeList
	listFooter := a.footerView()
	for _, want := range []string{"open", "filter", "quit"} {
		if !strings.Contains(listFooter, want) {
			t.Errorf("list footer missing %q:\n%s", want, listFooter)
		}
	}

	a.mode = modeGraph
	graphFooter := a.footerView()
	if !strings.Contains(graphFooter, "back") {
		t.Errorf("graph footer missing %q:\n%s", "back", graphFooter)
	}

	a.showHelp = true
	full := a.footerView()
	for _, want := range []string{"refresh", "top", "bottom"} {
		if !strings.Contains(full, want) {
			t.Errorf("? overlay missing %q:\n%s", want, full)
		}
	}
}

func TestResizeSizesViewportAndRenders(t *testing.T) {
	a := NewApp(nil, AppConfig{})

	// Simulate a loaded run list, then a window resize.
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a = m.(App)

	if a.width != 100 || a.height != 30 {
		t.Errorf("size = %dx%d, want 100x30", a.width, a.height)
	}
	if a.viewport.Width != 100 {
		t.Errorf("viewport width = %d, want 100", a.viewport.Width)
	}
	// Viewport height is window minus the footer keybar.
	if a.viewport.Height >= 30 || a.viewport.Height < 1 {
		t.Errorf("viewport height = %d, want < 30 and >= 1", a.viewport.Height)
	}
	out := a.View()
	if !strings.Contains(out, "rwxtui") {
		t.Errorf("rendered view missing home header:\n%s", out)
	}
}
