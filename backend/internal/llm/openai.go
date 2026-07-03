package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OpenAIProvider calls the OpenAI Chat Completions and Embeddings APIs.
type OpenAIProvider struct {
	apiKey     string
	model      string
	embedModel string
	http       *http.Client
}

func NewOpenAIProvider(apiKey, model string, hc *http.Client) *OpenAIProvider {
	return &OpenAIProvider{apiKey: apiKey, model: model, embedModel: "text-embedding-3-small", http: hc}
}

func (p *OpenAIProvider) Name() string { return "openai:" + p.model }

func (p *OpenAIProvider) Complete(ctx context.Context, system string, messages []Message) (string, error) {
	type omsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := []omsg{{Role: "system", Content: system}}
	for _, m := range messages {
		role := "user"
		if m.Role == RoleAssistant {
			role = "assistant"
		}
		msgs = append(msgs, omsg{Role: role, Content: m.Content})
	}
	payload := map[string]any{"model": p.model, "messages": msgs}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai api %d: %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai: empty choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func (p *OpenAIProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	payload := map[string]any{"model": p.embedModel, "input": texts}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai embeddings %d: %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		out[i] = d.Embedding
	}
	return out, nil
}
