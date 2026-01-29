// Package tools provides built-in tool implementations for the Gemini CLI.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"encoding/json"

	"github.com/linkalls/gmn/internal/api"
)

// BuiltinTool defines the interface for built-in tools
type BuiltinTool interface {
	// Name returns the tool name used in function calls
	Name() string
	// DisplayName returns the human-readable name for display
	DisplayName() string
	// Description returns the tool description
	Description() string
	// Parameters returns the JSON schema for the tool's parameters
	Parameters() json.RawMessage
	// Execute runs the tool with the given arguments
	Execute(args map[string]interface{}) (map[string]interface{}, error)
	// RequiresConfirmation returns whether this tool needs user confirmation
	RequiresConfirmation() bool
	// ConfirmationType returns the type of confirmation needed (edit, exec, etc.)
	ConfirmationType() string
}

// Registry holds all registered tools
type Registry struct {
	tools   map[string]BuiltinTool
	rootDir string
}

// NewRegistry creates a new tool registry
func NewRegistry(rootDir string) *Registry {
	r := &Registry{
		tools:   make(map[string]BuiltinTool),
		rootDir: rootDir,
	}
	r.registerBuiltins()
	return r
}

// registerBuiltins registers all built-in tools
func (r *Registry) registerBuiltins() {
	// File system tools
	r.Register(&ReadFileTool{rootDir: r.rootDir})
	r.Register(&WriteFileTool{rootDir: r.rootDir})
	r.Register(&ListDirectoryTool{rootDir: r.rootDir})
	r.Register(&GlobTool{rootDir: r.rootDir})
	r.Register(&SearchFileContentTool{rootDir: r.rootDir})
	r.Register(&EditFileTool{rootDir: r.rootDir})

	// Web tools
	r.Register(&WebSearchTool{})
	r.Register(&WebFetchTool{})

	// Shell tool
	r.Register(&ShellTool{rootDir: r.rootDir})
}

// Register adds a tool to the registry
func (r *Registry) Register(tool BuiltinTool) {
	r.tools[tool.Name()] = tool
}

// Get returns a tool by name
func (r *Registry) Get(name string) (BuiltinTool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// GetAll returns all registered tools
func (r *Registry) GetAll() []BuiltinTool {
	result := make([]BuiltinTool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// GetFunctionDeclarations returns API-compatible function declarations for all tools
func (r *Registry) GetFunctionDeclarations() []api.FunctionDecl {
	decls := make([]api.FunctionDecl, 0, len(r.tools))
	for _, tool := range r.tools {
		decls = append(decls, api.FunctionDecl{
			Name:        tool.Name(),
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
		})
	}
	return decls
}

// GetTools returns API-compatible Tool slice for request
func (r *Registry) GetTools() []api.Tool {
	return []api.Tool{
		{FunctionDeclarations: r.GetFunctionDeclarations()},
	}
}

// GetToolNames returns all registered tool names for completion
func (r *Registry) GetToolNames() []string {
	result := make([]string, 0, len(r.tools))
	for name := range r.tools {
		result = append(result, name)
	}
	return result
}
