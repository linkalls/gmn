// Package tools provides built-in tool implementations for the Gemini CLI.
// Copyright 2025 linkalls
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// =============================================================================
// WebSearchTool - Search the web
// =============================================================================

// WebSearchTool performs web searches using DuckDuckGo
type WebSearchTool struct{}

func (t *WebSearchTool) Name() string        { return "web_search" }
func (t *WebSearchTool) DisplayName() string { return "GoogleSearch" }
func (t *WebSearchTool) Description() string {
	return "Search the web using Google and return relevant results. Use this to find current information, documentation, or answers to questions."
}

func (t *WebSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "The search query to find information on the web"
			}
		},
		"required": ["query"]
	}`)
}

func (t *WebSearchTool) RequiresConfirmation() bool { return false }
func (t *WebSearchTool) ConfirmationType() string   { return "" }

func (t *WebSearchTool) Execute(args map[string]interface{}) (map[string]interface{}, error) {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return map[string]interface{}{"error": "query is required and cannot be empty"}, nil
	}

	results, err := t.searchDuckDuckGo(query)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("search failed: %v", err)}, nil
	}

	return map[string]interface{}{
		"query":   query,
		"results": results,
		"count":   len(results),
	}, nil
}

func (t *WebSearchTool) searchDuckDuckGo(query string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if i >= 10 {
			return
		}

		title := s.Find(".result__title a").Text()
		link, _ := s.Find(".result__title a").Attr("href")
		snippet := s.Find(".result__snippet").Text()

		// DuckDuckGo wraps URLs in a redirect, extract the actual URL
		if strings.Contains(link, "uddg=") {
			if u, err := url.Parse(link); err == nil {
				if uddg := u.Query().Get("uddg"); uddg != "" {
					link = uddg
				}
			}
		}

		if title != "" && link != "" {
			results = append(results, map[string]interface{}{
				"title":   strings.TrimSpace(title),
				"url":     link,
				"snippet": strings.TrimSpace(snippet),
			})
		}
	})

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found for query: %s", query)
	}

	return results, nil
}

// =============================================================================
// WebFetchTool - Fetch content from URLs
// =============================================================================

// WebFetchTool fetches and extracts content from web pages
type WebFetchTool struct{}

func (t *WebFetchTool) Name() string        { return "web_fetch" }
func (t *WebFetchTool) DisplayName() string { return "WebFetch" }
func (t *WebFetchTool) Description() string {
	return "Fetch and extract the main content from a URL. Use this to read web pages, documentation, or articles."
}

func (t *WebFetchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "The URL to fetch content from (must start with http:// or https://)"
			},
			"selector": {
				"type": "string",
				"description": "Optional CSS selector to extract specific content"
			}
		},
		"required": ["url"]
	}`)
}

func (t *WebFetchTool) RequiresConfirmation() bool { return true }
func (t *WebFetchTool) ConfirmationType() string   { return "fetch" }

func (t *WebFetchTool) Execute(args map[string]interface{}) (map[string]interface{}, error) {
	urlStr, ok := args["url"].(string)
	if !ok || strings.TrimSpace(urlStr) == "" {
		return map[string]interface{}{"error": "url is required and cannot be empty"}, nil
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return map[string]interface{}{"error": "url must be a valid HTTP or HTTPS URL"}, nil
	}

	// Convert GitHub blob URLs to raw URLs
	if strings.Contains(urlStr, "github.com") && strings.Contains(urlStr, "/blob/") {
		urlStr = strings.Replace(urlStr, "github.com", "raw.githubusercontent.com", 1)
		urlStr = strings.Replace(urlStr, "/blob/", "/", 1)
	}

	selector, _ := args["selector"].(string)

	content, title, err := t.fetchURL(urlStr, selector)
	if err != nil {
		return map[string]interface{}{"error": fmt.Sprintf("failed to fetch URL: %v", err)}, nil
	}

	return map[string]interface{}{
		"url":     urlStr,
		"title":   title,
		"content": content,
	}, nil
}

func (t *WebFetchTool) fetchURL(urlStr, selector string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")

	// For non-HTML content, return raw text
	if !strings.Contains(contentType, "text/html") {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 100000))
		if err != nil {
			return "", "", err
		}
		return string(body), "", nil
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", "", err
	}

	title := doc.Find("title").First().Text()

	// Remove unwanted elements
	doc.Find("script, style, nav, footer, header, aside, .sidebar, .nav, .menu, .ads").Remove()

	var content string
	if selector != "" {
		content = t.extractText(doc.Find(selector))
	} else {
		// Try common content selectors
		for _, sel := range []string{"article", "main", "[role=main]", ".content", ".post-content", "#content"} {
			if text := t.extractText(doc.Find(sel)); text != "" {
				content = text
				break
			}
		}
		if content == "" {
			content = t.extractText(doc.Find("body"))
		}
	}

	// Truncate if too long
	if len(content) > 50000 {
		content = content[:50000] + "\n\n[Content truncated...]"
	}

	return strings.TrimSpace(content), strings.TrimSpace(title), nil
}

func (t *WebFetchTool) extractText(s *goquery.Selection) string {
	var lines []string
	s.Find("p, h1, h2, h3, h4, h5, h6, li, pre, code, blockquote").Each(func(i int, el *goquery.Selection) {
		text := strings.TrimSpace(el.Text())
		if text != "" {
			nodeName := goquery.NodeName(el)
			switch nodeName {
			case "h1":
				text = "# " + text
			case "h2":
				text = "## " + text
			case "h3":
				text = "### " + text
			case "li":
				text = "- " + text
			case "pre", "code":
				text = "```\n" + text + "\n```"
			}
			lines = append(lines, text)
		}
	})

	result := strings.Join(lines, "\n\n")
	re := regexp.MustCompile(`\n{3,}`)
	return re.ReplaceAllString(result, "\n\n")
}
