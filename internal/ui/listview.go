package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/chrismo/crux/internal/rwx"
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
		left := theme.RunStatus(r.Status).Render(glyph + " " + padRight(result, 9))

		// Columns are padded by display width (not bytes) and truncated to fixed
		// widths, so multibyte titles and long definition paths can't shove the
		// row out of alignment.
		row := fmt.Sprintf("%s%s  %s  %s  %s  %s",
			cursor, left,
			padRight(r.DefinitionPath, 18),
			padRight(r.Title, 26),
			padLeft(humanizeAge(r.CreatedAt, now), 8),
			padLeft(runtimeStr(r.CompletedRuntimeSeconds), 5),
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

// padRight truncates s to w display cells (adding … when cut) and right-pads to
// exactly w cells. padLeft is the same but right-aligned. Both measure by
// display width so multibyte content stays column-aligned.
func padRight(s string, w int) string {
	s = runewidth.Truncate(s, w, "…")
	return s + strings.Repeat(" ", w-runewidth.StringWidth(s))
}

func padLeft(s string, w int) string {
	s = runewidth.Truncate(s, w, "…")
	return strings.Repeat(" ", w-runewidth.StringWidth(s)) + s
}
