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
}

func NewService(repo *Repository, resumes *resume.Repository, roles *role.Repository, questions *questionbank.Service, client *llm.Client) *Service {
	return &Service{repo: repo, resumes: resumes, roles: roles, questions: questions, llm: client}
}

// Analyze runs the dependency-aware agent graph and persists its final state.
func (s *Service) Analyze(ctx context.Context, userID, resumeID, roleID uuid.UUID) (Analysis, error) {
	st := &workflowState{userID: userID, resumeID: resumeID, roleID: roleID}
	if err := runWorkflow(ctx, s.agents(st)); err != nil {
		return Analysis{}, err
	}

	a := Analysis{
		ID:            uuid.New(),
		UserID:        userID,
		ResumeID:      resumeID,
		RoleID:        roleID,
		RoleName:      st.role.Name,
		GapScore:      st.gap.GapScore,
		MatchedSkills: st.gap.MatchedSkills,
		MissingSkills: st.gap.MissingSkills,
		Summary:       st.gap.Summary,
		RoleFit:       st.roleFit,
		InterviewPrep: st.interview,
		Roadmap:       st.roadmap,
		CreatedAt:     time.Now().UTC(),
	}
	if err := s.repo.Save(ctx, a); err != nil {
		return Analysis{}, err
	}
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
