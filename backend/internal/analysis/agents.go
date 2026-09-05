package analysis

import (
	"context"
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
	userID     uuid.UUID
	resumeID   uuid.UUID
	roleID     uuid.UUID
	resumeJSON string
	role       role.Role
	roleFit    RoleFitResult
	gap        GapResult
	roadmap    []RoadmapItem
	interview  []InterviewPrepItem
}

func (s *Service) agents(st *workflowState) []workflowAgent {
	return []workflowAgent{
		{name: "resume_analysis", run: func(ctx context.Context) error {
			res, err := s.resumes.Get(ctx, st.resumeID)
			if err != nil {
				return err
			}
			if res.UserID != st.userID {
				return httpx.NewError(http.StatusForbidden, "forbidden", "not your resume")
			}
			if res.Status != resume.StatusParsed {
				return httpx.NewError(http.StatusConflict, "not_ready", "resume is still being parsed")
			}
			targetRole, err := s.roles.Get(ctx, st.roleID)
			if err != nil {
				return err
			}
			st.resumeJSON, st.role = string(res.ParsedJSON), targetRole
			return nil
		}},
		{name: "role_fit", dependsOn: []string{"resume_analysis"}, run: func(ctx context.Context) error {
			system, user := llm.RoleFitPrompt(st.resumeJSON, st.role.Name, st.role.RequiredSkills)
			return s.llm.CompleteJSON(ctx, system, user, &st.roleFit)
		}},
		{name: "skill_gap", dependsOn: []string{"resume_analysis"}, run: func(ctx context.Context) error {
			system, user := llm.GapAnalysisPrompt(st.resumeJSON, st.role.Name, st.role.RequiredSkills)
			return s.llm.CompleteJSON(ctx, system, user, &st.gap)
		}},
		{name: "roadmap", dependsOn: []string{"skill_gap"}, run: func(ctx context.Context) error {
			if len(st.gap.MissingSkills) == 0 {
				st.roadmap = []RoadmapItem{}
				return nil
			}
			system, user := llm.RoadmapPrompt(st.role.Name, st.gap.MissingSkills)
			var out struct {
				Items []RoadmapItem `json:"items"`
			}
			if err := s.llm.CompleteJSON(ctx, system, user, &out); err != nil {
				return err
			}
			st.roadmap = out.Items
			return nil
		}},
		{name: "interview_prep", dependsOn: []string{"role_fit", "skill_gap"}, run: func(ctx context.Context) error {
			queryParts := append([]string{st.role.Name}, st.role.RequiredSkills...)
			queryParts = append(queryParts, st.gap.MissingSkills...)
			queryParts = append(queryParts, st.roleFit.Concerns...)
			query := strings.Join(queryParts, " ")
			questions, err := s.questions.Search(ctx, query, &st.role.ID, 20)
			if err != nil {
				slog.Warn("interview question retrieval failed; continuing analysis", "error", err)
				st.interview = []InterviewPrepItem{}
				return nil
			}
			st.interview = make([]InterviewPrepItem, 0, len(questions))
			for _, q := range questions {
				st.interview = append(st.interview, InterviewPrepItem{ID: q.ID, Topic: q.Topic, Prompt: q.Prompt})
			}
			return nil
		}},
	}
}
