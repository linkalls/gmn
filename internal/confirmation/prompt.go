// Package confirmation provides TUI-based confirmation prompts for destructive operations.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package confirmation

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// Theme represents the TUI theme style
type Theme string

const (
	ThemeMinimal  Theme = "minimal"  // Simple, clean design
	ThemeOpenCode Theme = "opencode" // OpenCode-inspired modern design
)

// CurrentTheme is the active theme (can be set globally)
var CurrentTheme Theme = ThemeOpenCode

// Outcome represents the result of a confirmation prompt
type Outcome string

const (
	OutcomeProceedOnce   Outcome = "proceed_once"   // Execute this time only
	OutcomeProceedAlways Outcome = "proceed_always" // Always allow this tool (session)
	OutcomeCancel        Outcome = "cancel"         // Cancel the operation
)

// ConfirmationType represents the type of confirmation
type ConfirmationType string

const (
	TypeEdit ConfirmationType = "edit" // File edit confirmation with diff
	TypeExec ConfirmationType = "exec" // Command execution confirmation
	TypeMCP  ConfirmationType = "mcp"  // MCP tool confirmation
)

// Details contains information for the confirmation prompt
type Details struct {
	Type            ConfirmationType
	Title           string
	ToolName        string
	FilePath        string
	OriginalContent string
	NewContent      string
	Command         string
	Args            map[string]interface{}
}

// AllowList tracks tools that have been allowed for the session
type AllowList struct {
	allowedTools map[string]bool
}

// NewAllowList creates a new allow list
func NewAllowList() *AllowList {
	return &AllowList{
		allowedTools: make(map[string]bool),
	}
}

// IsAllowed checks if a tool is in the allow list
func (a *AllowList) IsAllowed(toolName string) bool {
	return a.allowedTools[toolName]
}

// Allow adds a tool to the allow list
func (a *AllowList) Allow(toolName string) {
	a.allowedTools[toolName] = true
}

// =============================================================================
// OpenCode Theme Styles (Modern, sleek design)
// =============================================================================

var (
	// Colors
	accentColor  = lipgloss.Color("#7C3AED") // Purple
	successColor = lipgloss.Color("#10B981") // Green
	dangerColor  = lipgloss.Color("#EF4444") // Red
	warningColor = lipgloss.Color("#F59E0B") // Orange
	mutedColor   = lipgloss.Color("#6B7280") // Gray
	surfaceColor = lipgloss.Color("#1F2937") // Dark surface
	borderColor  = lipgloss.Color("#374151") // Border
	textColor    = lipgloss.Color("#F9FAFB") // Light text
	dimTextColor = lipgloss.Color("#9CA3AF") // Dim text

	// OpenCode styles
	ocContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderColor).
				Padding(1, 2).
				MarginTop(1).
				MarginBottom(1)

	ocHeaderStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			MarginBottom(1)

	ocTitleStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Bold(true)

	ocLabelStyle = lipgloss.NewStyle().
			Foreground(dimTextColor).
			Width(10)

	ocValueStyle = lipgloss.NewStyle().
			Foreground(textColor)

	ocDiffBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	ocDiffHeaderStyle = lipgloss.NewStyle().
				Foreground(dimTextColor).
				Bold(true).
				MarginBottom(1)

	ocAddedStyle = lipgloss.NewStyle().
			Foreground(successColor)

	ocRemovedStyle = lipgloss.NewStyle().
			Foreground(dangerColor)

	ocContextStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)

	ocButtonStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(surfaceColor).
			Padding(0, 2).
			MarginRight(1)

	ocButtonActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(accentColor).
				Bold(true).
				Padding(0, 2).
				MarginRight(1)

	ocHelpStyle = lipgloss.NewStyle().
			Foreground(dimTextColor).
			MarginTop(1)

	ocStatusBarStyle = lipgloss.NewStyle().
				Foreground(dimTextColor).
				Background(surfaceColor).
				Padding(0, 1)
)

// =============================================================================
// Minimal Theme Styles (Simple, clean design)
// =============================================================================

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	addedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	removedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
)

// =============================================================================
// Model
// =============================================================================

// model is the bubbletea model for the confirmation prompt
type model struct {
	details     Details
	outcome     Outcome
	viewport    viewport.Model
	ready       bool
	diff        string
	width       int
	height      int
	selectedBtn int // 0: Yes, 1: No, 2: Always
}

func initialModel(details Details) model {
	m := model{
		details:     details,
		outcome:     OutcomeCancel,
		selectedBtn: 0,
	}

	// Generate diff for edit confirmations
	if details.Type == TypeEdit {
		if CurrentTheme == ThemeOpenCode {
			m.diff = generateDiffOpenCode(details.OriginalContent, details.NewContent)
		} else {
			m.diff = generateDiff(details.OriginalContent, details.NewContent)
		}
	}

	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y", "enter":
			if m.selectedBtn == 0 {
				m.outcome = OutcomeProceedOnce
			} else if m.selectedBtn == 1 {
				m.outcome = OutcomeCancel
			} else {
				m.outcome = OutcomeProceedAlways
			}
			return m, tea.Quit
		case "n", "N":
			m.outcome = OutcomeCancel
			return m, tea.Quit
		case "a", "A":
			m.outcome = OutcomeProceedAlways
			return m, tea.Quit
		case "q", "esc":
			m.outcome = OutcomeCancel
			return m, tea.Quit
		case "tab", "right", "l":
			m.selectedBtn = (m.selectedBtn + 1) % 3
		case "shift+tab", "left", "h":
			m.selectedBtn = (m.selectedBtn + 2) % 3
		case "j", "down":
			if m.ready {
				m.viewport, _ = m.viewport.Update(msg)
			}
		case "k", "up":
			if m.ready {
				m.viewport, _ = m.viewport.Update(msg)
			}
		case "ctrl+c":
			m.outcome = OutcomeCancel
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 8
		footerHeight := 4

		if !m.ready {
			m.viewport = viewport.New(msg.Width-6, msg.Height-headerHeight-footerHeight)
			m.viewport.SetContent(m.diff)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 6
			m.viewport.Height = msg.Height - headerHeight - footerHeight
		}
	}

	return m, nil
}

func (m model) View() string {
	if CurrentTheme == ThemeOpenCode {
		return m.viewOpenCode()
	}
	return m.viewMinimal()
}

// viewOpenCode renders the OpenCode-style TUI
func (m model) viewOpenCode() string {
	var b strings.Builder

	// Header with icon
	icon := "🔐"
	if m.details.Type == TypeEdit {
		icon = "📝"
	} else if m.details.Type == TypeExec {
		icon = "⚡"
	}

	header := ocHeaderStyle.Render(fmt.Sprintf("%s CONFIRMATION REQUIRED", icon))
	b.WriteString(header)
	b.WriteString("\n\n")

	// Title
	title := ocTitleStyle.Render(m.details.Title)
	b.WriteString(title)
	b.WriteString("\n\n")

	// Info rows
	if m.details.ToolName != "" {
		b.WriteString(ocLabelStyle.Render("Tool"))
		b.WriteString(ocValueStyle.Render(m.details.ToolName))
		b.WriteString("\n")
	}

	if m.details.FilePath != "" {
		b.WriteString(ocLabelStyle.Render("File"))
		b.WriteString(ocValueStyle.Render(m.details.FilePath))
		b.WriteString("\n")
	}

	if m.details.Command != "" {
		b.WriteString(ocLabelStyle.Render("Command"))
		b.WriteString(ocValueStyle.Render(m.details.Command))
		b.WriteString("\n")
	}

	// Diff viewport for edit confirmations
	if m.details.Type == TypeEdit && m.ready {
		b.WriteString("\n")
		diffHeader := ocDiffHeaderStyle.Render("─── Changes ───")
		b.WriteString(diffHeader)
		b.WriteString("\n")
		b.WriteString(ocDiffBoxStyle.Render(m.viewport.View()))
		b.WriteString("\n")
	}

	// Buttons
	b.WriteString("\n")

	yesBtn := ocButtonStyle.Render(" Yes ")
	noBtn := ocButtonStyle.Render(" No ")
	alwaysBtn := ocButtonStyle.Render(" Always ")

	if m.selectedBtn == 0 {
		yesBtn = ocButtonActiveStyle.Render(" Yes ")
	} else if m.selectedBtn == 1 {
		noBtn = ocButtonActiveStyle.Render(" No ")
	} else {
		alwaysBtn = ocButtonActiveStyle.Render(" Always ")
	}

	b.WriteString(yesBtn)
	b.WriteString(noBtn)
	b.WriteString(alwaysBtn)
	b.WriteString("\n")

	// Help text
	help := ocHelpStyle.Render("y/n/a • ←/→ select • enter confirm • esc cancel")
	b.WriteString(help)

	// Wrap in container
	return ocContainerStyle.Render(b.String())
}

// viewMinimal renders the minimal-style TUI
func (m model) viewMinimal() string {
	var b strings.Builder

	// Title
	title := titleStyle.Render(fmt.Sprintf("🔒 %s", m.details.Title))
	b.WriteString(title)
	b.WriteString("\n")

	// Tool info
	if m.details.ToolName != "" {
		b.WriteString(infoStyle.Render(fmt.Sprintf("Tool: %s", m.details.ToolName)))
		b.WriteString("\n")
	}

	if m.details.FilePath != "" {
		b.WriteString(infoStyle.Render(fmt.Sprintf("File: %s", m.details.FilePath)))
		b.WriteString("\n")
	}

	if m.details.Command != "" {
		b.WriteString(infoStyle.Render(fmt.Sprintf("Command: %s", m.details.Command)))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Diff viewport for edit confirmations
	if m.details.Type == TypeEdit && m.ready {
		b.WriteString(boxStyle.Render(m.viewport.View()))
		b.WriteString("\n")
	}

	// Help text
	help := helpStyle.Render("[Y]es / [n]o / [a]lways allow this tool")
	b.WriteString(help)

	return b.String()
}

// generateDiffOpenCode creates a styled diff for OpenCode theme
func generateDiffOpenCode(original, new string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(original, new, true)
	diffs = dmp.DiffCleanupSemantic(diffs)

	var b strings.Builder
	lineNum := 1

	for _, diff := range diffs {
		lines := strings.Split(diff.Text, "\n")
		for i, line := range lines {
			if line == "" && i == len(lines)-1 {
				continue
			}

			lineNumStr := fmt.Sprintf("%4d ", lineNum)

			switch diff.Type {
			case diffmatchpatch.DiffInsert:
				b.WriteString(ocAddedStyle.Render(fmt.Sprintf("%s+ %s", lineNumStr, line)))
			case diffmatchpatch.DiffDelete:
				b.WriteString(ocRemovedStyle.Render(fmt.Sprintf("%s- %s", lineNumStr, line)))
			case diffmatchpatch.DiffEqual:
				b.WriteString(ocContextStyle.Render(fmt.Sprintf("%s  %s", lineNumStr, line)))
			}

			if i < len(lines)-1 {
				b.WriteString("\n")
				if diff.Type != diffmatchpatch.DiffDelete {
					lineNum++
				}
			}
		}
	}
	return b.String()
}

// generateDiff creates a colored diff between two strings (minimal theme)
func generateDiff(original, new string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(original, new, true)

	var b strings.Builder
	for _, diff := range diffs {
		lines := strings.Split(diff.Text, "\n")
		for i, line := range lines {
			if line == "" && i == len(lines)-1 {
				continue
			}
			switch diff.Type {
			case diffmatchpatch.DiffInsert:
				b.WriteString(addedStyle.Render("+ " + line))
			case diffmatchpatch.DiffDelete:
				b.WriteString(removedStyle.Render("- " + line))
			case diffmatchpatch.DiffEqual:
				b.WriteString("  " + line)
			}
			if i < len(lines)-1 {
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

// PromptConfirmation shows an interactive confirmation prompt
func PromptConfirmation(details Details) (Outcome, error) {
	m := initialModel(details)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return OutcomeCancel, err
	}

	return finalModel.(model).outcome, nil
}

// PromptConfirmationSimple shows a simple Y/n prompt without TUI (for non-TTY)
func PromptConfirmationSimple(details Details) (Outcome, error) {
	fmt.Printf("\n🔒 %s\n", details.Title)

	if details.ToolName != "" {
		fmt.Printf("   Tool: %s\n", details.ToolName)
	}
	if details.FilePath != "" {
		fmt.Printf("   File: %s\n", details.FilePath)
	}
	if details.Command != "" {
		fmt.Printf("   Command: %s\n", details.Command)
	}

	fmt.Print("\nProceed? [Y]es / [n]o / [a]lways: ")

	var input string
	fmt.Scanln(&input)
	input = strings.ToLower(strings.TrimSpace(input))

	switch input {
	case "y", "yes", "":
		return OutcomeProceedOnce, nil
	case "a", "always":
		return OutcomeProceedAlways, nil
	default:
		return OutcomeCancel, nil
	}
}
