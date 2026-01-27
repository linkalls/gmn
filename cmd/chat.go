package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/linkalls/gmn/internal/api"
	"github.com/linkalls/gmn/internal/confirmation"
	"github.com/linkalls/gmn/internal/input"
	"github.com/linkalls/gmn/internal/output"
	"github.com/linkalls/gmn/internal/tools"
	"github.com/spf13/cobra"
)

var (
	tuiTheme string // TUI theme: "opencode" or "minimal"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	RunE:  runChat,
}

// Tool execution styles (OpenCode-inspired)
var (
	// Modern accent colors
	accentPurple = lipgloss.Color("#7C3AED")
	accentGreen  = lipgloss.Color("#10B981")
	mutedGray    = lipgloss.Color("#6B7280")
	dimGray      = lipgloss.Color("#9CA3AF")

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
)

func init() {
	rootCmd.AddCommand(chatCmd)

	chatCmd.Flags().StringVarP(&model, "model", "m", "gemini-2.5-flash", "Model to use")
	chatCmd.Flags().StringArrayVarP(&files, "file", "f", nil, "Files to include in context")
	chatCmd.Flags().DurationVarP(&timeout, "timeout", "t", 5*time.Minute, "API timeout")
	chatCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug output")
	chatCmd.Flags().StringVar(&tuiTheme, "theme", "opencode", "TUI theme: opencode, minimal")
}

func runChat(cmd *cobra.Command, args []string) error {
	// Set TUI theme
	switch tuiTheme {
	case "minimal":
		confirmation.CurrentTheme = confirmation.ThemeMinimal
	default:
		confirmation.CurrentTheme = confirmation.ThemeOpenCode
	}

	// For chat, we don't want a short timeout context for the whole session.
	// We'll use a background context for setup, and per-request timeout.
	ctx := context.Background()

	// Initial prompt from args
	initialPrompt := ""
	if len(args) > 0 {
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

	// Initialize allow list for session
	allowList := confirmation.NewAllowList()

	// Prepare history
	var history []api.Content

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

	// If there is initial input, process it first
	if inputText != "" {
		fmt.Println("> " + strings.ReplaceAll(inputText, "\n", "\n  "))
		err := processWithToolLoop(ctx, apiClient, projectID, effectiveModel, inputText, &history, formatter, toolRegistry, allowList)
		if err != nil {
			formatter.WriteError(err)
		}
	}

	// Start REPL
	scanner := bufio.NewScanner(os.Stdin)
	if inputText != "" || initialPrompt == "" {
		fmt.Print("> ")
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Handle empty lines
		if strings.TrimSpace(line) == "" {
			fmt.Print("> ")
			continue
		}

		if line == "/exit" || line == "/quit" {
			break
		}

		// Process request with tool loop
		err := processWithToolLoop(ctx, apiClient, projectID, effectiveModel, line, &history, formatter, toolRegistry, allowList)
		if err != nil {
			formatter.WriteError(err)
		}

		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
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

		// Stream response with fallback
		stream, usedModel, err := generateStreamWithFallback(reqCtx, client, req, modelName)
		if err != nil {
			cancel()
			return err
		}

		// Update model name if fallback was used
		if usedModel != modelName {
			modelName = usedModel
		}

		var fullResponse strings.Builder
		var pendingToolCalls []*api.FunctionCall

		for event := range stream {
			if event.Type == "error" {
				cancel()
				return fmt.Errorf(event.Error)
			}

			// Handle tool calls
			if event.Type == "tool_call" && event.ToolCall != nil {
				pendingToolCalls = append(pendingToolCalls, event.ToolCall)
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

		cancel()

		// If no tool calls, we're done
		if len(pendingToolCalls) == 0 {
			// Add model response to history
			*history = append(*history, api.Content{
				Role:  "model",
				Parts: []api.Part{{Text: fullResponse.String()}},
			})
			success = true
			return nil
		}

		// Execute tool calls
		for _, fc := range pendingToolCalls {
			// Generate a response ID (use FunctionCall ID if present, otherwise generate one)
			responseID := fc.ID
			if responseID == "" {
				responseID = fmt.Sprintf("%s-%d", fc.Name, time.Now().UnixNano())
			}

			tool, ok := toolRegistry.Get(fc.Name)
			if !ok {
				// Unknown tool - add error response
				*history = append(*history,
					api.Content{
						Role:  "model",
						Parts: []api.Part{{FunctionCall: fc}},
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
					// User cancelled - add cancelled response
					*history = append(*history,
						api.Content{
							Role:  "model",
							Parts: []api.Part{{FunctionCall: fc}},
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

			// Add tool call and response to history
			*history = append(*history,
				api.Content{
					Role:  "model",
					Parts: []api.Part{{FunctionCall: fc}},
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

	// Use simple prompt if diff is not available or for non-edit types
	if details.OriginalContent == "" && details.NewContent == "" {
		return confirmation.PromptConfirmationSimple(details)
	}

	return confirmation.PromptConfirmation(details)
}

// displayToolCall shows a stylish tool call notification
func displayToolCall(fc *api.FunctionCall) {
	if tuiTheme == "minimal" {
		fmt.Fprintf(os.Stderr, "\n🔧 Calling: %s\n", fc.Name)
		return
	}

	// OpenCode style
	var argsPreview string
	if path, ok := fc.Args["path"].(string); ok {
		argsPreview = path
	} else if pattern, ok := fc.Args["pattern"].(string); ok {
		argsPreview = pattern
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
	if tuiTheme == "minimal" {
		fmt.Fprintf(os.Stderr, "   ✓ %s done\n\n", tool.DisplayName())
		return
	}

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
