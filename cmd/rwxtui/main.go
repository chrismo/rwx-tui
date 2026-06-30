// Command rwxtui is a local TUI for monitoring RWX runs with a better Flow
// dependency-graph viewer.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chrismo/rwx-tui/internal/rwx"
	"github.com/chrismo/rwx-tui/internal/ui"
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
	print      bool   // render once to stdout and exit (no interactive TUI)
	version    bool   // print version and exit
}

func parseFlags(args []string) (options, error) {
	fs := flag.NewFlagSet("rwxtui", flag.ContinueOnError)
	var o options
	fs.StringVar(&o.branch, "branch", "", "branch to resolve a run for (default: current git branch)")
	fs.StringVar(&o.definition, "definition", "", "RWX definition path (required when a branch has multiple)")
	fs.StringVar(&o.run, "run", "", "explicit run ID to open")
	fs.StringVar(&o.dir, "dir", ".", "checkout directory for the static-YAML fallback")
	fs.BoolVar(&o.print, "print", false, "render once to stdout and exit (no interactive TUI)")
	fs.BoolVar(&o.version, "version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return o, nil
}

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		os.Exit(2)
	}
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "rwxtui:", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.version {
		fmt.Printf("rwxtui %s (commit %s, built %s)\n", version, commit, date)
		return nil
	}
	if opts.run == "" {
		return fmt.Errorf("a run ID is required for now: pass --run <id> " +
			"(branch resolution via `rwx runs list` is the next step)")
	}

	client := rwx.NewClient()
	r, err := client.Results(context.Background(), opts.run)
	if err != nil {
		return err
	}

	model := ui.NewModel(r)

	if opts.print {
		fmt.Print(model.View())
		return nil
	}

	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}
