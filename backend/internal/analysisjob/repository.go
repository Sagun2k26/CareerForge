package analysisjob

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

var ErrNotFound = errors.New("analysis job not found")

type Job struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	ResumeID   uuid.UUID  `json:"resume_id"`
	RoleID     uuid.UUID  `json:"role_id"`
	Status     string     `json:"status"`
	AnalysisID *uuid.UUID `json:"analysis_id,omitempty"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) findActive(ctx context.Context, userID, resumeID, roleID uuid.UUID) (Job, bool, error) {
	var j Job
	err := r.pool.QueryRow(ctx, `
		SELECT id,user_id,resume_id,role_id,status,analysis_id,COALESCE(error,''),created_at,updated_at
		FROM analysis_jobs WHERE user_id=$1 AND resume_id=$2 AND role_id=$3
		AND status IN ('queued','running') AND created_at > NOW() - INTERVAL '2 minutes'
		ORDER BY created_at DESC LIMIT 1`, userID, resumeID, roleID).Scan(
		&j.ID, &j.UserID, &j.ResumeID, &j.RoleID, &j.Status, &j.AnalysisID, &j.Error, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, nil
	}
	return j, err == nil, err
}

func (r *Repository) Create(ctx context.Context, userID, resumeID, roleID uuid.UUID) (Job, error) {
	if existing, ok, err := r.findActive(ctx, userID, resumeID, roleID); err != nil {
		return Job{}, err
	} else if ok {
		return existing, nil
	}
	now := time.Now().UTC()
	j := Job{
		ID:        uuid.New(),
		UserID:    userID,
		ResumeID:  resumeID,
		RoleID:    roleID,
		Status:    StatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO analysis_jobs
			(id, user_id, resume_id, role_id, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		j.ID, j.UserID, j.ResumeID, j.RoleID, j.Status, j.CreatedAt, j.UpdatedAt,
	)
	return j, err
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Job, error) {
	var j Job
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, resume_id, role_id, status, analysis_id,
		       COALESCE(error, ''), created_at, updated_at
		FROM analysis_jobs
		WHERE id = $1`, id,
	).Scan(
		&j.ID, &j.UserID, &j.ResumeID, &j.RoleID, &j.Status, &j.AnalysisID,
		&j.Error, &j.CreatedAt, &j.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	return j, err
}

func (r *Repository) ClaimNext(ctx context.Context) (Job, bool, error) {
	var j Job
	err := r.pool.QueryRow(ctx, `
		WITH next_job AS (
			SELECT id
			FROM analysis_jobs
			WHERE status = 'queued'
			   OR (status = 'running' AND updated_at < NOW() - INTERVAL '10 minutes')
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		UPDATE analysis_jobs j
		SET status = 'running', error = NULL, updated_at = NOW()
		FROM next_job
		WHERE j.id = next_job.id
		RETURNING j.id, j.user_id, j.resume_id, j.role_id, j.status,
		          j.analysis_id, COALESCE(j.error, ''), j.created_at, j.updated_at`,
	).Scan(
		&j.ID, &j.UserID, &j.ResumeID, &j.RoleID, &j.Status, &j.AnalysisID,
		&j.Error, &j.CreatedAt, &j.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, err
	}
	return j, true, nil
}

func (r *Repository) Complete(ctx context.Context, jobID, analysisID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE analysis_jobs
		SET status = 'completed', analysis_id = $2, error = NULL, updated_at = NOW()
		WHERE id = $1`,
		jobID, analysisID,
	)
	return err
}

func (r *Repository) Fail(ctx context.Context, jobID uuid.UUID, message string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE analysis_jobs
		SET status = 'failed', error = $2, updated_at = NOW()
		WHERE id = $1`,
		jobID, message,
	)
	return err
}
