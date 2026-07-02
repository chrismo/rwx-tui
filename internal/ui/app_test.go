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
	for _, want := range []string{"move", "filter", "scope", "open", "quit"} {
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

	send := func(kt tea.KeyType) {
		m, _ := a.Update(tea.KeyMsg{Type: kt})
		a = m.(App)
	}

	// Graph nav is arrow-key only (letters type into the filter). Layout layer 0
	// (sorted) is [code, go, ~base-image]; layer 1 is [deps, ~base-config].
	if a.selectedNode != "code" {
		t.Fatalf("initial selection = %q, want code", a.selectedNode)
	}
	send(tea.KeyDown)
	if a.selectedNode != "deps" {
		t.Errorf("after down = %q, want deps", a.selectedNode)
	}
	send(tea.KeyUp)
	if a.selectedNode != "code" {
		t.Errorf("after up = %q, want code", a.selectedNode)
	}
	send(tea.KeyRight)
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
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyDown})
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

// Tab cycles the server-side fetch scope (all → mine → branch) with a re-fetch.
func TestListTabCyclesScope(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}})
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)
	if a.listScope() != "all" {
		t.Fatalf("initial scope = %q, want all", a.listScope())
	}

	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyTab})
	a = m.(App)
	if !a.cfg.Filter.Mine {
		t.Errorf("Tab should cycle to mine: %+v", a.cfg.Filter)
	}
	if a.mode != modeLoading {
		t.Error("scope change should trigger a reload (modeLoading)")
	}
}

// Typing narrows the run list client-side; esc clears it.
func TestListTypeToFilter(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}})
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)
	total := len(a.runs)

	pressRunes(&a, "prime") // matches only the prime-cache run's definition
	if got := len(a.visibleRuns()); got == 0 || got >= total {
		t.Errorf("filter 'prime' should narrow rows: %d of %d", got, total)
	}
	if a.listFilter != "prime" {
		t.Errorf("listFilter = %q, want prime", a.listFilter)
	}

	sendType(&a, tea.KeyEsc) // clears the filter
	if a.listFilter != "" {
		t.Errorf("esc should clear the list filter, got %q", a.listFilter)
	}
	if len(a.visibleRuns()) != total {
		t.Errorf("cleared filter should show all %d rows", total)
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

	pin := func() {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeySpace})
		a = m.(App)
	}

	pin() // pin code's cone (space)
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

	pin() // toggle the same node off (unpin)
	if len(a.pins) != 0 {
		t.Errorf("pins not cleared on second space: %v", a.pins)
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
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeySpace})
	a = m.(App)
	if a.filterInput.Value() != "" {
		t.Errorf("pinning should clear the filter, got %q", a.filterInput.Value())
	}
	fs := a.focusSet()
	if !fs["go-deps"] || !fs["node-deps"] {
		t.Errorf("both pins should be visible after adding from the finder: %v", fs)
	}
}

// Pins persist across trips out to the run list and into another run — so an
// elaborate pin set survives navigating between runs.
func TestPinsPersistAcrossRuns(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	a.hasList = true
	open := func(fixture string) {
		m, _ := a.Update(runOpenedMsg{run: loadRun(t, fixture)})
		a = m.(App)
		m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		a = m.(App)
	}

	open("run_failed.json")
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeySpace}) // pin the selected node
	a = m.(App)
	if len(a.pins) != 1 {
		t.Fatalf("expected 1 pin, got %v", a.pins)
	}
	pinned := a.pins[0].key

	// backspace returns to the run list; pins survive.
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	a = m.(App)
	if a.mode != modeList {
		t.Fatalf("backspace should return to the list, mode=%v", a.mode)
	}
	if len(a.pins) != 1 || a.pins[0].key != pinned {
		t.Fatalf("pins should persist at the list, got %v", a.pins)
	}

	// Opening another run keeps the pins.
	open("run_succeeded.json")
	if len(a.pins) != 1 || a.pins[0].key != pinned {
		t.Fatalf("pins should persist into the next run, got %v", a.pins)
	}
}

// --pin seeds substring-matched pins once, on the first run opened.
func TestSeedPinsFromConfig(t *testing.T) {
	a := NewApp(nil, AppConfig{Pins: []string{"deps"}})
	m, _ := a.Update(runOpenedMsg{run: loadRun(t, "sample_dag_failed.json")})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = m.(App)

	// "deps" matches go-deps, node-deps, py-deps — all pinned.
	got := map[string]bool{}
	for _, p := range a.pins {
		got[p.key] = true
	}
	for _, want := range []string{"go-deps", "node-deps", "py-deps"} {
		if !got[want] {
			t.Errorf("--pin deps should have pinned %s; pins=%v", want, a.pins)
		}
	}
	if !a.pinsSeeded {
		t.Error("pinsSeeded should be set after the first run open")
	}
}

func pressRunes(a *App, s string) {
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	*a = m.(App)
}

func sendType(a *App, kt tea.KeyType) {
	m, _ := a.Update(tea.KeyMsg{Type: kt})
	*a = m.(App)
}

// esc undoes the last focus action via the history stack: pinning from a filter,
// then esc, pops the pre-pin snapshot — restoring the filter and dropping the pin.
func TestEscUndoesPinAndRestoresFilter(t *testing.T) {
	a := openGraph(t, "sample_dag_failed.json")

	pressRunes(&a, "build")
	if a.filterInput.Value() != "build" {
		t.Fatalf("filter = %q, want build", a.filterInput.Value())
	}

	sendType(&a, tea.KeySpace) // pin; filter clears to the pin view
	if len(a.pins) != 1 || a.filterInput.Value() != "" {
		t.Fatalf("after pin: pins=%v filter=%q", a.pins, a.filterInput.Value())
	}

	sendType(&a, tea.KeyEsc) // undo the pin
	if len(a.pins) != 0 {
		t.Fatalf("esc should undo the pin, pins=%v", a.pins)
	}
	if a.filterInput.Value() != "build" {
		t.Errorf("esc should restore the filter, got %q", a.filterInput.Value())
	}
}

// space is a pure toggle: unpinning one of several pins just removes it and
// never resurrects a filter (that would override the surviving pins). Only esc
// (history) brings filters back.
func TestUnpinWithOtherPinsKeepsView(t *testing.T) {
	a := openGraph(t, "sample_dag_failed.json")

	pressRunes(&a, "web") // filter to *-web nodes
	a.selectedNode = "lint-web"
	sendType(&a, tea.KeySpace) // pin lint-web (filter clears)
	a.selectedNode = "node-deps"
	sendType(&a, tea.KeySpace) // pin node-deps
	if len(a.pins) != 2 {
		t.Fatalf("expected 2 pins, got %v", a.pins)
	}

	// Unpin lint-web with node-deps still pinned: no filter must come back.
	a.selectedNode = "lint-web"
	sendType(&a, tea.KeySpace)
	if len(a.pins) != 1 || a.pins[0].key != "node-deps" {
		t.Fatalf("expected only node-deps pinned, got %v", a.pins)
	}
	if a.filterInput.Value() != "" {
		t.Errorf("unpin must not resurrect a filter, got %q", a.filterInput.Value())
	}
	if vis := computeVisible(a.graph, a.currentOverlay()); !vis["node-deps"] {
		t.Errorf("node-deps should still be visible in its pin view: %v", vis)
	}
}

// esc with a live filter cancels the finder (not a history step); a second esc
// then walks the focus history.
func TestEscCancelsLiveFilterBeforeHistory(t *testing.T) {
	a := openGraph(t, "sample_dag_failed.json")

	a.selectedNode = "go-deps"
	sendType(&a, tea.KeySpace) // pin go-deps (history has the pre-pin snapshot)
	pressRunes(&a, "web")      // start a new finder search
	if a.filterInput.Value() != "web" {
		t.Fatalf("filter = %q, want web", a.filterInput.Value())
	}

	sendType(&a, tea.KeyEsc) // cancels the live filter, leaves the pin
	if a.filterInput.Value() != "" {
		t.Errorf("first esc should clear the live filter, got %q", a.filterInput.Value())
	}
	if len(a.pins) != 1 {
		t.Fatalf("first esc should not touch pins, got %v", a.pins)
	}

	sendType(&a, tea.KeyEsc) // now pops history: undoes the pin
	if len(a.pins) != 0 {
		t.Errorf("second esc should undo the pin, got %v", a.pins)
	}
}

// Pins accumulate, and esc pops only the most recent one (not all of them).
func TestPinsAccumulateAndEscPopsLast(t *testing.T) {
	a := openGraph(t, "run_failed.json")
	pin := func() {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeySpace})
		a = m.(App)
	}
	esc := func() {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
		a = m.(App)
	}

	pin()
	first := a.selectedNode
	a.moveSelection(1, 0) // move to a different visible node
	second := a.selectedNode
	if second == first {
		t.Fatalf("expected selection to move off %q for a distinct second pin", first)
	}
	pin()
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

	// esc peels one layer: from logs back to the node detail (pane stays open).
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if !a.detailOpen || a.logsContent != "" {
		t.Errorf("esc from logs should return to detail (open=%v logs=%q)", a.detailOpen, a.logsContent)
	}
	if !strings.Contains(a.bodyContent(), "status:") {
		t.Errorf("after esc from logs, detail should show:\n%s", a.bodyContent())
	}

	// A second esc closes the detail pane back to the graph.
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.detailOpen {
		t.Errorf("second esc should close the detail pane")
	}
}

// The detail/log pane must scroll by keyboard, not just the mouse wheel: with a
// long log open, Down moves the viewport.
func TestLogPaneKeyboardScroll(t *testing.T) {
	a := openGraph(t, "run_failed.json")
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open detail
	a = m.(App)
	m, _ = a.Update(logsLoadedMsg{content: strings.Repeat("log line\n", 200)})
	a = m.(App)

	before := a.viewport.YOffset
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyDown})
	a = m.(App)
	if a.viewport.YOffset <= before {
		t.Errorf("Down should scroll the log pane, YOffset %d -> %d", before, a.viewport.YOffset)
	}
}

// Graph mode is type-to-filter: printable keys build the filter live (no /),
// backspace deletes, and esc clears it.
func TestGraphFilterTyping(t *testing.T) {
	a := openGraph(t, "run_succeeded.json")

	press := func(s string) {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
		a = m.(App)
	}

	press("v")
	press("e")
	press("t")
	if got := a.filterInput.Value(); got != "vet" {
		t.Errorf("filter value = %q, want vet", got)
	}

	// backspace deletes the last character.
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	a = m.(App)
	if got := a.filterInput.Value(); got != "ve" {
		t.Errorf("backspace should delete last char, got %q", got)
	}

	// esc clears the filter entirely.
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.filterInput.Value() != "" {
		t.Errorf("esc should clear the filter, got %q", a.filterInput.Value())
	}
}
