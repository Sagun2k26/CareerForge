package interview

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
	"github.com/sagun-patwari/ai-career-platform/internal/llm"
	"github.com/sagun-patwari/ai-career-platform/internal/rag"
	"github.com/sagun-patwari/ai-career-platform/internal/role"
)

const completeMarker = "INTERVIEW_COMPLETE"

// Service drives the stateful interview loop.
type Service struct {
	repo  *Repository
	roles *role.Repository
	rag   *rag.Engine
	llm   *llm.Client
}

func NewService(repo *Repository, roles *role.Repository, engine *rag.Engine, client *llm.Client) *Service {
	return &Service{repo: repo, roles: roles, rag: engine, llm: client}
}

// TurnResult is returned to the client after each exchange.
type TurnResult struct {
	Session      Session `json:"session"`
	NextQuestion string  `json:"next_question,omitempty"`
	LastScore    *Score  `json:"last_score,omitempty"`
	Done         bool    `json:"done"`
}

// Start creates a session and asks the first question.
func (s *Service) Start(ctx context.Context, userID, roleID uuid.UUID) (TurnResult, error) {
	targetRole, err := s.roles.Get(ctx, roleID)
	if err != nil {
		return TurnResult{}, err
	}
	system := s.systemPrompt(ctx, targetRole)

	question, err := s.llm.Complete(ctx, system, []llm.Message{
		{Role: llm.RoleUser, Content: "Please begin the interview."},
	})
	if err != nil {
		return TurnResult{}, err
	}
	question = strings.TrimSpace(question)

	sess := Session{
		ID:         uuid.New(),
		UserID:     userID,
		RoleID:     roleID,
		RoleName:   targetRole.Name,
		Status:     StatusActive,
		Transcript: []Turn{{Role: "interviewer", Content: question}},
		Scores:     []Score{},
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, sess); err != nil {
		return TurnResult{}, err
	}
	return TurnResult{Session: sess, NextQuestion: question, Done: false}, nil
}

// Answer records the candidate's answer, scores it, and asks the next question.
func (s *Service) Answer(ctx context.Context, userID, sessionID uuid.UUID, answer string) (TurnResult, error) {
	sess, err := s.repo.Get(ctx, sessionID)
	if err != nil {
		return TurnResult{}, err
	}
	if sess.UserID != userID {
		return TurnResult{}, httpx.NewError(http.StatusForbidden, "forbidden", "not your session")
	}
	if sess.Status != StatusActive {
		return TurnResult{}, httpx.NewError(http.StatusConflict, "completed", "this interview is already complete")
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return TurnResult{}, httpx.NewError(http.StatusBadRequest, "empty_answer", "answer cannot be empty")
	}

	lastQuestion := lastInterviewerTurn(sess.Transcript)
	sess.Transcript = append(sess.Transcript, Turn{Role: "candidate", Content: answer})

	// Score the answer.
	scoreSys, scoreUser := llm.ScoreAnswerPrompt(sess.RoleName, lastQuestion, answer)
	var score Score
	if err := s.llm.CompleteJSON(ctx, scoreSys, scoreUser, &score); err != nil {
		return TurnResult{}, err
	}
	score.Question = lastQuestion
	score.Answer = answer
	sess.Scores = append(sess.Scores, score)

	// Ask the next question with the full conversation as context.
	targetRole, err := s.roles.Get(ctx, sess.RoleID)
	if err != nil {
		return TurnResult{}, err
	}
	system := s.systemPrompt(ctx, targetRole)
	next, err := s.llm.Complete(ctx, system, toLLMMessages(sess.Transcript))
	if err != nil {
		return TurnResult{}, err
	}
	next = strings.TrimSpace(next)

	result := TurnResult{LastScore: &score}
	if strings.Contains(next, completeMarker) {
		sess.Status = StatusComplete
		closing := "That's the end of the interview. Review your per-answer scores and feedback below — strong work."
		sess.Transcript = append(sess.Transcript, Turn{Role: "interviewer", Content: closing})
		result.Done = true
	} else {
		sess.Transcript = append(sess.Transcript, Turn{Role: "interviewer", Content: next})
		result.NextQuestion = next
	}

	if err := s.repo.Update(ctx, sess); err != nil {
		return TurnResult{}, err
	}
	result.Session = sess
	return result, nil
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Session, error) {
	sess, err := s.repo.Get(ctx, id)
	if err != nil {
		return Session{}, err
	}
	if sess.UserID != userID {
		return Session{}, httpx.NewError(http.StatusForbidden, "forbidden", "not your session")
	}
	return sess, nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Session, error) {
	return s.repo.ListByUser(ctx, userID)
}

// systemPrompt grounds the interviewer with role-specific focus areas pulled
// from the vector store (RAG).
func (s *Service) systemPrompt(ctx context.Context, targetRole role.Role) string {
	base := llm.InterviewerSystem(targetRole.Name)
	results, err := s.rag.Query(ctx, rag.CollectionQuestions, targetRole.Name, 3,
		map[string]string{"role_id": targetRole.ID.String()})
	if err != nil || len(results) == 0 {
		return base
	}
	var topics []string
	for _, r := range results {
		topics = append(topics, r.Content)
	}
	return base + "\n\nDraw inspiration from these role-relevant areas: " + strings.Join(topics, " | ")
}

func lastInterviewerTurn(transcript []Turn) string {
	for i := len(transcript) - 1; i >= 0; i-- {
		if transcript[i].Role == "interviewer" {
			return transcript[i].Content
		}
	}
	return ""
}

func toLLMMessages(transcript []Turn) []llm.Message {
	out := make([]llm.Message, 0, len(transcript))
	for _, t := range transcript {
		role := llm.RoleUser
		if t.Role == "interviewer" {
			role = llm.RoleAssistant
		}
		out = append(out, llm.Message{Role: role, Content: t.Content})
	}
	return out
}
