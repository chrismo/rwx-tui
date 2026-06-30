package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/chrismo/rwx-tui/internal/graph"
	"github.com/chrismo/rwx-tui/internal/rwx"
)

// ---- Graph view (a single run) -------------------------------------------

// Model renders a single run's graph. It is reused by the App router and by the
// headless --print path.
type Model struct {
	run    rwx.Run
	graph  *graph.Graph
	layout *graph.LayoutData
}

// NewModel builds the graph view from a fetched run.
func NewModel(run rwx.Run) Model {
	g := graph.Build(run)
	return Model{run: run, graph: g, layout: graph.Layout(g)}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	return Screen(m.run, m.graph, m.layout, graphOverlay{})
}

// graphOverlay carries the interactive overlays (cursor + focus/filter) into
// Screen. The zero value (used by --print) shows no overlays.
type graphOverlay struct {
	Selected string
	Focus    map[string]bool
	Filter   string
}

// Screen renders the full graph view (header, graph, legend) as a string. Pure,
// so it backs both View() and the headless --print path; ov is empty for
// --print, which has no cursor or filter.
func Screen(run rwx.Run, g *graph.Graph, l *graph.LayoutData, ov graphOverlay) string {
	var b strings.Builder

	status := run.ResultStatus
	if !run.Completed {
		status = "in progress"
	}
	header := fmt.Sprintf("RWX run %s · %s · %s", short(run.RunID), run.DefinitionPath, status)
	b.WriteString(theme.Header.Render(header))
	b.WriteString("\n")

	cp := graph.CriticalPath(g)
	if line := CriticalPathLine(cp); line != "" {
		b.WriteString(theme.Special.Render(line))
		b.WriteString("\n")
	}

	fi := graph.AnalyzeFailures(g)
	if line := FailureLine(fi); line != "" {
		b.WriteString(theme.Failure.Render(line))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(RenderGraph(g, l, RenderOpts{
		Crit: cp, Failure: fi,
		Selected: ov.Selected, Focus: ov.Focus, Filter: ov.Filter,
	}))
	b.WriteString("\n")
	b.WriteString(theme.Faint.Render(Legend()))
	b.WriteString("\n")
	return b.String()
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// HomeView renders the run-list landing (header, list, footer). Pure; backs both
// the App's list mode and the headless --print path.
func HomeView(runs []rwx.RunSummary, selected int, now time.Time) string {
	var b strings.Builder
	header := "rwxtui"
	if len(runs) > 0 && runs[0].RepositoryName != "" {
		header += " · " + runs[0].RepositoryName
	}
	b.WriteString(theme.Header.Render(header))
	b.WriteString("\n\n")
	b.WriteString(RenderRunList(runs, selected, now))
	b.WriteString("\n")
	return b.String()
}

// ---- App router (list <-> graph) -----------------------------------------

type appMode int

const (
	modeLoading appMode = iota
	modeList
	modeGraph
)

// AppConfig configures the root App.
type AppConfig struct {
	Run    string         // open this run directly, skipping the list
	Filter rwx.ListFilter // filter for the run list
}

// App is the root Bubble Tea model. It starts on the run list (the home) and
// opens a run's graph on selection; with AppConfig.Run set it opens that run
// directly.
type App struct {
	client *rwx.Client
	cfg    AppConfig
	now    func() time.Time

	mode    appMode
	hasList bool // a list exists to return to via esc

	keys     keyMap
	help     help.Model
	showHelp bool
	spinner  spinner.Model

	width    int
	height   int
	viewport viewport.Model

	runs        []rwx.RunSummary
	selected    int
	nextCursor  string // pagination cursor for the next page ("" = no more)
	loadingMore bool

	run          rwx.Run
	graph        *graph.Graph
	layout       *graph.LayoutData
	selectedNode string          // key of the highlighted graph node
	focus        map[string]bool // f-isolated subgraph (nil = no focus)
	filtering    bool            // the / filter input is active
	filterInput  textinput.Model
	detailOpen   bool   // detail pane shown for the selected node
	logsContent  string // fetched logs ("" = show task detail instead)

	err error
}

// NewApp builds the root model. The viewport is seeded with a sane default size
// so the first frame renders before the initial WindowSizeMsg arrives.
func NewApp(client *rwx.Client, cfg AppConfig) App {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = theme.Running
	ti := textinput.New()
	ti.Prompt = "filter: "
	ti.CharLimit = 64
	return App{
		filterInput: ti,
		client:   client,
		cfg:      cfg,
		now:      time.Now,
		mode:     modeLoading,
		keys:     defaultKeyMap(),
		help:     help.New(),
		spinner:  sp,
		width:    80,
		height:   24,
		viewport: viewport.New(80, 23),
	}
}

// bodyContent is the scrollable body for the current mode (the same pure-render
// output that --print uses, fed into the viewport).
func (a App) bodyContent() string {
	switch a.mode {
	case modeList:
		return HomeView(a.runs, a.selected, a.now())
	case modeGraph:
		if a.detailOpen {
			if a.logsContent != "" {
				return a.logsContent
			}
			return RenderDetail(a.run.FindTask(a.selectedNode))
		}
		return Screen(a.run, a.graph, a.layout, graphOverlay{
			Selected: a.selectedNode,
			Focus:    a.focus,
			Filter:   a.filterInput.Value(),
		})
	default:
		return ""
	}
}

// refresh re-feeds the viewport from the current state.
func (a *App) refresh() {
	a.viewport.SetContent(a.bodyContent())
}

// firstNode returns the top-left node key of a layout, or "".
func firstNode(l *graph.LayoutData) string {
	if l == nil || len(l.Layers) == 0 || len(l.Layers[0]) == 0 {
		return ""
	}
	return l.Layers[0][0]
}

// moveSelection moves the graph cursor by dLayer (rows) and dOrder (columns
// within a layer), clamped to the layout.
func (a *App) moveSelection(dLayer, dOrder int) {
	if a.layout == nil || a.selectedNode == "" {
		return
	}
	pos, ok := a.layout.Pos[a.selectedNode]
	if !ok {
		return
	}
	if dLayer != 0 {
		target := pos.Layer + dLayer
		if target < 0 || target >= len(a.layout.Layers) {
			return
		}
		row := a.layout.Layers[target]
		idx := pos.Order
		if idx >= len(row) {
			idx = len(row) - 1
		}
		a.selectedNode = row[idx]
	}
	if dOrder != 0 {
		row := a.layout.Layers[pos.Layer]
		idx := pos.Order + dOrder
		if idx < 0 || idx >= len(row) {
			return
		}
		a.selectedNode = row[idx]
	}
}

// ensureSelectedVisible scrolls the viewport so the selected node's row is in
// view. Best-effort: locates the node's label line in the rendered body.
func (a *App) ensureSelectedVisible() {
	if a.selectedNode == "" {
		return
	}
	lines := strings.Split(a.bodyContent(), "\n")
	target := -1
	for i, ln := range lines {
		if strings.Contains(ln, " "+a.selectedNode+" ") {
			target = i
			break
		}
	}
	if target < 0 {
		return
	}
	top := a.viewport.YOffset
	bottom := top + a.viewport.Height - 1
	switch {
	case target < top:
		a.viewport.SetYOffset(target)
	case target > bottom:
		a.viewport.SetYOffset(target - a.viewport.Height + 1)
	}
}

// resize sizes the viewport to the window minus the footer keybar.
func (a *App) resize() {
	footerH := lipgloss.Height(a.footerView())
	h := a.height - footerH
	if h < 1 {
		h = 1
	}
	a.viewport.Width = a.width
	a.viewport.Height = h
}

// footerView renders the mode-aware keybar (or the full ? overlay).
func (a App) footerView() string {
	a.help.ShowAll = a.showHelp
	return a.help.View(modeHelp{keys: a.keys, mode: a.mode})
}

type runsLoadedMsg struct {
	runs   []rwx.RunSummary
	cursor string
	append bool
	err    error
}

type runOpenedMsg struct {
	run rwx.Run
	err error
}

func listRunsCmd(c *rwx.Client, f rwx.ListFilter, appendPage bool) tea.Cmd {
	return func() tea.Msg {
		rl, err := c.ListRuns(context.Background(), f)
		return runsLoadedMsg{runs: rl.Runs, cursor: rl.NextCursor, append: appendPage, err: err}
	}
}

type logsLoadedMsg struct {
	content string
	err     error
}

func fetchLogsCmd(c *rwx.Client, runID, taskKey string) tea.Cmd {
	return func() tea.Msg {
		s, err := c.Logs(context.Background(), runID, taskKey)
		return logsLoadedMsg{content: s, err: err}
	}
}

func openRunCmd(c *rwx.Client, id string) tea.Cmd {
	return func() tea.Msg {
		r, err := c.Results(context.Background(), id)
		return runOpenedMsg{run: r, err: err}
	}
}

// reloadList applies a new filter and reloads the run list from the first page.
func (a App) reloadList(f rwx.ListFilter) (tea.Model, tea.Cmd) {
	a.cfg.Filter = f
	a.nextCursor = ""
	a.err = nil
	a.mode = modeLoading
	return a, tea.Batch(listRunsCmd(a.client, f, false), a.spinner.Tick)
}

func (a App) Init() tea.Cmd {
	var fetch tea.Cmd
	if a.cfg.Run != "" {
		fetch = openRunCmd(a.client, a.cfg.Run)
	} else {
		fetch = listRunsCmd(a.client, a.cfg.Filter, false)
	}
	return tea.Batch(fetch, a.spinner.Tick)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		a.resize()
		a.refresh()
		return a, nil
	case spinner.TickMsg:
		if a.mode != modeLoading {
			return a, nil // stop animating once loaded
		}
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(m)
		return a, cmd
	case tea.MouseMsg:
		switch m.Button {
		case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
			var cmd tea.Cmd
			a.viewport, cmd = a.viewport.Update(m)
			return a, cmd
		case tea.MouseButtonLeft:
			if m.Action == tea.MouseActionPress && a.mode == modeList {
				// HomeView is: header line, blank line, then one row per run.
				idx := m.Y + a.viewport.YOffset - 2
				if idx >= 0 && idx < len(a.runs) {
					a.selected = idx
					a.refresh()
				}
			}
			return a, nil
		}
		return a, nil
	case runsLoadedMsg:
		a.err = m.err
		a.loadingMore = false
		if m.append {
			a.runs = append(a.runs, m.runs...)
		} else {
			a.runs = m.runs
			a.selected = 0
		}
		a.nextCursor = m.cursor
		a.hasList = true
		a.mode = modeList
		if a.selected >= len(a.runs) {
			a.selected = 0
		}
		a.resize()
		a.refresh()
		if !m.append {
			a.viewport.GotoTop()
		}
		return a, nil
	case runOpenedMsg:
		a.err = m.err
		if m.err == nil {
			a.run = m.run
			a.graph = graph.Build(m.run)
			a.layout = graph.Layout(a.graph)
			a.selectedNode = firstNode(a.layout)
			a.mode = modeGraph
		} else if a.hasList {
			a.mode = modeList // stay usable: drop back to the list on error
		}
		a.resize()
		a.refresh()
		a.viewport.GotoTop()
		return a, nil
	case logsLoadedMsg:
		switch {
		case m.err != nil:
			a.logsContent = "error fetching logs: " + m.err.Error()
		case strings.TrimSpace(m.content) == "":
			a.logsContent = "(no logs)"
		default:
			a.logsContent = m.content
		}
		a.refresh()
		a.viewport.GotoTop()
		return a, nil
	case tea.KeyMsg:
		// While the / filter input is active, keystrokes edit it; esc clears and
		// closes, enter keeps the filter and closes.
		if a.filtering {
			switch {
			case key.Matches(m, a.keys.Back):
				a.filtering = false
				a.filterInput.Blur()
				a.filterInput.SetValue("")
			case m.Type == tea.KeyEnter:
				a.filtering = false
				a.filterInput.Blur()
			default:
				var cmd tea.Cmd
				a.filterInput, cmd = a.filterInput.Update(m)
				a.refresh()
				return a, cmd
			}
			a.refresh()
			return a, nil
		}
		model, cmd := a.handleKey(m)
		a = model.(App)
		a.refresh()
		// Graph nav keys move the node cursor; keep it in view. Free scrolling
		// is still available via the mouse wheel.
		if a.mode == modeGraph && !a.detailOpen {
			a.ensureSelectedVisible()
		}
		return a, cmd
	}
	return a, nil
}

func (a App) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Quit and help toggle are global.
	switch {
	case key.Matches(k, a.keys.Quit):
		return a, tea.Quit
	case key.Matches(k, a.keys.Help):
		a.showHelp = !a.showHelp
		return a, nil
	}

	switch a.mode {
	case modeList:
		switch {
		case key.Matches(k, a.keys.Up):
			if a.selected > 0 {
				a.selected--
			}
		case key.Matches(k, a.keys.Down):
			if a.selected < len(a.runs)-1 {
				a.selected++
			} else if a.nextCursor != "" && !a.loadingMore {
				// At the bottom with more pages: fetch and append the next page.
				a.loadingMore = true
				f := a.cfg.Filter
				f.Cursor = a.nextCursor
				return a, listRunsCmd(a.client, f, true)
			}
		case key.Matches(k, a.keys.Enter):
			if len(a.runs) > 0 {
				a.err = nil
				a.mode = modeLoading
				return a, tea.Batch(openRunCmd(a.client, a.runs[a.selected].ID), a.spinner.Tick)
			}
		case key.Matches(k, a.keys.All):
			return a.reloadList(rwx.ListFilter{Limit: a.cfg.Filter.Limit})
		case key.Matches(k, a.keys.Mine):
			return a.reloadList(rwx.ListFilter{Limit: a.cfg.Filter.Limit, Mine: true})
		case key.Matches(k, a.keys.Branch):
			if a.selected < len(a.runs) && a.runs[a.selected].Branch != "" {
				return a.reloadList(rwx.ListFilter{Limit: a.cfg.Filter.Limit, Branch: a.runs[a.selected].Branch})
			}
		case key.Matches(k, a.keys.Refresh):
			f := a.cfg.Filter
			f.Cursor = ""
			return a.reloadList(f)
		}
	case modeGraph:
		// When the detail pane is open it captures Back/Logs; other keys are
		// inert until it closes.
		if a.detailOpen {
			switch {
			case key.Matches(k, a.keys.Back):
				a.detailOpen = false
				a.logsContent = ""
			case key.Matches(k, a.keys.Logs):
				if a.selectedNode != "" {
					return a, fetchLogsCmd(a.client, a.run.RunID, a.selectedNode)
				}
			}
			return a, nil
		}
		switch {
		case key.Matches(k, a.keys.Back):
			if a.hasList {
				a.err = nil
				a.mode = modeList
				return a, nil
			}
			return a, tea.Quit
		case key.Matches(k, a.keys.Enter):
			if a.selectedNode != "" {
				a.detailOpen = true
				a.logsContent = ""
			}
		case key.Matches(k, a.keys.Up):
			a.moveSelection(-1, 0)
		case key.Matches(k, a.keys.Down):
			a.moveSelection(1, 0)
		case key.Matches(k, a.keys.Left):
			a.moveSelection(0, -1)
		case key.Matches(k, a.keys.Right):
			a.moveSelection(0, 1)
		case key.Matches(k, a.keys.Filter):
			a.filtering = true
			a.filterInput.Focus()
			return a, textinput.Blink
		case key.Matches(k, a.keys.Isolate):
			if a.focus != nil {
				a.focus = nil // toggle off
			} else if a.selectedNode != "" {
				a.focus = graph.Focus(a.graph, a.selectedNode)
			}
		}
	}
	return a, nil
}

func (a App) View() string {
	if a.err != nil && a.mode == modeLoading {
		return theme.Failure.Render(fmt.Sprintf("error: %v", a.err)) + "\n\npress q to quit\n"
	}
	switch a.mode {
	case modeLoading:
		return a.spinner.View() + " " + theme.Faint.Render("loading…")
	case modeList, modeGraph:
		footer := a.footerView()
		if a.filtering {
			footer = a.filterInput.View() + "\n" + footer
		}
		if a.err != nil && a.mode == modeList {
			footer = theme.Failure.Render(fmt.Sprintf("error: %v", a.err)) + "\n" + footer
		}
		return lipgloss.JoinVertical(lipgloss.Left, a.viewport.View(), footer)
	}
	return ""
}
