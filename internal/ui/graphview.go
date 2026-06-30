// Package ui holds the Bubble Tea models and rendering for the Flow viewer.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/chrismo/rwx-tui/internal/graph"
	"github.com/chrismo/rwx-tui/internal/rwx"
)

// stateGlyphs maps a display state to its glyph. Color comes from the theme
// (theme.State); glyphs live here so the render layout stays stable.
var stateGlyphs = map[rwx.DisplayState]string{
	rwx.StateRan:      "✓",
	rwx.StateCacheHit: "⚡",
	rwx.StateRunning:  "●",
	rwx.StateWaiting:  "○",
	rwx.StateFailed:   "✗",
	rwx.StateSkipped:  "⊘",
	rwx.StatePending:  "·",
}

func glyphFor(s rwx.DisplayState) string {
	if g, ok := stateGlyphs[s]; ok {
		return g
	}
	return "?"
}

// RenderGraph renders the layered layout top-down: layer 0 (roots) at the top,
// each layer a row of state-colored node cells. Edge routing is a follow-up;
// RenderOpts carries the overlays applied to the graph render.
type RenderOpts struct {
	Crit    *graph.CritPath    // critical path: thick border (may be nil)
	Failure *graph.FailureInfo // failures + blast radius (may be nil)
}

// this v1 conveys structure via layering and state via color/glyph. Critical-
// path nodes get a thick border; blast-radius nodes get a red border and a "↯"
// marker.
func RenderGraph(g *graph.Graph, l *graph.LayoutData, opts RenderOpts) string {
	rows := make([]string, 0, len(l.Layers))
	for _, layer := range l.Layers {
		cells := make([]string, 0, len(layer))
		for _, key := range layer {
			onCrit := opts.Crit != nil && opts.Crit.Contains(key)
			onBlast := opts.Failure != nil && opts.Failure.InBlast(key)
			cells = append(cells, renderCell(g.Node(key), onCrit, onBlast))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
	}
	// A simple top-down flow cue between layer rows; true edge routing is a
	// follow-up.
	return strings.Join(rows, "\n   │\n") + "\n"
}

func renderCell(n *graph.Node, onCrit, onBlast bool) string {
	fg := theme.State(n.State).GetForeground()
	label := fmt.Sprintf("%s %s", glyphFor(n.State), n.Key)
	if n.HasTiming && n.DurationSeconds > 0 {
		label += fmt.Sprintf(" (%ds)", n.DurationSeconds)
	}
	if onBlast {
		label += " ↯" // downstream of a failure
	}
	border := lipgloss.RoundedBorder()
	if onCrit {
		border = lipgloss.ThickBorder()
	}
	borderColor := fg
	if onBlast {
		borderColor = theme.Failure.GetForeground() // affected by failure
	}
	box := lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Foreground(fg).
		Bold(onCrit).
		Padding(0, 1).
		MarginRight(2)
	return box.Render(label)
}

// FailureLine summarizes a run's failures and blast radius as one line, or "" if
// nothing failed.
func FailureLine(fi *graph.FailureInfo) string {
	if fi == nil || len(fi.Failed) == 0 {
		return ""
	}
	line := "failed: " + strings.Join(fi.Failed, ", ")
	blast := fi.BlastKeys()
	if len(blast) == 0 {
		line += " · no downstream impact"
	} else {
		line += " · blast radius: " + strings.Join(blast, ", ")
	}
	return line
}

// CriticalPathLine summarizes the critical path as a one-line chain with total.
func CriticalPathLine(cp *graph.CritPath) string {
	if cp == nil || len(cp.Keys) == 0 {
		return ""
	}
	return fmt.Sprintf("critical path: %s · %ds", strings.Join(cp.Keys, " → "), cp.Total)
}

// Legend returns a one-line key of state glyphs for a footer.
func Legend() string {
	order := []rwx.DisplayState{
		rwx.StateRan, rwx.StateCacheHit, rwx.StateRunning,
		rwx.StateWaiting, rwx.StateFailed, rwx.StateSkipped, rwx.StatePending,
	}
	parts := make([]string, 0, len(order))
	for _, s := range order {
		parts = append(parts, fmt.Sprintf("%s %s", glyphFor(s), s))
	}
	return strings.Join(parts, "   ")
}
