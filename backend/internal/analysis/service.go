package analysis

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
	"github.com/sagun-patwari/ai-career-platform/internal/llm"
	"github.com/sagun-patwari/ai-career-platform/internal/resume"
	"github.com/sagun-patwari/ai-career-platform/internal/role"
)

// Service runs the gap analysis + roadmap generation flow.
type Service struct {
	repo     *Repository
	resumes  *resume.Repository
	roles    *role.Repository
	llm      *llm.Client
}

func NewService(repo *Repository, resumes *resume.Repository, roles *role.Repository, client *llm.Client) *Service {
	return &Service{repo: repo, resumes: resumes, roles: roles, llm: client}
}

// Analyze compares a parsed resume to a role and builds a roadmap.
func (s *Service) Analyze(ctx context.Context, userID, resumeID, roleID uuid.UUID) (Analysis, error) {
	res, err := s.resumes.Get(ctx, resumeID)
	if err != nil {
		return Analysis{}, err
	}
	if res.UserID != userID {
		return Analysis{}, httpx.NewError(http.StatusForbidden, "forbidden", "not your resume")
	}
	if res.Status != resume.StatusParsed {
		return Analysis{}, httpx.NewError(http.StatusConflict, "not_ready", "resume is still being parsed")
	}
	targetRole, err := s.roles.Get(ctx, roleID)
	if err != nil {
		return Analysis{}, err
	}

	// 1. Skill-gap analysis.
	gapSys, gapUser := llm.GapAnalysisPrompt(string(res.ParsedJSON), targetRole.Name, targetRole.RequiredSkills)
	var gap GapResult
	if err := s.llm.CompleteJSON(ctx, gapSys, gapUser, &gap); err != nil {
		return Analysis{}, err
	}

	// 2. Roadmap for the missing skills.
	roadSys, roadUser := llm.RoadmapPrompt(targetRole.Name, gap.MissingSkills)
	var roadmap struct {
		Items []RoadmapItem `json:"items"`
	}
	if err := s.llm.CompleteJSON(ctx, roadSys, roadUser, &roadmap); err != nil {
		return Analysis{}, err
	}

	a := Analysis{
		ID:            uuid.New(),
		UserID:        userID,
		ResumeID:      resumeID,
		RoleID:        roleID,
		RoleName:      targetRole.Name,
		GapScore:      gap.GapScore,
		MatchedSkills: gap.MatchedSkills,
		MissingSkills: gap.MissingSkills,
		Summary:       gap.Summary,
		Roadmap:       roadmap.Items,
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
