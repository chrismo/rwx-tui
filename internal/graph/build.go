// Package graph turns an rwx.Run into a dependency graph for layout and
// analysis. Nodes are the run's top-level tasks; edges are the `use:`
// dependencies parsed from each task's RawDefinition (the run payload is
// self-contained, so no checkout of .rwx/*.yml is needed).
package graph

import (
	"strings"

	"github.com/chrismo/rwx-tui/internal/rwx"
)

// Node is a task in the graph.
type Node struct {
	Key             string
	TaskType        string
	State           rwx.DisplayState
	DurationSeconds int  // ExecutionRuntimeSeconds, else CompletedRuntimeSeconds
	HasTiming       bool // whether DurationSeconds came from real timing data
}

// Edge points from a dependency to the task that depends on it (roots -> leaves),
// which is the direction the layered layout consumes.
type Edge struct {
	From string
	To   string
}

// Graph is the built dependency graph.
type Graph struct {
	Nodes []*Node
	Edges []Edge
	index map[string]*Node
}

// Node returns the node with the given key, or nil.
func (g *Graph) Node(key string) *Node {
	return g.index[key]
}

// Build constructs a Graph from a run's top-level tasks.
func Build(run rwx.Run) *Graph {
	g := &Graph{index: make(map[string]*Node, len(run.Tasks))}

	for i := range run.Tasks {
		t := run.Tasks[i]
		dur, has := duration(t)
		n := &Node{
			Key:             t.Key,
			TaskType:        t.TaskType,
			State:           t.DisplayState(),
			DurationSeconds: dur,
			HasTiming:       has,
		}
		g.Nodes = append(g.Nodes, n)
		g.index[t.Key] = n
	}

	for i := range run.Tasks {
		t := run.Tasks[i]
		for _, dep := range parseUse(t.RawDefinition) {
			from := g.resolveKey(dep)
			if g.index[from] == nil || g.index[t.Key] == nil {
				continue // skip edges to tasks we don't have a node for
			}
			g.Edges = append(g.Edges, Edge{From: from, To: t.Key})
		}
	}

	return g
}

// resolveKey maps a `use:` reference to an actual node key. Synthetic base
// tasks are referenced unprefixed in RawDefinition (e.g. `use: base-image`) but
// appear in Tasks with a `~` prefix.
func (g *Graph) resolveKey(ref string) string {
	if g.index[ref] != nil {
		return ref
	}
	if g.index["~"+ref] != nil {
		return "~" + ref
	}
	return ref
}

func duration(t rwx.Task) (int, bool) {
	if t.ExecutionRuntimeSeconds != nil {
		return *t.ExecutionRuntimeSeconds, true
	}
	if t.CompletedRuntimeSeconds != nil {
		return *t.CompletedRuntimeSeconds, true
	}
	return 0, false
}

// parseUse extracts the `use:` dependency keys from a task's RawDefinition. The
// RawDefinition is a small YAML fragment that can be wrapped in stray preamble
// (a bare `:` line, comments, indentation), so this scans line-by-line rather
// than parsing the whole fragment as YAML. It handles the three `use:` forms:
// flow (`use: [a, b]`), scalar (`use: a`), and a block sequence.
func parseUse(raw string) []string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		rest, ok := matchKey(line, "use")
		if !ok {
			continue
		}
		rest = stripComment(rest)
		switch {
		case rest == "":
			// Block sequence: collect following `- item` lines that are more
			// indented than the `use:` key, until indentation drops back.
			useIndent := indentOf(line)
			var deps []string
			for _, bl := range lines[i+1:] {
				if strings.TrimSpace(bl) == "" {
					continue
				}
				if indentOf(bl) <= useIndent {
					break
				}
				item := strings.TrimSpace(bl)
				if !strings.HasPrefix(item, "-") {
					break
				}
				if v := unquote(strings.TrimSpace(item[1:])); v != "" {
					deps = append(deps, v)
				}
			}
			return deps
		case strings.HasPrefix(rest, "["):
			inner := strings.TrimSuffix(strings.TrimPrefix(rest, "["), "]")
			var deps []string
			for _, part := range strings.Split(inner, ",") {
				if v := unquote(strings.TrimSpace(part)); v != "" {
					deps = append(deps, v)
				}
			}
			return deps
		default:
			return []string{unquote(rest)}
		}
	}
	return nil
}

// matchKey reports whether a line is `<key>:` (at any indentation) and returns
// the trimmed value after the colon.
func matchKey(line, key string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	prefix := key + ":"
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	return strings.TrimSpace(trimmed[len(prefix):]), true
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func stripComment(s string) string {
	// Only strip a comment that is clearly separated, to avoid eating a `#`
	// inside a value. RawDefinition `use:` values don't contain `#`.
	if i := strings.Index(s, " #"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
