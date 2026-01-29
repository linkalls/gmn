// Package tui provides a full-featured terminal user interface for gmn.
// Copyright 2025 linkalls
// SPDX-License-Identifier: Apache-2.0
package tui

import "github.com/charmbracelet/lipgloss"

// =============================================================================
// Codex/Gemini CLI Inspired Theme Colors
// =============================================================================

var (
	// Primary colors - Gemini-inspired gradient palette
	AccentColor  = lipgloss.Color("#8B5CF6") // Vibrant purple
	AccentColor2 = lipgloss.Color("#06B6D4") // Cyan for gradients
	SuccessColor = lipgloss.Color("#22C55E") // Bright green
	DangerColor  = lipgloss.Color("#EF4444") // Red
	WarningColor = lipgloss.Color("#FBBF24") // Amber
	InfoColor    = lipgloss.Color("#3B82F6") // Blue
	MagentaColor = lipgloss.Color("#EC4899") // Magenta for emphasis
	TealColor    = lipgloss.Color("#14B8A6") // Teal

	// Neutral colors - Codex-inspired dark theme
	TextColor       = lipgloss.Color("#F8FAFC") // Bright white text
	DimTextColor    = lipgloss.Color("#94A3B8") // Slate dim text
	MutedColor      = lipgloss.Color("#64748B") // Slate muted
	SurfaceColor    = lipgloss.Color("#1E293B") // Slate dark surface
	BackgroundColor = lipgloss.Color("#0F172A") // Slate darker background
	BorderColor     = lipgloss.Color("#334155") // Slate border
	HighlightColor  = lipgloss.Color("#475569") // Slate highlight

	// Special - Conversation colors
	UserColor   = lipgloss.Color("#22D3EE") // Cyan for user
	ModelColor  = lipgloss.Color("#A78BFA") // Light purple for model
	SystemColor = lipgloss.Color("#64748B") // Slate for system
	ThinkColor  = lipgloss.Color("#818CF8") // Indigo for thinking
)

// =============================================================================
// Base Styles - Enhanced with gradients and animations
// =============================================================================

var (
	// Container styles
	BaseContainerStyle = lipgloss.NewStyle().
				Padding(0, 1)

	BorderedContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(BorderColor)

	// Gradient border style (simulated with colors)
	GradientBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(AccentColor)

	// Text styles
	BoldStyle = lipgloss.NewStyle().Bold(true)

	DimStyle = lipgloss.NewStyle().
			Foreground(DimTextColor)

	MutedStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(DangerColor).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor)

	AccentStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	// Gradient text effect (using alternating colors)
	GradientTextStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true)
)

// =============================================================================
// Header Styles - Codex/Gemini inspired
// =============================================================================

var (
	HeaderStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(AccentColor).
			Padding(0, 1).
			Background(BackgroundColor)

	LogoStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	ModelBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(AccentColor).
			Padding(0, 1).
			Bold(true)

	YoloBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(WarningColor).
			Padding(0, 1).
			Bold(true)

	InfoBadgeStyle = lipgloss.NewStyle().
			Foreground(DimTextColor).
			Background(SurfaceColor).
			Padding(0, 1)

	// New: Status indicator badges
	OnlineBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(SuccessColor).
				Padding(0, 1).
				Bold(true)

	ProcessingBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(ThinkColor).
				Padding(0, 1).
				Bold(true)
)

// =============================================================================
// Sidebar Styles
// =============================================================================

var (
	SidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(BorderColor).
			Padding(0, 1).
			Background(BackgroundColor)

	SidebarTitleStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true).
				Padding(0, 0, 1, 0)

	SessionItemStyle = lipgloss.NewStyle().
				Foreground(TextColor).
				Padding(0, 1)

	SessionItemSelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(AccentColor).
					Padding(0, 1).
					Bold(true)

	SessionItemCurrentStyle = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Padding(0, 1)

	SessionInfoStyle = lipgloss.NewStyle().
				Foreground(DimTextColor).
				Padding(0, 1)
)

// =============================================================================
// Chat Styles
// =============================================================================

var (
	ChatContainerStyle = lipgloss.NewStyle().
				Padding(0, 1)

	UserMessageStyle = lipgloss.NewStyle().
				Foreground(UserColor).
				Bold(true)

	UserPromptStyle = lipgloss.NewStyle().
			Foreground(UserColor).
			Bold(true)

	ModelMessageStyle = lipgloss.NewStyle().
				Foreground(TextColor)

	ThinkingStyle = lipgloss.NewStyle().
			Foreground(DimTextColor).
			Italic(true)

	TimestampStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	CodeBlockStyle = lipgloss.NewStyle().
			Background(SurfaceColor).
			Padding(0, 1)
)

// =============================================================================
// Tool Styles
// =============================================================================

var (
	ToolCallStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	ToolNameStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	ToolResultStyle = lipgloss.NewStyle().
			Foreground(SuccessColor)

	ToolBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)

	ToolArgStyle = lipgloss.NewStyle().
			Foreground(DimTextColor)
)

// =============================================================================
// Input Styles
// =============================================================================

var (
	InputContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), true, false, false, false).
				BorderForeground(BorderColor).
				Padding(0, 1)

	InputPromptStyle = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Bold(true)

	InputPlaceholderStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	InputCursorStyle = lipgloss.NewStyle().
				Foreground(AccentColor)
)

// =============================================================================
// Status Bar Styles
// =============================================================================

var (
	StatusBarStyle = lipgloss.NewStyle().
			Background(SurfaceColor).
			Foreground(DimTextColor).
			Padding(0, 1)

	StatusKeyStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	StatusValueStyle = lipgloss.NewStyle().
				Foreground(DimTextColor)

	StatusDividerStyle = lipgloss.NewStyle().
				Foreground(BorderColor)
)

// =============================================================================
// Help Styles
// =============================================================================

var (
	HelpStyle = lipgloss.NewStyle().
			Foreground(DimTextColor)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(MutedColor)
)

// =============================================================================
// Spinner Styles
// =============================================================================

var (
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(AccentColor)

	SpinnerTextStyle = lipgloss.NewStyle().
				Foreground(DimTextColor)
)

// =============================================================================
// Scrollbar Styles
// =============================================================================

var (
	ScrollbarThumbStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	ScrollbarTrackStyle = lipgloss.NewStyle().
				Foreground(BorderColor)
)
