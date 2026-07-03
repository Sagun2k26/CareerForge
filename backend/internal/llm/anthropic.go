package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AnthropicProvider calls the Anthropic Messages API. Anthropic does not expose
// a first-party embeddings endpoint, so embeddings fall back to the same
// deterministic local scheme as the mock provider (swap in Voyage AI here for
// production-grade semantic search).
type AnthropicProvider struct {
	apiKey string
	model  string
	http   *http.Client
	mock   *MockProvider // reused only for Embed
}

func NewAnthropicProvider(apiKey, model string, hc *http.Client) *AnthropicProvider {
	return &AnthropicProvider{apiKey: apiKey, model: model, http: hc, mock: NewMockProvider()}
}

func (p *AnthropicProvider) Name() string { return "anthropic:" + p.model }

func (p *AnthropicProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return p.mock.Embed(ctx, texts)
}

func (p *AnthropicProvider) Complete(ctx context.Context, system string, messages []Message) (string, error) {
	type amsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload := map[string]any{
		"model":      p.model,
		"max_tokens": 1500,
		"system":     system,
	}
	msgs := make([]amsg, 0, len(messages))
	for _, m := range messages {
		role := "user"
		if m.Role == RoleAssistant {
			role = "assistant"
		}
		msgs = append(msgs, amsg{Role: role, Content: m.Content})
	}
	payload["messages"] = msgs

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("anthropic api %d: %s", resp.StatusCode, string(raw))
	}

	var parsed struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Content) == 0 {
		return "", fmt.Errorf("anthropic: empty content")
	}
	return parsed.Content[0].Text, nil
}
