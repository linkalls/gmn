// Package tools provides built-in tool implementations for the Gemini CLI.
// Copyright 2025 linkalls
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// =============================================================================
// ReadFileTool - Read file contents
// =============================================================================

// ReadFileTool reads file contents
type ReadFileTool struct {
	rootDir string
}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) DisplayName() string { return "ReadFile" }
func (t *ReadFileTool) Description() string {
	return "Read the contents of a file at the specified path. Use this when you need to examine the contents of an existing file."
}

func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "The path of the file to read (relative to working directory or absolute)"
			}
		},
		"required": ["path"]
	}`)
}

func (t *ReadFileTool) RequiresConfirmation() bool { return false }
func (t *ReadFileTool) ConfirmationType() string   { return "" }

func (t *ReadFileTool) Execute(args map[string]interface{}) (map[string]interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return map[string]interface{}{"error": "path is required and must be a string"}, nil
	}

	fullPath := t.resolvePath(path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to read file: %v", err)}, nil
	}

	return map[string]interface{}{
		"content": string(content),
		"path":    fullPath,
	}, nil
}

func (t *ReadFileTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.rootDir, path)
}

// =============================================================================
// WriteFileTool - Write file contents
// =============================================================================

// WriteFileTool writes content to a file
type WriteFileTool struct {
	rootDir string
}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) DisplayName() string { return "WriteFile" }
func (t *WriteFileTool) Description() string {
	return "Write content to a file at the specified path. If the file exists, it will be overwritten. If it doesn't exist, it will be created."
}

func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "The path of the file to write"
			},
			"content": {
				"type": "string",
				"description": "The content to write to the file"
			}
		},
		"required": ["path", "content"]
	}`)
}

func (t *WriteFileTool) RequiresConfirmation() bool { return true }
func (t *WriteFileTool) ConfirmationType() string   { return "edit" }

func (t *WriteFileTool) Execute(args map[string]interface{}) (map[string]interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return map[string]interface{}{"error": "path is required and must be a string"}, nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return map[string]interface{}{"error": "content is required and must be a string"}, nil
	}

	fullPath := t.resolvePath(path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to create directory: %v", err)}, nil
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to write file: %v", err)}, nil
	}

	return map[string]interface{}{
		"success": true,
		"path":    fullPath,
		"message": fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), fullPath),
	}, nil
}

func (t *WriteFileTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.rootDir, path)
}

// GetOriginalContent returns the current content of a file (for diff display)
func (t *WriteFileTool) GetOriginalContent(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}
	fullPath := t.resolvePath(path)
	content, err := os.ReadFile(fullPath)
	if os.IsNotExist(err) {
		return "", nil // New file
	}
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// GetNewContent returns the content that will be written
func (t *WriteFileTool) GetNewContent(args map[string]interface{}) (string, error) {
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}
	return content, nil
}

// =============================================================================
// ListDirectoryTool - List directory contents
// =============================================================================

// ListDirectoryTool lists the contents of a directory
type ListDirectoryTool struct {
	rootDir string
}

func (t *ListDirectoryTool) Name() string        { return "list_directory" }
func (t *ListDirectoryTool) DisplayName() string { return "ReadFolder" }
func (t *ListDirectoryTool) Description() string {
	return "List the contents of a directory. Returns file and subdirectory names."
}

func (t *ListDirectoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "The path of the directory to list (relative to working directory or absolute)"
			}
		},
		"required": ["path"]
	}`)
}

func (t *ListDirectoryTool) RequiresConfirmation() bool { return false }
func (t *ListDirectoryTool) ConfirmationType() string   { return "" }

func (t *ListDirectoryTool) Execute(args map[string]interface{}) (map[string]interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return map[string]interface{}{"error": "path is required and must be a string"}, nil
	}

	fullPath := t.resolvePath(path)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to read directory: %v", err)}, nil
	}

	files := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, map[string]interface{}{
			"name":  entry.Name(),
			"isDir": entry.IsDir(),
			"size":  info.Size(),
		})
	}

	return map[string]interface{}{
		"path":    fullPath,
		"entries": files,
	}, nil
}

func (t *ListDirectoryTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.rootDir, path)
}

// =============================================================================
// GlobTool - Find files matching a pattern
// =============================================================================

// GlobTool finds files matching a glob pattern
type GlobTool struct {
	rootDir string
}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) DisplayName() string { return "FindFiles" }
func (t *GlobTool) Description() string {
	return "Find files matching a glob pattern. Supports wildcards like *, **, and ?."
}

func (t *GlobTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "The glob pattern to match (e.g., '**/*.go', 'src/*.ts')"
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *GlobTool) RequiresConfirmation() bool { return false }
func (t *GlobTool) ConfirmationType() string   { return "" }

func (t *GlobTool) Execute(args map[string]interface{}) (map[string]interface{}, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return map[string]interface{}{"error": "pattern is required and must be a string"}, nil
	}

	var matches []string

	// Handle ** pattern by walking the directory tree
	if strings.Contains(pattern, "**") {
		matches = t.globRecursive(pattern)
	} else {
		fullPattern := filepath.Join(t.rootDir, pattern)
		var err error
		matches, err = filepath.Glob(fullPattern)
		if err != nil {
			return map[string]interface{}{"error": fmt.Sprintf("invalid pattern: %v", err)}, nil
		}
	}

	// Convert to relative paths
	relMatches := make([]string, 0, len(matches))
	for _, m := range matches {
		rel, err := filepath.Rel(t.rootDir, m)
		if err != nil {
			rel = m
		}
		relMatches = append(relMatches, rel)
	}

	return map[string]interface{}{
		"pattern": pattern,
		"matches": relMatches,
		"count":   len(relMatches),
	}, nil
}

func (t *GlobTool) globRecursive(pattern string) []string {
	var matches []string

	// Split pattern at **
	parts := strings.Split(pattern, "**")
	if len(parts) != 2 {
		// For simplicity, only handle one ** in the pattern
		return matches
	}

	prefix := strings.TrimSuffix(parts[0], string(filepath.Separator))
	suffix := strings.TrimPrefix(parts[1], string(filepath.Separator))

	startDir := t.rootDir
	if prefix != "" {
		startDir = filepath.Join(t.rootDir, prefix)
	}

	filepath.Walk(startDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		// Check if the file matches the suffix pattern
		if suffix == "" {
			matches = append(matches, path)
		} else {
			matched, _ := filepath.Match(suffix, filepath.Base(path))
			if matched {
				matches = append(matches, path)
			}
		}
		return nil
	})

	return matches
}

// =============================================================================
// SearchFileContentTool - Search for text in files
// =============================================================================

// SearchFileContentTool searches for text content in files
type SearchFileContentTool struct {
	rootDir string
}

func (t *SearchFileContentTool) Name() string        { return "search_file_content" }
func (t *SearchFileContentTool) DisplayName() string { return "SearchText" }
func (t *SearchFileContentTool) Description() string {
	return "Search for text or regex pattern in files. Returns matching lines with context."
}

func (t *SearchFileContentTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "The text or regex pattern to search for"
			},
			"path": {
				"type": "string",
				"description": "The file or directory path to search in"
			},
			"regex": {
				"type": "boolean",
				"description": "Whether to treat pattern as regex (default: false)"
			}
		},
		"required": ["pattern", "path"]
	}`)
}

func (t *SearchFileContentTool) RequiresConfirmation() bool { return false }
func (t *SearchFileContentTool) ConfirmationType() string   { return "" }

func (t *SearchFileContentTool) Execute(args map[string]interface{}) (map[string]interface{}, error) {
	pattern, ok := args["pattern"].(string)
	if !ok {
		return map[string]interface{}{"error": "pattern is required and must be a string"}, nil
	}

	path, ok := args["path"].(string)
	if !ok {
		return map[string]interface{}{"error": "path is required and must be a string"}, nil
	}

	isRegex, _ := args["regex"].(bool)

	fullPath := t.resolvePath(path)

	var re *regexp.Regexp
	var err error
	if isRegex {
		re, err = regexp.Compile(pattern)
		if err != nil {
			return map[string]interface{}{"error": fmt.Sprintf("invalid regex: %v", err)}, nil
		}
	}

	results := make([]map[string]interface{}, 0)

	info, err := os.Stat(fullPath)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("path not found: %v", err)}, nil
	}

	if info.IsDir() {
		// Search in directory
		filepath.Walk(fullPath, func(filePath string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			matches := t.searchInFile(filePath, pattern, re)
			results = append(results, matches...)
			return nil
		})
	} else {
		// Search in single file
		results = t.searchInFile(fullPath, pattern, re)
	}

	return map[string]interface{}{
		"pattern": pattern,
		"matches": results,
		"count":   len(results),
	}, nil
}

func (t *SearchFileContentTool) searchInFile(filePath, pattern string, re *regexp.Regexp) []map[string]interface{} {
	var results []map[string]interface{}

	file, err := os.Open(filePath)
	if err != nil {
		return results
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		var matched bool
		if re != nil {
			matched = re.MatchString(line)
		} else {
			matched = strings.Contains(line, pattern)
		}

		if matched {
			rel, _ := filepath.Rel(t.rootDir, filePath)
			results = append(results, map[string]interface{}{
				"file": rel,
				"line": lineNum,
				"text": line,
			})
		}
	}

	return results
}

func (t *SearchFileContentTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.rootDir, path)
}

// =============================================================================
// EditFileTool - Edit specific parts of a file
// =============================================================================

// EditFileTool edits specific parts of a file (search and replace)
type EditFileTool struct {
	rootDir string
}

func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) DisplayName() string { return "Edit" }
func (t *EditFileTool) Description() string {
	return "Edit a file by replacing specific text. Provide the old text to find and the new text to replace it with."
}

func (t *EditFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {
				"type": "string",
				"description": "The path of the file to edit"
			},
			"old_text": {
				"type": "string",
				"description": "The exact text to find and replace"
			},
			"new_text": {
				"type": "string",
				"description": "The text to replace with"
			}
		},
		"required": ["path", "old_text", "new_text"]
	}`)
}

func (t *EditFileTool) RequiresConfirmation() bool { return true }
func (t *EditFileTool) ConfirmationType() string   { return "edit" }

func (t *EditFileTool) Execute(args map[string]interface{}) (map[string]interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return map[string]interface{}{"error": "path is required and must be a string"}, nil
	}

	oldText, ok := args["old_text"].(string)
	if !ok {
		return map[string]interface{}{"error": "old_text is required and must be a string"}, nil
	}

	newText, ok := args["new_text"].(string)
	if !ok {
		return map[string]interface{}{"error": "new_text is required and must be a string"}, nil
	}

	fullPath := t.resolvePath(path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to read file: %v", err)}, nil
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, oldText) {
		return map[string]interface{}{"error": "old_text not found in file"}, nil
	}

	newContent := strings.Replace(contentStr, oldText, newText, 1)

	if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to write file: %v", err)}, nil
	}

	return map[string]interface{}{
		"success": true,
		"path":    fullPath,
		"message": "Successfully edited file",
	}, nil
}

func (t *EditFileTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.rootDir, path)
}

// GetOriginalContent returns the current content of a file (for diff display)
func (t *EditFileTool) GetOriginalContent(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}
	fullPath := t.resolvePath(path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// GetNewContent returns the content after edit (for diff display)
func (t *EditFileTool) GetNewContent(args map[string]interface{}) (string, error) {
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}

	oldText, ok := args["old_text"].(string)
	if !ok {
		return "", fmt.Errorf("old_text is required")
	}

	newText, ok := args["new_text"].(string)
	if !ok {
		return "", fmt.Errorf("new_text is required")
	}

	fullPath := t.resolvePath(path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	return strings.Replace(string(content), oldText, newText, 1), nil
}
