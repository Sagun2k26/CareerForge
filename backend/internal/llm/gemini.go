package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models/"

type GeminiProvider struct {
	apiKey     string
	model      string
	embedModel string
	http       *http.Client
}

func NewGeminiProvider(apiKey, model string, hc *http.Client) *GeminiProvider {
	if model == "" {
		model = "gemini-3.7-flash"
	}
	return &GeminiProvider{apiKey: apiKey, model: model, embedModel: "gemini-embedding-001", http: hc}
}

func (p *GeminiProvider) Name() string { return "gemini:" + p.model }

func (p *GeminiProvider) Complete(ctx context.Context, system string, messages []Message) (string, error) {
	return p.complete(ctx, system, messages, false)
}

// CompleteFast uses low thinking for straightforward latency-sensitive tasks.
func (p *GeminiProvider) CompleteFast(ctx context.Context, system string, messages []Message) (string, error) {
	return p.complete(ctx, system, messages, true)
}

func (p *GeminiProvider) complete(ctx context.Context, system string, messages []Message, fast bool) (string, error) {
	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Role  string `json:"role,omitempty"`
		Parts []part `json:"parts"`
	}

	contents := make([]content, 0, len(messages))
	for _, m := range messages {
		role := "user"
		if m.Role == RoleAssistant {
			role = "model"
		}
		contents = append(contents, content{Role: role, Parts: []part{{Text: m.Content}}})
	}

	payload := map[string]any{
		"system_instruction": content{Parts: []part{{Text: system}}},
		"contents":           contents,
	}
	if fast {
		payload["generationConfig"] = map[string]any{
			"thinkingConfig":  map[string]any{"thinkingLevel": "low"},
			"maxOutputTokens": 6144,
		}
	}

	var out struct {
		Candidates []struct {
			Content struct {
				Parts []part `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := p.post(ctx, p.model, "generateContent", payload, &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 {
		return "", fmt.Errorf("gemini: empty candidates")
	}
	var text strings.Builder
	for _, part := range out.Candidates[0].Content.Parts {
		text.WriteString(part.Text)
	}
	if text.Len() == 0 {
		return "", fmt.Errorf("gemini: empty content")
	}
	return text.String(), nil
}

func (p *GeminiProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		payload := map[string]any{
			"embedContentConfig": map[string]any{
				"taskType":             "SEMANTIC_SIMILARITY",
				"outputDimensionality": 768,
			},
			"content": map[string]any{"parts": []map[string]string{{"text": text}}},
		}
		var res struct {
			Embedding struct {
				Values []float32 `json:"values"`
			} `json:"embedding"`
		}
		if err := p.post(ctx, p.embedModel, "embedContent", payload, &res); err != nil {
			return nil, err
		}
		out[i] = normalize(res.Embedding.Values)
	}
	return out, nil
}

func (p *GeminiProvider) post(ctx context.Context, model, method string, payload, dst any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	model = strings.TrimPrefix(model, "models/")
	endpoint := geminiBaseURL + url.PathEscape(model) + ":" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("gemini api %d: %s", resp.StatusCode, string(raw))
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode gemini response: %w", err)
	}
	return nil
}
