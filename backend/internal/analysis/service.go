package analysis

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
	"github.com/sagun-patwari/ai-career-platform/internal/llm"
	"github.com/sagun-patwari/ai-career-platform/internal/questionbank"
	"github.com/sagun-patwari/ai-career-platform/internal/resume"
	"github.com/sagun-patwari/ai-career-platform/internal/role"
)

// Service coordinates the multi-agent career analysis workflow.
type Service struct {
	repo      *Repository
	resumes   *resume.Repository
	roles     *role.Repository
	questions *questionbank.Service
	llm       *llm.Client
	state     *RedisStateAdapter
}

func NewService(repo *Repository, resumes *resume.Repository, roles *role.Repository, questions *questionbank.Service, client *llm.Client, state *RedisStateAdapter) *Service {
	return &Service{repo: repo, resumes: resumes, roles: roles, questions: questions, llm: client, state: state}
}

// Analyze runs the dependency-aware agent graph with intermediate state in Redis,
// then persists the final analysis in PostgreSQL.
func (s *Service) Analyze(ctx context.Context, userID, resumeID, roleID uuid.UUID) (Analysis, error) {
	if err := s.ensureRedisState(); err != nil {
		return Analysis{}, err
	}

	analysisID := uuid.New()
	workflowID := analysisID.String()

	initial := workflowState{
		UserID:   userID,
		ResumeID: resumeID,
		RoleID:   roleID,
	}
	if err := s.state.Init(ctx, workflowID, initial); err != nil {
		return Analysis{}, err
	}

	if err := runWorkflow(ctx, s.agents(workflowID)); err != nil {
		return Analysis{}, err
	}

	st, err := s.loadWorkflowState(ctx, workflowID)
	if err != nil {
		return Analysis{}, err
	}

	a := Analysis{
		ID:            analysisID,
		UserID:        userID,
		ResumeID:      resumeID,
		RoleID:        roleID,
		RoleName:      st.Role.Name,
		GapScore:      st.Gap.GapScore,
		MatchedSkills: st.Gap.MatchedSkills,
		MissingSkills: st.Gap.MissingSkills,
		Summary:       st.Gap.Summary,
		RoleFit:       st.RoleFit,
		InterviewPrep: st.Interview,
		Roadmap:       st.Roadmap,
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.repo.Save(ctx, a); err != nil {
		return Analysis{}, err
	}

	_ = s.state.Delete(ctx, workflowID)
	return s.repo.Get(ctx, a.ID)
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Analysis, error) {
	a, err := s.repo.Get(ctx, id)
	if err != nil {
		return Analysis{}, err
	}
	if a.UserID != userID {
		return Analysis{}, httpx.NewError(http.StatusForbidden, "forbidden", "not your analysis")
	}
	return a, nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Analysis, error) {
	return s.repo.ListByUser(ctx, userID)
}

// UpdateProgress marks a roadmap item done/todo.
func (s *Service) UpdateProgress(ctx context.Context, userID, analysisID uuid.UUID, itemOrder int, status string) error {
	if status != ProgressTodo && status != ProgressDone {
		return httpx.NewError(http.StatusBadRequest, "invalid_status", "status must be 'todo' or 'done'")
	}
	a, err := s.repo.Get(ctx, analysisID)
	if err != nil {
		return err
	}
	if a.UserID != userID {
		return httpx.NewError(http.StatusForbidden, "forbidden", "not your analysis")
	}
	return s.repo.SetProgress(ctx, userID, analysisID, itemOrder, status)
}
