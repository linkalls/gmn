// Package output provides output formatting for geminimini.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/tomohiro-owada/gmn/internal/api"
)

// Formatter is the interface for output formatters
type Formatter interface {
	WriteResponse(resp *api.GenerateResponse) error
	WriteStreamEvent(event *api.StreamEvent) error
	WriteError(err error) error
}

// NewFormatter creates a formatter for the given format
func NewFormatter(format string, w io.Writer, errW io.Writer) (Formatter, error) {
	switch format {
	case "text":
		return &TextFormatter{w: w, errW: errW}, nil
	case "json":
		return &JSONFormatter{w: w, errW: errW}, nil
	case "stream-json":
		return &StreamJSONFormatter{w: w, errW: errW}, nil
	default:
		return nil, fmt.Errorf("unknown output format: %s", format)
	}
}

// TextFormatter outputs plain text (streaming)
type TextFormatter struct {
	w    io.Writer
	errW io.Writer
}

func (f *TextFormatter) WriteResponse(resp *api.GenerateResponse) error {
	if len(resp.Response.Candidates) > 0 && len(resp.Response.Candidates[0].Content.Parts) > 0 {
		text := resp.Response.Candidates[0].Content.Parts[0].Text
		_, err := fmt.Fprintln(f.w, text)
		return err
	}
	return nil
}

func (f *TextFormatter) WriteStreamEvent(event *api.StreamEvent) error {
	if event.Text != "" {
		_, err := fmt.Fprint(f.w, event.Text)
		return err
	}
	if event.Type == "done" {
		// Add final newline
		_, err := fmt.Fprintln(f.w)
		return err
	}
	return nil
}

func (f *TextFormatter) WriteError(err error) error {
	_, writeErr := fmt.Fprintf(f.errW, "Error: %v\n", err)
	return writeErr
}

// JSONFormatter outputs structured JSON (non-streaming)
type JSONFormatter struct {
	w    io.Writer
	errW io.Writer
}

// JSONResponse is the JSON output structure
type JSONResponse struct {
	Model        string             `json:"model"`
	Response     string             `json:"response"`
	Usage        *api.UsageMetadata `json:"usage,omitempty"`
	FinishReason string             `json:"finishReason,omitempty"`
}

// JSONError is the JSON error structure
type JSONError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (f *JSONFormatter) WriteResponse(resp *api.GenerateResponse) error {
	out := JSONResponse{}
	if resp.Response.UsageMetadata.TotalTokenCount > 0 {
		out.Usage = &resp.Response.UsageMetadata
	}
	if len(resp.Response.Candidates) > 0 {
		out.FinishReason = resp.Response.Candidates[0].FinishReason
		if len(resp.Response.Candidates[0].Content.Parts) > 0 {
			out.Response = resp.Response.Candidates[0].Content.Parts[0].Text
		}
	}

	enc := json.NewEncoder(f.w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func (f *JSONFormatter) WriteStreamEvent(event *api.StreamEvent) error {
	// JSONFormatter collects all events, not used directly
	return nil
}

func (f *JSONFormatter) WriteError(err error) error {
	out := JSONError{}
	out.Error.Message = err.Error()

	enc := json.NewEncoder(f.errW)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// StreamJSONFormatter outputs NDJSON (streaming)
type StreamJSONFormatter struct {
	w    io.Writer
	errW io.Writer
}

func (f *StreamJSONFormatter) WriteResponse(resp *api.GenerateResponse) error {
	// Not used for streaming
	return nil
}

func (f *StreamJSONFormatter) WriteStreamEvent(event *api.StreamEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = f.w.Write(append(data, '\n'))
	return err
}

func (f *StreamJSONFormatter) WriteError(err error) error {
	event := api.StreamEvent{Type: "error", Error: err.Error()}
	data, _ := json.Marshal(event)
	_, writeErr := f.errW.Write(append(data, '\n'))
	return writeErr
}
