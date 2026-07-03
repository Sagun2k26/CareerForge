package llm

import (
	"net/http"
	"time"
)

// FromConfig builds a Client for the given provider name. Unknown providers and
// "mock" both yield the deterministic mock, guaranteeing the app always boots.
func FromConfig(provider, model, apiKey string, timeout time.Duration) *Client {
	hc := &http.Client{Timeout: timeout}
	var p Provider
	switch provider {
	case "anthropic":
		p = NewAnthropicProvider(apiKey, model, hc)
	case "openai":
		p = NewOpenAIProvider(apiKey, model, hc)
	default:
		p = NewMockProvider()
	}
	return New(p, timeout)
}
