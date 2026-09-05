// Package llm is a provider-agnostic wrapper over hosted LLM providers
// plus a deterministic mock used in development, tests, and graceful fallback.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type Provider interface {
	Complete(ctx context.Context, system string, messages []Message) (string, error)
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Name() string
}

type fastProvider interface {
	CompleteFast(ctx context.Context, system string, messages []Message) (string, error)
}

type Client struct {
	provider Provider
	fallback Provider
	timeout  time.Duration
	retries  int
}

func New(provider Provider, timeout time.Duration) *Client {
	c := &Client{provider: provider, timeout: timeout, retries: 0}
	if provider != nil && provider.Name() != "mock" {
		c.fallback = NewMockProvider()
	}
	return c
}

func (c *Client) Provider() string { return c.provider.Name() }

func (c *Client) runPrimaryCompletion(ctx context.Context, call func(context.Context) (string, error)) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		cctx, cancel := context.WithTimeout(ctx, c.timeout)
		out, err := call(cctx)
		cancel()
		if err == nil {
			return out, nil
		}
		lastErr = err
		if nonRetryableCompletionError(err) {
			return "", err
		}
		if attempt < c.retries {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 200 * time.Millisecond):
			}
		}
	}
	return "", fmt.Errorf("llm complete failed after retries: %w", lastErr)
}

func nonRetryableCompletionError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "gemini api 400:") ||
		strings.Contains(msg, "gemini api 401:") ||
		strings.Contains(msg, "gemini api 403:") ||
		strings.Contains(msg, "gemini api 429:")
}

func (c *Client) fallbackComplete(ctx context.Context, system string, messages []Message, cause error) (string, error) {
	if c.fallback == nil || ctx.Err() != nil {
		return "", cause
	}
	slog.Warn("llm provider failed; using mock fallback", "provider", c.provider.Name(), "error", cause)
	out, err := c.fallback.Complete(ctx, system, messages)
	if err != nil {
		return "", fmt.Errorf("primary llm failed: %v; mock fallback failed: %w", cause, err)
	}
	return out, nil
}

func (c *Client) Complete(ctx context.Context, system string, messages []Message) (string, error) {
	out, err := c.runPrimaryCompletion(ctx, func(cctx context.Context) (string, error) {
		return c.provider.Complete(cctx, system, messages)
	})
	if err == nil {
		return out, nil
	}
	return c.fallbackComplete(ctx, system, messages, err)
}

func (c *Client) CompleteFast(ctx context.Context, system string, messages []Message) (string, error) {
	p, ok := c.provider.(fastProvider)
	if !ok {
		return c.Complete(ctx, system, messages)
	}
	out, err := c.runPrimaryCompletion(ctx, func(cctx context.Context) (string, error) {
		return p.CompleteFast(cctx, system, messages)
	})
	if err == nil {
		return out, nil
	}
	return c.fallbackComplete(ctx, system, messages, err)
}

func (c *Client) CompletePrompt(ctx context.Context, system, user string) (string, error) {
	return c.Complete(ctx, system, []Message{{Role: RoleUser, Content: user}})
}

func (c *Client) CompleteJSON(ctx context.Context, system, user string, dst any) error {
	messages := []Message{{Role: RoleUser, Content: user}}
	raw, err := c.Complete(ctx, system, messages)
	if err != nil {
		return err
	}
	if err := decodeJSON(raw, dst); err == nil {
		return nil
	} else if c.fallback != nil && ctx.Err() == nil {
		// The primary provider may return prose/truncated JSON even when the HTTP
		// request succeeds. Use the schema-correct mock rather than failing the feature.
		slog.Warn("llm returned invalid JSON; using mock fallback", "provider", c.provider.Name(), "error", err)
		fallbackRaw, fallbackErr := c.fallback.Complete(ctx, system, messages)
		if fallbackErr != nil {
			return fmt.Errorf("decode model JSON: %v; mock fallback failed: %w", err, fallbackErr)
		}
		return decodeJSON(fallbackRaw, dst)
	} else {
		return err
	}
}

func (c *Client) CompleteJSONFast(ctx context.Context, system, user string, dst any) error {
	messages := []Message{{Role: RoleUser, Content: user}}
	raw, err := c.CompleteFast(ctx, system, messages)
	if err != nil {
		return err
	}
	if err := decodeJSON(raw, dst); err == nil {
		return nil
	} else if c.fallback != nil && ctx.Err() == nil {
		slog.Warn("llm returned invalid JSON; using mock fallback", "provider", c.provider.Name(), "error", err)
		fallbackRaw, fallbackErr := c.fallback.Complete(ctx, system, messages)
		if fallbackErr != nil {
			return fmt.Errorf("decode model JSON: %v; mock fallback failed: %w", err, fallbackErr)
		}
		return decodeJSON(fallbackRaw, dst)
	} else {
		return err
	}
}

func decodeJSON(raw string, dst any) error {
	clean := extractJSON(raw)
	if clean == "" {
		return errors.New("no JSON found in model response")
	}
	if err := json.Unmarshal([]byte(clean), dst); err != nil {
		return fmt.Errorf("decode model JSON: %w", err)
	}
	return nil
}

func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	vecs, err := c.provider.Embed(cctx, texts)
	cancel()
	if err == nil {
		return vecs, nil
	}
	if c.fallback == nil || ctx.Err() != nil {
		return nil, err
	}
	slog.Warn("embedding provider failed; using mock fallback", "provider", c.provider.Name(), "error", err)
	return c.fallback.Embed(ctx, texts)
}

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

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		if j := strings.IndexByte(s, '\n'); j >= 0 {
			if !strings.ContainsAny(s[:j], "{[") {
				s = s[j+1:]
			}
		}
		if k := strings.LastIndex(s, "```"); k >= 0 {
			s = s[:k]
		}
	}
	s = strings.TrimSpace(s)
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
