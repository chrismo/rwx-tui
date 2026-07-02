package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/chrismo/crux/internal/rwx"
)

func loadRunList(t *testing.T) []rwx.RunSummary {
	t.Helper()
	data, err := os.ReadFile("../rwx/testdata/runs_list.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var rl rwx.RunList
	if err := json.Unmarshal(data, &rl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return rl.Runs
}

func TestRenderRunList(t *testing.T) {
	runs := loadRunList(t)
	now := time.Date(2026, 6, 30, 21, 0, 0, 0, time.UTC)
	out := RenderRunList(runs, 1, now)

	if !strings.Contains(out, ".rwx/ci.yml") {
		t.Error("expected the definition path in output")
	}
	if !strings.Contains(out, "›") {
		t.Error("expected a cursor marker for the selected row")
	}
	// The selected row (index 1) gets the marker; index 0 does not.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != len(runs) {
		t.Fatalf("rendered %d lines, want %d", len(lines), len(runs))
	}
	if strings.Contains(lines[0], "›") {
		t.Error("row 0 should not be selected")
	}
	if !strings.Contains(lines[1], "›") {
		t.Error("row 1 should be selected")
	}
}

func TestRenderRunListEmpty(t *testing.T) {
	out := RenderRunList(nil, 0, time.Now())
	if !strings.Contains(out, "no runs") {
		t.Errorf("expected empty-state message, got %q", out)
	}
}

// padRight/padLeft must produce exact display-cell widths regardless of
// multibyte or wide runes, or the run-list columns drift out of alignment.
func TestPadCellWidths(t *testing.T) {
	cases := []struct {
		s string
		w int
	}{
		{"hi", 5},                   // short: pad
		{"a-very-long-path.yml", 8}, // long: truncate + …
		{"—", 5},                    // multibyte em-dash (3 bytes, 1 cell)
		{"日本語テスト", 6},          // wide runes (2 cells each)
		{"", 4},                     // empty
	}
	for _, c := range cases {
		if got := runewidth.StringWidth(padRight(c.s, c.w)); got != c.w {
			t.Errorf("padRight(%q, %d) width = %d, want %d", c.s, c.w, got, c.w)
		}
		if got := runewidth.StringWidth(padLeft(c.s, c.w)); got != c.w {
			t.Errorf("padLeft(%q, %d) width = %d, want %d", c.s, c.w, got, c.w)
		}
	}
}

func TestHumanizeAge(t *testing.T) {
	now := time.Date(2026, 6, 30, 21, 0, 0, 0, time.UTC)
	tests := []struct {
		iso  string
		want string
	}{
		{"2026-06-30T20:59:30Z", "30s ago"},
		{"2026-06-30T20:45:00Z", "15m ago"},
		{"2026-06-30T18:00:00Z", "3h ago"},
		{"2026-06-27T21:00:00Z", "3d ago"},
		{"not-a-time", "?"},
	}
	for _, tt := range tests {
		if got := humanizeAge(tt.iso, now); got != tt.want {
			t.Errorf("humanizeAge(%q) = %q, want %q", tt.iso, got, tt.want)
		}
	}
}
