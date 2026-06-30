package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/chrismo/rwx-tui/internal/rwx"
)

// Theme holds the UI's semantic styles, built on lipgloss.AdaptiveColor so they
// read well on both light and dark terminals. All color application goes through
// here — no scattered lipgloss.Color literals. In a non-color terminal (tests,
// pipes) lipgloss emits no escapes, so only glyphs/text show.
type Theme struct {
	Success  lipgloss.Style // ran / succeeded — green
	CacheHit lipgloss.Style // cache hit — cyan
	Running  lipgloss.Style // running / in-progress — yellow
	Muted    lipgloss.Style // waiting / skipped / pending / no_result — gray
	Failure  lipgloss.Style // failed — red
	Special  lipgloss.Style // debugged / sandboxed / critical-path line — magenta
	Header   lipgloss.Style // bold headers
	Selected lipgloss.Style // selected row/node
	Faint    lipgloss.Style // de-emphasized footer/legend text
}

func adaptive(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

func defaultTheme() Theme {
	fg := func(light, dark string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(adaptive(light, dark))
	}
	return Theme{
		Success:  fg("#207520", "#5FD75F"),
		CacheHit: fg("#0087AF", "#5FD7FF"),
		Running:  fg("#AF8700", "#FFD75F"),
		Muted:    fg("#6C6C6C", "#8A8A8A"),
		Failure:  fg("#D70000", "#FF5F5F"),
		Special:  fg("#8700AF", "#D787FF"),
		Header:   lipgloss.NewStyle().Bold(true),
		Selected: lipgloss.NewStyle().Bold(true),
		Faint:    lipgloss.NewStyle().Faint(true),
	}
}

// theme is the package-level default. Keeping it package-level lets the pure
// render funcs apply styling without threading a Theme through every signature
// (AdaptiveColor resolves per-render, so no per-instance state is needed).
var theme = defaultTheme()

// State returns the style for a task display state.
func (t Theme) State(s rwx.DisplayState) lipgloss.Style {
	switch s {
	case rwx.StateRan:
		return t.Success
	case rwx.StateCacheHit:
		return t.CacheHit
	case rwx.StateRunning:
		return t.Running
	case rwx.StateFailed:
		return t.Failure
	case rwx.StateWaiting, rwx.StateSkipped, rwx.StatePending:
		return t.Muted
	default:
		return t.Muted
	}
}

// RunStatus returns the style for a run-level status (list rows).
func (t Theme) RunStatus(s rwx.RunStatus) lipgloss.Style {
	switch s.Execution {
	case "in_progress":
		return t.Running
	case "waiting", "aborted":
		return t.Muted
	}
	switch s.Result {
	case "succeeded":
		return t.Success
	case "failed":
		return t.Failure
	case "debugged", "sandboxed":
		return t.Special
	default:
		return t.Muted
	}
}
