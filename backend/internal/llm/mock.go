package llm

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
)

// MockProvider returns deterministic, schema-correct responses so the entire
// app runs and is testable with zero external dependencies or API keys. It
// inspects the task marker embedded in the system prompt to decide what to
// return.
type MockProvider struct{}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Complete(_ context.Context, system string, messages []Message) (string, error) {
	switch taskFromSystem(system) {
	case TaskResumeParse:
		return mockResumeJSON, nil
	case TaskGapAnalysis:
		return mockGapJSON, nil
	case TaskRoadmap:
		return mockRoadmapJSON, nil
	case TaskScoreAnswer:
		return mockScoreJSON(lastUser(messages)), nil
	case TaskModelAnswer:
		return mockModelAnswerJSON, nil
	case TaskTailorResume:
		return mockTailorJSON, nil
	case TaskInterview:
		return mockInterviewQuestion(messages), nil
	default:
		return "This is a mock response. Configure LLM_PROVIDER and LLM_API_KEY to use a real model.", nil
	}
}

// Embed produces deterministic 256-dim vectors by hashing the input. They are
// not semantically meaningful but are stable, which is enough to exercise the
// RAG pipeline end-to-end without an embedding API.
func (m *MockProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	const dim = 256
	out := make([][]float32, len(texts))
	for i, t := range texts {
		vec := make([]float32, dim)
		// Seed each dimension from a salted hash so vectors differ per-input but
		// are reproducible.
		for d := 0; d < dim; d++ {
			h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", d, t)))
			u := binary.BigEndian.Uint32(h[:4])
			vec[d] = (float32(u%1000) / 500.0) - 1.0 // roughly [-1, 1]
		}
		out[i] = normalize(vec)
	}
	return out, nil
}

func normalize(v []float32) []float32 {
	var sum float32
	for _, x := range v {
		sum += x * x
	}
	if sum == 0 {
		return v
	}
	inv := float32(1.0) / sqrt32(sum)
	for i := range v {
		v[i] *= inv
	}
	return v
}

func sqrt32(x float32) float32 {
	// Newton's method; good enough for normalization.
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 12; i++ {
		z = 0.5 * (z + x/z)
	}
	return z
}

func lastUser(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			return messages[i].Content
		}
	}
	return ""
}

func mockInterviewQuestion(messages []Message) string {
	// Count prior assistant turns to advance through a small scripted set.
	asked := 0
	for _, msg := range messages {
		if msg.Role == RoleAssistant {
			asked++
		}
	}
	questions := []string{
		"To start: walk me through a project you're proud of and your specific contribution.",
		"How would you design a rate limiter for a public API? Talk through the data structures.",
		"Tell me about a time you debugged a tricky production issue. How did you isolate the cause?",
		"How do you decide between a SQL and a NoSQL store for a new feature?",
	}
	if asked >= len(questions) {
		return "INTERVIEW_COMPLETE"
	}
	return questions[asked]
}

func mockScoreJSON(answer string) string {
	// Longer answers score a little higher, just to make the mock feel alive.
	base := 6
	if len(strings.Fields(answer)) > 40 {
		base = 8
	}
	return fmt.Sprintf(`{"clarity": %d, "correctness": %d, "depth": %d, "feedback": "Solid answer. Add a concrete example and quantify the impact to make it stronger."}`,
		base, base-1, base)
}

const mockResumeJSON = `{
  "name": "Alex Candidate",
  "headline": "Backend Engineer",
  "skills": ["Go", "PostgreSQL", "REST APIs", "Docker", "Git"],
  "experience": [
    {"title": "Software Engineer", "company": "Acme Corp", "duration": "2022-2024",
     "highlights": ["Built internal billing service in Go", "Cut p99 latency by 40%"]}
  ],
  "education": [{"degree": "B.Tech Computer Science", "institution": "State University", "year": "2022"}],
  "summary": "Backend engineer with 2 years building Go services and relational data models."
}`

const mockGapJSON = `{
  "gap_score": 68,
  "matched_skills": ["Go", "PostgreSQL", "REST APIs", "Docker"],
  "missing_skills": ["System Design", "Kubernetes", "Distributed Caching"],
  "summary": "You're well-positioned with strong backend fundamentals. Focusing on system design and container orchestration will close most of the gap to this role."
}`

const mockRoadmapJSON = `{
  "items": [
    {"order": 1, "topic": "System Design Fundamentals", "why": "Core to senior backend interviews and the target role.",
     "resources": ["Grokking System Design", "Designing Data-Intensive Applications (ch. 1-4)"], "milestone": "Design a URL shortener end-to-end and explain trade-offs."},
    {"order": 2, "topic": "Distributed Caching", "why": "The role requires high-throughput services.",
     "resources": ["Redis University RU101", "Cache invalidation patterns article"], "milestone": "Add a Redis cache layer to a sample API and measure latency gains."},
    {"order": 3, "topic": "Kubernetes", "why": "Deployments at the target company run on K8s.",
     "resources": ["Kubernetes Up & Running", "kubernetes.io interactive tutorials"], "milestone": "Deploy a multi-service app to a local kind cluster."}
  ]
}`

const mockModelAnswerJSON = `{
  "model_answer": "A strong answer frames the problem, states assumptions, proposes a baseline design, then iterates on bottlenecks. For a rate limiter: start with a token-bucket per client key in Redis, discuss atomicity via Lua scripts, then cover sharding and clock skew.",
  "key_points": ["Clarify requirements and scale", "Pick a concrete algorithm (token bucket)", "Address atomicity and storage", "Discuss failure modes and trade-offs"]
}`

const mockTailorJSON = `{
  "summary": "Backend engineer who ships reliable Go services and owns performance from API to database.",
  "highlighted_skills": ["Go", "PostgreSQL", "REST APIs", "Performance Optimization", "Docker"],
  "experience": [
    {"title": "Software Engineer", "company": "Acme Corp",
     "bullets": ["Designed and shipped a Go billing service handling 5M requests/day", "Reduced p99 latency 40% via query tuning and caching"]}
  ],
  "changes": ["Led with performance impact to match the JD's emphasis on scale", "Surfaced Go and PostgreSQL first as the JD's core stack"]
}`
