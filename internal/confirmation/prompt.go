// Package confirmation provides TUI-based confirmation prompts for destructive operations.
// Copyright 2025 linkalls
// SPDX-License-Identifier: Apache-2.0
package confirmation

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// YoloMode skips all confirmation prompts when true
var YoloMode bool = false

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
	TypeEdit  ConfirmationType = "edit"  // File edit confirmation with diff
	TypeExec  ConfirmationType = "exec"  // Command execution confirmation
	TypeMCP   ConfirmationType = "mcp"   // MCP tool confirmation
	TypeShell ConfirmationType = "shell" // Shell command confirmation
	TypeFetch ConfirmationType = "fetch" // Web fetch confirmation
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
	URL             string
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
	hasDiff     bool
}

func initialModel(details Details) model {
	m := model{
		details:     details,
		outcome:     OutcomeCancel,
		selectedBtn: 0,
		hasDiff:     false,
	}

	// Generate diff for edit confirmations
	if details.Type == TypeEdit && details.OriginalContent != "" && details.NewContent != "" {
		m.diff = generateDiffOpenCode(details.OriginalContent, details.NewContent)
		m.hasDiff = true
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
		case "y", "Y":
			m.outcome = OutcomeProceedOnce
			return m, tea.Quit
		case "enter":
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
			if m.ready && m.hasDiff {
				m.viewport, _ = m.viewport.Update(msg)
			}
		case "k", "up":
			if m.ready && m.hasDiff {
				m.viewport, _ = m.viewport.Update(msg)
			}
		case "ctrl+c":
			m.outcome = OutcomeCancel
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if m.hasDiff {
			headerHeight := 10
			footerHeight := 4

			if !m.ready {
				m.viewport = viewport.New(msg.Width-6, msg.Height-headerHeight-footerHeight)
				m.viewport.SetContent(m.diff)
				m.ready = true
			} else {
				m.viewport.Width = msg.Width - 6
				m.viewport.Height = msg.Height - headerHeight - footerHeight
			}
		} else {
			m.ready = true
		}
	}

	return m, nil
}

func (m model) View() string {
	return m.viewOpenCode()
}

// viewOpenCode renders the OpenCode-style TUI
func (m model) viewOpenCode() string {
	var b strings.Builder

	// Determine icon and color based on type
	var icon string
	var headerColor lipgloss.Color
	switch m.details.Type {
	case TypeEdit:
		icon = "📝"
		headerColor = accentColor
	case TypeShell:
		icon = "💻"
		headerColor = warningColor
	case TypeFetch:
		icon = "🌐"
		headerColor = lipgloss.Color("#3B82F6") // Blue
	case TypeExec:
		icon = "⚡"
		headerColor = warningColor
	default:
		icon = "🔐"
		headerColor = accentColor
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(headerColor).
		Bold(true).
		MarginBottom(1)

	header := headerStyle.Render(fmt.Sprintf("%s %s", icon, m.details.Title))
	b.WriteString(header)
	b.WriteString("\n\n")

	// Info rows based on type
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

	if m.details.URL != "" {
		b.WriteString(ocLabelStyle.Render("URL"))
		urlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Underline(true)
		b.WriteString(urlStyle.Render(m.details.URL))
		b.WriteString("\n")
	}

	if m.details.Command != "" {
		b.WriteString(ocLabelStyle.Render("Command"))
		cmdStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FCD34D")).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1)
		b.WriteString(cmdStyle.Render(m.details.Command))
		b.WriteString("\n")
	}

	// Show args for shell/fetch if available
	if m.details.Type == TypeShell || m.details.Type == TypeFetch || m.details.Type == TypeMCP {
		if len(m.details.Args) > 0 {
			b.WriteString("\n")
			argsJSON, _ := json.MarshalIndent(m.details.Args, "", "  ")
			argsBox := lipgloss.NewStyle().
				Foreground(dimTextColor).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderColor).
				Padding(0, 1).
				Render(string(argsJSON))
			b.WriteString(argsBox)
			b.WriteString("\n")
		}
	}

	// Diff viewport for edit confirmations
	if m.details.Type == TypeEdit && m.ready && m.hasDiff {
		b.WriteString("\n")
		diffHeader := ocDiffHeaderStyle.Render("─── Changes ───")
		b.WriteString(diffHeader)
		b.WriteString("\n")
		b.WriteString(ocDiffBoxStyle.Render(m.viewport.View()))
		b.WriteString("\n")
	}

	// Buttons
	b.WriteString("\n")

	yesBtn := ocButtonStyle.Render(" [Y]es ")
	noBtn := ocButtonStyle.Render(" [N]o ")
	alwaysBtn := ocButtonStyle.Render(" [A]lways ")

	if m.selectedBtn == 0 {
		yesBtn = ocButtonActiveStyle.Render(" [Y]es ")
	} else if m.selectedBtn == 1 {
		noBtn = ocButtonActiveStyle.Render(" [N]o ")
	} else {
		alwaysBtn = ocButtonActiveStyle.Render(" [A]lways ")
	}

	b.WriteString(yesBtn)
	b.WriteString(" ")
	b.WriteString(noBtn)
	b.WriteString(" ")
	b.WriteString(alwaysBtn)
	b.WriteString("\n")

	// Help text
	help := ocHelpStyle.Render("y/n/a • ←/→ select • enter confirm • esc cancel")
	b.WriteString(help)

	// Wrap in container
	return ocContainerStyle.Render(b.String())
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

// PromptConfirmation shows an interactive confirmation prompt using TUI
// If YoloMode is enabled, it automatically approves all operations
func PromptConfirmation(details Details) (Outcome, error) {
	// YOLO mode - skip all confirmations
	if YoloMode {
		return OutcomeProceedOnce, nil
	}

	m := initialModel(details)

	// Use alt screen only for diff views to avoid flickering for simple prompts
	var opts []tea.ProgramOption
	if details.Type == TypeEdit && details.OriginalContent != "" && details.NewContent != "" {
		opts = append(opts, tea.WithAltScreen())
	}

	p := tea.NewProgram(m, opts...)
	finalModel, err := p.Run()
	if err != nil {
		return OutcomeCancel, err
	}

	return finalModel.(model).outcome, nil
}
