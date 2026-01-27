package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/linkalls/gmn/internal/api"
	"github.com/linkalls/gmn/internal/input"
	"github.com/linkalls/gmn/internal/output"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat session",
	RunE:  runChat,
}

func init() {
	rootCmd.AddCommand(chatCmd)

	chatCmd.Flags().StringVarP(&model, "model", "m", "gemini-2.5-flash", "Model to use")
	chatCmd.Flags().StringArrayVarP(&files, "file", "f", nil, "Files to include in context")
	chatCmd.Flags().DurationVarP(&timeout, "timeout", "t", 5*time.Minute, "API timeout")
	chatCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug output")
}

func runChat(cmd *cobra.Command, args []string) error {
	// For chat, we don't want a short timeout context for the whole session.
	// We'll use a background context for setup, and per-request timeout.
	ctx := context.Background()

	// Initial prompt from args
	initialPrompt := ""
	if len(args) > 0 {
		initialPrompt = strings.Join(args, " ")
	}

	// Setup client (reusing logic from root.go)
	apiClient, projectID, err := setupClient(ctx)
	if err != nil {
		return err
	}

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
		err := processChatRequest(ctx, apiClient, projectID, inputText, &history, formatter)
		if err != nil {
			formatter.WriteError(err)
		}
	}

	// Start REPL
	scanner := bufio.NewScanner(os.Stdin)
	if inputText != "" || initialPrompt == "" {
		// Only print prompt if we are ready for user input (or if we just finished initial input)
		// If initial input was from pipe, scanner.Scan() will return false immediately, so this prompt won't matter much.
		// But for interactive mode:
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

		// Process request
		err := processChatRequest(ctx, apiClient, projectID, line, &history, formatter)
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

func processChatRequest(ctx context.Context, client *api.Client, projectID string, text string, history *[]api.Content, formatter output.Formatter) error {
	// Add user message to history
	*history = append(*history, api.Content{
		Role:  "user",
		Parts: []api.Part{{Text: text}},
	})

	// Helper to revert on failure
	success := false
	defer func() {
		if !success {
			// Revert user message to maintain alternating roles
			if len(*history) > 0 {
				*history = (*history)[:len(*history)-1]
			}
		}
	}()

	// Generate user prompt ID
	userPromptID := fmt.Sprintf("gmn-chat-%d", time.Now().UnixNano())

	// Build request
	req := &api.GenerateRequest{
		Model:        model,
		Project:      projectID,
		UserPromptID: userPromptID,
		Request: api.InnerRequest{
			Contents: *history,
			Config: api.GenerationConfig{
				Temperature:     1.0,
				TopP:            0.95,
				MaxOutputTokens: 8192,
			},
		},
	}

	// Create a context with timeout for this request
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Stream response
	stream, err := client.GenerateStream(reqCtx, req)
	if err != nil {
		return err
	}

	var fullResponse strings.Builder

	for event := range stream {
		if event.Type == "error" {
			return fmt.Errorf(event.Error)
		}

		// For chat, we stream to stdout
		if err := formatter.WriteStreamEvent(&event); err != nil {
			return err
		}

		if event.Text != "" {
			fullResponse.WriteString(event.Text)
		}
	}

	// Add model response to history
	*history = append(*history, api.Content{
		Role:  "model",
		Parts: []api.Part{{Text: fullResponse.String()}},
	})

	success = true
	return nil
}
