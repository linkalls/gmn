// Package tui provides a full-featured terminal user interface for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HeaderModel represents the header component
type HeaderModel struct {
	width     int
	modelName string
	yoloMode  bool
	cwd       string
}

// NewHeaderModel creates a new header model
func NewHeaderModel(modelName string, yoloMode bool, cwd string) HeaderModel {
	return HeaderModel{
		modelName: modelName,
		yoloMode:  yoloMode,
		cwd:       cwd,
	}
}

// SetWidth sets the width of the header
func (h *HeaderModel) SetWidth(width int) {
	h.width = width
}

// SetModel sets the model name
func (h *HeaderModel) SetModel(modelName string) {
	h.modelName = modelName
}

// View renders the header
func (h HeaderModel) View() string {
	// Logo with gradient effect (simulated)
	logo := lipgloss.NewStyle().
		Foreground(AccentColor).
		Bold(true).
		Render("✨ gmn")

	// Subtitle
	subtitle := lipgloss.NewStyle().
		Foreground(DimTextColor).
		Render("Gemini CLI")

	// Model badge with icon
	modelIcon := "🤖"
	modelBadge := ModelBadgeStyle.Render(modelIcon + " " + h.modelName)

	// Build badges
	var badges []string
	badges = append(badges, modelBadge)

	if h.yoloMode {
		yoloBadge := YoloBadgeStyle.Render("⚡ YOLO")
		badges = append(badges, yoloBadge)
	}

	// Status badge
	statusBadge := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(SuccessColor).
		Padding(0, 1).
		Bold(true).
		Render("● Ready")
	badges = append(badges, statusBadge)

	// CWD badge with folder icon
	cwdBadge := InfoBadgeStyle.Render("📁 " + truncatePath(h.cwd, 40))

	// Build header line with better spacing
	headerLine := fmt.Sprintf("%s %s  %s", logo, subtitle, strings.Join(badges, " "))

	// Combine
	content := headerLine + "\n" + cwdBadge

	return HeaderStyle.Width(h.width).Render(content)
}

// truncatePath truncates a path for display
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// =============================================================================
// Sidebar Component
// =============================================================================

// SessionInfo represents a session in the sidebar
type SessionInfo struct {
	ID        string
	Name      string
	Messages  int
	UpdatedAt string
	IsCurrent bool
}

// SidebarModel represents the sidebar component
type SidebarModel struct {
	sessions     []SessionInfo
	selected     int
	height       int
	width        int
	focused      bool
	scrollOffset int
}

// NewSidebarModel creates a new sidebar model
func NewSidebarModel() SidebarModel {
	return SidebarModel{
		sessions: []SessionInfo{},
		selected: 0,
		width:    25,
	}
}

// SetSessions sets the session list
func (s *SidebarModel) SetSessions(sessions []SessionInfo) {
	s.sessions = sessions
}

// SetSize sets the sidebar dimensions
func (s *SidebarModel) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetFocused sets focus state
func (s *SidebarModel) SetFocused(focused bool) {
	s.focused = focused
}

// SelectedSession returns the currently selected session
func (s *SidebarModel) SelectedSession() *SessionInfo {
	if len(s.sessions) == 0 || s.selected >= len(s.sessions) {
		return nil
	}
	return &s.sessions[s.selected]
}

// MoveUp moves selection up
func (s *SidebarModel) MoveUp() {
	if s.selected > 0 {
		s.selected--
		s.ensureVisible()
	}
}

// MoveDown moves selection down
func (s *SidebarModel) MoveDown() {
	if s.selected < len(s.sessions)-1 {
		s.selected++
		s.ensureVisible()
	}
}

// ensureVisible ensures the selected item is visible
func (s *SidebarModel) ensureVisible() {
	visibleItems := s.height - 4 // Account for title and padding
	if visibleItems < 1 {
		visibleItems = 1
	}

	if s.selected < s.scrollOffset {
		s.scrollOffset = s.selected
	}
	if s.selected >= s.scrollOffset+visibleItems {
		s.scrollOffset = s.selected - visibleItems + 1
	}
}

// View renders the sidebar
func (s SidebarModel) View() string {
	var b strings.Builder

	// Title
	title := SidebarTitleStyle.Render("📋 Sessions")
	b.WriteString(title)
	b.WriteString("\n")

	if len(s.sessions) == 0 {
		emptyMsg := DimStyle.Render("No sessions")
		b.WriteString(emptyMsg)
	} else {
		visibleItems := s.height - 4
		if visibleItems < 1 {
			visibleItems = 1
		}

		endIdx := s.scrollOffset + visibleItems
		if endIdx > len(s.sessions) {
			endIdx = len(s.sessions)
		}

		for i := s.scrollOffset; i < endIdx; i++ {
			sess := s.sessions[i]

			// Display name
			name := sess.ID
			if sess.Name != "" {
				name = sess.Name
			}

			// Truncate if needed
			maxNameLen := s.width - 4
			if maxNameLen < 10 {
				maxNameLen = 10
			}
			if len(name) > maxNameLen {
				name = name[:maxNameLen-3] + "..."
			}

			// Style based on selection and current
			var style lipgloss.Style
			if i == s.selected && s.focused {
				style = SessionItemSelectedStyle
			} else if sess.IsCurrent {
				style = SessionItemCurrentStyle
			} else {
				style = SessionItemStyle
			}

			// Icon
			icon := "  "
			if sess.IsCurrent {
				icon = "▸ "
			}

			b.WriteString(style.Render(icon + name))
			b.WriteString("\n")

			// Info line
			info := fmt.Sprintf("  %d msgs · %s", sess.Messages, sess.UpdatedAt)
			b.WriteString(SessionInfoStyle.Render(info))
			b.WriteString("\n")
		}

		// Scroll indicator
		if len(s.sessions) > visibleItems {
			scrollInfo := fmt.Sprintf("─ %d/%d ─", s.selected+1, len(s.sessions))
			b.WriteString(DimStyle.Render(scrollInfo))
		}
	}

	borderStyle := SidebarStyle.Width(s.width).Height(s.height)
	if s.focused {
		borderStyle = borderStyle.BorderForeground(AccentColor)
	}

	return borderStyle.Render(b.String())
}

// =============================================================================
// Chat View Component
// =============================================================================

// MessageType represents the type of message
type MessageType int

const (
	MessageTypeUser MessageType = iota
	MessageTypeModel
	MessageTypeTool
	MessageTypeError
	MessageTypeSystem
)

// ChatMessage represents a single message
type ChatMessage struct {
	Type      MessageType
	Content   string
	ToolName  string
	ToolArgs  string
	Timestamp string
	Rendered  string // Pre-rendered content for Markdown
}

// ChatViewModel represents the chat display area
type ChatViewModel struct {
	viewport    viewport.Model
	messages    []ChatMessage
	width       int
	height      int
	focused     bool
	renderer    *MarkdownRenderer
	loading     bool
	loadingText string
}

// NewChatViewModel creates a new chat view model
func NewChatViewModel() ChatViewModel {
	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = true

	return ChatViewModel{
		viewport: vp,
		messages: []ChatMessage{},
		renderer: NewMarkdownRenderer(80),
	}
}

// SetSize sets the chat view dimensions
func (c *ChatViewModel) SetSize(width, height int) {
	c.width = width
	c.height = height
	c.viewport.Width = width
	c.viewport.Height = height
	c.renderer.SetWidth(width - 4)
	c.updateContent()
}

// SetFocused sets focus state
func (c *ChatViewModel) SetFocused(focused bool) {
	c.focused = focused
}

// SetLoading sets loading state
func (c *ChatViewModel) SetLoading(loading bool, text string) {
	c.loading = loading
	c.loadingText = text
}

// AddMessage adds a message to the chat
func (c *ChatViewModel) AddMessage(msg ChatMessage) {
	// Render markdown for model messages
	if msg.Type == MessageTypeModel && c.renderer != nil {
		msg.Rendered = c.renderer.Render(msg.Content)
	}
	c.messages = append(c.messages, msg)
	c.updateContent()
	c.viewport.GotoBottom()
}

// UpdateLastMessage updates the last message (for streaming)
func (c *ChatViewModel) UpdateLastMessage(content string) {
	if len(c.messages) > 0 {
		last := &c.messages[len(c.messages)-1]
		last.Content = content
		if last.Type == MessageTypeModel && c.renderer != nil {
			last.Rendered = c.renderer.Render(content)
		}
		c.updateContent()
		c.viewport.GotoBottom()
	}
}

// Clear clears all messages
func (c *ChatViewModel) Clear() {
	c.messages = []ChatMessage{}
	c.updateContent()
}

// updateContent rebuilds the viewport content
func (c *ChatViewModel) updateContent() {
	var b strings.Builder

	for _, msg := range c.messages {
		b.WriteString(c.renderMessage(msg))
		b.WriteString("\n\n")
	}

	c.viewport.SetContent(b.String())
}

// renderMessage renders a single message
func (c *ChatViewModel) renderMessage(msg ChatMessage) string {
	switch msg.Type {
	case MessageTypeUser:
		return c.renderUserMessage(msg)
	case MessageTypeModel:
		return c.renderModelMessage(msg)
	case MessageTypeTool:
		return c.renderToolMessage(msg)
	case MessageTypeError:
		return c.renderErrorMessage(msg)
	case MessageTypeSystem:
		return c.renderSystemMessage(msg)
	default:
		return msg.Content
	}
}

func (c *ChatViewModel) renderUserMessage(msg ChatMessage) string {
	header := UserPromptStyle.Render("❯ You")
	if msg.Timestamp != "" {
		header += " " + TimestampStyle.Render(msg.Timestamp)
	}

	content := msg.Content
	// Truncate long user messages for display
	lines := strings.Split(content, "\n")
	if len(lines) > 10 {
		content = strings.Join(lines[:10], "\n") + "\n" + DimStyle.Render("...")
	}

	return header + "\n" + content
}

func (c *ChatViewModel) renderModelMessage(msg ChatMessage) string {
	header := AccentStyle.Render("✨ Gemini")
	if msg.Timestamp != "" {
		header += " " + TimestampStyle.Render(msg.Timestamp)
	}

	content := msg.Content
	if msg.Rendered != "" {
		content = msg.Rendered
	}

	return header + "\n" + content
}

func (c *ChatViewModel) renderToolMessage(msg ChatMessage) string {
	header := ToolCallStyle.Render("⚡ TOOL") + " " + ToolNameStyle.Render(msg.ToolName)
	if msg.ToolArgs != "" {
		header += " " + ToolArgStyle.Render("→ "+msg.ToolArgs)
	}

	content := ""
	if msg.Content != "" {
		// Tool result
		if strings.HasPrefix(msg.Content, "✓") {
			content = SuccessStyle.Render(msg.Content)
		} else if strings.HasPrefix(msg.Content, "✗") {
			content = ErrorStyle.Render(msg.Content)
		} else {
			content = msg.Content
		}
	}

	if content != "" {
		return header + "\n" + content
	}
	return header
}

func (c *ChatViewModel) renderErrorMessage(msg ChatMessage) string {
	return ErrorStyle.Render("✗ Error: " + msg.Content)
}

func (c *ChatViewModel) renderSystemMessage(msg ChatMessage) string {
	return DimStyle.Render("─── " + msg.Content + " ───")
}

// Update handles viewport updates
func (c *ChatViewModel) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.viewport, cmd = c.viewport.Update(msg)
	return cmd
}

// View renders the chat view
func (c *ChatViewModel) View() string {
	borderStyle := ChatContainerStyle.Width(c.width).Height(c.height)
	if c.focused {
		borderStyle = borderStyle.Copy().BorderForeground(AccentColor)
	}

	content := c.viewport.View()

	// Add loading indicator at bottom if loading
	if c.loading {
		loadingText := c.loadingText
		if loadingText == "" {
			loadingText = "Thinking..."
		}
		loading := "\n" + SpinnerTextStyle.Render("⠋ "+loadingText)
		content = content + loading
	}

	return content
}

// =============================================================================
// Input Component
// =============================================================================

// InputModel represents the text input component
type InputModel struct {
	value       string
	cursor      int
	width       int
	height      int
	focused     bool
	placeholder string
	history     []string
	historyIdx  int
}

// NewInputModel creates a new input model
func NewInputModel() InputModel {
	return InputModel{
		value:       "",
		cursor:      0,
		placeholder: "Type a message... (Enter to send, Shift+Enter for new line)",
		height:      3,
		history:     []string{},
		historyIdx:  -1,
	}
}

// SetWidth sets the input width
func (i *InputModel) SetWidth(width int) {
	i.width = width
}

// SetFocused sets focus state
func (i *InputModel) SetFocused(focused bool) {
	i.focused = focused
}

// Value returns the current value
func (i *InputModel) Value() string {
	return i.value
}

// SetValue sets the value
func (i *InputModel) SetValue(value string) {
	i.value = value
	i.cursor = len(value)
}

// Reset clears the input
func (i *InputModel) Reset() {
	// Add to history if not empty
	if i.value != "" {
		i.history = append(i.history, i.value)
	}
	i.value = ""
	i.cursor = 0
	i.historyIdx = -1
}

// InsertChar inserts a character at cursor
func (i *InputModel) InsertChar(c rune) {
	i.value = i.value[:i.cursor] + string(c) + i.value[i.cursor:]
	i.cursor++
}

// InsertString inserts a string at cursor
func (i *InputModel) InsertString(s string) {
	i.value = i.value[:i.cursor] + s + i.value[i.cursor:]
	i.cursor += len(s)
}

// DeleteChar deletes character before cursor (backspace)
func (i *InputModel) DeleteChar() {
	if i.cursor > 0 {
		i.value = i.value[:i.cursor-1] + i.value[i.cursor:]
		i.cursor--
	}
}

// DeleteCharForward deletes character at cursor (delete)
func (i *InputModel) DeleteCharForward() {
	if i.cursor < len(i.value) {
		i.value = i.value[:i.cursor] + i.value[i.cursor+1:]
	}
}

// MoveLeft moves cursor left
func (i *InputModel) MoveLeft() {
	if i.cursor > 0 {
		i.cursor--
	}
}

// MoveRight moves cursor right
func (i *InputModel) MoveRight() {
	if i.cursor < len(i.value) {
		i.cursor++
	}
}

// MoveToStart moves cursor to start
func (i *InputModel) MoveToStart() {
	i.cursor = 0
}

// MoveToEnd moves cursor to end
func (i *InputModel) MoveToEnd() {
	i.cursor = len(i.value)
}

// HistoryUp navigates to previous history item
func (i *InputModel) HistoryUp() {
	if len(i.history) == 0 {
		return
	}
	if i.historyIdx == -1 {
		i.historyIdx = len(i.history) - 1
	} else if i.historyIdx > 0 {
		i.historyIdx--
	}
	i.value = i.history[i.historyIdx]
	i.cursor = len(i.value)
}

// HistoryDown navigates to next history item
func (i *InputModel) HistoryDown() {
	if i.historyIdx == -1 {
		return
	}
	if i.historyIdx < len(i.history)-1 {
		i.historyIdx++
		i.value = i.history[i.historyIdx]
	} else {
		i.historyIdx = -1
		i.value = ""
	}
	i.cursor = len(i.value)
}

// DeleteWord deletes word before cursor
func (i *InputModel) DeleteWord() {
	if i.cursor == 0 {
		return
	}

	// Find start of word
	start := i.cursor - 1
	for start > 0 && i.value[start-1] == ' ' {
		start--
	}
	for start > 0 && i.value[start-1] != ' ' {
		start--
	}

	i.value = i.value[:start] + i.value[i.cursor:]
	i.cursor = start
}

// DeleteLine clears the line
func (i *InputModel) DeleteLine() {
	i.value = ""
	i.cursor = 0
}

// View renders the input
func (i *InputModel) View() string {
	prompt := InputPromptStyle.Render("❯ ")

	var content string
	if i.value == "" && !i.focused {
		content = InputPlaceholderStyle.Render(i.placeholder)
	} else {
		// Show value with cursor
		if i.focused {
			before := i.value[:i.cursor]
			after := i.value[i.cursor:]
			cursor := InputCursorStyle.Render("█")
			content = before + cursor + after
		} else {
			content = i.value
		}
	}

	// Handle multiline
	lines := strings.Split(content, "\n")
	if len(lines) > 5 {
		// Truncate to show last 5 lines
		content = "...\n" + strings.Join(lines[len(lines)-5:], "\n")
	}

	inputLine := prompt + content

	style := InputContainerStyle.Width(i.width)
	if i.focused {
		style = style.Copy().BorderForeground(AccentColor)
	}

	return style.Render(inputLine)
}

// =============================================================================
// Status Bar Component
// =============================================================================

// StatusBarModel represents the status bar
type StatusBarModel struct {
	width        int
	inputTokens  int
	outputTokens int
	model        string
	sessionID    string
	helpText     string
}

// NewStatusBarModel creates a new status bar model
func NewStatusBarModel() StatusBarModel {
	return StatusBarModel{
		helpText: "?:help  C-b:sidebar  C-e:context  C-c:quit",
	}
}

// SetWidth sets the status bar width
func (s *StatusBarModel) SetWidth(width int) {
	s.width = width
}

// SetTokens sets token counts
func (s *StatusBarModel) SetTokens(input, output int) {
	s.inputTokens = input
	s.outputTokens = output
}

// SetModel sets the model name
func (s *StatusBarModel) SetModel(model string) {
	s.model = model
}

// SetSessionID sets the session ID
func (s *StatusBarModel) SetSessionID(sessionID string) {
	s.sessionID = sessionID
}

// View renders the status bar
func (s StatusBarModel) View() string {
	// Left side: tokens
	left := ""
	if s.inputTokens > 0 || s.outputTokens > 0 {
		left = fmt.Sprintf("tokens: %d↑ %d↓",
			s.inputTokens,
			s.outputTokens)
	}

	// Right side: help hints
	right := s.helpText

	// Calculate spacing
	leftLen := len(left)
	rightLen := len(right)
	spaces := s.width - leftLen - rightLen - 2

	if spaces < 1 {
		spaces = 1
	}

	content := StatusValueStyle.Render(left) +
		strings.Repeat(" ", spaces) +
		HelpStyle.Render(right)

	return StatusBarStyle.Width(s.width).Render(content)
}

// =============================================================================
// Spinner Component
// =============================================================================

// SpinnerModel represents a loading spinner
type SpinnerModel struct {
	spinner spinner.Model
	text    string
	active  bool
}

// NewSpinnerModel creates a new spinner model
func NewSpinnerModel() SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	return SpinnerModel{
		spinner: s,
		text:    "Thinking...",
	}
}

// Start starts the spinner
func (s *SpinnerModel) Start(text string) tea.Cmd {
	s.active = true
	s.text = text
	return s.spinner.Tick
}

// Stop stops the spinner
func (s *SpinnerModel) Stop() {
	s.active = false
}

// Update updates the spinner
func (s *SpinnerModel) Update(msg tea.Msg) tea.Cmd {
	if !s.active {
		return nil
	}
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return cmd
}

// View renders the spinner
func (s SpinnerModel) View() string {
	if !s.active {
		return ""
	}
	return s.spinner.View() + " " + SpinnerTextStyle.Render(s.text)
}

// IsActive returns whether the spinner is active
func (s SpinnerModel) IsActive() bool {
	return s.active
}
