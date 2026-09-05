package llm

import (
	"net/http"
	"time"
)

// FromConfig builds a client for the configured provider. Unknown providers
// and "mock" use the deterministic mock so local development always boots.
func FromConfig(provider, model, apiKey string, timeout time.Duration) *Client {
	hc := &http.Client{Timeout: timeout}
	var p Provider
	switch provider {
	case "gemini":
		p = NewGeminiProvider(apiKey, model, hc)
	case "anthropic":
		p = NewAnthropicProvider(apiKey, model, hc)
	case "openai":
		p = NewOpenAIProvider(apiKey, model, hc)
	default:
		p = NewMockProvider()
	}
	return New(p, timeout)
}
