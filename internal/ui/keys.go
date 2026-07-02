package ui

import "github.com/charmbracelet/bubbles/key"

// keyMap is the single source of truth for key bindings. The footer keybar is
// generated from it (via bubbles/help), so the labels can't drift from behavior.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Enter   key.Binding
	Back    key.Binding
	ToList  key.Binding
	Quit    key.Binding
	Pin     key.Binding
	Logs    key.Binding
	Top     key.Binding
	Bottom  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
		Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
		Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		ToList:  key.NewBinding(key.WithKeys("backspace"), key.WithHelp("⌫", "list")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Pin:     key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "pin")),
		Logs:    key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "logs")),
		Top:     key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:  key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
	}
}

// modeHelp adapts the keymap to bubbles/help's KeyMap interface, tailoring the
// always-visible one-line keybar to the current mode. (There is no separate ?
// overlay — the keybar shows everything that applies.)
type modeHelp struct {
	keys keyMap
	mode appMode
}

func (h modeHelp) ShortHelp() []key.Binding {
	switch h.mode {
	case modeList:
		// Type-to-filter like the graph; Tab cycles the server-side fetch scope.
		return []key.Binding{
			key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "move")),
			key.NewBinding(key.WithKeys("runes"), key.WithHelp("type", "filter")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("⇥", "scope")),
			h.keys.Enter,
			key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("^R", "refresh")),
			key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "quit")),
		}
	case modeGraph:
		// Graph mode is type-to-filter: printable keys build the filter live, so
		// these labels describe the non-letter actions plus the typing hint.
		return []key.Binding{
			key.NewBinding(key.WithKeys("up", "down", "left", "right"), key.WithHelp("↑↓←→", "move")),
			key.NewBinding(key.WithKeys("runes"), key.WithHelp("type", "filter")),
			h.keys.Pin,
			h.keys.Enter,
			h.keys.Back,
			h.keys.ToList,
			key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "quit")),
		}
	default:
		return []key.Binding{h.keys.Quit}
	}
}

// FullHelp is required by help.KeyMap but unused (no overlay); it mirrors
// ShortHelp so nothing is lost if a caller ever renders it.
func (h modeHelp) FullHelp() [][]key.Binding {
	return [][]key.Binding{h.ShortHelp()}
}
