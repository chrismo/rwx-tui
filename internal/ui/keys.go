package ui

import "github.com/charmbracelet/bubbles/key"

// keyMap is the single source of truth for key bindings. The footer keybar and
// the ? help overlay are generated from it (via bubbles/help), so labels can't
// drift from behavior.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Enter   key.Binding
	Back    key.Binding
	Quit    key.Binding
	Help    key.Binding
	Filter  key.Binding
	Refresh key.Binding
	Top     key.Binding
	Bottom  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "back")),
		Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "open")),
		Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:    key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Top:     key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:  key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
	}
}

// modeHelp adapts the keymap to bubbles/help's KeyMap interface, tailoring the
// always-visible short help to the current mode while the ? overlay (FullHelp)
// shows everything.
type modeHelp struct {
	keys keyMap
	mode appMode
}

func (h modeHelp) ShortHelp() []key.Binding {
	switch h.mode {
	case modeList:
		return []key.Binding{h.keys.Up, h.keys.Down, h.keys.Enter, h.keys.Filter, h.keys.Refresh, h.keys.Help, h.keys.Quit}
	case modeGraph:
		return []key.Binding{h.keys.Up, h.keys.Down, h.keys.Back, h.keys.Filter, h.keys.Help, h.keys.Quit}
	default:
		return []key.Binding{h.keys.Help, h.keys.Quit}
	}
}

func (h modeHelp) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{h.keys.Up, h.keys.Down, h.keys.Left, h.keys.Right, h.keys.Top, h.keys.Bottom},
		{h.keys.Enter, h.keys.Back, h.keys.Filter, h.keys.Refresh},
		{h.keys.Help, h.keys.Quit},
	}
}
