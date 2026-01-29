// Package tui provides a full-featured terminal user interface for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines key bindings for the TUI
type KeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding

	// Actions
	Submit key.Binding
	Cancel key.Binding
	Help   key.Binding
	Quit   key.Binding

	// Panels
	FocusChat     key.Binding
	FocusSidebar  key.Binding
	FocusInput    key.Binding
	ToggleSidebar key.Binding
	ToggleContext key.Binding
	TogglePreview key.Binding

	// Commands
	NewSession  key.Binding
	SaveSession key.Binding
	LoadSession key.Binding
	ClearChat   key.Binding
	SwitchModel key.Binding
	ShowStats   key.Binding

	// Editor
	NewLine    key.Binding
	DeleteWord key.Binding
	DeleteLine key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup/C-u", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdn/C-d", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),

		// Actions
		Submit: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Help: key.NewBinding(
			key.WithKeys("?", "f1"),
			key.WithHelp("?/F1", "toggle help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "ctrl+q"),
			key.WithHelp("C-c/C-q", "quit"),
		),

		// Panels
		FocusChat: key.NewBinding(
			key.WithKeys("ctrl+1"),
			key.WithHelp("C-1", "focus chat"),
		),
		FocusSidebar: key.NewBinding(
			key.WithKeys("ctrl+2"),
			key.WithHelp("C-2", "focus sidebar"),
		),
		FocusInput: key.NewBinding(
			key.WithKeys("ctrl+3", "i"),
			key.WithHelp("C-3/i", "focus input"),
		),
		ToggleSidebar: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("C-b", "toggle sidebar"),
		),
		ToggleContext: key.NewBinding(
			key.WithKeys("ctrl+e"),
			key.WithHelp("C-e", "toggle context"),
		),
		TogglePreview: key.NewBinding(
			key.WithKeys("ctrl+p"),
			key.WithHelp("C-p", "toggle preview"),
		),

		// Commands
		NewSession: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("C-n", "new session"),
		),
		SaveSession: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("C-s", "save session"),
		),
		LoadSession: key.NewBinding(
			key.WithKeys("ctrl+o"),
			key.WithHelp("C-o", "load session"),
		),
		ClearChat: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("C-l", "clear chat"),
		),
		SwitchModel: key.NewBinding(
			key.WithKeys("ctrl+m"),
			key.WithHelp("C-m", "switch model"),
		),
		ShowStats: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("C-t", "show stats"),
		),

		// Editor
		NewLine: key.NewBinding(
			key.WithKeys("shift+enter", "ctrl+enter"),
			key.WithHelp("S-enter", "new line"),
		),
		DeleteWord: key.NewBinding(
			key.WithKeys("ctrl+w"),
			key.WithHelp("C-w", "delete word"),
		),
		DeleteLine: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("C-u", "delete line"),
		),
	}
}

// ShortHelp returns keybindings for the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Submit, k.Help, k.ToggleSidebar, k.ToggleContext, k.Quit}
}

// FullHelp returns all keybindings for the full help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown, k.Home, k.End},
		{k.Submit, k.Cancel, k.Help, k.Quit},
		{k.FocusChat, k.FocusSidebar, k.FocusInput, k.ToggleSidebar, k.ToggleContext, k.TogglePreview},
		{k.NewSession, k.SaveSession, k.LoadSession, k.ClearChat},
	}
}
