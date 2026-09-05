package resume

import (
	"context"
	"encoding/json"

	"github.com/sagun-patwari/ai-career-platform/internal/llm"
)

// analysisAgent is the specialized resume agent: it uses the text extractor as
// a tool, then asks the model for a typed candidate profile.
type analysisAgent struct{ llm *llm.Client }

func (a analysisAgent) Run(ctx context.Context, filename string, data []byte) (json.RawMessage, error) {
	text, err := extractText(filename, data)
	if err != nil {
		return nil, err
	}
	system, user := llm.ResumeParsePrompt(text)
	var profile map[string]any
	if err := a.llm.CompleteJSON(ctx, system, user, &profile); err != nil {
		return nil, err
	}
	return json.Marshal(profile)
}
