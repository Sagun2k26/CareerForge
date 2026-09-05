package resume

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
	"github.com/sagun-patwari/ai-career-platform/internal/llm"
	"github.com/sagun-patwari/ai-career-platform/internal/worker"
)

// Service coordinates storage, async resume-agent execution and tailoring.
type Service struct {
	repo       *Repository
	llm        *llm.Client
	pool       *worker.Pool
	agent      analysisAgent
	storageDir string
}

func NewService(repo *Repository, client *llm.Client, pool *worker.Pool, storageDir string) (*Service, error) {
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &Service{repo: repo, llm: client, pool: pool, agent: analysisAgent{llm: client}, storageDir: storageDir}, nil
}

// Upload stores the raw file and enqueues the Resume Analysis Agent so the HTTP
// request returns quickly while extraction + model parsing happens in background.
func (s *Service) Upload(ctx context.Context, userID uuid.UUID, filename string, data []byte) (Resume, error) {
	if len(data) == 0 {
		return Resume{}, httpx.NewError(http.StatusBadRequest, "empty_file", "uploaded file is empty")
	}
	id := uuid.New()
	path := filepath.Join(s.storageDir, id.String()+"_"+filepath.Base(filename))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return Resume{}, fmt.Errorf("write file: %w", err)
	}
	res := Resume{ID: id, UserID: userID, Filename: filename, FileURL: path, Status: StatusUploaded, CreatedAt: time.Now().UTC()}
	if err := s.repo.Create(ctx, res); err != nil {
		return Resume{}, err
	}

	s.pool.Enqueue(worker.Job{
		Name: "resume_analysis:" + id.String(),
		Run: func(jobCtx context.Context) error {
			return s.runAnalysisAgent(jobCtx, id, filename, data)
		},
	})
	return res, nil
}

func (s *Service) runAnalysisAgent(ctx context.Context, id uuid.UUID, filename string, data []byte) error {
	_ = s.repo.SetStatus(ctx, id, StatusParsing)
	parsed, err := s.agent.Run(ctx, filename, data)
	if err != nil {
		_ = s.repo.SetStatus(ctx, id, StatusFailed)
		return err
	}
	return s.repo.SetParsed(ctx, id, parsed)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (Resume, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Resume, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *Service) Tailor(ctx context.Context, userID, resumeID uuid.UUID, jobDescription string) (Tailored, error) {
	res, err := s.repo.Get(ctx, resumeID)
	if err != nil {
		return Tailored{}, err
	}
	if res.UserID != userID {
		return Tailored{}, httpx.NewError(http.StatusForbidden, "forbidden", "not your resume")
	}
	if res.Status != StatusParsed {
		return Tailored{}, httpx.NewError(http.StatusConflict, "not_ready", "resume is still being parsed")
	}
	system, user := llm.TailorResumePrompt(string(res.ParsedJSON), jobDescription)
	var content map[string]any
	if err := s.llm.CompleteJSON(ctx, system, user, &content); err != nil {
		return Tailored{}, err
	}
	raw, _ := json.Marshal(content)
	t := Tailored{ID: uuid.New(), ResumeID: resumeID, UserID: userID, JobDescription: jobDescription, ContentJSON: raw, CreatedAt: time.Now().UTC()}
	if err := s.repo.CreateTailored(ctx, t); err != nil {
		return Tailored{}, err
	}
	return t, nil
}
