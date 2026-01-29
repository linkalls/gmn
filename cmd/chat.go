package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/linkalls/gmn/internal/api"
	"github.com/linkalls/gmn/internal/cli"
	"github.com/linkalls/gmn/internal/confirmation"
	"github.com/linkalls/gmn/internal/input"
	"github.com/linkalls/gmn/internal/output"
	"github.com/linkalls/gmn/internal/session"
	"github.com/linkalls/gmn/internal/tools"
	"github.com/linkalls/gmn/internal/tui"
	"github.com/spf13/cobra"
)

var (
	yoloMode      bool   // Skip all confirmations
	chatPrompt    string // Initial prompt from -p flag (chat-specific)
	shellPath     string // Custom shell path
	resumeSession string // Session ID to resume
	useTUI        bool   // Use full TUI mode
	sessionTokens struct {
		input  int
		output int
	}
	sessionStartTime time.Time // Track session start for Ctrl+C stats
)

// Spinner for loading indicator
type spinner struct {
	frames  []string
	current int
	mu      sync.Mutex
	stop    chan struct{}
	done    chan struct{}
	message string
}

func newSpinner(message string) *spinner {
	return &spinner{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		message: message,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (s *spinner) Start() {
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		defer close(s.done)

		spinStyle := lipgloss.NewStyle().Foreground(accentPurple).Bold(true)
		msgStyle := lipgloss.NewStyle().Foreground(dimGray)

		for {
			select {
			case <-s.stop:
				// Clear the spinner line
				fmt.Fprint(os.Stderr, "\r\033[K")
				return
			case <-ticker.C:
				s.mu.Lock()
				frame := s.frames[s.current]
				s.current = (s.current + 1) % len(s.frames)
				s.mu.Unlock()

				fmt.Fprintf(os.Stderr, "\r%s %s", spinStyle.Render(frame), msgStyle.Render(s.message))
			}
		}
	}()
}

func (s *spinner) Stop() {
	close(s.stop)
	<-s.done
}

// DefaultShell returns the default shell for the current OS
func DefaultShell() string {
	if runtime.GOOS == "windows" {
		// Check if Git Bash is available
		gitBashPaths := []string{
			`C:\Program Files\Git\bin\bash.exe`,
			`C:\Program Files (x86)\Git\bin\bash.exe`,
		}
		for _, p := range gitBashPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		return "powershell"
	}
	return "bash"
}

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	RunE:  runChat,
}

// TUI styles
var (
	// Modern accent colors
	accentPurple = lipgloss.Color("#7C3AED")
	accentGreen  = lipgloss.Color("#10B981")
	accentBlue   = lipgloss.Color("#3B82F6")
	mutedGray    = lipgloss.Color("#6B7280")
	dimGray      = lipgloss.Color("#9CA3AF")
	borderColor  = lipgloss.Color("#374151")

	// Header styles
	logoStyle = lipgloss.NewStyle().
			Foreground(accentPurple).
			Bold(true)

	modelBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accentPurple).
			Padding(0, 1).
			Bold(true)

	infoBadgeStyle = lipgloss.NewStyle().
			Foreground(dimGray).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1)

	headerBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 2).
			MarginBottom(1)

	promptStyle = lipgloss.NewStyle().
			Foreground(accentGreen).
			Bold(true)

	// Tool styles
	toolCallStyle = lipgloss.NewStyle().
			Foreground(accentPurple).
			Bold(true)
	toolNameStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Bold(true)
	toolResultStyle = lipgloss.NewStyle().
			Foreground(accentGreen)
	toolBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#374151")).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	// Stats styles
	statsBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 2).
			MarginTop(1)
)

func init() {
	rootCmd.AddCommand(chatCmd)

	chatCmd.Flags().StringVarP(&model, "model", "m", "", "Model to use (default determined by tier)")
	chatCmd.Flags().StringVarP(&chatPrompt, "prompt", "p", "", "Initial prompt (alternative to positional args)")
	chatCmd.Flags().StringArrayVarP(&files, "file", "f", nil, "Files to include in context")
	chatCmd.Flags().DurationVarP(&timeout, "timeout", "t", 5*time.Minute, "API timeout")
	chatCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug output")
	chatCmd.Flags().BoolVar(&yoloMode, "yolo", false, "Skip all confirmation prompts (dangerous!)")
	chatCmd.Flags().StringVar(&shellPath, "shell", "", "Shell to use for commands (default: auto-detect)")
	chatCmd.Flags().StringVarP(&resumeSession, "resume", "r", "", "Resume a previous session (ID, name, or 'last')")
	chatCmd.Flags().BoolVar(&useTUI, "tui", true, "Use full TUI mode (default: true)")

	chatCmd.RegisterFlagCompletionFunc("model", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return AvailableModels, cobra.ShellCompDirectiveNoFileComp
	})
}

// displayHeader shows a rich header with model info
func displayHeader(modelName string, yolo bool) {
	// Logo and version
	logo := logoStyle.Render("✨ gmn")
	versionBadge := lipgloss.NewStyle().
		Foreground(dimGray).
		Render("Gemini CLI")

	// Model badge
	modelBadge := modelBadgeStyle.Render(modelName)

	// Info badges
	var badges []string
	badges = append(badges, modelBadge)

	if yolo {
		yoloBadge := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#EF4444")).
			Padding(0, 1).
			Bold(true).
			Render("⚡ YOLO")
		badges = append(badges, yoloBadge)
	}

	cwd, _ := os.Getwd()
	cwdBadge := infoBadgeStyle.Render("📁 " + cwd)

	// Build header
	header := fmt.Sprintf("%s %s  %s\n%s", logo, versionBadge, strings.Join(badges, " "), cwdBadge)
	fmt.Fprintln(os.Stderr, headerBoxStyle.Render(header))

	// Shortcuts bar (Codex-style)
	shortcutStyle := lipgloss.NewStyle().Foreground(dimGray)
	keyStyle := lipgloss.NewStyle().Foreground(accentPurple).Bold(true)

	shortcuts := fmt.Sprintf(
		"%s %s  %s %s  %s %s  %s %s  %s %s",
		keyStyle.Render("/help"),
		shortcutStyle.Render("commands"),
		keyStyle.Render("/model"),
		shortcutStyle.Render("switch"),
		keyStyle.Render("/stats"),
		shortcutStyle.Render("usage"),
		keyStyle.Render("/clear"),
		shortcutStyle.Render("reset"),
		keyStyle.Render("Ctrl+C"),
		shortcutStyle.Render("exit"),
	)
	fmt.Fprintln(os.Stderr, shortcuts)
	fmt.Fprintln(os.Stderr)
}

// displayStats shows session statistics
func displayStats(inputTokens, outputTokens int, duration time.Duration) {
	totalTokens := inputTokens + outputTokens

	tokenStyle := lipgloss.NewStyle().Foreground(accentBlue).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(dimGray)
	headerStyle := lipgloss.NewStyle().Foreground(accentPurple).Bold(true)

	// Calculate cost estimate (rough approximation for Gemini)
	// Gemini 2.5 Flash: ~$0.075/1M input, ~$0.30/1M output
	inputCost := float64(inputTokens) * 0.000000075
	outputCost := float64(outputTokens) * 0.00000030
	totalCost := inputCost + outputCost

	// Format stats
	stats := fmt.Sprintf(
		"%s\n\n  %s %s tokens\n  %s %s tokens\n  %s %s tokens\n  %s %s\n  %s ~$%.6f",
		headerStyle.Render("📊 Session Stats"),
		labelStyle.Render("Input:   "),
		tokenStyle.Render(fmt.Sprintf("%d", inputTokens)),
		labelStyle.Render("Output:  "),
		tokenStyle.Render(fmt.Sprintf("%d", outputTokens)),
		labelStyle.Render("Total:   "),
		tokenStyle.Render(fmt.Sprintf("%d", totalTokens)),
		labelStyle.Render("Duration:"),
		tokenStyle.Render(duration.Round(time.Second).String()),
		labelStyle.Render("Est Cost:"),
		totalCost,
	)

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, statsBoxStyle.Render(stats))
}

// displayPrompt shows the input prompt
func displayPrompt() {
	fmt.Fprint(os.Stderr, promptStyle.Render("❯ "))
}

// displayConversationHistory shows previous conversation when resuming a session
func displayConversationHistory(history []api.Content) {
	if len(history) == 0 {
		return
	}

	userStyle := lipgloss.NewStyle().Foreground(accentBlue).Bold(true)
	modelStyle := lipgloss.NewStyle().Foreground(accentPurple)
	separatorStyle := lipgloss.NewStyle().Foreground(dimGray)

	fmt.Fprintln(os.Stderr, separatorStyle.Render("─── Previous conversation ───"))
	fmt.Fprintln(os.Stderr)

	for _, content := range history {
		// Skip function responses (tool results)
		hasText := false
		for _, part := range content.Parts {
			if part.Text != "" {
				hasText = true
				break
			}
		}
		if !hasText {
			continue
		}

		if content.Role == "user" {
			// User message
			for _, part := range content.Parts {
				if part.Text != "" {
					// Truncate long messages
					text := part.Text
					lines := strings.Split(text, "\n")
					if len(lines) > 5 {
						text = strings.Join(lines[:5], "\n") + "\n..."
					} else if len(text) > 500 {
						text = text[:500] + "..."
					}
					fmt.Fprintln(os.Stderr, userStyle.Render("❯ ")+text)
				}
			}
		} else if content.Role == "model" {
			// Model response
			for _, part := range content.Parts {
				if part.Text != "" {
					// Truncate long responses
					text := part.Text
					lines := strings.Split(text, "\n")
					if len(lines) > 10 {
						text = strings.Join(lines[:10], "\n") + "\n..."
					} else if len(text) > 1000 {
						text = text[:1000] + "..."
					}
					fmt.Fprintln(os.Stderr, modelStyle.Render(text))
				}
			}
		}
		fmt.Fprintln(os.Stderr)
	}

	fmt.Fprintln(os.Stderr, separatorStyle.Render("─── Continue conversation ───"))
	fmt.Fprintln(os.Stderr)
}

func runChat(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	sessionStartTime = startTime // Store globally for signal handler

	// Set YOLO mode if requested
	if yoloMode {
		confirmation.YoloMode = true
	}

	// Set shell path for tools
	if shellPath == "" {
		shellPath = DefaultShell()
	}
	tools.SetShellPath(shellPath)

	// For chat, we don't want a short timeout context for the whole session.
	// We'll use a background context for setup, and per-request timeout.
	ctx := context.Background()

	// Initial prompt from -p flag or args
	initialPrompt := chatPrompt
	if initialPrompt == "" && len(args) > 0 {
		initialPrompt = strings.Join(args, " ")
	}

	// Setup client (reusing logic from root.go)
	apiClient, projectID, userTier, err := setupClient(ctx)
	if err != nil {
		return err
	}

	// Apply tier-based default model if user didn't specify
	effectiveModel := getEffectiveModel(model, userTier, cmd.Flags().Changed("model"))

	// Initialize tool registry with current working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	toolRegistry := tools.NewRegistry(cwd)

	// Initialize session manager
	sessionMgr, err := session.NewManager()
	if err != nil {
		// Non-fatal: continue without session management
		fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(dimGray).Render("⚠ Session management unavailable: "+err.Error()))
		sessionMgr = nil
	}

	// Use TUI mode if enabled (default)
	if useTUI {
		tuiConfig := tui.Config{
			Model:           effectiveModel,
			YoloMode:        yoloMode,
			Cwd:             cwd,
			ProjectID:       projectID,
			Timeout:         timeout,
			AvailableModels: AvailableModels,
			InitialPrompt:   initialPrompt,
			ResumeSession:   resumeSession,
		}
		return tui.Run(tuiConfig, apiClient, sessionMgr, toolRegistry)
	}

	// Legacy REPL mode (--tui=false)
	return runLegacyREPL(cmd, apiClient, projectID, effectiveModel, initialPrompt, cwd, toolRegistry, sessionMgr, startTime)
}

// runLegacyREPL runs the legacy liner-based REPL
func runLegacyREPL(cmd *cobra.Command, apiClient *api.Client, projectID, effectiveModel, initialPrompt, cwd string, toolRegistry *tools.Registry, sessionMgr *session.Manager, startTime time.Time) error {
	ctx := context.Background()

	// Setup signal handler for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr) // New line after ^C
		displayStats(sessionTokens.input, sessionTokens.output, time.Since(sessionStartTime))
		os.Exit(0)
	}()
	defer signal.Stop(sigChan)

	// Display rich header
	displayHeader(effectiveModel, yoloMode)

	// Initialize allow list for session
	allowList := confirmation.NewAllowList()

	// Prepare history
	var history []api.Content
	var currentSession *session.Session

	// Check if resuming a session
	if resumeSession != "" && sessionMgr != nil {
		var loadErr error
		if resumeSession == "last" {
			currentSession, loadErr = sessionMgr.LoadLatest()
		} else {
			currentSession, loadErr = sessionMgr.Load(resumeSession)
		}
		if loadErr != nil {
			fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("✗ Failed to load session: "+loadErr.Error()))
		} else {
			// Restore history from session
			for _, msg := range currentSession.Messages {
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
				history = append(history, content)
			}
			sessionTokens.input = currentSession.Tokens.Input
			sessionTokens.output = currentSession.Tokens.Output
			effectiveModel = currentSession.Model
			fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(accentGreen).Render("✓ Resumed session: "+currentSession.ID))
			if currentSession.Name != "" {
				fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(dimGray).Render("  Name: "+currentSession.Name))
			}
			fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(dimGray).Render(fmt.Sprintf("  Messages: %d", len(history))))
			fmt.Fprintln(os.Stderr)

			// Display conversation history
			displayConversationHistory(history)
		}
	}

	// Create new session if not resuming
	if currentSession == nil && sessionMgr != nil {
		currentSession = sessionMgr.NewSession(effectiveModel)
	}

	// Prepare initial input (files + prompt)
	inputText, err := input.PrepareInput(initialPrompt, files)
	if err != nil {
		return err
	}

	// Create formatter (force text format for chat for now)
	formatter, err := output.NewFormatter("text", os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	// Auto-save function
	autoSave := func() {
		if sessionMgr != nil && currentSession != nil {
			// Convert history to session format
			currentSession.Messages = make([]map[string]interface{}, len(history))
			for i, h := range history {
				parts := make([]map[string]interface{}, len(h.Parts))
				for j, p := range h.Parts {
					parts[j] = map[string]interface{}{"text": p.Text}
				}
				currentSession.Messages[i] = map[string]interface{}{
					"role":  h.Role,
					"parts": parts,
				}
			}
			currentSession.Tokens.Input = sessionTokens.input
			currentSession.Tokens.Output = sessionTokens.output
			currentSession.Model = effectiveModel
			sessionMgr.Save(currentSession)
		}
	}

	// If there is initial input, process it first
	if inputText != "" {
		userStyle := lipgloss.NewStyle().Foreground(accentBlue)
		fmt.Fprintln(os.Stderr, userStyle.Render("❯ "+strings.Split(inputText, "\n")[0]))
		if strings.Contains(inputText, "\n") {
			fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(dimGray).Render("  (+ file contents)"))
		}
		fmt.Fprintln(os.Stderr)

		err := processWithToolLoop(ctx, apiClient, projectID, effectiveModel, inputText, &history, formatter, toolRegistry, allowList)
		if err != nil {
			formatter.WriteError(err)
		}
		autoSave() // Auto-save after each interaction
	}

	// Start REPL
	replConfig := cli.REPLConfig{
		Prompt:          "❯ ",
		AvailableModels: AvailableModels,
		ToolNames:       toolRegistry.GetToolNames(),
		OnCommand: func(line string) (handled bool, exit bool) {
			switch strings.ToLower(strings.TrimSpace(line)) {
			case "/exit", "/quit", "/q":
				autoSave() // Save before exit
				displayStats(sessionTokens.input, sessionTokens.output, time.Since(startTime))
				return true, true // handled and exit
			case "/help", "/h":
				showHelp()
				return true, false
			case "/clear":
				history = nil
				fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(accentGreen).Render("✓ Conversation cleared"))
				return true, false
			case "/stats":
				displayStats(sessionTokens.input, sessionTokens.output, time.Since(startTime))
				return true, false
			case "/sessions":
				// List all sessions
				if sessionMgr == nil {
					fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("✗ Session management not available"))
					return true, false
				}
				sessions, err := sessionMgr.List()
				if err != nil {
					fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("✗ "+err.Error()))
					return true, false
				}
				if len(sessions) == 0 {
					fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(dimGray).Render("No saved sessions"))
					return true, false
				}
				fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(accentBlue).Bold(true).Render("📋 Saved Sessions"))
				for i, s := range sessions {
					if i >= 10 {
						fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(dimGray).Render(fmt.Sprintf("  ... and %d more", len(sessions)-10)))
						break
					}
					name := s.ID
					if s.Name != "" {
						name = s.Name + " (" + s.ID + ")"
					}
					current := ""
					if currentSession != nil && s.ID == currentSession.ID {
						current = " ← current"
					}
					fmt.Fprintf(os.Stderr, "  %s %s%s\n",
						lipgloss.NewStyle().Foreground(accentPurple).Render(name),
						lipgloss.NewStyle().Foreground(dimGray).Render(fmt.Sprintf("[%d msgs, %s]", len(s.Messages), s.UpdatedAt.Format("01/02 15:04"))),
						lipgloss.NewStyle().Foreground(accentGreen).Render(current))
				}
				return true, false
			default:
				// Check for /model command
				if line == "/model" || strings.HasPrefix(strings.ToLower(line), "/model ") {
					parts := strings.Fields(line)
					if len(parts) == 1 {
						// /model without argument - show current model and available models
						fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(accentPurple).Bold(true).Render("Current model: "+effectiveModel))
						fmt.Fprintf(os.Stderr, "Available models: %s\n", strings.Join(AvailableModels, ", "))
						fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(dimGray).Render("Usage: /model <model-name>"))
					} else if len(parts) == 2 {
						newModel := parts[1]
						// Validate model
						valid := false
						for _, m := range AvailableModels {
							if m == newModel {
								valid = true
								break
							}
						}
						if valid {
							effectiveModel = newModel
							if currentSession != nil {
								currentSession.Model = effectiveModel
							}
							displayHeader(effectiveModel, yoloMode)
							fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(accentGreen).Render("✓ Model switched to "+newModel))
						} else {
							fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("✗ Invalid model: "+newModel))
							fmt.Fprintf(os.Stderr, "Available models: %s\n", strings.Join(AvailableModels, ", "))
						}
					} else {
						fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(dimGray).Render("Usage: /model <model-name>"))
					}
					return true, false
				}

				// Check for /save command
				if line == "/save" || strings.HasPrefix(strings.ToLower(line), "/save ") {
					if sessionMgr == nil || currentSession == nil {
						fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("✗ Session management not available"))
						return true, false
					}
					parts := strings.Fields(line)
					if len(parts) == 2 {
						currentSession.Name = parts[1]
					}
					autoSave()
					msg := "✓ Session saved: " + currentSession.ID
					if currentSession.Name != "" {
						msg = "✓ Session saved as: " + currentSession.Name
					}
					fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(accentGreen).Render(msg))
					return true, false
				}

				// Check for /load command
				if strings.HasPrefix(strings.ToLower(line), "/load ") {
					if sessionMgr == nil {
						fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("✗ Session management not available"))
						return true, false
					}
					parts := strings.Fields(line)
					if len(parts) != 2 {
						fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(dimGray).Render("Usage: /load <session-id-or-name>"))
						return true, false
					}
					loadedSession, err := sessionMgr.Load(parts[1])
					if err != nil {
						fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("✗ "+err.Error()))
						return true, false
					}
					// Restore session
					history = nil
					for _, msg := range loadedSession.Messages {
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
						history = append(history, content)
					}
					currentSession = loadedSession
					sessionTokens.input = loadedSession.Tokens.Input
					sessionTokens.output = loadedSession.Tokens.Output
					effectiveModel = loadedSession.Model
					fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(accentGreen).Render("✓ Loaded session: "+loadedSession.ID))
					fmt.Fprintln(os.Stderr, lipgloss.NewStyle().Foreground(dimGray).Render(fmt.Sprintf("  Messages: %d, Model: %s", len(history), effectiveModel)))
					fmt.Fprintln(os.Stderr)
					displayConversationHistory(history)
					return true, false
				}

				return false, false
			}
		},
		OnInput: func(line string) {
			err := processWithToolLoop(ctx, apiClient, projectID, effectiveModel, line, &history, formatter, toolRegistry, allowList)
			if err != nil {
				formatter.WriteError(err)
			}
			autoSave() // Auto-save after each interaction
		},
		OnExit: func() {
			autoSave() // Save on exit
			displayStats(sessionTokens.input, sessionTokens.output, time.Since(startTime))
		},
	}

	return cli.StartREPL(replConfig)
}

// showHelp displays available commands
func showHelp() {
	helpStyle := lipgloss.NewStyle().Foreground(dimGray)
	cmdStyle := lipgloss.NewStyle().Foreground(accentPurple).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(accentBlue).Bold(true)

	fmt.Fprintln(os.Stderr)

	// Commands section
	fmt.Fprintln(os.Stderr, sectionStyle.Render("📋 Commands"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", cmdStyle.Render("/help, /h    "), helpStyle.Render("Show this help"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", cmdStyle.Render("/exit, /q    "), helpStyle.Render("Exit and show stats"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", cmdStyle.Render("/clear       "), helpStyle.Render("Clear conversation history"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", cmdStyle.Render("/stats       "), helpStyle.Render("Show token usage stats"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", cmdStyle.Render("/model       "), helpStyle.Render("Show/switch model (e.g., /model gemini-2.5-flash)"))
	fmt.Fprintln(os.Stderr)

	// Sessions section
	fmt.Fprintln(os.Stderr, sectionStyle.Render("💾 Sessions"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", cmdStyle.Render("/sessions    "), helpStyle.Render("List saved sessions"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", cmdStyle.Render("/save [name] "), helpStyle.Render("Save current session (optional name)"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", cmdStyle.Render("/load <id>   "), helpStyle.Render("Load a saved session"))
	fmt.Fprintln(os.Stderr)

	// Tools section
	fmt.Fprintln(os.Stderr, sectionStyle.Render("🔧 Available Tools"))
	toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true)
	fmt.Fprintf(os.Stderr, "  %s  %s\n", toolStyle.Render("read_file        "), helpStyle.Render("Read file contents"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", toolStyle.Render("write_file       "), helpStyle.Render("Write to file (requires confirmation)"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", toolStyle.Render("edit_file        "), helpStyle.Render("Edit file (requires confirmation)"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", toolStyle.Render("list_directory   "), helpStyle.Render("List directory contents"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", toolStyle.Render("glob             "), helpStyle.Render("Find files by pattern"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", toolStyle.Render("search_file      "), helpStyle.Render("Search text in files"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", toolStyle.Render("web_search       "), helpStyle.Render("Search the web"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", toolStyle.Render("web_fetch        "), helpStyle.Render("Fetch web page (requires confirmation)"))
	fmt.Fprintf(os.Stderr, "  %s  %s\n", toolStyle.Render("shell            "), helpStyle.Render("Run shell command (requires confirmation)"))
	fmt.Fprintln(os.Stderr)

	// Tips section
	fmt.Fprintln(os.Stderr, sectionStyle.Render("💡 Tips"))
	fmt.Fprintf(os.Stderr, "  %s\n", helpStyle.Render("• Sessions auto-save after each message"))
	fmt.Fprintf(os.Stderr, "  %s\n", helpStyle.Render("• Resume with: gmn chat -r last"))
	fmt.Fprintf(os.Stderr, "  %s\n", helpStyle.Render("• Use --yolo to skip all confirmations"))
	fmt.Fprintf(os.Stderr, "  %s\n", helpStyle.Render("• Press Ctrl+C to exit with stats"))
	fmt.Fprintf(os.Stderr, "  %s\n", helpStyle.Render("• Use -p flag for initial prompt"))
	fmt.Fprintln(os.Stderr)
}

// generateStreamWithFallback attempts to stream with fallback models on retryable errors
func generateStreamWithFallback(
	ctx context.Context,
	client *api.Client,
	req *api.GenerateRequest,
	modelName string,
) (<-chan api.StreamEvent, string, error) {
	fallbackModels := GetFallbackModels(modelName)

	for attempt, fallback := range fallbackModels {
		if attempt > 0 {
			req.Model = fallback
			if debug {
				fmt.Fprintf(os.Stderr, "Falling back to model: %s\n", fallback)
			}
		}

		stream, err := client.GenerateStream(ctx, req)
		if err != nil {
			if isRetryableError(err) && attempt < len(fallbackModels)-1 {
				if debug {
					fmt.Fprintf(os.Stderr, "Model %s failed: %v, trying fallback...\n", req.Model, err)
				}
				continue
			}
			return nil, req.Model, err
		}
		return stream, req.Model, nil
	}
	return nil, modelName, fmt.Errorf("all fallback models failed")
}

// processWithToolLoop handles a chat request with automatic tool execution
func processWithToolLoop(
	ctx context.Context,
	client *api.Client,
	projectID string,
	modelName string,
	text string,
	history *[]api.Content,
	formatter output.Formatter,
	toolRegistry *tools.Registry,
	allowList *confirmation.AllowList,
) error {
	const maxIterations = 10

	// Add user message to history
	*history = append(*history, api.Content{
		Role:  "user",
		Parts: []api.Part{{Text: text}},
	})

	// Helper to revert on failure
	historyLenBefore := len(*history)
	success := false
	defer func() {
		if !success {
			// Revert all changes to history
			*history = (*history)[:historyLenBefore-1]
		}
	}()

	for i := 0; i < maxIterations; i++ {
		// Generate user prompt ID
		userPromptID := fmt.Sprintf("gmn-chat-%d-%d", time.Now().UnixNano(), i)

		// Build request with tools
		req := &api.GenerateRequest{
			Model:        modelName,
			Project:      projectID,
			UserPromptID: userPromptID,
			Request: api.InnerRequest{
				Contents: *history,
				Config: api.GenerationConfig{
					Temperature:     1.0,
					TopP:            0.95,
					MaxOutputTokens: 8192,
				},
				Tools: toolRegistry.GetTools(),
			},
		}

		// Create a context with timeout for this request
		reqCtx, cancel := context.WithTimeout(ctx, timeout)

		// Start spinner while waiting for response
		spin := newSpinner("Thinking...")
		spin.Start()

		// Stream response with fallback
		stream, usedModel, err := generateStreamWithFallback(reqCtx, client, req, modelName)
		if err != nil {
			spin.Stop()
			cancel()
			return err
		}

		// Update model name if fallback was used
		if usedModel != modelName {
			modelName = usedModel
		}

		var fullResponse strings.Builder
		var pendingToolCallParts []*api.Part // Store full Parts with thought_signature for Gemini 3 Pro
		spinnerStopped := false

		for event := range stream {
			// Stop spinner on first content
			if !spinnerStopped {
				spin.Stop()
				spinnerStopped = true
			}

			if event.Type == "error" {
				cancel()
				return fmt.Errorf(event.Error)
			}

			// Track token usage
			if event.Type == "done" && event.Usage != nil {
				sessionTokens.input += event.Usage.PromptTokenCount
				sessionTokens.output += event.Usage.CandidatesTokenCount
			}

			// Handle tool calls
			if event.Type == "tool_call" && event.ToolCall != nil {
				// Use ToolCallPart if available (contains thought_signature), otherwise create Part
				if event.ToolCallPart != nil {
					pendingToolCallParts = append(pendingToolCallParts, event.ToolCallPart)
				} else {
					pendingToolCallParts = append(pendingToolCallParts, &api.Part{FunctionCall: event.ToolCall})
				}
				// Display tool call notification (OpenCode style)
				displayToolCall(event.ToolCall)
				continue
			}

			// Stream text content
			if err := formatter.WriteStreamEvent(&event); err != nil {
				cancel()
				return err
			}

			if event.Text != "" {
				fullResponse.WriteString(event.Text)
			}
		}

		// Ensure spinner is stopped
		if !spinnerStopped {
			spin.Stop()
		}

		cancel()

		// If no tool calls, we're done
		if len(pendingToolCallParts) == 0 {
			// Add model response to history
			*history = append(*history, api.Content{
				Role:  "model",
				Parts: []api.Part{{Text: fullResponse.String()}},
			})
			success = true
			return nil
		}

		// Execute tool calls
		for _, fcPart := range pendingToolCallParts {
			fc := fcPart.FunctionCall
			// Generate a response ID (use FunctionCall ID if present, otherwise generate one)
			responseID := fc.ID
			if responseID == "" {
				responseID = fmt.Sprintf("%s-%d", fc.Name, time.Now().UnixNano())
			}

			tool, ok := toolRegistry.Get(fc.Name)
			if !ok {
				// Unknown tool - add error response (preserve thought_signature)
				*history = append(*history,
					api.Content{
						Role:  "model",
						Parts: []api.Part{*fcPart}, // Use full Part with thought_signature
					},
					api.Content{
						Role: "user",
						Parts: []api.Part{{FunctionResp: &api.FunctionResp{
							ID:       responseID,
							Name:     fc.Name,
							Response: map[string]interface{}{"error": "unknown tool: " + fc.Name},
						}}},
					},
				)
				continue
			}

			// Check if confirmation is required
			if tool.RequiresConfirmation() && !allowList.IsAllowed(fc.Name) {
				outcome, err := promptToolConfirmation(tool, fc.Args)
				if err != nil {
					return fmt.Errorf("confirmation error: %w", err)
				}

				switch outcome {
				case confirmation.OutcomeCancel:
					// User cancelled - add cancelled response (preserve thought_signature)
					*history = append(*history,
						api.Content{
							Role:  "model",
							Parts: []api.Part{*fcPart}, // Use full Part with thought_signature
						},
						api.Content{
							Role: "user",
							Parts: []api.Part{{FunctionResp: &api.FunctionResp{
								ID:       responseID,
								Name:     fc.Name,
								Response: map[string]interface{}{"error": "operation cancelled by user"},
							}}},
						},
					)
					continue

				case confirmation.OutcomeProceedAlways:
					allowList.Allow(fc.Name)
				}
			}

			// Execute the tool
			result, err := tool.Execute(fc.Args)
			if err != nil {
				result = map[string]interface{}{"error": err.Error()}
			}

			// Display result (OpenCode style)
			displayToolResult(tool, result)

			// Add tool call and response to history (preserve thought_signature for Gemini 3 Pro)
			*history = append(*history,
				api.Content{
					Role:  "model",
					Parts: []api.Part{*fcPart}, // Use full Part with thought_signature
				},
				api.Content{
					Role: "user",
					Parts: []api.Part{{FunctionResp: &api.FunctionResp{
						ID:       responseID,
						Name:     fc.Name,
						Response: result,
					}}},
				},
			)
		}

		// Continue the loop to get the model's response after tool execution
	}

	return fmt.Errorf("max tool iterations (%d) reached", maxIterations)
}

// promptToolConfirmation shows a confirmation prompt for a tool
func promptToolConfirmation(tool tools.BuiltinTool, args map[string]interface{}) (confirmation.Outcome, error) {
	details := confirmation.Details{
		Type:     confirmation.ConfirmationType(tool.ConfirmationType()),
		Title:    fmt.Sprintf("Allow %s?", tool.DisplayName()),
		ToolName: tool.Name(),
		Args:     args,
	}

	// Get file path if available
	if path, ok := args["path"].(string); ok {
		details.FilePath = path
	}

	// Get URL if available (for web_fetch)
	if urlStr, ok := args["url"].(string); ok {
		details.URL = urlStr
	}

	// Get command if available (for shell)
	if cmd, ok := args["command"].(string); ok {
		details.Command = cmd
	}

	// For edit confirmations, try to get diff content
	if tool.ConfirmationType() == "edit" {
		if getter, ok := tool.(interface {
			GetOriginalContent(map[string]interface{}) (string, error)
			GetNewContent(map[string]interface{}) (string, error)
		}); ok {
			if orig, err := getter.GetOriginalContent(args); err == nil {
				details.OriginalContent = orig
			}
			if newC, err := getter.GetNewContent(args); err == nil {
				details.NewContent = newC
			}
		}
	}

	return confirmation.PromptConfirmation(details)
}

// displayToolCall shows a stylish tool call notification
func displayToolCall(fc *api.FunctionCall) {
	// OpenCode style
	var argsPreview string
	if path, ok := fc.Args["path"].(string); ok {
		argsPreview = path
	} else if pattern, ok := fc.Args["pattern"].(string); ok {
		argsPreview = pattern
	} else if url, ok := fc.Args["url"].(string); ok {
		argsPreview = url
	} else if cmd, ok := fc.Args["command"].(string); ok {
		if len(cmd) > 40 {
			argsPreview = cmd[:37] + "..."
		} else {
			argsPreview = cmd
		}
	} else if query, ok := fc.Args["query"].(string); ok {
		if len(query) > 40 {
			argsPreview = query[:37] + "..."
		} else {
			argsPreview = query
		}
	}

	header := toolCallStyle.Render("⚡ TOOL")
	name := toolNameStyle.Render(fc.Name)

	if argsPreview != "" {
		argStyle := lipgloss.NewStyle().Foreground(dimGray)
		fmt.Fprintf(os.Stderr, "\n%s %s %s\n", header, name, argStyle.Render("→ "+argsPreview))
	} else {
		fmt.Fprintf(os.Stderr, "\n%s %s\n", header, name)
	}
}

// displayToolResult shows a stylish tool result notification
func displayToolResult(tool tools.BuiltinTool, result map[string]interface{}) {
	// OpenCode style
	successStyle := lipgloss.NewStyle().Foreground(accentGreen).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(dimGray)

	// Check for error
	if errMsg, hasErr := result["error"].(string); hasErr {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true)
		fmt.Fprintf(os.Stderr, "   %s %s\n\n", errorStyle.Render("✗"), dimStyle.Render(errMsg))
		return
	}

	// Success with brief info
	var info string
	if count, ok := result["count"].(int); ok {
		info = fmt.Sprintf("(%d items)", count)
	} else if msg, ok := result["message"].(string); ok {
		if len(msg) > 50 {
			info = msg[:47] + "..."
		} else {
			info = msg
		}
	}

	if info != "" {
		fmt.Fprintf(os.Stderr, "   %s %s %s\n\n",
			successStyle.Render("✓"),
			tool.DisplayName(),
			dimStyle.Render(info))
	} else {
		fmt.Fprintf(os.Stderr, "   %s %s\n\n",
			successStyle.Render("✓"),
			tool.DisplayName())
	}
}
