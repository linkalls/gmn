// Package tui provides a full-featured terminal user interface for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ContextItem represents an item in the context
type ContextItem struct {
	Type      ContextType
	Path      string
	Name      string
	Size      int64
	LineCount int
	AddedAt   time.Time
}

// ContextType represents the type of context item
type ContextType int

const (
	ContextTypeFile ContextType = iota
	ContextTypeDirectory
	ContextTypeURL
	ContextTypeClipboard
)

// ActivityItem represents an activity in the feed
type ActivityItem struct {
	Type      ActivityType
	Title     string
	Detail    string
	Status    ActivityStatus
	Timestamp time.Time
	Duration  time.Duration
}

// ActivityType represents the type of activity
type ActivityType int

const (
	ActivityTypeAPI ActivityType = iota
	ActivityTypeTool
	ActivityTypeFile
	ActivityTypeSearch
	ActivityTypeShell
	ActivityTypeThinking
)

// ActivityStatus represents the status of an activity
type ActivityStatus int

const (
	ActivityStatusPending ActivityStatus = iota
	ActivityStatusRunning
	ActivityStatusSuccess
	ActivityStatusError
)

// ContextPanelModel represents the context/activity panel
type ContextPanelModel struct {
	width          int
	height         int
	contextItems   []ContextItem
	activities     []ActivityItem
	maxActivities  int
	showContext    bool
	showActivities bool
	focused        bool
}

// NewContextPanelModel creates a new context panel model
func NewContextPanelModel() ContextPanelModel {
	return ContextPanelModel{
		contextItems:   []ContextItem{},
		activities:     []ActivityItem{},
		maxActivities:  10,
		showContext:    true,
		showActivities: true,
	}
}

// SetSize sets the panel dimensions
func (c *ContextPanelModel) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// SetFocused sets the focus state
func (c *ContextPanelModel) SetFocused(focused bool) {
	c.focused = focused
}

// AddContextItem adds a context item
func (c *ContextPanelModel) AddContextItem(item ContextItem) {
	item.AddedAt = time.Now()
	c.contextItems = append(c.contextItems, item)
}

// RemoveContextItem removes a context item by path
func (c *ContextPanelModel) RemoveContextItem(path string) {
	for i, item := range c.contextItems {
		if item.Path == path {
			c.contextItems = append(c.contextItems[:i], c.contextItems[i+1:]...)
			break
		}
	}
}

// ClearContext clears all context items
func (c *ContextPanelModel) ClearContext() {
	c.contextItems = []ContextItem{}
}

// AddActivity adds an activity item
func (c *ContextPanelModel) AddActivity(activity ActivityItem) {
	activity.Timestamp = time.Now()
	c.activities = append([]ActivityItem{activity}, c.activities...)

	// Trim to max activities
	if len(c.activities) > c.maxActivities {
		c.activities = c.activities[:c.maxActivities]
	}
}

// UpdateLastActivity updates the most recent activity
func (c *ContextPanelModel) UpdateLastActivity(status ActivityStatus, duration time.Duration) {
	if len(c.activities) > 0 {
		c.activities[0].Status = status
		c.activities[0].Duration = duration
	}
}

// ToggleContext toggles context display
func (c *ContextPanelModel) ToggleContext() {
	c.showContext = !c.showContext
}

// ToggleActivities toggles activities display
func (c *ContextPanelModel) ToggleActivities() {
	c.showActivities = !c.showActivities
}

// View renders the context panel
func (c ContextPanelModel) View() string {
	var sections []string

	// Context section
	if c.showContext {
		sections = append(sections, c.renderContext())
	}

	// Activities section
	if c.showActivities {
		sections = append(sections, c.renderActivities())
	}

	content := strings.Join(sections, "\n")

	style := ContextPanelStyle.Width(c.width).Height(c.height)
	if c.focused {
		style = style.BorderForeground(AccentColor)
	}

	return style.Render(content)
}

// renderContext renders the context section
func (c ContextPanelModel) renderContext() string {
	var b strings.Builder

	// Title
	title := lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true).
		Render("📂 Context")
	b.WriteString(title)
	b.WriteString("\n")

	if len(c.contextItems) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(DimTextColor).Render("  No files in context"))
	} else {
		for _, item := range c.contextItems {
			b.WriteString(c.renderContextItem(item))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderContextItem renders a single context item
func (c ContextPanelModel) renderContextItem(item ContextItem) string {
	var icon string
	var style lipgloss.Style

	switch item.Type {
	case ContextTypeFile:
		icon = c.getFileIcon(item.Path)
		style = lipgloss.NewStyle().Foreground(InfoColor)
	case ContextTypeDirectory:
		icon = "📁"
		style = lipgloss.NewStyle().Foreground(WarningColor)
	case ContextTypeURL:
		icon = "🌐"
		style = lipgloss.NewStyle().Foreground(AccentColor)
	case ContextTypeClipboard:
		icon = "📋"
		style = lipgloss.NewStyle().Foreground(SuccessColor)
	}

	name := item.Name
	if name == "" {
		name = filepath.Base(item.Path)
	}

	// Truncate if needed
	maxLen := c.width - 10
	if maxLen < 10 {
		maxLen = 10
	}
	if len(name) > maxLen {
		name = name[:maxLen-3] + "..."
	}

	// Info line
	info := ""
	if item.LineCount > 0 {
		info = fmt.Sprintf(" (%d lines)", item.LineCount)
	} else if item.Size > 0 {
		info = fmt.Sprintf(" (%s)", formatSize(item.Size))
	}

	return fmt.Sprintf("  %s %s%s",
		icon,
		style.Render(name),
		lipgloss.NewStyle().Foreground(DimTextColor).Render(info),
	)
}

// renderActivities renders the activities section
func (c ContextPanelModel) renderActivities() string {
	var b strings.Builder

	// Title with divider
	divider := lipgloss.NewStyle().Foreground(BorderColor).Render(strings.Repeat("─", c.width-4))
	b.WriteString(divider)
	b.WriteString("\n")

	title := lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true).
		Render("⚡ Activity")
	b.WriteString(title)
	b.WriteString("\n")

	if len(c.activities) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(DimTextColor).Render("  No recent activity"))
	} else {
		// Calculate how many activities we can show
		availableHeight := c.height - 6 // Account for headers and padding
		maxShow := availableHeight / 2
		if maxShow < 1 {
			maxShow = 1
		}
		if maxShow > len(c.activities) {
			maxShow = len(c.activities)
		}

		for i := 0; i < maxShow; i++ {
			b.WriteString(c.renderActivityItem(c.activities[i]))
			b.WriteString("\n")
		}

		// Show count if more
		if len(c.activities) > maxShow {
			more := lipgloss.NewStyle().Foreground(DimTextColor).Render(
				fmt.Sprintf("  ... and %d more", len(c.activities)-maxShow),
			)
			b.WriteString(more)
		}
	}

	return b.String()
}

// renderActivityItem renders a single activity item
func (c ContextPanelModel) renderActivityItem(item ActivityItem) string {
	var icon string
	var statusIcon string
	var style lipgloss.Style

	// Activity type icon
	switch item.Type {
	case ActivityTypeAPI:
		icon = "🔮"
	case ActivityTypeTool:
		icon = "⚙️"
	case ActivityTypeFile:
		icon = "📄"
	case ActivityTypeSearch:
		icon = "🔍"
	case ActivityTypeShell:
		icon = "💻"
	case ActivityTypeThinking:
		icon = "💭"
	}

	// Status icon and style
	switch item.Status {
	case ActivityStatusPending:
		statusIcon = "○"
		style = lipgloss.NewStyle().Foreground(DimTextColor)
	case ActivityStatusRunning:
		statusIcon = "◐"
		style = lipgloss.NewStyle().Foreground(AccentColor)
	case ActivityStatusSuccess:
		statusIcon = "✓"
		style = lipgloss.NewStyle().Foreground(SuccessColor)
	case ActivityStatusError:
		statusIcon = "✗"
		style = lipgloss.NewStyle().Foreground(DangerColor)
	}

	// Title
	title := item.Title
	maxLen := c.width - 15
	if maxLen < 10 {
		maxLen = 10
	}
	if len(title) > maxLen {
		title = title[:maxLen-3] + "..."
	}

	// Duration
	duration := ""
	if item.Duration > 0 {
		duration = lipgloss.NewStyle().Foreground(DimTextColor).Render(
			fmt.Sprintf(" %s", item.Duration.Round(time.Millisecond*100)),
		)
	}

	return fmt.Sprintf("  %s %s %s%s",
		icon,
		style.Render(statusIcon),
		style.Render(title),
		duration,
	)
}

// getFileIcon returns an icon for a file based on extension
func (c ContextPanelModel) getFileIcon(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "🔵"
	case ".py":
		return "🐍"
	case ".js", ".ts":
		return "🟨"
	case ".rs":
		return "🦀"
	case ".md":
		return "📝"
	case ".json", ".yaml", ".yml":
		return "📋"
	case ".html", ".css":
		return "🌐"
	case ".sh", ".bash":
		return "💻"
	default:
		return "📄"
	}
}

// formatSize formats a file size
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// =============================================================================
// Context Panel Style
// =============================================================================

var ContextPanelStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder(), false, false, false, true).
	BorderForeground(BorderColor).
	Padding(0, 1)
