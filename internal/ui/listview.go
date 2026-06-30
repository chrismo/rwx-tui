package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/chrismo/rwx-tui/internal/rwx"
)

// runGlyph returns a glyph and color for a run-level status. In-progress runs
// are distinguished from their (not yet meaningful) result. Color comes from the
// theme (theme.RunStatus); only the glyph lives here.
func runGlyph(s rwx.RunStatus) string {
	switch s.Execution {
	case "in_progress":
		return "●"
	case "waiting":
		return "○"
	case "aborted":
		return "⊘"
	}
	switch s.Result {
	case "succeeded":
		return "✓"
	case "failed":
		return "✗"
	case "debugged", "sandboxed":
		return "◆"
	default:
		return "○"
	}
}

// RenderRunList renders the run-list rows, most recent first, with the selected
// row marked. now is injected so the relative ages are testable.
func RenderRunList(runs []rwx.RunSummary, selected int, now time.Time) string {
	if len(runs) == 0 {
		return theme.Faint.Render("  no runs found") + "\n"
	}
	var b strings.Builder
	for i, r := range runs {
		cursor := "  "
		if i == selected {
			cursor = "› "
		}
		glyph := runGlyph(r.Status)
		result := r.Status.Result
		if r.Status.Execution == "in_progress" {
			result = "running"
		}
		left := theme.RunStatus(r.Status).Render(fmt.Sprintf("%s %-9s", glyph, result))

		row := fmt.Sprintf("%s%s  %-13s  %-26s  %8s  %5s",
			cursor, left,
			r.DefinitionPath,
			truncate(r.Title, 26),
			humanizeAge(r.CreatedAt, now),
			runtimeStr(r.CompletedRuntimeSeconds),
		)
		if i == selected {
			row = theme.Selected.Render(row)
		}
		b.WriteString(row)
		b.WriteString("\n")
	}
	return b.String()
}

func runtimeStr(secs *int) string {
	if secs == nil {
		return "—"
	}
	return fmt.Sprintf("%ds", *secs)
}

// humanizeAge renders an ISO-8601 timestamp as a coarse "N<unit> ago" relative
// to now. Returns "?" if the timestamp can't be parsed.
func humanizeAge(iso string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return "?"
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours())/24)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
