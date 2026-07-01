// Command crux is a local TUI for monitoring RWX runs with a better Flow
// dependency-graph viewer.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chrismo/crux/internal/rwx"
	"github.com/chrismo/crux/internal/ui"
)

// Build metadata, injected at build time via -ldflags -X (see build.sh).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// options holds the parsed command-line configuration.
type options struct {
	branch     string // branch to resolve a run for (default: current git branch)
	definition string // .rwx definition path, required when a branch has several
	run        string // explicit run ID to open
	dir        string // checkout dir for the static-YAML fallback (default: cwd)
	pin        string // comma-separated substring terms to pre-pin in the graph
	print      bool   // render once to stdout and exit (no interactive TUI)
	version    bool   // print version and exit
}

func parseFlags(args []string) (options, error) {
	fs := flag.NewFlagSet("crux", flag.ContinueOnError)
	var o options
	fs.StringVar(&o.branch, "branch", "", "branch to resolve a run for (default: current git branch)")
	fs.StringVar(&o.definition, "definition", "", "RWX definition path (required when a branch has multiple)")
	fs.StringVar(&o.run, "run", "", "explicit run ID to open")
	fs.StringVar(&o.dir, "dir", ".", "checkout directory for the static-YAML fallback")
	fs.StringVar(&o.pin, "pin", "", "comma-separated substring terms to pre-pin (e.g. --pin api,deploy)")
	fs.BoolVar(&o.print, "print", false, "render once to stdout and exit (no interactive TUI)")
	fs.BoolVar(&o.version, "version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return o, nil
}

// splitPins turns a comma-separated --pin value into trimmed, non-empty terms.
func splitPins(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var terms []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			terms = append(terms, t)
		}
	}
	return terms
}

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		os.Exit(2)
	}
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "crux:", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.version {
		fmt.Printf("crux %s (commit %s, built %s)\n", version, commit, date)
		return nil
	}

	client := rwx.NewClient()
	if err := client.CheckVersion(context.Background()); err != nil {
		return err
	}
	filter := rwx.ListFilter{Limit: 30, Branch: opts.branch}

	// Headless render: one-shot fetch + print, no TUI loop.
	if opts.print {
		return printOnce(client, opts, filter)
	}

	app := ui.NewApp(client, ui.AppConfig{Run: opts.run, Filter: filter, Pins: splitPins(opts.pin)})
	_, err := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func printOnce(client *rwx.Client, opts options, filter rwx.ListFilter) error {
	ctx := context.Background()
	if opts.run != "" {
		r, err := client.Results(ctx, opts.run)
		if err != nil {
			return err
		}
		fmt.Print(ui.NewModel(r).View())
		return nil
	}
	rl, err := client.ListRuns(ctx, filter)
	if err != nil {
		return err
	}
	fmt.Print(ui.HomeView(rl.Runs, 0, time.Now(), ui.FilterLabel(filter)))
	return nil
}
