package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
	return Screen(m.run, m.graph, m.layout)
}

// Screen renders the full graph view (header, graph, legend) as a string. Pure,
// so it backs both View() and the headless --print path.
func Screen(run rwx.Run, g *graph.Graph, l *graph.LayoutData) string {
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

	b.WriteString(RenderGraph(g, l, RenderOpts{Crit: cp, Failure: fi}))
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
	b.WriteString(theme.Faint.Render("↑/↓ move · enter: open · q: quit"))
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

	runs     []rwx.RunSummary
	selected int

	run    rwx.Run
	graph  *graph.Graph
	layout *graph.LayoutData

	err error
}

// NewApp builds the root model.
func NewApp(client *rwx.Client, cfg AppConfig) App {
	return App{client: client, cfg: cfg, now: time.Now, mode: modeLoading}
}

type runsLoadedMsg struct {
	runs []rwx.RunSummary
	err  error
}

type runOpenedMsg struct {
	run rwx.Run
	err error
}

func listRunsCmd(c *rwx.Client, f rwx.ListFilter) tea.Cmd {
	return func() tea.Msg {
		rl, err := c.ListRuns(context.Background(), f)
		return runsLoadedMsg{runs: rl.Runs, err: err}
	}
}

func openRunCmd(c *rwx.Client, id string) tea.Cmd {
	return func() tea.Msg {
		r, err := c.Results(context.Background(), id)
		return runOpenedMsg{run: r, err: err}
	}
}

func (a App) Init() tea.Cmd {
	if a.cfg.Run != "" {
		return openRunCmd(a.client, a.cfg.Run)
	}
	return listRunsCmd(a.client, a.cfg.Filter)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case runsLoadedMsg:
		a.err = m.err
		a.runs = m.runs
		a.hasList = true
		a.mode = modeList
		if a.selected >= len(a.runs) {
			a.selected = 0
		}
		return a, nil
	case runOpenedMsg:
		a.err = m.err
		if m.err == nil {
			a.run = m.run
			a.graph = graph.Build(m.run)
			a.layout = graph.Layout(a.graph)
			a.mode = modeGraph
		} else if a.hasList {
			a.mode = modeList // stay usable: drop back to the list on error
		}
		return a, nil
	case tea.KeyMsg:
		return a.handleKey(m)
	}
	return a, nil
}

func (a App) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.mode {
	case modeList:
		switch k.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "up", "k":
			if a.selected > 0 {
				a.selected--
			}
		case "down", "j":
			if a.selected < len(a.runs)-1 {
				a.selected++
			}
		case "enter":
			if len(a.runs) > 0 {
				a.err = nil
				a.mode = modeLoading
				return a, openRunCmd(a.client, a.runs[a.selected].ID)
			}
		}
	case modeGraph:
		switch k.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "esc", "backspace", "h", "left":
			if a.hasList {
				a.err = nil
				a.mode = modeList
				return a, nil
			}
			return a, tea.Quit
		}
	case modeLoading:
		if k.String() == "ctrl+c" {
			return a, tea.Quit
		}
	}
	return a, nil
}

func (a App) View() string {
	if a.err != nil && a.mode != modeList {
		return fmt.Sprintf("error: %v\n\npress q to quit\n", a.err)
	}
	switch a.mode {
	case modeLoading:
		return theme.Faint.Render("loading…") + "\n"
	case modeList:
		out := HomeView(a.runs, a.selected, a.now())
		if a.err != nil {
			out += theme.Failure.Render(fmt.Sprintf("error: %v", a.err)) + "\n"
		}
		return out
	case modeGraph:
		return Screen(a.run, a.graph, a.layout) +
			theme.Faint.Render("esc: back · q: quit") + "\n"
	}
	return ""
}
