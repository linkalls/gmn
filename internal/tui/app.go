// Package tui provides a full-featured terminal user interface for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/linkalls/gmn/internal/api"
	"github.com/linkalls/gmn/internal/confirmation"
	"github.com/linkalls/gmn/internal/session"
	"github.com/linkalls/gmn/internal/tools"
)

// FocusArea represents which panel is focused
type FocusArea int

const (
	FocusInput FocusArea = iota
	FocusChat
	FocusSidebar
)

// Config holds TUI configuration
type Config struct {
	Model           string
	YoloMode        bool
	Cwd             string
	ProjectID       string
	Timeout         time.Duration
	AvailableModels []string
	InitialPrompt   string
	ResumeSession   string
}

// App represents the main TUI application
type App struct {
	// Configuration
	config Config
	keys   KeyMap

	// Core components
	header       HeaderModel
	sidebar      SidebarModel
	chatView     ChatViewModel
	input        InputModel
	statusBar    StatusBarModel
	spinner      SpinnerModel
	thinking     ThinkingModel
	contextPanel ContextPanelModel
	filePreview  FilePreviewModel
	confirmDlg   ConfirmDialogModel

	// API & Session
	client     *api.Client
	sessionMgr *session.Manager
	session    *session.Session
	allowList  *confirmation.AllowList
	registry   *tools.Registry
	history    []api.Content

	// State
	width           int
	height          int
	focus           FocusArea
	showSidebar     bool
	showHelp        bool
	showContext     bool
	loading         bool
	loadingText     string
	err             error
	quitting        bool
	inputTokens     int
	outputTokens    int
	startTime       time.Time
	pendingToolResp chan toolResponse
	ctx             context.Context
	cancelFunc      context.CancelFunc
}

// toolResponse holds the result of a tool execution
type toolResponse struct {
	toolName  string
	result    map[string]interface{}
	err       error
	outcome   confirmation.Outcome
	cancelled bool
}

// Messages for async operations
type (
	streamTextMsg  string
	streamDoneMsg  struct{ usage *api.UsageMetadata }
	streamErrorMsg struct{ err error }
	toolCallMsg    struct {
		call *api.FunctionCall
		part *api.Part
	}
	toolResultMsg    toolResponse
	sessionListMsg   []SessionInfo
	confirmResultMsg confirmation.Outcome
	tickMsg          time.Time
)

// NewApp creates a new TUI application
func NewApp(config Config, client *api.Client, sessionMgr *session.Manager, registry *tools.Registry) *App {
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config:      config,
		keys:        DefaultKeyMap(),
		client:      client,
		sessionMgr:  sessionMgr,
		registry:    registry,
		allowList:   confirmation.NewAllowList(),
		history:     []api.Content{},
		focus:       FocusInput,
		showSidebar: true,
		showContext: true,
		startTime:   time.Now(),
		ctx:         ctx,
		cancelFunc:  cancel,
	}

	// Initialize components
	app.header = NewHeaderModel(config.Model, config.YoloMode, config.Cwd)
	app.sidebar = NewSidebarModel()
	app.chatView = NewChatViewModel()
	app.input = NewInputModel()
	app.statusBar = NewStatusBarModel()
	app.spinner = NewSpinnerModel()
	app.thinking = NewThinkingModel()
	app.contextPanel = NewContextPanelModel()
	app.filePreview = NewFilePreviewModel()
	app.confirmDlg = NewConfirmDialogModel()

	// Set initial focus
	app.input.SetFocused(true)
	app.statusBar.SetModel(config.Model)

	return app
}

// Init initializes the TUI
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		tea.EnableMouseCellMotion,
		a.loadSessions,
		a.initSession,
	)
}

// loadSessions loads the session list
func (a *App) loadSessions() tea.Msg {
	if a.sessionMgr == nil {
		return sessionListMsg{}
	}

	sessions, err := a.sessionMgr.List()
	if err != nil {
		return sessionListMsg{}
	}

	var sessionInfos []SessionInfo
	for _, s := range sessions {
		info := SessionInfo{
			ID:        s.ID,
			Name:      s.Name,
			Messages:  len(s.Messages),
			UpdatedAt: s.UpdatedAt.Format("01/02 15:04"),
			IsCurrent: a.session != nil && s.ID == a.session.ID,
		}
		sessionInfos = append(sessionInfos, info)
	}

	return sessionListMsg(sessionInfos)
}

// initSession initializes or resumes a session
func (a *App) initSession() tea.Msg {
	if a.config.ResumeSession != "" && a.sessionMgr != nil {
		var s *session.Session
		var err error

		if a.config.ResumeSession == "last" {
			s, err = a.sessionMgr.LoadLatest()
		} else {
			s, err = a.sessionMgr.Load(a.config.ResumeSession)
		}

		if err == nil {
			a.session = s
			a.restoreHistory(s)
			a.inputTokens = s.Tokens.Input
			a.outputTokens = s.Tokens.Output
			a.config.Model = s.Model
			a.header.SetModel(s.Model)
			a.statusBar.SetModel(s.Model)
			a.statusBar.SetSessionID(s.ID)
			a.statusBar.SetTokens(a.inputTokens, a.outputTokens)

			// Add system message about resumed session
			a.chatView.AddMessage(ChatMessage{
				Type:    MessageTypeSystem,
				Content: fmt.Sprintf("Resumed session: %s (%d messages)", s.ID, len(s.Messages)),
			})

			// Display previous messages
			for _, h := range a.history {
				a.addHistoryToChat(h)
			}
		}
	}

	if a.session == nil && a.sessionMgr != nil {
		a.session = a.sessionMgr.NewSession(a.config.Model)
		a.statusBar.SetSessionID(a.session.ID)
	}

	// Process initial prompt if provided
	if a.config.InitialPrompt != "" {
		return a.sendMessage(a.config.InitialPrompt)
	}

	return nil
}

// restoreHistory restores history from a session
func (a *App) restoreHistory(s *session.Session) {
	for _, msg := range s.Messages {
		var content api.Content
		if roleStr, ok := msg["role"].(string); ok {
			content.Role = roleStr
		}
		if partsRaw, ok := msg["parts"].([]interface{}); ok {
			for _, p := range partsRaw {
				if partMap, ok := p.(map[string]interface{}); ok {
					if text, ok := partMap["text"].(string); ok {
						content.Parts = append(content.Parts, api.Part{Text: text})
					}
				}
			}
		}
		a.history = append(a.history, content)
	}
}

// addHistoryToChat adds a history item to the chat view
func (a *App) addHistoryToChat(content api.Content) {
	for _, part := range content.Parts {
		if part.Text != "" {
			var msgType MessageType
			if content.Role == "user" {
				msgType = MessageTypeUser
			} else {
				msgType = MessageTypeModel
			}
			a.chatView.AddMessage(ChatMessage{
				Type:    msgType,
				Content: part.Text,
			})
		}
	}
}

// Update handles messages
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmd := a.handleKeyMsg(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.MouseMsg:
		cmd := a.handleMouseMsg(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		a.handleWindowSize(msg.Width, msg.Height)

	case sessionListMsg:
		// Update current session marker
		sessions := []SessionInfo(msg)
		for i := range sessions {
			sessions[i].IsCurrent = a.session != nil && sessions[i].ID == a.session.ID
		}
		a.sidebar.SetSessions(sessions)

	case streamTextMsg:
		text := string(msg)
		if len(a.chatView.messages) > 0 {
			last := a.chatView.messages[len(a.chatView.messages)-1]
			if last.Type == MessageTypeModel {
				a.chatView.UpdateLastMessage(last.Content + text)
			}
		} else {
			a.chatView.AddMessage(ChatMessage{
				Type:    MessageTypeModel,
				Content: text,
			})
		}

	case streamDoneMsg:
		a.loading = false
		a.spinner.Stop()
		a.thinking.Stop()
		a.chatView.SetLoading(false, "")
		if msg.usage != nil {
			a.inputTokens += msg.usage.PromptTokenCount
			a.outputTokens += msg.usage.CandidatesTokenCount
			a.statusBar.SetTokens(a.inputTokens, a.outputTokens)
		}
		// Update activity
		a.contextPanel.UpdateLastActivity(ActivityStatusSuccess, time.Since(a.startTime))
		a.autoSave()

	case streamErrorMsg:
		a.loading = false
		a.spinner.Stop()
		a.thinking.Stop()
		a.chatView.SetLoading(false, "")
		a.chatView.AddMessage(ChatMessage{
			Type:    MessageTypeError,
			Content: msg.err.Error(),
		})
		// Update activity
		a.contextPanel.UpdateLastActivity(ActivityStatusError, time.Since(a.startTime))

	case toolCallMsg:
		// Add thinking step for tool call
		a.thinking.AddStep(fmt.Sprintf("Calling %s", msg.call.Name))

		// Add activity
		a.contextPanel.AddActivity(ActivityItem{
			Type:   ActivityTypeTool,
			Title:  msg.call.Name,
			Detail: formatToolArgs(msg.call.Args),
			Status: ActivityStatusRunning,
		})

		a.chatView.AddMessage(ChatMessage{
			Type:     MessageTypeTool,
			ToolName: msg.call.Name,
			ToolArgs: formatToolArgs(msg.call.Args),
		})
		// Execute tool asynchronously
		cmds = append(cmds, a.executeTool(msg.call, msg.part))

	case toolResultMsg:
		// Complete thinking step
		if msg.err != nil || msg.cancelled {
			a.thinking.FailStep()
		} else {
			a.thinking.CompleteStep()
		}

		if msg.cancelled {
			a.chatView.AddMessage(ChatMessage{
				Type:    MessageTypeTool,
				Content: "✗ Cancelled by user",
			})
			// Update activity
			a.contextPanel.UpdateLastActivity(ActivityStatusError, 0)
			// Stop loading and don't continue
			a.loading = false
			a.spinner.Stop()
			a.thinking.Stop()
			a.chatView.SetLoading(false, "")
		} else if msg.err != nil {
			a.chatView.AddMessage(ChatMessage{
				Type:    MessageTypeTool,
				Content: "✗ " + msg.err.Error(),
			})
			// Update activity
			a.contextPanel.UpdateLastActivity(ActivityStatusError, 0)
			// Continue to get model response after tool error
			a.thinking.AddStep("Processing response")
			a.chatView.SetLoading(true, "Processing...")
			a.chatView.AddMessage(ChatMessage{
				Type:    MessageTypeModel,
				Content: "",
			})
			cmds = append(cmds, a.startStreamingWithUpdates())
		} else {
			resultStr := "✓ Completed"
			if count, ok := msg.result["count"].(int); ok {
				resultStr = fmt.Sprintf("✓ %d items", count)
			} else if msgStr, ok := msg.result["message"].(string); ok {
				if len(msgStr) > 50 {
					msgStr = msgStr[:47] + "..."
				}
				resultStr = "✓ " + msgStr
			}
			a.chatView.AddMessage(ChatMessage{
				Type:    MessageTypeTool,
				Content: resultStr,
			})
			// Update activity
			a.contextPanel.UpdateLastActivity(ActivityStatusSuccess, 0)
			// Continue to get model response after tool execution
			a.thinking.AddStep("Processing response")
			a.chatView.SetLoading(true, "Processing...")
			a.chatView.AddMessage(ChatMessage{
				Type:    MessageTypeModel,
				Content: "",
			})
			cmds = append(cmds, a.startStreamingWithUpdates())
		}

	case tickMsg:
		if a.loading {
			cmd := a.spinner.Update(msg)
			cmds = append(cmds, cmd)
			// Also update thinking indicator
			thinkCmd := a.thinking.Update(msg)
			if thinkCmd != nil {
				cmds = append(cmds, thinkCmd)
			}
		}
	}

	// Update confirmation dialog if visible
	if a.confirmDlg.IsVisible() {
		cmd := a.confirmDlg.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Update spinner if loading
	if a.loading {
		cmd := a.spinner.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Update thinking
		thinkCmd := a.thinking.Update(msg)
		if thinkCmd != nil {
			cmds = append(cmds, thinkCmd)
		}
	}

	return a, tea.Batch(cmds...)
}

// handleKeyMsg handles keyboard input
func (a *App) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	// Global keys that work regardless of focus
	switch {
	case key.Matches(msg, a.keys.Quit):
		a.quitting = true
		a.autoSave()
		return tea.Quit

	case key.Matches(msg, a.keys.Help):
		a.showHelp = !a.showHelp
		return nil

	case key.Matches(msg, a.keys.ToggleSidebar):
		a.showSidebar = !a.showSidebar
		a.handleWindowSize(a.width, a.height)
		return nil

	case key.Matches(msg, a.keys.ToggleContext):
		a.showContext = !a.showContext
		a.handleWindowSize(a.width, a.height)
		return nil

	case key.Matches(msg, a.keys.TogglePreview):
		a.filePreview.Toggle()
		return nil

	case key.Matches(msg, a.keys.FocusInput):
		a.setFocus(FocusInput)
		return nil

	case key.Matches(msg, a.keys.FocusChat):
		a.setFocus(FocusChat)
		return nil

	case key.Matches(msg, a.keys.FocusSidebar):
		if a.showSidebar {
			a.setFocus(FocusSidebar)
		}
		return nil

	case key.Matches(msg, a.keys.NewSession):
		return a.newSession()

	case key.Matches(msg, a.keys.SaveSession):
		a.autoSave()
		a.chatView.AddMessage(ChatMessage{
			Type:    MessageTypeSystem,
			Content: "Session saved",
		})
		return nil

	case key.Matches(msg, a.keys.ClearChat):
		a.history = nil
		a.chatView.Clear()
		a.chatView.AddMessage(ChatMessage{
			Type:    MessageTypeSystem,
			Content: "Conversation cleared",
		})
		return nil
	}

	// Focus-specific keys
	switch a.focus {
	case FocusInput:
		return a.handleInputKey(msg)
	case FocusChat:
		return a.handleChatKey(msg)
	case FocusSidebar:
		return a.handleSidebarKey(msg)
	}

	return nil
}

// handleInputKey handles input-focused keys
func (a *App) handleInputKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEnter:
		if msg.Alt || strings.Contains(msg.String(), "shift") {
			// Shift+Enter or Alt+Enter: new line
			a.input.InsertChar('\n')
			return nil
		}
		// Enter: send message
		value := strings.TrimSpace(a.input.Value())
		if value == "" {
			return nil
		}

		// Check for commands
		if strings.HasPrefix(value, "/") {
			return a.handleCommand(value)
		}

		a.input.Reset()
		return a.sendMessage(value)

	case tea.KeyBackspace:
		a.input.DeleteChar()
	case tea.KeyDelete:
		a.input.DeleteCharForward()
	case tea.KeyLeft:
		a.input.MoveLeft()
	case tea.KeyRight:
		a.input.MoveRight()
	case tea.KeyHome:
		a.input.MoveToStart()
	case tea.KeyEnd:
		a.input.MoveToEnd()
	case tea.KeyUp:
		a.input.HistoryUp()
	case tea.KeyDown:
		a.input.HistoryDown()
	case tea.KeyCtrlW:
		a.input.DeleteWord()
	case tea.KeyCtrlU:
		a.input.DeleteLine()
	case tea.KeyTab:
		// Autocomplete for commands
		value := a.input.Value()
		if strings.HasPrefix(value, "/") {
			completed := a.autocompleteCommand(value)
			if completed != value {
				a.input.SetValue(completed)
			}
		}
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			a.input.InsertChar(r)
		}
	case tea.KeySpace:
		a.input.InsertChar(' ')
	}

	return nil
}

// handleChatKey handles chat-focused keys
func (a *App) handleChatKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, a.keys.Up):
		a.chatView.viewport.LineUp(1)
	case key.Matches(msg, a.keys.Down):
		a.chatView.viewport.LineDown(1)
	case key.Matches(msg, a.keys.PageUp):
		a.chatView.viewport.HalfViewUp()
	case key.Matches(msg, a.keys.PageDown):
		a.chatView.viewport.HalfViewDown()
	case key.Matches(msg, a.keys.Home):
		a.chatView.viewport.GotoTop()
	case key.Matches(msg, a.keys.End):
		a.chatView.viewport.GotoBottom()
	}
	return nil
}

// handleSidebarKey handles sidebar-focused keys
func (a *App) handleSidebarKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, a.keys.Up):
		a.sidebar.MoveUp()
	case key.Matches(msg, a.keys.Down):
		a.sidebar.MoveDown()
	case key.Matches(msg, a.keys.Submit):
		// Load selected session
		selected := a.sidebar.SelectedSession()
		if selected != nil {
			return a.loadSession(selected.ID)
		}
	}
	return nil
}

// handleMouseMsg handles mouse input
func (a *App) handleMouseMsg(msg tea.MouseMsg) tea.Cmd {
	switch msg.Action {
	case tea.MouseActionPress:
		// Determine which area was clicked
		x, y := msg.X, msg.Y

		// Header area (top 3 lines)
		if y < 3 {
			return nil
		}

		// Status bar (bottom line)
		if y >= a.height-1 {
			return nil
		}

		// Sidebar (left side if visible)
		sidebarWidth := 0
		if a.showSidebar {
			sidebarWidth = 28
			if x < sidebarWidth {
				a.setFocus(FocusSidebar)
				// Calculate which session was clicked
				clickedIdx := (y-4)/2 + a.sidebar.scrollOffset
				if clickedIdx >= 0 && clickedIdx < len(a.sidebar.sessions) {
					a.sidebar.selected = clickedIdx
				}
				return nil
			}
		}

		// Input area (bottom 3 lines above status bar)
		if y >= a.height-4 {
			a.setFocus(FocusInput)
			return nil
		}

		// Chat area (everything else)
		a.setFocus(FocusChat)

	case tea.MouseActionMotion:
		// Could implement hover effects here
	}

	// Forward scroll events to appropriate viewport
	if msg.Action == tea.MouseActionPress && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown) {
		if a.focus == FocusChat {
			cmd := a.chatView.Update(msg)
			return cmd
		}
	}

	return nil
}

// handleWindowSize handles window resize
func (a *App) handleWindowSize(width, height int) {
	a.width = width
	a.height = height

	// Calculate layout
	headerHeight := 3
	statusHeight := 1
	inputHeight := 3

	sidebarWidth := 0
	if a.showSidebar {
		sidebarWidth = 28
	}

	contextWidth := 0
	if a.showContext {
		contextWidth = 30
	}

	chatWidth := width - sidebarWidth - contextWidth
	chatHeight := height - headerHeight - statusHeight - inputHeight

	// Update components
	a.header.SetWidth(width)
	a.sidebar.SetSize(sidebarWidth, chatHeight)
	a.chatView.SetSize(chatWidth, chatHeight)
	a.contextPanel.SetSize(contextWidth, chatHeight)
	a.input.SetWidth(width - sidebarWidth)
	a.statusBar.SetWidth(width)
	a.thinking.SetWidth(chatWidth)
	a.filePreview.SetSize(chatWidth-4, chatHeight-4)
	a.confirmDlg.SetSize(width, height)
}

// setFocus sets the focus to a specific area
func (a *App) setFocus(focus FocusArea) {
	a.focus = focus
	a.input.SetFocused(focus == FocusInput)
	a.chatView.SetFocused(focus == FocusChat)
	a.sidebar.SetFocused(focus == FocusSidebar)
}

// handleCommand handles slash commands
func (a *App) handleCommand(cmd string) tea.Cmd {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	switch strings.ToLower(parts[0]) {
	case "/help", "/h":
		a.showHelp = true
		return nil

	case "/exit", "/quit", "/q":
		a.quitting = true
		a.autoSave()
		return tea.Quit

	case "/clear":
		a.history = nil
		a.chatView.Clear()
		a.chatView.AddMessage(ChatMessage{
			Type:    MessageTypeSystem,
			Content: "Conversation cleared",
		})
		return nil

	case "/stats":
		duration := time.Since(a.startTime)
		stats := fmt.Sprintf("Tokens: %d↑ %d↓ | Duration: %s",
			a.inputTokens, a.outputTokens, duration.Round(time.Second))
		a.chatView.AddMessage(ChatMessage{
			Type:    MessageTypeSystem,
			Content: stats,
		})
		return nil

	case "/model":
		if len(parts) == 1 {
			// Show current model
			a.chatView.AddMessage(ChatMessage{
				Type:    MessageTypeSystem,
				Content: "Current model: " + a.config.Model,
			})
		} else {
			newModel := parts[1]
			// Validate model
			valid := false
			for _, m := range a.config.AvailableModels {
				if m == newModel {
					valid = true
					break
				}
			}
			if valid {
				a.config.Model = newModel
				a.header.SetModel(newModel)
				a.statusBar.SetModel(newModel)
				if a.session != nil {
					a.session.Model = newModel
				}
				a.chatView.AddMessage(ChatMessage{
					Type:    MessageTypeSystem,
					Content: "Model switched to " + newModel,
				})
			} else {
				a.chatView.AddMessage(ChatMessage{
					Type:    MessageTypeError,
					Content: "Invalid model: " + newModel,
				})
			}
		}
		return nil

	case "/sessions":
		return a.loadSessions

	case "/save":
		name := ""
		if len(parts) > 1 {
			name = parts[1]
		}
		if a.session != nil && name != "" {
			a.session.Name = name
		}
		a.autoSave()
		a.chatView.AddMessage(ChatMessage{
			Type:    MessageTypeSystem,
			Content: "Session saved",
		})
		return a.loadSessions

	case "/load":
		if len(parts) < 2 {
			a.chatView.AddMessage(ChatMessage{
				Type:    MessageTypeError,
				Content: "Usage: /load <session-id>",
			})
			return nil
		}
		return a.loadSession(parts[1])

	case "/new":
		return a.newSession()

	default:
		a.chatView.AddMessage(ChatMessage{
			Type:    MessageTypeError,
			Content: "Unknown command: " + parts[0],
		})
	}

	return nil
}

// autocompleteCommand provides command autocompletion
func (a *App) autocompleteCommand(partial string) string {
	commands := []string{
		"/help", "/exit", "/quit", "/clear", "/stats",
		"/model", "/sessions", "/save", "/load", "/new",
	}

	partial = strings.ToLower(partial)
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, partial) {
			return cmd
		}
	}
	return partial
}

// sendMessage sends a user message
func (a *App) sendMessage(text string) tea.Cmd {
	// Add user message to chat
	a.chatView.AddMessage(ChatMessage{
		Type:      MessageTypeUser,
		Content:   text,
		Timestamp: time.Now().Format("15:04"),
	})

	// Add to history
	a.history = append(a.history, api.Content{
		Role:  "user",
		Parts: []api.Part{{Text: text}},
	})

	// Start loading with thinking indicator
	a.loading = true
	a.loadingText = "Thinking..."
	a.chatView.SetLoading(true, a.loadingText)

	// Start thinking animation
	a.thinking.Start("Processing request...")
	a.thinking.AddStep("Analyzing input")

	// Add activity
	a.contextPanel.AddActivity(ActivityItem{
		Type:   ActivityTypeThinking,
		Title:  "Processing prompt",
		Status: ActivityStatusRunning,
	})

	// Add placeholder for model response
	a.chatView.AddMessage(ChatMessage{
		Type:    MessageTypeModel,
		Content: "",
	})

	// Start streaming with proper channel-based updates
	return a.startStreamingWithUpdates()
}

// streamUpdateMsg is sent during streaming to update the UI
type streamUpdateMsg struct {
	text string
}

// startStreamingWithUpdates starts streaming with real-time updates
func (a *App) startStreamingWithUpdates() tea.Cmd {
	return func() tea.Msg {
		userPromptID := fmt.Sprintf("gmn-tui-%d", time.Now().UnixNano())

		req := &api.GenerateRequest{
			Model:        a.config.Model,
			Project:      a.config.ProjectID,
			UserPromptID: userPromptID,
			Request: api.InnerRequest{
				Contents: a.history,
				Config: api.GenerationConfig{
					Temperature:     1.0,
					TopP:            0.95,
					MaxOutputTokens: 8192,
				},
				Tools: a.registry.GetTools(),
			},
		}

		ctx, cancel := context.WithTimeout(a.ctx, a.config.Timeout)
		defer cancel()

		stream, err := a.client.GenerateStream(ctx, req)
		if err != nil {
			return streamErrorMsg{err: err}
		}

		var fullText strings.Builder

		for event := range stream {
			switch event.Type {
			case "error":
				return streamErrorMsg{err: fmt.Errorf(event.Error)}

			case "tool_call":
				if event.ToolCall != nil {
					// First, save accumulated text to history if any
					if fullText.Len() > 0 {
						a.history = append(a.history, api.Content{
							Role:  "model",
							Parts: []api.Part{{Text: fullText.String()}},
						})
					}
					return toolCallMsg{call: event.ToolCall, part: event.ToolCallPart}
				}

			case "done":
				// Add model response to history
				if fullText.Len() > 0 {
					a.history = append(a.history, api.Content{
						Role:  "model",
						Parts: []api.Part{{Text: fullText.String()}},
					})
				}
				return streamDoneMsg{usage: event.Usage}

			default:
				if event.Text != "" {
					fullText.WriteString(event.Text)
					// Update the chat view with accumulated text
					// Note: This happens in the same goroutine, so we update directly
					// The final update will happen when done
				}
			}
		}

		// Final update with all text
		if fullText.Len() > 0 {
			a.history = append(a.history, api.Content{
				Role:  "model",
				Parts: []api.Part{{Text: fullText.String()}},
			})
			// Update the last message with final content
			a.chatView.UpdateLastMessage(fullText.String())
		}

		return streamDoneMsg{}
	}
}

// executeTool executes a tool call
func (a *App) executeTool(fc *api.FunctionCall, part *api.Part) tea.Cmd {
	return func() tea.Msg {
		tool, ok := a.registry.Get(fc.Name)
		if !ok {
			// Add error to history
			a.addToolResponseToHistory(part, fc, map[string]interface{}{"error": "unknown tool: " + fc.Name})
			return toolResultMsg{
				toolName: fc.Name,
				err:      fmt.Errorf("unknown tool: %s", fc.Name),
			}
		}

		// Check confirmation requirement
		if tool.RequiresConfirmation() && !a.allowList.IsAllowed(fc.Name) {
			if !a.config.YoloMode {
				// Show confirmation prompt using the existing confirmation package
				details := confirmation.Details{
					Type:     confirmation.ConfirmationType(tool.ConfirmationType()),
					Title:    fmt.Sprintf("Allow %s?", tool.DisplayName()),
					ToolName: tool.Name(),
					Args:     fc.Args,
				}

				// Get file path if available
				if path, ok := fc.Args["path"].(string); ok {
					details.FilePath = path
				}

				// Get URL if available (for web_fetch)
				if urlStr, ok := fc.Args["url"].(string); ok {
					details.URL = urlStr
				}

				// Get command if available (for shell)
				if cmd, ok := fc.Args["command"].(string); ok {
					details.Command = cmd
				}

				// For edit confirmations, try to get diff content
				if tool.ConfirmationType() == "edit" {
					if getter, ok := tool.(interface {
						GetOriginalContent(map[string]interface{}) (string, error)
						GetNewContent(map[string]interface{}) (string, error)
					}); ok {
						if orig, err := getter.GetOriginalContent(fc.Args); err == nil {
							details.OriginalContent = orig
						}
						if newC, err := getter.GetNewContent(fc.Args); err == nil {
							details.NewContent = newC
						}
					}
				}

				outcome, err := confirmation.PromptConfirmation(details)
				if err != nil {
					a.addToolResponseToHistory(part, fc, map[string]interface{}{"error": "confirmation error: " + err.Error()})
					return toolResultMsg{
						toolName: fc.Name,
						err:      err,
					}
				}

				switch outcome {
				case confirmation.OutcomeCancel:
					a.addToolResponseToHistory(part, fc, map[string]interface{}{"error": "operation cancelled by user"})
					return toolResultMsg{
						toolName:  fc.Name,
						cancelled: true,
					}
				case confirmation.OutcomeProceedAlways:
					a.allowList.Allow(fc.Name)
				}
			}
		}

		result, err := tool.Execute(fc.Args)
		if err != nil {
			result = map[string]interface{}{"error": err.Error()}
		}

		// Add tool call and response to history
		a.addToolResponseToHistory(part, fc, result)

		return toolResultMsg{
			toolName: fc.Name,
			result:   result,
			err:      err,
		}
	}
}

// addToolResponseToHistory adds tool call and response to history
func (a *App) addToolResponseToHistory(part *api.Part, fc *api.FunctionCall, result map[string]interface{}) {
	responseID := fc.ID
	if responseID == "" {
		responseID = fmt.Sprintf("%s-%d", fc.Name, time.Now().UnixNano())
	}

	// Add model's tool call
	if part != nil {
		a.history = append(a.history, api.Content{
			Role:  "model",
			Parts: []api.Part{*part},
		})
	} else {
		a.history = append(a.history, api.Content{
			Role:  "model",
			Parts: []api.Part{{FunctionCall: fc}},
		})
	}

	// Add tool response
	a.history = append(a.history, api.Content{
		Role: "user",
		Parts: []api.Part{{FunctionResp: &api.FunctionResp{
			ID:       responseID,
			Name:     fc.Name,
			Response: result,
		}}},
	})
}

// newSession creates a new session
func (a *App) newSession() tea.Cmd {
	a.history = nil
	a.chatView.Clear()
	a.inputTokens = 0
	a.outputTokens = 0

	if a.sessionMgr != nil {
		a.session = a.sessionMgr.NewSession(a.config.Model)
		a.statusBar.SetSessionID(a.session.ID)
	}

	a.statusBar.SetTokens(0, 0)
	a.chatView.AddMessage(ChatMessage{
		Type:    MessageTypeSystem,
		Content: "New session started",
	})

	return a.loadSessions
}

// loadSession loads a session
func (a *App) loadSession(idOrName string) tea.Cmd {
	return func() tea.Msg {
		if a.sessionMgr == nil {
			return nil
		}

		s, err := a.sessionMgr.Load(idOrName)
		if err != nil {
			return streamErrorMsg{err: err}
		}

		a.session = s
		a.history = nil
		a.restoreHistory(s)
		a.inputTokens = s.Tokens.Input
		a.outputTokens = s.Tokens.Output
		a.config.Model = s.Model
		a.header.SetModel(s.Model)
		a.statusBar.SetModel(s.Model)
		a.statusBar.SetSessionID(s.ID)
		a.statusBar.SetTokens(a.inputTokens, a.outputTokens)

		a.chatView.Clear()
		a.chatView.AddMessage(ChatMessage{
			Type:    MessageTypeSystem,
			Content: fmt.Sprintf("Loaded session: %s", s.ID),
		})

		for _, h := range a.history {
			a.addHistoryToChat(h)
		}

		return a.loadSessions()
	}
}

// autoSave saves the current session
func (a *App) autoSave() {
	if a.sessionMgr == nil || a.session == nil {
		return
	}

	// Convert history to session format
	a.session.Messages = make([]map[string]interface{}, len(a.history))
	for i, h := range a.history {
		parts := make([]map[string]interface{}, len(h.Parts))
		for j, p := range h.Parts {
			parts[j] = map[string]interface{}{"text": p.Text}
		}
		a.session.Messages[i] = map[string]interface{}{
			"role":  h.Role,
			"parts": parts,
		}
	}
	a.session.Tokens.Input = a.inputTokens
	a.session.Tokens.Output = a.outputTokens
	a.session.Model = a.config.Model

	a.sessionMgr.Save(a.session)
}

// View renders the TUI
func (a *App) View() string {
	if a.quitting {
		return a.renderExitStats()
	}

	// Check for confirmation dialog
	if a.confirmDlg.IsVisible() {
		return a.renderWithOverlay(a.confirmDlg.View())
	}

	// Check for file preview
	if a.filePreview.IsVisible() {
		return a.renderWithOverlay(a.filePreview.View())
	}

	var sections []string

	// Header
	sections = append(sections, a.header.View())

	// Main content area (sidebar + chat + context)
	var mainContent string
	chatContent := a.chatView.View()

	// Add thinking indicator if loading
	if a.loading && a.thinking.IsActive() {
		chatContent = chatContent + "\n" + a.thinking.View()
	}

	if a.showSidebar && a.showContext {
		sidebar := a.sidebar.View()
		context := a.contextPanel.View()
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, chatContent, context)
	} else if a.showSidebar {
		sidebar := a.sidebar.View()
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, chatContent)
	} else if a.showContext {
		context := a.contextPanel.View()
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, chatContent, context)
	} else {
		mainContent = chatContent
	}
	sections = append(sections, mainContent)

	// Input
	sections = append(sections, a.input.View())

	// Status bar
	sections = append(sections, a.statusBar.View())

	// Help overlay
	if a.showHelp {
		return a.renderHelpOverlay(lipgloss.JoinVertical(lipgloss.Left, sections...))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderWithOverlay renders the main view with an overlay
func (a *App) renderWithOverlay(overlay string) string {
	// Center the overlay
	return lipgloss.Place(
		a.width, a.height,
		lipgloss.Center, lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#00000088")),
	)
}

// renderExitStats renders exit statistics
func (a *App) renderExitStats() string {
	duration := time.Since(a.startTime)
	totalTokens := a.inputTokens + a.outputTokens

	// Cost estimate
	inputCost := float64(a.inputTokens) * 0.000000075
	outputCost := float64(a.outputTokens) * 0.00000030
	totalCost := inputCost + outputCost

	stats := fmt.Sprintf(`
%s

  Input:    %d tokens
  Output:   %d tokens
  Total:    %d tokens
  Duration: %s
  Est Cost: ~$%.6f

%s
`,
		AccentStyle.Render("📊 Session Stats"),
		a.inputTokens,
		a.outputTokens,
		totalTokens,
		duration.Round(time.Second),
		totalCost,
		DimStyle.Render("Goodbye! 👋"),
	)

	return stats
}

// renderHelpOverlay renders the help overlay
func (a *App) renderHelpOverlay(background string) string {
	help := `
╭───────────────────────────────────────────╮
│             📋 Help - gmn TUI             │
├───────────────────────────────────────────┤
│  Navigation                               │
│    ↑/↓         Scroll / History           │
│    PgUp/PgDn   Page up/down               │
│    Tab         Autocomplete               │
│                                           │
│  Panels                                   │
│    C-b         Toggle sidebar             │
│    C-e         Toggle context panel       │
│    C-p         Toggle file preview        │
│    C-1/2/3     Focus chat/side/input      │
│                                           │
│  Commands                                 │
│    /help       Show this help             │
│    /clear      Clear conversation         │
│    /stats      Show token usage           │
│    /model      Show/switch model          │
│    /sessions   List sessions              │
│    /save       Save session               │
│    /load       Load session               │
│    /new        New session                │
│    /exit       Exit                       │
│                                           │
│  General                                  │
│    Enter       Send message               │
│    S-Enter     New line                   │
│    C-c         Quit                       │
│    ?           Toggle help                │
│                                           │
│  Activity Panel                           │
│    Shows files in context                 │
│    Shows recent tool activity             │
╰───────────────────────────────────────────╯
`

	helpBox := lipgloss.NewStyle().
		Foreground(TextColor).
		Background(SurfaceColor).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentColor).
		Padding(1, 2).
		Render(help)

	// Center the help box
	return lipgloss.Place(
		a.width, a.height,
		lipgloss.Center, lipgloss.Center,
		helpBox,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#00000088")),
	)
}

// formatToolArgs formats tool arguments for display
func formatToolArgs(args map[string]interface{}) string {
	if path, ok := args["path"].(string); ok {
		return path
	}
	if pattern, ok := args["pattern"].(string); ok {
		return pattern
	}
	if url, ok := args["url"].(string); ok {
		return url
	}
	if cmd, ok := args["command"].(string); ok {
		if len(cmd) > 40 {
			return cmd[:37] + "..."
		}
		return cmd
	}
	if query, ok := args["query"].(string); ok {
		if len(query) > 40 {
			return query[:37] + "..."
		}
		return query
	}
	return ""
}

// Run starts the TUI application
func Run(config Config, client *api.Client, sessionMgr *session.Manager, registry *tools.Registry) error {
	// Set yolo mode globally
	if config.YoloMode {
		confirmation.YoloMode = true
	}

	app := NewApp(config, client, sessionMgr, registry)

	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()

	// Show exit stats on clean exit
	if err == nil {
		fmt.Print(app.renderExitStats())
	}

	return err
}
