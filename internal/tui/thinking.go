// Package tui provides a full-featured terminal user interface for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ThinkingStep represents a step in the thinking process
type ThinkingStep struct {
	Label     string
	Status    StepStatus
	StartTime time.Time
	Duration  time.Duration
}

// StepStatus represents the status of a thinking step
type StepStatus int

const (
	StepPending StepStatus = iota
	StepActive
	StepCompleted
	StepFailed
)

// ThinkingModel represents the thinking/processing indicator
type ThinkingModel struct {
	spinner    spinner.Model
	steps      []ThinkingStep
	active     bool
	width      int
	startTime  time.Time
	message    string
	showSteps  bool
	frameCount int
}

// NewThinkingModel creates a new thinking model
func NewThinkingModel() ThinkingModel {
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(AccentColor)

	return ThinkingModel{
		spinner:   s,
		steps:     []ThinkingStep{},
		showSteps: true,
	}
}

// Start starts the thinking indicator
func (t *ThinkingModel) Start(message string) tea.Cmd {
	t.active = true
	t.message = message
	t.startTime = time.Now()
	t.steps = []ThinkingStep{}
	return t.spinner.Tick
}

// Stop stops the thinking indicator
func (t *ThinkingModel) Stop() {
	t.active = false
}

// SetMessage sets the thinking message
func (t *ThinkingModel) SetMessage(message string) {
	t.message = message
}

// AddStep adds a thinking step
func (t *ThinkingModel) AddStep(label string) {
	// Complete any active step
	for i := range t.steps {
		if t.steps[i].Status == StepActive {
			t.steps[i].Status = StepCompleted
			t.steps[i].Duration = time.Since(t.steps[i].StartTime)
		}
	}

	// Add new step
	t.steps = append(t.steps, ThinkingStep{
		Label:     label,
		Status:    StepActive,
		StartTime: time.Now(),
	})
}

// CompleteStep completes the current step
func (t *ThinkingModel) CompleteStep() {
	for i := range t.steps {
		if t.steps[i].Status == StepActive {
			t.steps[i].Status = StepCompleted
			t.steps[i].Duration = time.Since(t.steps[i].StartTime)
			break
		}
	}
}

// FailStep marks the current step as failed
func (t *ThinkingModel) FailStep() {
	for i := range t.steps {
		if t.steps[i].Status == StepActive {
			t.steps[i].Status = StepFailed
			t.steps[i].Duration = time.Since(t.steps[i].StartTime)
			break
		}
	}
}

// SetWidth sets the width
func (t *ThinkingModel) SetWidth(width int) {
	t.width = width
}

// Update updates the thinking model
func (t *ThinkingModel) Update(msg tea.Msg) tea.Cmd {
	if !t.active {
		return nil
	}

	var cmd tea.Cmd
	t.spinner, cmd = t.spinner.Update(msg)
	t.frameCount++
	return cmd
}

// View renders the thinking indicator
func (t ThinkingModel) View() string {
	if !t.active {
		return ""
	}

	var b strings.Builder

	// Animated border
	borderChars := []string{"◐", "◓", "◑", "◒"}
	borderChar := borderChars[t.frameCount%len(borderChars)]

	// Header with spinner
	elapsed := time.Since(t.startTime).Round(time.Millisecond * 100)
	header := fmt.Sprintf("%s %s %s %s",
		lipgloss.NewStyle().Foreground(AccentColor).Render(borderChar),
		t.spinner.View(),
		lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render(t.message),
		lipgloss.NewStyle().Foreground(DimTextColor).Render(fmt.Sprintf("(%s)", elapsed)),
	)
	b.WriteString(header)
	b.WriteString("\n")

	// Progress bar animation
	progressWidth := 30
	if t.width > 50 {
		progressWidth = t.width / 3
	}
	progress := t.renderProgressBar(progressWidth)
	b.WriteString("  " + progress)
	b.WriteString("\n")

	// Steps
	if t.showSteps && len(t.steps) > 0 {
		for _, step := range t.steps {
			b.WriteString(t.renderStep(step))
			b.WriteString("\n")
		}
	}

	return ThinkingBoxStyle.Width(t.width - 4).Render(b.String())
}

// renderProgressBar renders an animated progress bar
func (t ThinkingModel) renderProgressBar(width int) string {
	// Animated scanning effect
	scanPos := t.frameCount % (width * 2)
	if scanPos > width {
		scanPos = width*2 - scanPos
	}

	var bar strings.Builder
	for i := 0; i < width; i++ {
		distance := abs(i - scanPos)
		if distance == 0 {
			bar.WriteString(lipgloss.NewStyle().Foreground(AccentColor).Render("█"))
		} else if distance == 1 {
			bar.WriteString(lipgloss.NewStyle().Foreground(AccentColor).Render("▓"))
		} else if distance == 2 {
			bar.WriteString(lipgloss.NewStyle().Foreground(HighlightColor).Render("▒"))
		} else if distance == 3 {
			bar.WriteString(lipgloss.NewStyle().Foreground(BorderColor).Render("░"))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(SurfaceColor).Render("░"))
		}
	}

	return bar.String()
}

// renderStep renders a single step
func (t ThinkingModel) renderStep(step ThinkingStep) string {
	var icon string
	var style lipgloss.Style

	switch step.Status {
	case StepPending:
		icon = "○"
		style = lipgloss.NewStyle().Foreground(MutedColor)
	case StepActive:
		// Animated icon for active step
		icons := []string{"◐", "◓", "◑", "◒"}
		icon = icons[t.frameCount%len(icons)]
		style = lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
	case StepCompleted:
		icon = "✓"
		style = lipgloss.NewStyle().Foreground(SuccessColor)
	case StepFailed:
		icon = "✗"
		style = lipgloss.NewStyle().Foreground(DangerColor)
	}

	label := style.Render(fmt.Sprintf("  %s %s", icon, step.Label))

	// Show duration for completed/failed steps
	if step.Status == StepCompleted || step.Status == StepFailed {
		duration := lipgloss.NewStyle().Foreground(DimTextColor).Render(
			fmt.Sprintf(" (%s)", step.Duration.Round(time.Millisecond*100)),
		)
		label += duration
	}

	return label
}

// IsActive returns whether the thinking indicator is active
func (t ThinkingModel) IsActive() bool {
	return t.active
}

// abs returns absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// =============================================================================
// Thinking Box Style
// =============================================================================

var ThinkingBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(AccentColor).
	Padding(0, 1).
	MarginTop(1).
	MarginBottom(1)
