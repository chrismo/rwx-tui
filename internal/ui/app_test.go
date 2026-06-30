package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chrismo/rwx-tui/internal/rwx"
)

// The footer keybar is generated from the keymap, so labels can't drift from
// behavior. Verify the mode-aware short help and the ? full overlay.
func TestFooterKeybarByMode(t *testing.T) {
	a := NewApp(nil, AppConfig{})

	a.mode = modeList
	listFooter := a.footerView()
	for _, want := range []string{"open", "all", "mine", "quit"} {
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

func TestMouseClickSelectsRow(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = m.(App)

	// Rows start after the header + blank line, so row index 2 is at Y=4.
	m, _ = a.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: 4})
	a = m.(App)
	if a.selected != 2 {
		t.Errorf("click at Y=4 selected %d, want 2", a.selected)
	}

	// A wheel event must not panic and leaves selection unchanged.
	m, _ = a.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	a = m.(App)
	if a.selected != 2 {
		t.Errorf("wheel changed selection to %d, want 2", a.selected)
	}
}

func TestGraphSelectionNav(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	m, _ := a.Update(runOpenedMsg{run: loadRun(t, "run_succeeded.json")})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = m.(App)

	send := func(s string) {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
		a = m.(App)
	}

	// Layout layer 0 (sorted) is [code, go, ~base-image]; layer 1 is
	// [deps, ~base-config].
	if a.selectedNode != "code" {
		t.Fatalf("initial selection = %q, want code", a.selectedNode)
	}
	send("j") // down a layer
	if a.selectedNode != "deps" {
		t.Errorf("after down = %q, want deps", a.selectedNode)
	}
	send("k") // up a layer
	if a.selectedNode != "code" {
		t.Errorf("after up = %q, want code", a.selectedNode)
	}
	send("l") // right within layer
	if a.selectedNode != "go" {
		t.Errorf("after right = %q, want go", a.selectedNode)
	}
}

func TestListPaginationAppends(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}})
	runs := loadRunList(t)
	m, _ := a.Update(runsLoadedMsg{runs: runs, cursor: "CURSOR"})
	a = m.(App)
	if a.nextCursor != "CURSOR" || len(a.runs) != len(runs) {
		t.Fatalf("initial page: cursor=%q n=%d", a.nextCursor, len(a.runs))
	}

	// Pressing down at the last row with a cursor requests the next page.
	a.selected = len(a.runs) - 1
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	a = m.(App)
	if !a.loadingMore {
		t.Error("down at bottom should set loadingMore")
	}

	// The appended page grows the list and clears the cursor.
	m, _ = a.Update(runsLoadedMsg{runs: runs[:2], cursor: "", append: true})
	a = m.(App)
	if len(a.runs) != len(runs)+2 {
		t.Errorf("after append n=%d, want %d", len(a.runs), len(runs)+2)
	}
	if a.nextCursor != "" || a.loadingMore {
		t.Errorf("append should clear cursor (%q) and loadingMore (%v)", a.nextCursor, a.loadingMore)
	}
}

func TestListFilterToggleMine(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}})
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)

	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	a = m.(App)
	if !a.cfg.Filter.Mine {
		t.Error("m should set the Mine filter")
	}
	if a.mode != modeLoading {
		t.Error("filter toggle should trigger a reload (modeLoading)")
	}
}

func openGraph(t *testing.T, fixture string) App {
	t.Helper()
	a := NewApp(nil, AppConfig{})
	m, _ := a.Update(runOpenedMsg{run: loadRun(t, fixture)})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m.(App)
}

func TestGraphFocusToggle(t *testing.T) {
	a := openGraph(t, "run_failed.json")
	if a.selectedNode != "code" {
		t.Fatalf("initial selection = %q, want code", a.selectedNode)
	}

	press := func(s string) {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
		a = m.(App)
	}

	press("f") // isolate code's cone
	if a.focus == nil {
		t.Fatal("focus not set after f")
	}
	if !a.focus["code"] || !a.focus["deps"] {
		t.Errorf("focus should include code's cone: %v", a.focus)
	}
	if a.focus["go"] {
		t.Errorf("focus should exclude go (unrelated): %v", a.focus)
	}

	press("f") // toggle off
	if a.focus != nil {
		t.Errorf("focus not cleared on second f: %v", a.focus)
	}
}

func TestDetailPaneAndLogs(t *testing.T) {
	a := openGraph(t, "run_failed.json") // selection starts on "code"

	// enter opens the detail pane for the selected node.
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if !a.detailOpen {
		t.Fatal("enter did not open the detail pane")
	}
	if !strings.Contains(a.bodyContent(), "status:") {
		t.Errorf("detail body missing status:\n%s", a.bodyContent())
	}

	// A logs result replaces the body with the log content.
	m, _ = a.Update(logsLoadedMsg{content: "line one\nline two"})
	a = m.(App)
	if !strings.Contains(a.bodyContent(), "line two") {
		t.Errorf("logs not shown in body:\n%s", a.bodyContent())
	}

	// esc closes the pane and clears logs.
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.detailOpen || a.logsContent != "" {
		t.Errorf("esc should close detail and clear logs (open=%v logs=%q)", a.detailOpen, a.logsContent)
	}
}

func TestGraphFilterTyping(t *testing.T) {
	a := openGraph(t, "run_succeeded.json")

	press := func(s string) {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
		a = m.(App)
	}

	press("/")
	if !a.filtering {
		t.Fatal("/ did not activate the filter input")
	}
	press("v")
	press("e")
	press("t")
	if got := a.filterInput.Value(); got != "vet" {
		t.Errorf("filter value = %q, want vet", got)
	}

	// Enter keeps the filter and closes the input.
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if a.filtering {
		t.Error("enter should close the filter input")
	}
	if a.filterInput.Value() != "vet" {
		t.Error("enter should keep the filter value")
	}
}
