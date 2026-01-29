// Package cli provides CLI utilities for gmn.
// Copyright 2025 linkalls
// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/linkalls/gmn/internal/tools"
	"github.com/peterh/liner"
)

// REPLConfig holds configuration for the REPL
type REPLConfig struct {
	Prompt          string
	AvailableModels []string
	ToolNames       []string
	OnCommand       func(line string) (handled bool, exit bool) // Return handled=true if command, exit=true to quit
	OnInput         func(line string)                           // Handle regular input
	OnExit          func()                                      // Called on exit
}

// StartREPL starts an interactive REPL with completion and history
func StartREPL(config REPLConfig) error {
	line := liner.NewLiner()
	defer line.Close()

	// Enable Ctrl+C to abort current input (not exit)
	line.SetCtrlCAborts(true)

	// Set completer
	line.SetCompleter(func(line string) []string {
		// Split line into words
		words := strings.Fields(line)
		if len(words) == 0 {
			return nil
		}

		lastWord := words[len(words)-1]

		// If starting with /model, complete models
		if len(words) >= 2 && words[0] == "/model" && len(words) == 2 {
			var matches []string
			for _, model := range config.AvailableModels {
				if strings.HasPrefix(model, lastWord) {
					matches = append(matches, model)
				}
			}
			return matches
		}

		// If starting with /, complete commands
		if strings.HasPrefix(lastWord, "/") {
			commands := []string{"/help", "/exit", "/quit", "/clear", "/stats", "/model", "/sessions", "/save", "/load"}
			var matches []string
			for _, cmd := range commands {
				if strings.HasPrefix(cmd, lastWord) {
					matches = append(matches, cmd)
				}
			}
			return matches
		}

		// Otherwise, no completion
		return nil
	})

	// Load history
	if f, err := os.Open(".gmn_history"); err == nil {
		line.ReadHistory(f)
		f.Close()
	}

	for {
		input, err := line.Prompt(config.Prompt)
		if err != nil {
			if err == liner.ErrPromptAborted {
				// Ctrl+C pressed - show message and exit gracefully
				fmt.Fprintln(os.Stderr) // New line after ^C
				break
			}
			// EOF (Ctrl+D) - exit
			if err.Error() == "EOF" {
				fmt.Fprintln(os.Stderr)
				break
			}
			return err
		}

		line.AppendHistory(input)

		line := strings.TrimSpace(input)
		if line == "" {
			continue
		}

		// Check if it's a command
		if config.OnCommand != nil {
			handled, exit := config.OnCommand(line)
			if exit {
				break
			}
			if handled {
				continue
			}
		}

		// Handle regular input
		if config.OnInput != nil {
			config.OnInput(line)
		}
	}

	// Save history
	if f, err := os.Create(".gmn_history"); err == nil {
		line.WriteHistory(f)
		f.Close()
	}

	if config.OnExit != nil {
		config.OnExit()
	}

	return nil
}

// GetToolNamesFromRegistry extracts tool names from registry
func GetToolNamesFromRegistry(registry *tools.Registry) []string {
	return registry.GetToolNames()
}
