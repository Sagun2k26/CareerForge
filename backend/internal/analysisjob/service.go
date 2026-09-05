package analysisjob

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/analysis"
	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
)

type Service struct {
	repo     *Repository
	analysis *analysis.Service
	logger   *slog.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewService(repo *Repository, analysisSvc *analysis.Service, logger *slog.Logger) *Service {
	return &Service{repo: repo, analysis: analysisSvc, logger: logger}
}

func (s *Service) Enqueue(ctx context.Context, userID, resumeID, roleID uuid.UUID) (Job, error) {
	return s.repo.Create(ctx, userID, resumeID, roleID)
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Job, error) {
	j, err := s.repo.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if j.UserID != userID {
		return Job{}, httpx.NewError(http.StatusForbidden, "forbidden", "not your analysis job")
	}
	return j, nil
}

func (s *Service) Start(parent context.Context, workerCount int) {
	if workerCount < 1 {
		workerCount = 1
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel

	for i := 0; i < workerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i+1)
	}
	s.logger.Info("analysis workers started", "count", workerCount)
}

func (s *Service) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Service) worker(ctx context.Context, workerID int) {
	defer s.wg.Done()

	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, ok, err := s.repo.ClaimNext(ctx)
			if err != nil {
				if ctx.Err() == nil {
					s.logger.Error("claim analysis job", "worker", workerID, "error", err)
				}
				continue
			}
			if !ok {
				continue
			}
			s.execute(ctx, workerID, job)
		}
	}
}

func (s *Service) execute(parent context.Context, workerID int, job Job) {
	s.logger.Info("analysis job started", "worker", workerID, "job_id", job.ID)

	ctx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()

	result, err := s.analysis.Analyze(ctx, job.UserID, job.ResumeID, job.RoleID)
	if err != nil {
		s.logger.Error("analysis job failed", "worker", workerID, "job_id", job.ID, "error", err)
		_ = s.repo.Fail(context.Background(), job.ID, "analysis failed")
		return
	}

	if err := s.repo.Complete(context.Background(), job.ID, result.ID); err != nil {
		s.logger.Error("mark analysis job complete", "job_id", job.ID, "error", err)
		return
	}
	s.logger.Info("analysis job completed", "worker", workerID, "job_id", job.ID, "analysis_id", result.ID)
}
