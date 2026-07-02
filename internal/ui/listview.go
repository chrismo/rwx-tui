package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/chrismo/crux/internal/rwx"
)

// FilterRunList narrows runs by a case-insensitive substring over title,
// definition path, and branch (the list view filter). Empty term returns runs
// unchanged. Shared by the interactive list and the headless --print path.
func FilterRunList(runs []rwx.RunSummary, term string) []rwx.RunSummary {
	if term == "" {
		return runs
	}
	f := strings.ToLower(term)
	out := make([]rwx.RunSummary, 0, len(runs))
	for _, r := range runs {
		if strings.Contains(strings.ToLower(r.Title), f) ||
			strings.Contains(strings.ToLower(r.DefinitionPath), f) ||
			strings.Contains(strings.ToLower(r.Branch), f) {
			out = append(out, r)
		}
	}
	return out
}

// ScopeLabel names the Tab-cycle fetch scope ("all"/"mine"/"branch"). Used to
// track and advance the cycle, not for display.
func ScopeLabel(f rwx.ListFilter) string {
	switch {
	case f.Mine:
		return "mine"
	case f.Branch != "":
		return "branch"
	default:
		return "all"
	}
}

// FetchLabel describes the full server-side fetch state for the header, ""
// meaning the default (all). Unlike ScopeLabel it also reflects result-status
// (e.g. --failed), which is orthogonal to the all/mine/branch cycle.
func FetchLabel(f rwx.ListFilter) string {
	var parts []string
	if f.Mine {
		parts = append(parts, "mine")
	}
	if f.Branch != "" {
		parts = append(parts, "branch: "+f.Branch)
	}
	if f.ResultStatus != "" {
		parts = append(parts, f.ResultStatus)
	}
	return strings.Join(parts, " · ")
}

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
