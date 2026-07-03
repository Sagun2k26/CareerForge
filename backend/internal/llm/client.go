// Package llm is a provider-agnostic wrapper over hosted LLM providers
// (Anthropic / OpenAI) plus a deterministic mock used in development and tests.
//
// Every part of the system that talks to a model goes through Client, which
// centralizes prompt construction, retries, timeouts and JSON extraction. To
// switch providers you only change configuration; no calling code changes.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Role identifies the speaker of a chat message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single chat turn.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Provider is the minimal surface a backend (Anthropic, OpenAI, mock) must
// implement. Keeping it tiny makes new providers cheap to add.
type Provider interface {
	// Complete returns a single assistant completion for the conversation.
	Complete(ctx context.Context, system string, messages []Message) (string, error)
	// Embed returns one vector per input string.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Name identifies the provider for logging.
	Name() string
}

// Client wraps a Provider with timeouts, retries and convenience helpers.
type Client struct {
	provider Provider
	timeout  time.Duration
	retries  int
}

// New builds a Client around the given provider.
func New(provider Provider, timeout time.Duration) *Client {
	return &Client{provider: provider, timeout: timeout, retries: 2}
}

// Provider returns the underlying provider name (for health checks).
func (c *Client) Provider() string { return c.provider.Name() }

// Complete runs a chat completion with retry + timeout.
func (c *Client) Complete(ctx context.Context, system string, messages []Message) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		cctx, cancel := context.WithTimeout(ctx, c.timeout)
		out, err := c.provider.Complete(cctx, system, messages)
		cancel()
		if err == nil {
			return out, nil
		}
		lastErr = err
		// simple linear backoff
		time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
	}
	return "", fmt.Errorf("llm complete failed after retries: %w", lastErr)
}

// CompletePrompt is a convenience for a single user prompt with a system prompt.
func (c *Client) CompletePrompt(ctx context.Context, system, user string) (string, error) {
	return c.Complete(ctx, system, []Message{{Role: RoleUser, Content: user}})
}

// CompleteJSON runs a completion and unmarshals the (possibly fenced) JSON
// response into dst. It tolerates models that wrap JSON in ```json fences or
// surround it with prose.
func (c *Client) CompleteJSON(ctx context.Context, system, user string, dst any) error {
	raw, err := c.CompletePrompt(ctx, system, user)
	if err != nil {
		return err
	}
	clean := extractJSON(raw)
	if clean == "" {
		return errors.New("no JSON found in model response")
	}
	if err := json.Unmarshal([]byte(clean), dst); err != nil {
		return fmt.Errorf("decode model JSON: %w", err)
	}
	return nil
}

// Embed proxies to the provider.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	return c.provider.Embed(cctx, texts)
}

// EmbedOne is a single-text convenience wrapper.
func (c *Client) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, errors.New("empty embedding result")
	}
	return vecs[0], nil
}

// extractJSON pulls the first JSON object/array out of a model response.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip code fences.
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		if j := strings.IndexByte(s, '\n'); j >= 0 {
			// drop an optional language tag line like "json"
			if !strings.ContainsAny(s[:j], "{[") {
				s = s[j+1:]
			}
		}
		if k := strings.LastIndex(s, "```"); k >= 0 {
			s = s[:k]
		}
	}
	s = strings.TrimSpace(s)
	// Find the outermost JSON delimiters.
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return ""
	}
	open := s[start]
	close := byte('}')
	if open == '[' {
		close = ']'
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}
