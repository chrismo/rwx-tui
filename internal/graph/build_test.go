package graph

import (
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/chrismo/rwx-tui/internal/rwx"
)

func TestParseUse(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "flow sequence",
			raw:  "  - key: deps\n    use: [code, go]\n    run: go mod download\n",
			want: []string{"code", "go"},
		},
		{
			name: "scalar",
			raw:  "  - key: vet\n    use: deps\n    run: go vet ./...\n",
			want: []string{"deps"},
		},
		{
			name: "block sequence stops at next key",
			raw:  "  - key: x\n    use:\n      - a\n      - b\n    run: echo hi\n",
			want: []string{"a", "b"},
		},
		{
			name: "no use is roots",
			raw:  "  - key: go\n    call: golang/install 1.2.1\n",
			want: nil,
		},
		{
			name: "quoted scalar",
			raw:  "    use: \"deps\"\n",
			want: []string{"deps"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUse(tt.raw)
			if !equalStrings(got, tt.want) {
				t.Errorf("parseUse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildFromRealRun(t *testing.T) {
	data, err := os.ReadFile("../rwx/testdata/run_succeeded.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var run rwx.Run
	if err := json.Unmarshal(data, &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	g := Build(run)

	wantKeys := []string{"code", "go", "deps", "vet", "test", "build", "~base-image", "~base-config"}
	if len(g.Nodes) != len(wantKeys) {
		t.Fatalf("node count = %d, want %d", len(g.Nodes), len(wantKeys))
	}
	for _, k := range wantKeys {
		if g.Node(k) == nil {
			t.Errorf("missing node %q", k)
		}
	}

	// Edges are dependency -> dependent. `~base-config use: base-image` must
	// resolve to the ~-prefixed node key.
	wantEdges := []Edge{
		{From: "code", To: "deps"},
		{From: "go", To: "deps"},
		{From: "deps", To: "vet"},
		{From: "deps", To: "test"},
		{From: "deps", To: "build"},
		{From: "~base-image", To: "~base-config"},
	}
	if !equalEdges(g.Edges, wantEdges) {
		t.Errorf("edges = %v, want %v", g.Edges, wantEdges)
	}

	// State and timing flow through from the results payload.
	if got := g.Node("go").State; got != rwx.StateCacheHit {
		t.Errorf("go state = %q, want cache-hit", got)
	}
	if got := g.Node("vet").State; got != rwx.StateRan {
		t.Errorf("vet state = %q, want ran", got)
	}
	if got := g.Node("vet").DurationSeconds; got != 8 {
		t.Errorf("vet duration = %d, want 8", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalEdges(a, b []Edge) bool {
	if len(a) != len(b) {
		return false
	}
	norm := func(e []Edge) []Edge {
		c := append([]Edge(nil), e...)
		sort.Slice(c, func(i, j int) bool {
			if c[i].From != c[j].From {
				return c[i].From < c[j].From
			}
			return c[i].To < c[j].To
		})
		return c
	}
	ca, cb := norm(a), norm(b)
	for i := range ca {
		if ca[i] != cb[i] {
			return false
		}
	}
	return true
}
