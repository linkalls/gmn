// Package cmd provides the CLI commands for gmn.
// Copyright 2025 linkalls
// SPDX-License-Identifier: Apache-2.0
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/linkalls/gmn/internal/api"
	"github.com/linkalls/gmn/internal/auth"
	"github.com/linkalls/gmn/internal/config"
	"github.com/linkalls/gmn/internal/input"
	"github.com/linkalls/gmn/internal/output"
	"github.com/spf13/cobra"
)

// Model constants for tier-based defaults and fallback
const (
	// Default models based on tier
	ModelStandardDefault = "gemini-3-pro-preview" // For standard-tier (Pro subscription)
	ModelFreeDefault     = "gemini-2.5-flash"     // For free-tier
)

// AvailableModels defines all supported models for completion
var AvailableModels = []string{
	"gemini-3-pro-preview",
	"gemini-3-flash-preview",
	"gemini-2.5-flash",
	"gemini-2.5-pro",
}

// FallbackModels defines the fallback order when a model fails
var FallbackModels = []string{
	"gemini-3-pro-preview",
	"gemini-3-flash-preview",
	"gemini-2.5-flash",
}

var (
	version = "dev"

	prompt       string
	model        string
	outputFormat string
	files        []string
	timeout      time.Duration
	debug        bool
)

var rootCmd = &cobra.Command{
	Use:   "gmn [prompt]",
	Short: "A lightweight Gemini CLI",
	Long: `gmn is a lightweight Gemini CLI written in Go.
It uses Google OAuth authentication from ~/.gemini/ for API access.

Examples:
  gmn "Hello, world"
  gmn "Explain Go generics" -m gemini-2.5-pro
  cat file.go | gmn "Review this code"
  gmn "Add error handling" -f main.go`,
	RunE:    run,
	Version: version,
	Args:    cobra.MaximumNArgs(1),
}

func init() {
	rootCmd.Flags().StringVarP(&prompt, "prompt", "p", "", "Prompt to send to Gemini (required)")
	rootCmd.Flags().StringVarP(&model, "model", "m", "", "Model to use (default determined by tier)")
	rootCmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: text, json, stream-json")
	rootCmd.Flags().StringArrayVarP(&files, "file", "f", nil, "Files to include in context")
	rootCmd.Flags().DurationVarP(&timeout, "timeout", "t", 5*time.Minute, "API timeout")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug output")

	rootCmd.RegisterFlagCompletionFunc("model", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return AvailableModels, cobra.ShellCompDirectiveNoFileComp
	})

}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// SetVersion sets the version string
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

func run(cmd *cobra.Command, args []string) error {
	// Handle positional argument as prompt
	if len(args) > 0 {
		prompt = args[0]
	}
	// Setup context with timeout and signal handling
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create formatter
	formatter, err := output.NewFormatter(outputFormat, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	// Prepare input
	inputText, err := input.PrepareInput(prompt, files)
	if err != nil {
		formatter.WriteError(err)
		return err
	}

	if inputText == "" {
		err := fmt.Errorf("no input provided")
		formatter.WriteError(err)
		return err
	}

	apiClient, projectID, userTier, err := setupClient(ctx)
	if err != nil {
		formatter.WriteError(err)
		return err
	}

	// Apply tier-based default model if user didn't specify
	effectiveModel := getEffectiveModel(model, userTier, cmd.Flags().Changed("model"))

	// Generate a simple user prompt ID
	userPromptID := fmt.Sprintf("gmn-%d", time.Now().UnixNano())

	// Build request (Code Assist API format)
	req := &api.GenerateRequest{
		Model:        effectiveModel,
		Project:      projectID,
		UserPromptID: userPromptID,
		Request: api.InnerRequest{
			Contents: []api.Content{{
				Role:  "user",
				Parts: []api.Part{{Text: inputText}},
			}},
			Config: api.GenerationConfig{
				Temperature:     1.0,
				TopP:            0.95,
				MaxOutputTokens: 8192,
			},
		},
	}

	// Execute based on output format
	switch outputFormat {
	case "json":
		return runNonStreaming(ctx, apiClient, req, formatter)
	default:
		return runStreaming(ctx, apiClient, req, formatter)
	}
}

func runNonStreaming(ctx context.Context, client *api.Client, req *api.GenerateRequest, formatter output.Formatter) error {
	resp, err := client.Generate(ctx, req)
	if err != nil {
		formatter.WriteError(err)
		return err
	}
	return formatter.WriteResponse(resp)
}

func runStreaming(ctx context.Context, client *api.Client, req *api.GenerateRequest, formatter output.Formatter) error {
	return runStreamingWithFallback(ctx, client, req, formatter, GetFallbackModels(req.Model))
}

func runStreamingWithFallback(ctx context.Context, client *api.Client, req *api.GenerateRequest, formatter output.Formatter, fallbackModels []string) error {
	currentModel := req.Model

	for attempt, fallbackModel := range fallbackModels {
		if attempt > 0 {
			// Use fallback model
			currentModel = fallbackModel
			req.Model = currentModel
			if debug {
				fmt.Fprintf(os.Stderr, "Falling back to model: %s\n", currentModel)
			}
		}

		stream, err := client.GenerateStream(ctx, req)
		if err != nil {
			// Check if this is a retryable error (429, 503, model not available)
			if isRetryableError(err) && attempt < len(fallbackModels)-1 {
				if debug {
					fmt.Fprintf(os.Stderr, "Model %s failed: %v, trying fallback...\n", currentModel, err)
				}
				continue
			}
			formatter.WriteError(err)
			return err
		}

		hasError := false
		for event := range stream {
			if event.Type == "error" {
				// Check if this is a retryable error
				if isRetryableStreamError(event.Error) && attempt < len(fallbackModels)-1 {
					hasError = true
					if debug {
						fmt.Fprintf(os.Stderr, "Model %s stream error: %s, trying fallback...\n", currentModel, event.Error)
					}
					break
				}
				formatter.WriteError(fmt.Errorf(event.Error))
				return fmt.Errorf(event.Error)
			}
			if err := formatter.WriteStreamEvent(&event); err != nil {
				return err
			}
		}

		if !hasError {
			return nil
		}
	}

	return fmt.Errorf("all fallback models failed")
}

// isRetryableError checks if the error is retryable (rate limit, service unavailable, model not found, etc.)
func isRetryableError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "RESOURCE_EXHAUSTED") ||
		strings.Contains(errStr, "UNAVAILABLE") ||
		strings.Contains(errStr, "NOT_FOUND") ||
		strings.Contains(errStr, "model not found") ||
		strings.Contains(errStr, "Model not found")
}

// isRetryableStreamError checks if the stream error is retryable
func isRetryableStreamError(errStr string) bool {
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "RESOURCE_EXHAUSTED") ||
		strings.Contains(errStr, "UNAVAILABLE") ||
		strings.Contains(errStr, "NOT_FOUND") ||
		strings.Contains(errStr, "model not found") ||
		strings.Contains(errStr, "Model not found")
}

// getEffectiveModel returns the model to use based on tier and user preference
func getEffectiveModel(specifiedModel string, userTier string, userSpecified bool) string {
	// If user explicitly specified a model, use it
	if userSpecified {
		return specifiedModel
	}

	// Apply tier-based default
	switch userTier {
	case "standard-tier":
		if debug {
			fmt.Fprintf(os.Stderr, "Using tier-based default model: %s (tier: %s)\n", ModelStandardDefault, userTier)
		}
		return ModelStandardDefault
	default:
		// Free tier or unknown tier uses flash model
		if debug {
			fmt.Fprintf(os.Stderr, "Using default model: %s (tier: %s)\n", ModelFreeDefault, userTier)
		}
		return ModelFreeDefault
	}
}

// GetFallbackModels returns the fallback model list, starting from the specified model
func GetFallbackModels(currentModel string) []string {
	// Find current model in the fallback list
	startIdx := 0
	for i, m := range FallbackModels {
		if m == currentModel {
			startIdx = i
			break
		}
	}

	// Return models starting from current model's position
	if startIdx > 0 {
		return FallbackModels[startIdx:]
	}
	return FallbackModels
}

func setupClient(ctx context.Context) (*api.Client, string, string, error) {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to load config: %w", err)
	}
	_ = cfg // Will be used for MCP

	// Load credentials
	authMgr, err := auth.NewManager()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to initialize auth: %w", err)
	}

	creds, err := authMgr.LoadCredentials()
	if err != nil {
		return nil, "", "", err
	}

	// Refresh if expired
	if creds.IsExpired() {
		if debug {
			fmt.Fprintln(os.Stderr, "Token expired, refreshing...")
		}
		creds, err = authMgr.RefreshToken(creds)
		if err != nil {
			return nil, "", "", err
		}
	}

	// Create API client
	httpClient := authMgr.HTTPClient(creds)
	apiClient := api.NewClient(httpClient)

	// Try to load cached project ID first
	cachedState, _ := config.LoadCachedState()
	projectID := cachedState.ProjectID
	userTier := cachedState.UserTier

	// If no cached project ID, fetch from API
	if projectID == "" {
		if debug {
			fmt.Fprintln(os.Stderr, "Loading Code Assist status...")
		}
		loadResp, err := apiClient.LoadCodeAssist(ctx)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to load Code Assist: %w", err)
		}
		projectID = loadResp.CloudAICompanionProject

		// Cache the project ID for next time
		if loadResp.CurrentTier != nil {
			userTier = loadResp.CurrentTier.ID
		}
		_ = config.SaveCachedState(&config.CachedState{
			ProjectID: projectID,
			UserTier:  userTier,
		})

		if debug {
			fmt.Fprintf(os.Stderr, "Project ID: %s (cached)\n", projectID)
			if loadResp.CurrentTier != nil {
				fmt.Fprintf(os.Stderr, "Tier: %s\n", loadResp.CurrentTier.ID)
			}
		}
	} else if debug {
		fmt.Fprintf(os.Stderr, "Using cached Project ID: %s\n", projectID)
		fmt.Fprintf(os.Stderr, "Using cached Tier: %s\n", userTier)
	}

	return apiClient, projectID, userTier, nil
}
