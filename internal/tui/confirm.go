// Package tui provides a full-featured terminal user interface for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmationType represents the type of confirmation
type ConfirmationType int

const (
	ConfirmTypeFile ConfirmationType = iota
	ConfirmTypeCommand
	ConfirmTypeNetwork
	ConfirmTypeDangerous
)

// ConfirmChoice represents the user's choice
type ConfirmChoice int

const (
	ConfirmChoiceNone ConfirmChoice = iota
	ConfirmChoiceYes
	ConfirmChoiceNo
	ConfirmChoiceAlways
)

// ConfirmDialogModel represents a confirmation dialog
type ConfirmDialogModel struct {
	visible     bool
	confirmType ConfirmationType
	title       string
	message     string
	detail      string
	toolName    string
	filePath    string
	command     string
	url         string
	oldContent  string
	newContent  string
	selected    int
	width       int
	height      int
	showDiff    bool
	diffScroll  int
	resultChan  chan ConfirmChoice
	onResult    func(ConfirmChoice)
}

// NewConfirmDialogModel creates a new confirmation dialog
func NewConfirmDialogModel() ConfirmDialogModel {
	return ConfirmDialogModel{
		visible:  false,
		selected: 0,
	}
}

// Show shows the confirmation dialog
func (c *ConfirmDialogModel) Show(opts ConfirmDialogOptions) {
	c.visible = true
	c.confirmType = opts.Type
	c.title = opts.Title
	c.message = opts.Message
	c.detail = opts.Detail
	c.toolName = opts.ToolName
	c.filePath = opts.FilePath
	c.command = opts.Command
	c.url = opts.URL
	c.oldContent = opts.OldContent
	c.newContent = opts.NewContent
	c.onResult = opts.OnResult
	c.selected = 0
	c.showDiff = false
	c.diffScroll = 0
}

// Hide hides the dialog
func (c *ConfirmDialogModel) Hide() {
	c.visible = false
}

// IsVisible returns visibility
func (c ConfirmDialogModel) IsVisible() bool {
	return c.visible
}

// SetSize sets the dialog dimensions
func (c *ConfirmDialogModel) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// ConfirmDialogOptions holds dialog options
type ConfirmDialogOptions struct {
	Type       ConfirmationType
	Title      string
	Message    string
	Detail     string
	ToolName   string
	FilePath   string
	Command    string
	URL        string
	OldContent string
	NewContent string
	OnResult   func(ConfirmChoice)
}

// Update handles input
func (c *ConfirmDialogModel) Update(msg tea.Msg) tea.Cmd {
	if !c.visible {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "h"))):
			if c.selected > 0 {
				c.selected--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("right", "l"))):
			if c.selected < 2 {
				c.selected++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			c.selected = (c.selected + 1) % 3
		case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
			c.selected = (c.selected + 2) % 3
		case key.Matches(msg, key.NewBinding(key.WithKeys("y", "Y"))):
			c.selectChoice(ConfirmChoiceYes)
			return nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("n", "N"))):
			c.selectChoice(ConfirmChoiceNo)
			return nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("a", "A"))):
			c.selectChoice(ConfirmChoiceAlways)
			return nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			switch c.selected {
			case 0:
				c.selectChoice(ConfirmChoiceYes)
			case 1:
				c.selectChoice(ConfirmChoiceNo)
			case 2:
				c.selectChoice(ConfirmChoiceAlways)
			}
			return nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("escape", "q"))):
			c.selectChoice(ConfirmChoiceNo)
			return nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
			// Toggle diff view
			if c.oldContent != "" || c.newContent != "" {
				c.showDiff = !c.showDiff
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if c.showDiff && c.diffScroll > 0 {
				c.diffScroll--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if c.showDiff {
				c.diffScroll++
			}
		}
	}

	return nil
}

// selectChoice handles choice selection
func (c *ConfirmDialogModel) selectChoice(choice ConfirmChoice) {
	c.visible = false
	if c.onResult != nil {
		c.onResult(choice)
	}
}

// View renders the dialog
func (c ConfirmDialogModel) View() string {
	if !c.visible {
		return ""
	}

	var b strings.Builder

	// Icon and title
	icon := c.getIcon()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(c.getTitleColor())
	b.WriteString(fmt.Sprintf("%s %s\n", icon, titleStyle.Render(c.title)))
	b.WriteString("\n")

	// Message
	if c.message != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(TextColor).Render(c.message))
		b.WriteString("\n\n")
	}

	// Details based on type
	switch c.confirmType {
	case ConfirmTypeFile:
		c.renderFileDetails(&b)
	case ConfirmTypeCommand:
		c.renderCommandDetails(&b)
	case ConfirmTypeNetwork:
		c.renderNetworkDetails(&b)
	case ConfirmTypeDangerous:
		c.renderDangerousDetails(&b)
	}

	// Detail section
	if c.detail != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(DimTextColor).Italic(true).Render(c.detail))
		b.WriteString("\n")
	}

	// Diff view
	if c.showDiff && (c.oldContent != "" || c.newContent != "") {
		b.WriteString("\n")
		b.WriteString(c.renderDiffView())
		b.WriteString("\n")
	}

	// Buttons
	b.WriteString("\n")
	b.WriteString(c.renderButtons())

	// Hints
	b.WriteString("\n\n")
	hints := []string{"y:Yes", "n:No", "a:Always"}
	if c.oldContent != "" || c.newContent != "" {
		hints = append(hints, "d:Diff")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(DimTextColor).Render(strings.Join(hints, "  ")))

	// Box it up
	dialogWidth := c.width - 20
	if dialogWidth < 50 {
		dialogWidth = 50
	}
	if dialogWidth > 80 {
		dialogWidth = 80
	}

	dialog := ConfirmDialogStyle.Width(dialogWidth).Render(b.String())

	return dialog
}

// getIcon returns the appropriate icon
func (c ConfirmDialogModel) getIcon() string {
	switch c.confirmType {
	case ConfirmTypeFile:
		return "📄"
	case ConfirmTypeCommand:
		return "💻"
	case ConfirmTypeNetwork:
		return "🌐"
	case ConfirmTypeDangerous:
		return "⚠️"
	default:
		return "❓"
	}
}

// getTitleColor returns the title color
func (c ConfirmDialogModel) getTitleColor() lipgloss.Color {
	switch c.confirmType {
	case ConfirmTypeDangerous:
		return DangerColor
	case ConfirmTypeCommand:
		return WarningColor
	default:
		return AccentColor
	}
}

// renderFileDetails renders file operation details
func (c ConfirmDialogModel) renderFileDetails(b *strings.Builder) {
	if c.toolName != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(DimTextColor).Render("Tool: "))
		b.WriteString(lipgloss.NewStyle().Foreground(AccentColor).Bold(true).Render(c.toolName))
		b.WriteString("\n")
	}
	if c.filePath != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(DimTextColor).Render("File: "))
		b.WriteString(lipgloss.NewStyle().Foreground(InfoColor).Render(c.filePath))
		b.WriteString("\n")
	}
}

// renderCommandDetails renders command details
func (c ConfirmDialogModel) renderCommandDetails(b *strings.Builder) {
	if c.command != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(DimTextColor).Render("Command:\n"))
		cmdStyle := lipgloss.NewStyle().
			Background(SurfaceColor).
			Foreground(WarningColor).
			Padding(0, 1)
		b.WriteString(cmdStyle.Render("$ " + c.command))
		b.WriteString("\n")
	}
}

// renderNetworkDetails renders network details
func (c ConfirmDialogModel) renderNetworkDetails(b *strings.Builder) {
	if c.url != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(DimTextColor).Render("URL: "))
		b.WriteString(lipgloss.NewStyle().Foreground(InfoColor).Underline(true).Render(c.url))
		b.WriteString("\n")
	}
}

// renderDangerousDetails renders dangerous operation details
func (c ConfirmDialogModel) renderDangerousDetails(b *strings.Builder) {
	warning := lipgloss.NewStyle().
		Foreground(DangerColor).
		Bold(true).
		Render("⚠️  This operation may be destructive!")
	b.WriteString(warning)
	b.WriteString("\n\n")
	c.renderFileDetails(b)
	c.renderCommandDetails(b)
}

// renderDiffView renders the diff view
func (c ConfirmDialogModel) renderDiffView() string {
	var b strings.Builder

	header := lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true).
		Render("─── Changes ───")
	b.WriteString(header)
	b.WriteString("\n")

	diffLines := computeDiff(c.oldContent, c.newContent)

	// Calculate visible range
	maxLines := 10
	startLine := c.diffScroll
	if startLine > len(diffLines)-maxLines {
		startLine = len(diffLines) - maxLines
	}
	if startLine < 0 {
		startLine = 0
	}
	endLine := startLine + maxLines
	if endLine > len(diffLines) {
		endLine = len(diffLines)
	}

	for i := startLine; i < endLine; i++ {
		line := diffLines[i]
		var style lipgloss.Style
		var prefix string

		switch line.Type {
		case DiffLineAdded:
			prefix = "+"
			style = lipgloss.NewStyle().Foreground(SuccessColor)
		case DiffLineRemoved:
			prefix = "-"
			style = lipgloss.NewStyle().Foreground(DangerColor)
		case DiffLineContext:
			prefix = " "
			style = lipgloss.NewStyle().Foreground(DimTextColor)
		case DiffLineHeader:
			prefix = ""
			style = lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
		}

		b.WriteString(style.Render(prefix + line.Content))
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(diffLines) > maxLines {
		b.WriteString(lipgloss.NewStyle().Foreground(DimTextColor).Render(
			fmt.Sprintf("─── %d/%d lines (↑↓ to scroll) ───", startLine+1, len(diffLines)),
		))
	}

	return b.String()
}

// renderButtons renders the action buttons
func (c ConfirmDialogModel) renderButtons() string {
	buttons := []string{"Yes", "No", "Always"}
	var rendered []string

	for i, btn := range buttons {
		var style lipgloss.Style
		if i == c.selected {
			// Selected button
			switch i {
			case 0: // Yes
				style = ConfirmButtonSelectedStyle
			case 1: // No
				style = CancelButtonSelectedStyle
			case 2: // Always
				style = AlwaysButtonSelectedStyle
			}
		} else {
			style = ButtonStyle
		}
		rendered = append(rendered, style.Render(btn))
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, rendered...)
}

// =============================================================================
// Confirm Dialog Styles
// =============================================================================

var (
	ConfirmDialogStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(AccentColor).
				Padding(1, 2).
				Background(BackgroundColor)

	ButtonStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(SurfaceColor).
			Padding(0, 2).
			MarginRight(1)

	ConfirmButtonSelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(SuccessColor).
					Padding(0, 2).
					MarginRight(1).
					Bold(true)

	CancelButtonSelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(DangerColor).
					Padding(0, 2).
					MarginRight(1).
					Bold(true)

	AlwaysButtonSelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(AccentColor).
					Padding(0, 2).
					MarginRight(1).
					Bold(true)
)
