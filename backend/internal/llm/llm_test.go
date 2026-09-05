package llm

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestExtractJSON(t *testing.T) {
	cases := map[string]string{
		"```json\n{\"a\":1}\n```":            `{"a":1}`,
		"here is the result: {\"a\":1} done": `{"a":1}`,
		"[1, 2, 3]":                          `[1, 2, 3]`,
		"{\"nested\": {\"b\": 2}}":           `{"nested": {"b": 2}}`,
	}
	for in, want := range cases {
		if got := extractJSON(in); got != want {
			t.Errorf("extractJSON(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMockResumeParseProducesValidJSON(t *testing.T) {
	client := New(NewMockProvider(), time.Second)
	system, user := ResumeParsePrompt("Alex - Go engineer, 2 years.")
	var profile map[string]any
	if err := client.CompleteJSON(context.Background(), system, user, &profile); err != nil {
		t.Fatalf("CompleteJSON: %v", err)
	}
	if _, ok := profile["skills"]; !ok {
		t.Fatalf("expected parsed profile to contain skills, got %v", profile)
	}
}

func TestMockAnalysisPromptsDecode(t *testing.T) {
	client := New(NewMockProvider(), time.Second)
	resume := `{"skills":["Go"]}`
	role := "Backend Engineer"
	required := []string{"Go", "System Design"}

	fitSys, fitUser := RoleFitPrompt(resume, role, required)
	var fit struct {
		Score int    `json:"score"`
		Level string `json:"level"`
	}
	if err := client.CompleteJSON(context.Background(), fitSys, fitUser, &fit); err != nil {
		t.Fatalf("role fit: %v", err)
	}
	if fit.Score <= 0 || fit.Level == "" {
		t.Fatalf("invalid role fit: %+v", fit)
	}

	gapSys, gapUser := GapAnalysisPrompt(resume, role, required)
	var gap struct {
		GapScore      int      `json:"gap_score"`
		MissingSkills []string `json:"missing_skills"`
	}
	if err := client.CompleteJSON(context.Background(), gapSys, gapUser, &gap); err != nil {
		t.Fatalf("gap: %v", err)
	}
	if gap.GapScore <= 0 {
		t.Fatalf("expected positive gap score, got %d", gap.GapScore)
	}

	roadSys, roadUser := RoadmapPrompt(role, gap.MissingSkills)
	var roadmap struct {
		Items []map[string]any `json:"items"`
	}
	if err := client.CompleteJSON(context.Background(), roadSys, roadUser, &roadmap); err != nil {
		t.Fatalf("roadmap: %v", err)
	}
	if len(roadmap.Items) == 0 {
		t.Fatal("expected at least one roadmap item")
	}
}

func TestMockEmbedIsDeterministicAndNormalized(t *testing.T) {
	m := NewMockProvider()
	a, err := m.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := m.Embed(context.Background(), []string{"hello"})
	if len(a[0]) != 768 {
		t.Fatalf("expected 768 dims, got %d", len(a[0]))
	}
	for i := range a[0] {
		if a[0][i] != b[0][i] {
			t.Fatal("embeddings are not deterministic")
		}
	}
	var sum float64
	for _, x := range a[0] {
		sum += float64(x) * float64(x)
	}
	if math.Abs(sum-1.0) > 0.01 {
		t.Fatalf("embedding should be unit length, got norm^2 = %f", sum)
	}
}

func TestScoreAnswerRoundTrip(t *testing.T) {
	client := New(NewMockProvider(), time.Second)
	sys, user := ScoreAnswerPrompt("Backend Engineer", "What is a goroutine?", "A lightweight thread managed by the Go runtime that runs concurrently.")
	raw, err := client.CompletePrompt(context.Background(), sys, user)
	if err != nil {
		t.Fatal(err)
	}
	var score struct {
		Clarity int `json:"clarity"`
	}
	if err := json.Unmarshal([]byte(extractJSON(raw)), &score); err != nil {
		t.Fatalf("decode score: %v", err)
	}
	if score.Clarity < 0 || score.Clarity > 10 {
		t.Fatalf("clarity out of range: %d", score.Clarity)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGeminiProviderGenerationAndEmbedding(t *testing.T) {
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("x-goog-api-key") != "key" {
			t.Fatal("missing Gemini API key header")
		}
		body := `{"candidates":[{"content":{"parts":[{"text":"hello"}]}}]}`
		if strings.Contains(r.URL.Path, ":embedContent") {
			body = `{"embedding":{"values":[0.1,0.2]}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	p := NewGeminiProvider("key", "gemini-test", hc)

	got, err := p.Complete(context.Background(), "system", []Message{{Role: RoleUser, Content: "hi"}})
	if err != nil || got != "hello" {
		t.Fatalf("complete = %q, %v", got, err)
	}
	vecs, err := p.Embed(context.Background(), []string{"hello"})
	if err != nil || len(vecs) != 1 || len(vecs[0]) != 2 {
		t.Fatalf("embed = %v, %v", vecs, err)
	}
}
