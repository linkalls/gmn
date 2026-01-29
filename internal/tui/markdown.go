// Package tui provides a full-featured terminal user interface for gmn.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// MarkdownRenderer renders markdown content with syntax highlighting
type MarkdownRenderer struct {
	width int
}

// NewMarkdownRenderer creates a new markdown renderer
func NewMarkdownRenderer(width int) *MarkdownRenderer {
	return &MarkdownRenderer{
		width: width,
	}
}

// SetWidth sets the render width
func (r *MarkdownRenderer) SetWidth(width int) {
	r.width = width
}

// Render renders markdown content
func (r *MarkdownRenderer) Render(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	var inCodeBlock bool
	var codeBlockLang string
	var codeBlockContent []string

	for _, line := range lines {
		// Check for code block start/end
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of code block
				codeContent := strings.Join(codeBlockContent, "\n")
				rendered := r.renderCodeBlock(codeContent, codeBlockLang)
				result = append(result, rendered)
				codeBlockContent = nil
				inCodeBlock = false
			} else {
				// Start of code block
				codeBlockLang = strings.TrimPrefix(line, "```")
				codeBlockLang = strings.TrimSpace(codeBlockLang)
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			codeBlockContent = append(codeBlockContent, line)
			continue
		}

		// Process markdown elements
		result = append(result, r.renderLine(line))
	}

	// Handle unclosed code block
	if inCodeBlock && len(codeBlockContent) > 0 {
		codeContent := strings.Join(codeBlockContent, "\n")
		rendered := r.renderCodeBlock(codeContent, codeBlockLang)
		result = append(result, rendered)
	}

	return strings.Join(result, "\n")
}

// renderLine renders a single markdown line
func (r *MarkdownRenderer) renderLine(line string) string {
	// Headers
	if strings.HasPrefix(line, "### ") {
		content := strings.TrimPrefix(line, "### ")
		return lipgloss.NewStyle().Bold(true).Foreground(InfoColor).Render("### " + content)
	}
	if strings.HasPrefix(line, "## ") {
		content := strings.TrimPrefix(line, "## ")
		return lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Render("## " + content)
	}
	if strings.HasPrefix(line, "# ") {
		content := strings.TrimPrefix(line, "# ")
		return lipgloss.NewStyle().Bold(true).Foreground(AccentColor).Underline(true).Render("# " + content)
	}

	// Bullet points
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		prefix := line[:2]
		content := line[2:]
		return lipgloss.NewStyle().Foreground(WarningColor).Render(prefix) + r.renderInline(content)
	}

	// Numbered lists
	if matched, _ := regexp.MatchString(`^\d+\. `, line); matched {
		idx := strings.Index(line, ". ")
		num := line[:idx+2]
		content := line[idx+2:]
		return lipgloss.NewStyle().Foreground(WarningColor).Render(num) + r.renderInline(content)
	}

	// Blockquotes
	if strings.HasPrefix(line, "> ") {
		content := strings.TrimPrefix(line, "> ")
		return lipgloss.NewStyle().Foreground(DimTextColor).Italic(true).Render("│ " + content)
	}

	// Horizontal rule
	if line == "---" || line == "***" || line == "___" {
		return lipgloss.NewStyle().Foreground(BorderColor).Render(strings.Repeat("─", r.width))
	}

	// Regular line with inline formatting
	return r.renderInline(line)
}

// renderInline renders inline markdown elements
func (r *MarkdownRenderer) renderInline(text string) string {
	// Bold **text** or __text__
	boldRe := regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
	text = boldRe.ReplaceAllStringFunc(text, func(match string) string {
		content := strings.Trim(match, "*_")
		return lipgloss.NewStyle().Bold(true).Render(content)
	})

	// Italic *text* or _text_
	italicRe := regexp.MustCompile(`(?:^|[^*_])\*([^*]+?)\*(?:[^*_]|$)|(?:^|[^*_])_([^_]+?)_(?:[^*_]|$)`)
	text = italicRe.ReplaceAllStringFunc(text, func(match string) string {
		// Simple italic detection
		if strings.Contains(match, "*") || strings.Contains(match, "_") {
			content := strings.Trim(match, "*_ ")
			return lipgloss.NewStyle().Italic(true).Render(content)
		}
		return match
	})

	// Inline code `text`
	codeRe := regexp.MustCompile("`([^`]+)`")
	text = codeRe.ReplaceAllStringFunc(text, func(match string) string {
		content := strings.Trim(match, "`")
		return lipgloss.NewStyle().
			Background(SurfaceColor).
			Foreground(WarningColor).
			Render(content)
	})

	// Links [text](url)
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	text = linkRe.ReplaceAllStringFunc(text, func(match string) string {
		// Extract text and url
		parts := linkRe.FindStringSubmatch(match)
		if len(parts) >= 3 {
			linkText := parts[1]
			url := parts[2]
			return lipgloss.NewStyle().Foreground(InfoColor).Underline(true).Render(linkText) +
				lipgloss.NewStyle().Foreground(DimTextColor).Render(" ("+url+")")
		}
		return match
	})

	return text
}

// renderCodeBlock renders a code block with syntax highlighting
func (r *MarkdownRenderer) renderCodeBlock(content, lang string) string {
	// Header with language
	header := ""
	if lang != "" {
		header = lipgloss.NewStyle().
			Foreground(DimTextColor).
			Background(SurfaceColor).
			Padding(0, 1).
			Render("  " + lang)
	}

	// Apply basic syntax highlighting based on language
	highlighted := r.highlightCode(content, lang)

	// Box style for code
	codeStyle := lipgloss.NewStyle().
		Background(SurfaceColor).
		Padding(0, 1).
		Width(r.width - 4)

	if header != "" {
		return header + "\n" + codeStyle.Render(highlighted)
	}
	return codeStyle.Render(highlighted)
}

// highlightCode applies basic syntax highlighting
func (r *MarkdownRenderer) highlightCode(code, lang string) string {
	lines := strings.Split(code, "\n")
	var result []string

	for _, line := range lines {
		highlighted := r.highlightLine(line, lang)
		result = append(result, highlighted)
	}

	return strings.Join(result, "\n")
}

// highlightLine highlights a single line of code
func (r *MarkdownRenderer) highlightLine(line, lang string) string {
	// Define styles for syntax elements
	keywordStyle := lipgloss.NewStyle().Foreground(AccentColor)
	stringStyle := lipgloss.NewStyle().Foreground(SuccessColor)
	commentStyle := lipgloss.NewStyle().Foreground(DimTextColor).Italic(true)
	numberStyle := lipgloss.NewStyle().Foreground(WarningColor)
	funcStyle := lipgloss.NewStyle().Foreground(InfoColor)

	// Common keywords for various languages
	keywords := []string{
		// Go
		"func", "return", "if", "else", "for", "range", "switch", "case", "default",
		"package", "import", "var", "const", "type", "struct", "interface", "map",
		"chan", "go", "defer", "select", "break", "continue", "fallthrough", "goto",
		"nil", "true", "false", "iota", "append", "make", "new", "len", "cap",
		// JavaScript/TypeScript
		"function", "async", "await", "class", "extends", "constructor", "this",
		"let", "const", "var", "export", "import", "from", "require", "module",
		"try", "catch", "finally", "throw", "typeof", "instanceof", "in", "of",
		"null", "undefined", "NaN", "Infinity",
		// Python
		"def", "class", "self", "lambda", "with", "as", "yield", "assert",
		"pass", "raise", "except", "finally", "global", "nonlocal", "del",
		"and", "or", "not", "is", "in", "None", "True", "False",
		// Rust
		"fn", "let", "mut", "pub", "impl", "trait", "struct", "enum",
		"match", "loop", "while", "mod", "use", "crate", "super", "self",
		"Self", "move", "ref", "where", "unsafe", "dyn", "Box", "Vec",
		// Common
		"string", "int", "bool", "float", "void", "any", "object",
	}

	result := line

	// Highlight comments (simple detection)
	if strings.Contains(line, "//") && !strings.Contains(line, "://") {
		idx := strings.Index(line, "//")
		before := line[:idx]
		comment := line[idx:]
		result = before + commentStyle.Render(comment)
		return result
	}
	if strings.HasPrefix(strings.TrimSpace(line), "#") && lang == "python" {
		return commentStyle.Render(line)
	}

	// Highlight strings (simple detection for double quotes)
	stringRe := regexp.MustCompile(`"[^"]*"`)
	result = stringRe.ReplaceAllStringFunc(result, func(s string) string {
		return stringStyle.Render(s)
	})

	// Highlight single-quoted strings
	singleStringRe := regexp.MustCompile(`'[^']*'`)
	result = singleStringRe.ReplaceAllStringFunc(result, func(s string) string {
		return stringStyle.Render(s)
	})

	// Highlight numbers
	numRe := regexp.MustCompile(`\b\d+(\.\d+)?\b`)
	result = numRe.ReplaceAllStringFunc(result, func(s string) string {
		return numberStyle.Render(s)
	})

	// Highlight keywords
	for _, kw := range keywords {
		kwRe := regexp.MustCompile(`\b` + kw + `\b`)
		result = kwRe.ReplaceAllStringFunc(result, func(s string) string {
			return keywordStyle.Render(s)
		})
	}

	// Highlight function calls (word followed by parenthesis)
	funcRe := regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\(`)
	result = funcRe.ReplaceAllStringFunc(result, func(s string) string {
		name := strings.TrimSuffix(s, "(")
		return funcStyle.Render(name) + "("
	})

	return result
}
