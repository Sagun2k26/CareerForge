package llm

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
)

// MockProvider returns deterministic, schema-correct responses so the entire
// app runs and is testable with zero external dependencies or API keys.
type MockProvider struct{}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Complete(_ context.Context, system string, messages []Message) (string, error) {
	switch taskFromSystem(system) {
	case TaskResumeParse:
		return mockResumeJSON(lastUser(messages)), nil
	case TaskRoleFit:
		return mockRoleFitJSON, nil
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
	case TaskQuestionBank:
		return mockQuestionBankJSON(lastUser(messages)), nil
	case TaskInterview:
		return mockInterviewQuestion(messages), nil
	default:
		return "This is a mock response. Configure LLM_PROVIDER and LLM_API_KEY to use a real model.", nil
	}
}

// Embed produces deterministic 256-dim vectors by hashing the input. They are
// stable for tests but intentionally not semantically meaningful.
func (m *MockProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	const dim = 768
	out := make([][]float32, len(texts))
	for i, t := range texts {
		vec := make([]float32, dim)
		for d := 0; d < dim; d++ {
			h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", d, t)))
			u := binary.BigEndian.Uint32(h[:4])
			vec[d] = (float32(u%1000) / 500.0) - 1.0
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

func mockQuestionBankJSON(user string) string {
	role, topic := "Software Engineer", "the selected topic"
	for _, line := range strings.Split(user, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Role: ") {
			role = strings.TrimSpace(strings.TrimPrefix(line, "Role: "))
		}
		if strings.HasPrefix(line, "Topic: ") {
			topic = strings.TrimSpace(strings.TrimPrefix(line, "Topic: "))
		}
	}
	templates := []struct{ q, a string }{
		{"Explain the core concepts of {topic} and where they matter in a {role} role.", "Start with the main concepts and their purpose, then connect them to a realistic {topic} workload. Mention the most important trade-off or failure mode an interviewer would expect."},
		{"What are the most important trade-offs when designing with {topic}?", "Discuss performance, correctness, complexity, operability, and cost where relevant. A strong answer explains when one trade-off is preferable instead of presenting a single universal choice."},
		{"Describe a production problem involving {topic} and how you would debug it.", "Begin with symptoms and observable signals, form hypotheses, then narrow the problem using logs, metrics, traces, or targeted tests. Finish with the fix and how you would prevent recurrence."},
		{"How would you test a system or component that relies on {topic}?", "Use unit tests for local logic, integration tests for real boundaries, and a small number of end-to-end tests for critical flows. Include failure cases, concurrency where relevant, and deterministic test data."},
		{"What common mistakes do engineers make with {topic}?", "Call out misuse of defaults, poor failure handling, missing observability, and incorrect assumptions about scale or consistency. Explain the consequence of each mistake and the safer alternative."},
		{"How does {topic} behave under high load?", "Explain the likely bottlenecks first, then cover resource limits, backpressure, contention, and scaling strategy. Mention which metrics you would watch before changing the design."},
		{"How would you make a {topic}-based design highly available?", "Remove single points of failure, use redundancy where appropriate, define health checks and failover behavior, and design for partial failures. Also explain the consistency or complexity cost of higher availability."},
		{"How would you monitor and troubleshoot {topic} in production?", "Track golden signals such as latency, traffic, errors, and saturation, plus domain-specific metrics. Correlate metrics with logs and traces and alert on user-impacting symptoms rather than every internal anomaly."},
		{"When would you choose {topic}, and when would you avoid it?", "Choose it when its strengths match the workload and operational constraints. Avoid it when a simpler primitive solves the problem or when its consistency, latency, cost, or maintenance trade-offs are a poor fit."},
		{"How would you explain {topic} to a junior engineer?", "Define it in simple terms, give one concrete example, then explain why the abstraction exists. End with one limitation so the explanation stays accurate rather than oversimplified."},
		{"Compare two common approaches within {topic} and explain when each wins.", "Compare them using workload shape, latency, throughput, correctness, operational complexity, and failure behavior. State assumptions first because the better choice depends on context."},
		{"How would you migrate an existing system to use {topic} safely?", "Use an incremental rollout, compatibility layer or dual path where useful, validate behavior with metrics, and keep rollback simple. Avoid a big-bang migration unless the system is trivial."},
		{"What failure modes should you expect with {topic}?", "Cover dependency failure, timeout, overload, bad configuration, data inconsistency, and partial failure as applicable. For each, explain detection, containment, and recovery."},
		{"How would you optimize the performance of {topic}?", "Measure before optimizing, identify the actual bottleneck, and improve the highest-impact path first. Discuss caching, batching, concurrency, indexing, connection reuse, or algorithmic changes only when relevant to the measured issue."},
		{"What security concerns should you consider when using {topic}?", "Apply least privilege, validate untrusted input, protect secrets and data in transit and at rest, and audit sensitive operations. The exact controls depend on whether {topic} is exposed across a trust boundary."},
		{"How do consistency and correctness considerations appear in {topic}?", "State the invariants the system must preserve, then explain where races, retries, duplication, stale reads, or partial writes could violate them. Use idempotency, transactions, locking, or versioning as appropriate."},
		{"How would you handle retries and timeouts around {topic}?", "Set bounded timeouts based on the request budget, retry only transient and idempotent operations, and use backoff with jitter. Avoid retry storms and preserve enough time for upstream callers to recover."},
		{"How would you scale a system centered on {topic} from thousands to millions of requests?", "First identify whether the bottleneck is compute, storage, network, contention, or a shared dependency. Then scale stateless work horizontally and partition, cache, batch, or redesign the constrained stateful component as needed."},
		{"What design decisions around {topic} would you document for an on-call engineer?", "Document architecture, dependencies, SLOs, key dashboards, common failure signatures, safe mitigations, rollback steps, and escalation points. The goal is reducing diagnosis time during incidents."},
		{"Design a small end-to-end system that demonstrates strong understanding of {topic} for a {role} interview.", "Clarify requirements and scale, propose the simplest viable architecture, then walk through data flow, failure handling, observability, and major trade-offs. Keep the design proportional to the problem instead of adding unnecessary components."},
	}
	type item struct {
		Prompt string `json:"prompt"`
		Answer string `json:"answer"`
	}
	out := struct {
		Questions []item `json:"questions"`
	}{Questions: make([]item, 0, 20)}
	for _, t := range templates {
		q := strings.ReplaceAll(strings.ReplaceAll(t.q, "{topic}", topic), "{role}", role)
		a := strings.ReplaceAll(strings.ReplaceAll(t.a, "{topic}", topic), "{role}", role)
		out.Questions = append(out.Questions, item{Prompt: q, Answer: a})
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func mockInterviewQuestion(messages []Message) string {
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

func mockScoreJSON(prompt string) string {
	answer := promptField(prompt, "Candidate answer:")
	question := promptField(prompt, "Question:")
	words := cleanWords(answer)

	if looksLikeNonsense(answer, words) {
		return `{"clarity": 0, "correctness": 0, "depth": 0, "feedback": "The response is not meaningful enough to evaluate. Answer the question with a clear technical explanation."}`
	}

	qWords := keywordSet(question)
	aWords := keywordSet(answer)
	overlap := 0
	for w := range aWords {
		if qWords[w] {
			overlap++
		}
	}

	clarity, correctness, depth := 2, 2, 1
	if len(words) >= 10 {
		clarity = 4
		depth = 3
	}
	if len(words) >= 25 {
		clarity = 6
		depth = 5
	}
	if overlap > 0 {
		correctness = 4
	}
	if overlap >= 2 && len(words) >= 15 {
		correctness = 6
	}

	feedback := "The answer is understandable, but add more question-specific technical detail."
	if len(words) < 8 {
		feedback = "The answer is too brief to establish correctness or depth. Explain the core idea and give a concrete technical detail."
	} else if overlap == 0 {
		correctness = minInt(correctness, 2)
		feedback = "The response does not appear sufficiently tied to the question. Address the specific concept being asked and explain why."
	} else if depth >= 5 {
		feedback = "Good structure. Strengthen it with a concrete example, trade-off, or failure case."
	}

	return fmt.Sprintf(`{"clarity": %d, "correctness": %d, "depth": %d, "feedback": %q}`,
		clarity, correctness, depth, feedback)
}

func promptField(prompt, prefix string) string {
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func cleanWords(s string) []string {
	parts := strings.Fields(strings.ToLower(s))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(p, `.,!?;:()[]{}<>\"'`+"`"+`-_=/`)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func looksLikeNonsense(answer string, words []string) bool {
	if len(strings.TrimSpace(answer)) < 4 || len(words) < 2 {
		return true
	}
	meaningful := 0
	unique := map[string]bool{}
	for _, w := range words {
		unique[w] = true
		if len(w) >= 3 && !allSameRune(w) {
			meaningful++
		}
	}
	if meaningful < 2 {
		return true
	}
	if len(words) >= 4 && len(unique) <= 2 {
		return true
	}
	return false
}

func allSameRune(s string) bool {
	if s == "" {
		return true
	}
	var first rune
	seen := false
	for _, r := range s {
		if !seen {
			first = r
			seen = true
			continue
		}
		if r != first {
			return false
		}
	}
	return true
}

func keywordSet(s string) map[string]bool {
	stop := map[string]bool{
		"what": true, "when": true, "where": true, "which": true, "would": true,
		"how": true, "why": true, "does": true, "with": true, "from": true,
		"that": true, "this": true, "your": true, "about": true, "into": true,
		"for": true, "and": true, "the": true, "are": true, "you": true,
	}
	out := map[string]bool{}
	for _, w := range cleanWords(s) {
		if len(w) >= 3 && !stop[w] {
			out[w] = true
		}
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mockResumeJSON(prompt string) string {
	text := prompt
	if i := strings.Index(prompt, "Resume text:"); i >= 0 {
		text = strings.TrimSpace(prompt[i+len("Resume text:"):])
	}

	name := "Candidate"
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "@") || len(line) > 80 {
			continue
		}
		name = line
		break
	}

	known := []string{
		"Go", "Golang", "C++", "Python", "SQL", "PostgreSQL", "MySQL", "MongoDB",
		"Redis", "Docker", "Kubernetes", "Kafka", "Git", "TypeScript", "JavaScript",
		"React", "Next.js", "Node.js", "gRPC", "REST APIs", "System Design",
	}
	lower := strings.ToLower(text)
	skills := make([]string, 0, 12)
	seen := map[string]bool{}
	for _, skill := range known {
		if strings.Contains(lower, strings.ToLower(skill)) && !seen[skill] {
			skills = append(skills, skill)
			seen[skill] = true
		}
	}
	if len(skills) == 0 {
		skills = []string{"Software Engineering"}
	}

	headline := "Software Engineer"
	for _, h := range []string{"Backend Engineer", "Software Engineer", "DevOps Engineer", "Data Engineer", "Frontend Engineer"} {
		if strings.Contains(lower, strings.ToLower(h)) {
			headline = h
			break
		}
	}

	profile := map[string]any{
		"name":       name,
		"headline":   headline,
		"skills":     skills,
		"experience": []any{},
		"education":  []any{},
		"summary":    fmt.Sprintf("%s with experience reflected in the uploaded resume and skills including %s.", headline, strings.Join(skills, ", ")),
	}
	b, _ := json.Marshal(profile)
	return string(b)
}

const mockRoleFitJSON = `{
  "score": 72,
  "level": "moderate",
  "strengths": ["Strong Go backend fundamentals", "Production database and API experience"],
  "concerns": ["Needs deeper system design", "Limited Kubernetes depth"],
  "summary": "The candidate has a solid backend foundation and is a credible match, with a few infrastructure gaps to close."
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
