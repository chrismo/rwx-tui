package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/chrismo/crux/internal/graph"
	"github.com/chrismo/crux/internal/rwx"
)

func inflightRun() rwx.Run {
	return rwx.Run{
		RunID:     "r1",
		Completed: false,
		Tasks: []rwx.Task{
			{Key: "a", Status: rwx.TaskStatus{Execution: "running"}},
			{Key: "b", Status: rwx.TaskStatus{Execution: "waiting"}},
		},
	}
}

func TestLivePollerStartsOnlyForInFlight(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	if _, cmd := a.Update(runOpenedMsg{run: inflightRun()}); cmd == nil {
		t.Error("in-flight run should start polling")
	}
	b := NewApp(nil, AppConfig{})
	if _, cmd := b.Update(runOpenedMsg{run: loadRun(t, "run_succeeded.json")}); cmd != nil {
		t.Error("completed run should not poll")
	}
}

func TestPollMsgRefreshesInFlight(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	m, _ := a.Update(runOpenedMsg{run: inflightRun()})
	a = m.(App)
	if _, cmd := a.Update(pollMsg{}); cmd == nil {
		t.Error("pollMsg should refresh while in-flight")
	}
}

func TestRefreshPreservesSelection(t *testing.T) {
	a := openGraph(t, "run_succeeded.json")
	a.selectedNode = "test"
	m, _ := a.Update(runRefreshedMsg{run: loadRun(t, "run_succeeded.json")})
	a = m.(App)
	if a.selectedNode != "test" {
		t.Errorf("selection = %q, want test (preserved across refresh)", a.selectedNode)
	}
}

func TestPollIntervalBackoff(t *testing.T) {
	mk := func(n int, exec string) rwx.Run {
		r := rwx.Run{}
		for i := 0; i < n; i++ {
			r.Tasks = append(r.Tasks, rwx.Task{Status: rwx.TaskStatus{Execution: exec}})
		}
		return r
	}
	if got := pollInterval(mk(5, "running")); got != 2*time.Second {
		t.Errorf("many active = %v, want 2s", got)
	}
	if got := pollInterval(mk(1, "running")); got != 4*time.Second {
		t.Errorf("few active = %v, want 4s", got)
	}
	if got := pollInterval(mk(2, "finished")); got != 6*time.Second {
		t.Errorf("none active = %v, want 6s", got)
	}
}

// The footer keybar is generated from the keymap, so labels can't drift from
// behavior. It is mode-aware and there is no separate ? overlay.
func TestFooterKeybarByMode(t *testing.T) {
	a := NewApp(nil, AppConfig{})

	a.mode = modeList
	listFooter := a.footerView()
	for _, want := range []string{"open", "all", "mine", "quit"} {
		if !strings.Contains(listFooter, want) {
			t.Errorf("list footer missing %q:\n%s", want, listFooter)
		}
	}
	// List-only actions must not leak graph bindings.
	if strings.Contains(listFooter, "pin") {
		t.Errorf("list footer should not show graph bindings:\n%s", listFooter)
	}

	a.mode = modeGraph
	graphFooter := a.footerView()
	for _, want := range []string{"back", "pin", "filter", "list"} {
		if !strings.Contains(graphFooter, want) {
			t.Errorf("graph footer missing %q:\n%s", want, graphFooter)
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
	if !strings.Contains(out, "crux") {
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

func TestGraphPinToggle(t *testing.T) {
	a := openGraph(t, "run_failed.json")
	if a.selectedNode != "code" {
		t.Fatalf("initial selection = %q, want code", a.selectedNode)
	}

	press := func(s string) {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
		a = m.(App)
	}

	press("p") // pin code's cone
	if len(a.pins) != 1 || a.pins[0].key != "code" {
		t.Fatalf("pins = %v, want [code]", a.pins)
	}
	fs := a.focusSet()
	if !fs["code"] || !fs["deps"] {
		t.Errorf("focus cone should include code's cone: %v", fs)
	}
	if fs["go"] {
		t.Errorf("focus cone should exclude go (unrelated): %v", fs)
	}

	press("p") // toggle the same node off (unpin)
	if len(a.pins) != 0 {
		t.Errorf("pins not cleared on second p: %v", a.pins)
	}
	if a.focusSet() != nil {
		t.Errorf("focusSet should be nil with no pins")
	}
}

// A second pin narrows the view to the intersection of cones: pinning a node
// with several parents, then one of those parents, drops the sibling parents.
func TestPinsIntersect(t *testing.T) {
	run := loadRun(t, "sample_dag_failed.json")
	g := graph.Build(run)
	a := &App{graph: g, layout: graph.Layout(g), filterInput: textinput.New()}

	// integration has three parents: build-api, build-worker, build-web.
	a.togglePin("integration")
	fs := a.focusSet()
	for _, p := range []string{"build-api", "build-worker", "build-web"} {
		if !fs[p] {
			t.Fatalf("cone(integration) should include parent %s: %v", p, fs)
		}
	}

	// build-api is already visible, so pinning it refines (intersects) — the
	// sibling parents drop out.
	a.togglePin("build-api")
	if len(a.pins) != 2 || a.pins[1].refine != true {
		t.Fatalf("second pin of a visible node should refine: %+v", a.pins)
	}
	fs = a.focusSet()
	if !fs["integration"] || !fs["build-api"] {
		t.Errorf("pinned anchors must stay visible: %v", fs)
	}
	if fs["build-worker"] || fs["build-web"] {
		t.Errorf("intersection should hide sibling parents build-worker/build-web: %v", fs)
	}
}

// Pinning a node from outside the current pin view (found via the global
// filter) adds it via union — both cones stay visible.
func TestPinsUnionWhenAddedFromElsewhere(t *testing.T) {
	run := loadRun(t, "sample_dag_failed.json")
	g := graph.Build(run)
	a := &App{graph: g, layout: graph.Layout(g), filterInput: textinput.New()}

	a.togglePin("go-deps") // Go branch
	if a.focusSet()["node-deps"] {
		t.Fatal("precondition: node-deps should be outside go-deps' cone")
	}
	a.togglePin("node-deps") // outside the current view → union (add)
	if len(a.pins) != 2 || a.pins[1].refine != false {
		t.Fatalf("pinning a node outside the view should add (union): %+v", a.pins)
	}
	fs := a.focusSet()
	if !fs["build-api"] || !fs["build-web"] {
		t.Errorf("union should keep both cones (build-api + build-web): %v", fs)
	}
}

// The filter is a global finder: while active it searches the whole graph,
// overriding the pin view, and pinning clears it (snapping back to the pins).
func TestFilterFindsOutsidePinsThenPinClearsIt(t *testing.T) {
	a := openGraph(t, "sample_dag_failed.json")
	a.togglePin("go-deps") // pin the Go branch

	// node-deps is outside go-deps' cone; the global filter still finds it.
	a.filterInput.SetValue("node-deps")
	vis := computeVisible(a.graph, a.currentOverlay())
	if !vis["node-deps"] {
		t.Fatalf("active filter should find node-deps globally: %v", vis)
	}

	// Pin it: the filter clears and the view returns to the pins (now unioned).
	a.selectedNode = "node-deps"
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	a = m.(App)
	if a.filterInput.Value() != "" {
		t.Errorf("pinning should clear the filter, got %q", a.filterInput.Value())
	}
	fs := a.focusSet()
	if !fs["go-deps"] || !fs["node-deps"] {
		t.Errorf("both pins should be visible after adding from the finder: %v", fs)
	}
}

// Pins accumulate, and esc pops only the most recent one (not all of them).
func TestPinsAccumulateAndEscPopsLast(t *testing.T) {
	a := openGraph(t, "run_failed.json")
	press := func(s string) {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
		a = m.(App)
	}
	esc := func() {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
		a = m.(App)
	}

	press("p")
	first := a.selectedNode
	a.moveSelection(1, 0) // move to a different visible node
	second := a.selectedNode
	if second == first {
		t.Fatalf("expected selection to move off %q for a distinct second pin", first)
	}
	press("p")
	if len(a.pins) != 2 {
		t.Fatalf("expected 2 accumulated pins, got %v", a.pins)
	}

	esc() // pops only the last pin
	if len(a.pins) != 1 || a.pins[0].key != first {
		t.Fatalf("esc should pop the last pin, leaving [%s], got %v", first, a.pins)
	}
	esc() // pops the remaining pin
	if len(a.pins) != 0 {
		t.Fatalf("esc should pop the remaining pin, got %v", a.pins)
	}
	esc() // nothing left: must NOT leave the grid
	if a.mode != modeGraph {
		t.Errorf("esc with nothing to dismiss should stay in the grid, mode=%v", a.mode)
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
