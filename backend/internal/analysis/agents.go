package analysis

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
	"github.com/sagun-patwari/ai-career-platform/internal/llm"
	"github.com/sagun-patwari/ai-career-platform/internal/resume"
	"github.com/sagun-patwari/ai-career-platform/internal/role"
)

type workflowState struct {
	UserID     uuid.UUID           `json:"user_id"`
	ResumeID   uuid.UUID           `json:"resume_id"`
	RoleID     uuid.UUID           `json:"role_id"`
	ResumeJSON string              `json:"resume_json"`
	Role       role.Role           `json:"role"`
	RoleFit    RoleFitResult       `json:"role_fit"`
	Gap        GapResult           `json:"gap"`
	Roadmap    []RoadmapItem       `json:"roadmap"`
	Interview  []InterviewPrepItem `json:"interview"`
}

func (s *Service) loadWorkflowState(ctx context.Context, workflowID string) (workflowState, error) {
	var st workflowState
	_, err := s.state.Load(ctx, workflowID, &st)
	return st, err
}

func (s *Service) patchWorkflowState(ctx context.Context, workflowID string, fn func(*workflowState)) error {
	return s.state.Patch(ctx, workflowID, func(raw json.RawMessage) (any, error) {
		var st workflowState
		if err := json.Unmarshal(raw, &st); err != nil {
			return nil, err
		}
		fn(&st)
		return st, nil
	})
}

func (s *Service) agents(workflowID string) []workflowAgent {
	return []workflowAgent{
		{name: "resume_analysis", run: func(ctx context.Context) error {
			st, err := s.loadWorkflowState(ctx, workflowID)
			if err != nil {
				return err
			}

			res, err := s.resumes.Get(ctx, st.ResumeID)
			if err != nil {
				return err
			}

			if res.UserID != st.UserID {
				return httpx.NewError(http.StatusForbidden, "forbidden", "not your resume")
			}
			if res.Status != resume.StatusParsed {
				return httpx.NewError(http.StatusConflict, "not_ready", "resume is still being parsed")
			}

			targetRole, err := s.roles.Get(ctx, st.RoleID)
			if err != nil {
				return err
			}

			return s.patchWorkflowState(ctx, workflowID, func(next *workflowState) {
				next.ResumeJSON = string(res.ParsedJSON)
				next.Role = targetRole
			})
		}},
		{name: "role_fit", dependsOn: []string{"resume_analysis"}, run: func(ctx context.Context) error {
			st, err := s.loadWorkflowState(ctx, workflowID)
			if err != nil {
				return err
			}

			system, user := llm.RoleFitPrompt(st.ResumeJSON, st.Role.Name, st.Role.RequiredSkills)
			var out RoleFitResult
			if err := s.llm.CompleteJSON(ctx, system, user, &out); err != nil {
				return err
			}

			return s.patchWorkflowState(ctx, workflowID, func(next *workflowState) {
				next.RoleFit = out
			})
		}},
		{name: "skill_gap", dependsOn: []string{"resume_analysis"}, run: func(ctx context.Context) error {
			st, err := s.loadWorkflowState(ctx, workflowID)
			if err != nil {
				return err
			}

			system, user := llm.GapAnalysisPrompt(st.ResumeJSON, st.Role.Name, st.Role.RequiredSkills)
			var out GapResult
			if err := s.llm.CompleteJSON(ctx, system, user, &out); err != nil {
				return err
			}

			return s.patchWorkflowState(ctx, workflowID, func(next *workflowState) {
				next.Gap = out
			})
		}},
		{name: "roadmap", dependsOn: []string{"skill_gap"}, run: func(ctx context.Context) error {
			st, err := s.loadWorkflowState(ctx, workflowID)
			if err != nil {
				return err
			}

			if len(st.Gap.MissingSkills) == 0 {
				return s.patchWorkflowState(ctx, workflowID, func(next *workflowState) {
					next.Roadmap = []RoadmapItem{}
				})
			}

			system, user := llm.RoadmapPrompt(st.Role.Name, st.Gap.MissingSkills)
			var out struct {
				Items []RoadmapItem `json:"items"`
			}
			if err := s.llm.CompleteJSON(ctx, system, user, &out); err != nil {
				return err
			}

			return s.patchWorkflowState(ctx, workflowID, func(next *workflowState) {
				next.Roadmap = out.Items
			})
		}},
		{name: "interview_prep", dependsOn: []string{"role_fit", "skill_gap"}, run: func(ctx context.Context) error {
			st, err := s.loadWorkflowState(ctx, workflowID)
			if err != nil {
				return err
			}

			queryParts := append([]string{st.Role.Name}, st.Role.RequiredSkills...)
			queryParts = append(queryParts, st.Gap.MissingSkills...)
			queryParts = append(queryParts, st.RoleFit.Concerns...)
			query := strings.Join(queryParts, " ")

			questions, err := s.questions.Search(ctx, query, &st.Role.ID, 20)
			if err != nil {
				slog.Warn("interview question retrieval failed; continuing analysis", "error", err)
				return s.patchWorkflowState(ctx, workflowID, func(next *workflowState) {
					next.Interview = []InterviewPrepItem{}
				})
			}

			items := make([]InterviewPrepItem, 0, len(questions))
			for _, q := range questions {
				items = append(items, InterviewPrepItem{ID: q.ID, Topic: q.Topic, Prompt: q.Prompt})
			}
			return s.patchWorkflowState(ctx, workflowID, func(next *workflowState) {
				next.Interview = items
			})
		}},
	}
}
