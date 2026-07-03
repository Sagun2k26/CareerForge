package questionbank

import (
	"context"
	"encoding/json"
	"os"

	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/llm"
	"github.com/sagun-patwari/ai-career-platform/internal/rag"
)

// Service owns question persistence, vector indexing and retrieval.
type Service struct {
	repo *Repository
	rag  *rag.Engine
	llm  *llm.Client
}

func NewService(repo *Repository, engine *rag.Engine, client *llm.Client) *Service {
	return &Service{repo: repo, rag: engine, llm: client}
}

// Index stores questions in Postgres and embeds them into the vector store.
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

// ListByRole browses questions for a role/topic.
func (s *Service) ListByRole(ctx context.Context, userID, roleID uuid.UUID, topic string) ([]Question, error) {
	return s.repo.ListByRole(ctx, userID, roleID, topic)
}

// Search retrieves the most semantically relevant questions to a query, scoped
// to a role when provided. This is the RAG retrieval path.
func (s *Service) Search(ctx context.Context, query string, roleID *uuid.UUID, n int) ([]Question, error) {
	if n <= 0 {
		n = 5
	}
	var where map[string]string
	if roleID != nil {
		where = map[string]string{"role_id": roleID.String()}
	}
	results, err := s.rag.Query(ctx, rag.CollectionQuestions, query, n, where)
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
	// Preserve similarity order returned by the vector store.
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

// ModelAnswerResult bundles the answer and the key points an answer should hit.
type ModelAnswerResult struct {
	Question    string   `json:"question"`
	ModelAnswer string   `json:"model_answer"`
	KeyPoints   []string `json:"key_points"`
}

// ModelAnswer returns a cached model answer or generates and caches one.
func (s *Service) ModelAnswer(ctx context.Context, questionID uuid.UUID, roleName string) (ModelAnswerResult, error) {
	q, err := s.repo.Get(ctx, questionID)
	if err != nil {
		return ModelAnswerResult{}, err
	}
	if q.ModelAnswer != "" {
		return ModelAnswerResult{Question: q.Prompt, ModelAnswer: q.ModelAnswer}, nil
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

// MarkPracticed toggles a question's practiced state for a user.
func (s *Service) MarkPracticed(ctx context.Context, userID, questionID uuid.UUID, practiced bool) error {
	return s.repo.SetPracticed(ctx, userID, questionID, practiced)
}

// ---- Seeding --------------------------------------------------------------

type seedQuestion struct {
	RoleName string `json:"role_name"`
	Topic    string `json:"topic"`
	Prompt   string `json:"prompt"`
}

// SeedFromFile loads questions from JSON (keyed by role name), resolves role
// ids, and indexes them. It is idempotent: ids are derived deterministically
// from role + prompt so re-seeding does not duplicate.
func (s *Service) SeedFromFile(ctx context.Context, path string, roleIDByName map[string]uuid.UUID) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var items []seedQuestion
	if err := json.Unmarshal(raw, &items); err != nil {
		return 0, err
	}
	var questions []Question
	for _, it := range items {
		roleID, ok := roleIDByName[it.RoleName]
		if !ok {
			continue
		}
		// Deterministic id from role + prompt.
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(roleID.String()+it.Prompt))
		questions = append(questions, Question{ID: id, RoleID: roleID, Topic: it.Topic, Prompt: it.Prompt})
	}
	if err := s.Index(ctx, questions); err != nil {
		return 0, err
	}
	return len(questions), nil
}
