package questionbank

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/llm"
	"github.com/sagun-patwari/ai-career-platform/internal/rag"
)

type Service struct {
	repo *Repository
	rag  *rag.Engine
	llm  *llm.Client
}

func NewService(repo *Repository, engine *rag.Engine, client *llm.Client) *Service {
	return &Service{repo: repo, rag: engine, llm: client}
}

// Index is for the small internal retrieval corpus, not dynamic Question Bank sets.
func (s *Service) Index(ctx context.Context, questions []Question) error {
	docs := make([]rag.Document, 0, len(questions))
	for _, q := range questions {
		if err := s.repo.Upsert(ctx, q); err != nil {
			return err
		}
		docs = append(docs, rag.Document{
			ID:      q.ID.String(),
			Content: q.Prompt,
			Metadata: map[string]string{
				"role_id":     q.RoleID.String(),
				"topic":       q.Topic,
				"question_id": q.ID.String(),
			},
		})
	}
	return s.rag.Upsert(ctx, rag.CollectionQuestions, docs)
}

func (s *Service) ListByRole(ctx context.Context, userID, roleID uuid.UUID, topic string) ([]Question, error) {
	return s.repo.ListByRole(ctx, userID, roleID, topic)
}

func (s *Service) Search(ctx context.Context, query string, roleID *uuid.UUID, n int) ([]Question, error) {
	if n <= 0 {
		n = 20
	}
	var where map[string]string
	if roleID != nil {
		where = map[string]string{"role_id": roleID.String()}
	}
	rewritten := rewriteRetrievalQuery(query)
	results, err := s.rag.Query(ctx, rag.CollectionQuestions, rewritten, n, where)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(results))
	for _, r := range results {
		if id, err := uuid.Parse(r.ID); err == nil {
			ids = append(ids, id)
		}
	}
	questions, err := s.repo.GetMany(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]Question, len(questions))
	for _, q := range questions {
		byID[q.ID] = q
	}
	ordered := make([]Question, 0, len(ids))
	for _, id := range ids {
		if q, ok := byID[id]; ok {
			ordered = append(ordered, q)
		}
	}
	return ordered, nil
}

// TopicQuestions is cache-first. A cache miss makes one Gemini request for all
// 20 Q&A pairs. If Gemini is unavailable or quota-limited, the deterministic
// mock provider supplies a fallback set, which is also cached in Postgres.
func (s *Service) TopicQuestions(ctx context.Context, userID, roleID uuid.UUID, roleName, topic string) ([]Question, bool, error) {
	topic = strings.TrimSpace(topic)
	roleName = strings.TrimSpace(roleName)
	if topic == "" || roleName == "" {
		return nil, false, fmt.Errorf("role and topic are required")
	}

	cached, err := s.repo.ListCachedByTopic(ctx, userID, roleID, topic, 20)
	if err != nil {
		return nil, false, err
	}
	if len(cached) == 20 {
		return cached, true, nil
	}

	system, user := llm.QuestionBankPrompt(roleName, topic)
	var out struct {
		Questions []struct {
			Prompt string `json:"prompt"`
			Answer string `json:"answer"`
		} `json:"questions"`
	}

	source := "generated"
	if err := s.llm.CompleteJSONFast(ctx, system, user, &out); err != nil {
		slog.Warn("question bank falling back to mock", "role", roleName, "topic", topic, "error", err)
		mockClient := llm.New(llm.NewMockProvider(), 2*time.Second)
		if mockErr := mockClient.CompleteJSONFast(ctx, system, user, &out); mockErr != nil {
			return nil, false, fmt.Errorf("question generation failed: primary=%v fallback=%w", err, mockErr)
		}
		source = "mock"
	}

	seen := make(map[string]bool, 20)
	generated := make([]Question, 0, 20)
	for _, item := range out.Questions {
		prompt := strings.TrimSpace(item.Prompt)
		answer := strings.TrimSpace(item.Answer)
		key := strings.ToLower(prompt)
		if prompt == "" || answer == "" || seen[key] {
			continue
		}
		seen[key] = true
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(roleID.String()+"|dynamic|"+strings.ToLower(topic)+"|"+prompt))
		q := Question{
			ID:          id,
			RoleID:      roleID,
			Topic:       topic,
			Prompt:      prompt,
			ModelAnswer: answer,
			Source:      source,
		}
		if err := s.repo.UpsertCached(ctx, q); err != nil {
			return nil, false, err
		}
		generated = append(generated, q)
		if len(generated) == 20 {
			break
		}
	}
	if len(generated) != 20 {
		return nil, false, fmt.Errorf("question generation returned %d valid questions; expected 20", len(generated))
	}

	items, err := s.repo.ListCachedByTopic(ctx, userID, roleID, topic, 20)
	if err != nil {
		return nil, false, err
	}
	return items, false, nil
}

type ModelAnswerResult struct {
	Question    string   `json:"question"`
	ModelAnswer string   `json:"model_answer"`
	KeyPoints   []string `json:"key_points"`
}

func (s *Service) ModelAnswer(ctx context.Context, questionID uuid.UUID, roleName string) (ModelAnswerResult, error) {
	q, err := s.repo.Get(ctx, questionID)
	if err != nil {
		return ModelAnswerResult{}, err
	}
	if q.ModelAnswer != "" {
		return ModelAnswerResult{Question: q.Prompt, ModelAnswer: q.ModelAnswer, KeyPoints: []string{}}, nil
	}
	system, user := llm.ModelAnswerPrompt(roleName, q.Prompt)
	var out struct {
		ModelAnswer string   `json:"model_answer"`
		KeyPoints   []string `json:"key_points"`
	}
	if err := s.llm.CompleteJSON(ctx, system, user, &out); err != nil {
		return ModelAnswerResult{}, err
	}
	_ = s.repo.SaveModelAnswer(ctx, questionID, out.ModelAnswer)
	return ModelAnswerResult{Question: q.Prompt, ModelAnswer: out.ModelAnswer, KeyPoints: out.KeyPoints}, nil
}

func (s *Service) MarkPracticed(ctx context.Context, userID, questionID uuid.UUID, practiced bool) error {
	return s.repo.SetPracticed(ctx, userID, questionID, practiced)
}

type seedQuestion struct {
	RoleName string `json:"role_name"`
	Topic    string `json:"topic"`
	Prompt   string `json:"prompt"`
	Answer   string `json:"answer"`
}

// SeedFromFile maintains a tiny cross-role corpus for internal retrieval. It is
// intentionally separate from dynamically generated Question Bank sets.
func (s *Service) SeedFromFile(ctx context.Context, path string, roleIDByName map[string]uuid.UUID) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var items []seedQuestion
	if err := json.Unmarshal(raw, &items); err != nil {
		return 0, err
	}

	questions := make([]Question, 0, len(items))
	for _, it := range items {
		roleID, ok := roleIDByName[it.RoleName]
		if !ok {
			continue
		}
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(roleID.String()+"|seed|"+it.Prompt))
		q := Question{ID: id, RoleID: roleID, Topic: it.Topic, Prompt: it.Prompt, ModelAnswer: it.Answer, Source: "seed"}
		if err := s.repo.Upsert(ctx, q); err != nil {
			return 0, err
		}
		questions = append(questions, q)
	}

	if s.rag.Count(rag.CollectionQuestions) == 0 {
		docs := make([]rag.Document, 0, len(questions))
		for _, q := range questions {
			docs = append(docs, rag.Document{
				ID:      q.ID.String(),
				Content: q.Prompt,
				Metadata: map[string]string{
					"role_id":     q.RoleID.String(),
					"topic":       q.Topic,
					"question_id": q.ID.String(),
				},
			})
		}
		if err := s.rag.Upsert(ctx, rag.CollectionQuestions, docs); err != nil {
			return 0, err
		}
	}
	return len(questions), nil
}

func rewriteRetrievalQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" { return q }
	l := strings.ToLower(q)
	if strings.Contains(l, "system design") { return q + " scalability load balancing caching database partitioning replication consistency queues rate limiting reliability tradeoffs" }
	if strings.Contains(l, "concurrency") { return q + " goroutines channels mutex race conditions worker pools synchronization" }
	if strings.Contains(l, "database") || strings.Contains(l, "sql") { return q + " indexing query plans transactions isolation partitioning replication" }
	return q
}
