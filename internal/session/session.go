// Package session provides session management for gmn chat.
// SPDX-License-Identifier: Apache-2.0
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session represents a chat session
type Session struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name,omitempty"`
	Model     string                   `json:"model"`
	CreatedAt time.Time                `json:"created_at"`
	UpdatedAt time.Time                `json:"updated_at"`
	Messages  []map[string]interface{} `json:"messages"`
	Tokens    TokenUsage               `json:"tokens"`
}

// TokenUsage tracks token usage
type TokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// Manager handles session operations
type Manager struct {
	sessionsDir string
	currentID   string
}

// NewManager creates a new session manager
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".gmn", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Manager{
		sessionsDir: sessionsDir,
	}, nil
}

// NewSession creates a new session with auto-generated ID
func (m *Manager) NewSession(model string) *Session {
	now := time.Now()
	id := now.Format("20060102-150405")
	m.currentID = id

	return &Session{
		ID:        id,
		Model:     model,
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []map[string]interface{}{},
		Tokens:    TokenUsage{},
	}
}

// Save saves a session to disk
func (m *Manager) Save(session *Session) error {
	session.UpdatedAt = time.Now()

	filename := session.ID + ".json"
	if session.Name != "" {
		// Also save with name as alias
		filename = session.ID + ".json"
	}

	path := filepath.Join(m.sessionsDir, filename)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	// If session has a name, create a symlink or alias file
	if session.Name != "" {
		aliasPath := filepath.Join(m.sessionsDir, session.Name+".json")
		// Remove existing alias if any
		os.Remove(aliasPath)
		// Create alias by copying (Windows doesn't support symlinks well)
		if err := os.WriteFile(aliasPath, data, 0644); err != nil {
			// Ignore alias creation errors
		}
	}

	return nil
}

// Load loads a session by ID or name
func (m *Manager) Load(idOrName string) (*Session, error) {
	// Try exact match first
	path := filepath.Join(m.sessionsDir, idOrName+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try to find by prefix
		matches, _ := filepath.Glob(filepath.Join(m.sessionsDir, idOrName+"*.json"))
		if len(matches) == 0 {
			return nil, fmt.Errorf("session not found: %s", idOrName)
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("multiple sessions match '%s', be more specific", idOrName)
		}
		path = matches[0]
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	m.currentID = session.ID
	return &session, nil
}

// LoadLatest loads the most recent session
func (m *Manager) LoadLatest() (*Session, error) {
	sessions, err := m.List()
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}

	return m.Load(sessions[0].ID)
}

// List returns all sessions sorted by update time (newest first)
func (m *Manager) List() ([]*Session, error) {
	files, err := os.ReadDir(m.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*Session
	seen := make(map[string]bool)

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}

		path := filepath.Join(m.sessionsDir, f.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		// Skip duplicates (aliases)
		if seen[session.ID] {
			continue
		}
		seen[session.ID] = true

		sessions = append(sessions, &session)
	}

	// Sort by update time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Delete removes a session
func (m *Manager) Delete(idOrName string) error {
	session, err := m.Load(idOrName)
	if err != nil {
		return err
	}

	// Remove main file
	path := filepath.Join(m.sessionsDir, session.ID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Remove alias if exists
	if session.Name != "" {
		aliasPath := filepath.Join(m.sessionsDir, session.Name+".json")
		os.Remove(aliasPath)
	}

	return nil
}

// GetCurrentID returns the current session ID
func (m *Manager) GetCurrentID() string {
	return m.currentID
}

// Rename renames a session
func (m *Manager) Rename(idOrName, newName string) error {
	session, err := m.Load(idOrName)
	if err != nil {
		return err
	}

	// Remove old alias if exists
	if session.Name != "" {
		oldAliasPath := filepath.Join(m.sessionsDir, session.Name+".json")
		os.Remove(oldAliasPath)
	}

	session.Name = newName
	return m.Save(session)
}
