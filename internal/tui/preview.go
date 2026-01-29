// Package tui provides a full-featured terminal user interface for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// DiffLine represents a line in the diff
type DiffLine struct {
	Type    DiffLineType
	Content string
	OldNum  int
	NewNum  int
}

// DiffLineType represents the type of diff line
type DiffLineType int

const (
	DiffLineContext DiffLineType = iota
	DiffLineAdded
	DiffLineRemoved
	DiffLineHeader
)

// FilePreviewModel represents the file preview/diff component
type FilePreviewModel struct {
	viewport    viewport.Model
	width       int
	height      int
	visible     bool
	title       string
	filePath    string
	previewType PreviewType
	content     string
	diffLines   []DiffLine
	lineNumbers bool
	syntaxHl    bool
	oldContent  string
	newContent  string
}

// PreviewType represents the type of preview
type PreviewType int

const (
	PreviewTypeFile PreviewType = iota
	PreviewTypeDiff
	PreviewTypeCommand
	PreviewTypeOutput
)

// NewFilePreviewModel creates a new file preview model
func NewFilePreviewModel() FilePreviewModel {
	vp := viewport.New(60, 20)
	vp.MouseWheelEnabled = true

	return FilePreviewModel{
		viewport:    vp,
		visible:     false,
		lineNumbers: true,
		syntaxHl:    true,
	}
}

// SetSize sets the preview dimensions
func (f *FilePreviewModel) SetSize(width, height int) {
	f.width = width
	f.height = height
	f.viewport.Width = width - 4
	f.viewport.Height = height - 4
	f.updateContent()
}

// Show shows the preview
func (f *FilePreviewModel) Show() {
	f.visible = true
}

// Hide hides the preview
func (f *FilePreviewModel) Hide() {
	f.visible = false
}

// IsVisible returns visibility state
func (f *FilePreviewModel) IsVisible() bool {
	return f.visible
}

// Toggle toggles visibility
func (f *FilePreviewModel) Toggle() {
	f.visible = !f.visible
}

// SetFilePreview sets a file preview
func (f *FilePreviewModel) SetFilePreview(title, path, content string) {
	f.previewType = PreviewTypeFile
	f.title = title
	f.filePath = path
	f.content = content
	f.updateContent()
}

// SetDiffPreview sets a diff preview
func (f *FilePreviewModel) SetDiffPreview(title, path, oldContent, newContent string) {
	f.previewType = PreviewTypeDiff
	f.title = title
	f.filePath = path
	f.oldContent = oldContent
	f.newContent = newContent
	f.diffLines = computeDiff(oldContent, newContent)
	f.updateContent()
}

// SetCommandPreview sets a command preview
func (f *FilePreviewModel) SetCommandPreview(command, explanation string) {
	f.previewType = PreviewTypeCommand
	f.title = "Command"
	f.content = command
	f.oldContent = explanation
	f.updateContent()
}

// SetOutputPreview sets an output preview
func (f *FilePreviewModel) SetOutputPreview(title, output string) {
	f.previewType = PreviewTypeOutput
	f.title = title
	f.content = output
	f.updateContent()
}

// ScrollUp scrolls up
func (f *FilePreviewModel) ScrollUp(lines int) {
	f.viewport.LineUp(lines)
}

// ScrollDown scrolls down
func (f *FilePreviewModel) ScrollDown(lines int) {
	f.viewport.LineDown(lines)
}

// updateContent updates the viewport content
func (f *FilePreviewModel) updateContent() {
	var content string

	switch f.previewType {
	case PreviewTypeFile:
		content = f.renderFileContent()
	case PreviewTypeDiff:
		content = f.renderDiffContent()
	case PreviewTypeCommand:
		content = f.renderCommandContent()
	case PreviewTypeOutput:
		content = f.renderOutputContent()
	}

	f.viewport.SetContent(content)
}

// renderFileContent renders file content
func (f *FilePreviewModel) renderFileContent() string {
	lines := strings.Split(f.content, "\n")
	var b strings.Builder

	lineNumWidth := len(fmt.Sprintf("%d", len(lines)))

	for i, line := range lines {
		if f.lineNumbers {
			lineNum := lipgloss.NewStyle().
				Foreground(DimTextColor).
				Width(lineNumWidth).
				Align(lipgloss.Right).
				Render(fmt.Sprintf("%d", i+1))
			b.WriteString(lineNum)
			b.WriteString(" │ ")
		}

		// Syntax highlight (basic)
		highlighted := f.highlightLine(line, f.filePath)
		b.WriteString(highlighted)
		b.WriteString("\n")
	}

	return b.String()
}

// renderDiffContent renders diff content
func (f *FilePreviewModel) renderDiffContent() string {
	var b strings.Builder

	oldLineNumWidth := len(fmt.Sprintf("%d", countLines(f.oldContent)))
	newLineNumWidth := len(fmt.Sprintf("%d", countLines(f.newContent)))

	for _, line := range f.diffLines {
		var prefix string
		var style lipgloss.Style
		var lineNumStyle lipgloss.Style

		switch line.Type {
		case DiffLineContext:
			prefix = "  "
			style = lipgloss.NewStyle().Foreground(TextColor)
			lineNumStyle = lipgloss.NewStyle().Foreground(DimTextColor)
		case DiffLineAdded:
			prefix = "+ "
			style = lipgloss.NewStyle().Foreground(SuccessColor).Background(lipgloss.Color("#0d3321"))
			lineNumStyle = lipgloss.NewStyle().Foreground(SuccessColor)
		case DiffLineRemoved:
			prefix = "- "
			style = lipgloss.NewStyle().Foreground(DangerColor).Background(lipgloss.Color("#3d1515"))
			lineNumStyle = lipgloss.NewStyle().Foreground(DangerColor)
		case DiffLineHeader:
			prefix = ""
			style = lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
			lineNumStyle = style
		}

		// Line numbers
		if line.Type != DiffLineHeader {
			oldNum := " "
			newNum := " "
			if line.OldNum > 0 {
				oldNum = fmt.Sprintf("%d", line.OldNum)
			}
			if line.NewNum > 0 {
				newNum = fmt.Sprintf("%d", line.NewNum)
			}

			lineNums := fmt.Sprintf("%*s %*s │",
				oldLineNumWidth, lineNumStyle.Render(oldNum),
				newLineNumWidth, lineNumStyle.Render(newNum),
			)
			b.WriteString(lipgloss.NewStyle().Foreground(BorderColor).Render(lineNums))
		}

		b.WriteString(style.Render(prefix + line.Content))
		b.WriteString("\n")
	}

	return b.String()
}

// renderCommandContent renders command content
func (f *FilePreviewModel) renderCommandContent() string {
	var b strings.Builder

	// Command
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(WarningColor).Render("$ "))
	b.WriteString(lipgloss.NewStyle().Foreground(TextColor).Render(f.content))
	b.WriteString("\n\n")

	// Explanation
	if f.oldContent != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(DimTextColor).Italic(true).Render(f.oldContent))
	}

	return b.String()
}

// renderOutputContent renders output content
func (f *FilePreviewModel) renderOutputContent() string {
	return lipgloss.NewStyle().Foreground(TextColor).Render(f.content)
}

// highlightLine does basic syntax highlighting
func (f *FilePreviewModel) highlightLine(line, path string) string {
	if !f.syntaxHl {
		return line
	}

	// Keywords
	keywords := []string{"func", "def", "class", "if", "else", "for", "while", "return", "import", "from", "package", "var", "const", "let", "type", "struct", "interface"}
	for _, kw := range keywords {
		line = strings.ReplaceAll(line, " "+kw+" ", " "+lipgloss.NewStyle().Foreground(AccentColor).Bold(true).Render(kw)+" ")
		if strings.HasPrefix(line, kw+" ") {
			line = lipgloss.NewStyle().Foreground(AccentColor).Bold(true).Render(kw) + line[len(kw):]
		}
	}

	// Strings
	if strings.Contains(line, `"`) || strings.Contains(line, "'") {
		// Basic string highlighting (not perfect but functional)
	}

	// Comments
	if idx := strings.Index(line, "//"); idx != -1 {
		before := line[:idx]
		comment := line[idx:]
		line = before + lipgloss.NewStyle().Foreground(DimTextColor).Italic(true).Render(comment)
	}
	if idx := strings.Index(line, "#"); idx != -1 && !strings.HasPrefix(strings.TrimSpace(line), "#!/") {
		before := line[:idx]
		comment := line[idx:]
		line = before + lipgloss.NewStyle().Foreground(DimTextColor).Italic(true).Render(comment)
	}

	return line
}

// View renders the file preview
func (f FilePreviewModel) View() string {
	if !f.visible {
		return ""
	}

	var b strings.Builder

	// Title bar
	titleIcon := "📄"
	switch f.previewType {
	case PreviewTypeDiff:
		titleIcon = "📝"
	case PreviewTypeCommand:
		titleIcon = "💻"
	case PreviewTypeOutput:
		titleIcon = "📤"
	}

	title := fmt.Sprintf("%s %s", titleIcon, f.title)
	if f.filePath != "" {
		title += " • " + lipgloss.NewStyle().Foreground(DimTextColor).Render(f.filePath)
	}

	titleBar := lipgloss.NewStyle().
		Background(SurfaceColor).
		Foreground(TextColor).
		Bold(true).
		Padding(0, 1).
		Width(f.width - 2).
		Render(title)

	b.WriteString(titleBar)
	b.WriteString("\n")

	// Content
	b.WriteString(f.viewport.View())

	// Scroll indicator
	if f.viewport.TotalLineCount() > f.viewport.VisibleLineCount() {
		percent := int(f.viewport.ScrollPercent() * 100)
		scrollInfo := lipgloss.NewStyle().
			Foreground(DimTextColor).
			Render(fmt.Sprintf("─── %d%% ───", percent))
		b.WriteString("\n")
		b.WriteString(scrollInfo)
	}

	// Hints
	hints := lipgloss.NewStyle().
		Foreground(DimTextColor).
		Render("↑↓:scroll  q:close")
	b.WriteString("\n")
	b.WriteString(hints)

	return FilePreviewStyle.Width(f.width).Height(f.height).Render(b.String())
}

// computeDiff computes a simple diff between two contents
func computeDiff(oldContent, newContent string) []DiffLine {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diff []DiffLine

	// Header
	diff = append(diff, DiffLine{
		Type:    DiffLineHeader,
		Content: "Changes:",
	})

	// Simple line-by-line diff (LCS would be better but more complex)
	oldIdx, newIdx := 0, 0
	oldNum, newNum := 1, 1

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		if oldIdx >= len(oldLines) {
			// Rest is additions
			diff = append(diff, DiffLine{
				Type:    DiffLineAdded,
				Content: newLines[newIdx],
				NewNum:  newNum,
			})
			newIdx++
			newNum++
		} else if newIdx >= len(newLines) {
			// Rest is deletions
			diff = append(diff, DiffLine{
				Type:    DiffLineRemoved,
				Content: oldLines[oldIdx],
				OldNum:  oldNum,
			})
			oldIdx++
			oldNum++
		} else if oldLines[oldIdx] == newLines[newIdx] {
			// Same line
			diff = append(diff, DiffLine{
				Type:    DiffLineContext,
				Content: oldLines[oldIdx],
				OldNum:  oldNum,
				NewNum:  newNum,
			})
			oldIdx++
			newIdx++
			oldNum++
			newNum++
		} else {
			// Different - show removal then addition
			diff = append(diff, DiffLine{
				Type:    DiffLineRemoved,
				Content: oldLines[oldIdx],
				OldNum:  oldNum,
			})
			diff = append(diff, DiffLine{
				Type:    DiffLineAdded,
				Content: newLines[newIdx],
				NewNum:  newNum,
			})
			oldIdx++
			newIdx++
			oldNum++
			newNum++
		}
	}

	return diff
}

// countLines counts lines in content
func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

// =============================================================================
// File Preview Style
// =============================================================================

var FilePreviewStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(AccentColor).
	Padding(0, 1)
