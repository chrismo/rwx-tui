package ui

import (
	"strings"
	"testing"
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
