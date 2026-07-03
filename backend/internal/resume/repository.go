// Package resume handles uploads, text extraction, LLM parsing into a
// structured profile, and JD-tailored versions.
package resume

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Status tracks the async parse lifecycle.
const (
	StatusUploaded = "uploaded"
	StatusParsing  = "parsing"
	StatusParsed   = "parsed"
	StatusFailed   = "failed"
)

// Resume is a stored resume with its parsed profile.
type Resume struct {
	ID         uuid.UUID       `json:"id"`
	UserID     uuid.UUID       `json:"user_id"`
	Filename   string          `json:"filename"`
	FileURL    string          `json:"file_url"`
	Status     string          `json:"status"`
	ParsedJSON json.RawMessage `json:"parsed,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Tailored is a JD-specific rewrite of a resume.
type Tailored struct {
	ID             uuid.UUID       `json:"id"`
	ResumeID       uuid.UUID       `json:"resume_id"`
	UserID         uuid.UUID       `json:"user_id"`
	JobDescription string          `json:"job_description"`
	ContentJSON    json.RawMessage `json:"content"`
	CreatedAt      time.Time       `json:"created_at"`
}

var ErrNotFound = errors.New("resume not found")

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Create(ctx context.Context, res Resume) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO resumes (id, user_id, filename, file_url, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		res.ID, res.UserID, res.Filename, res.FileURL, res.Status, res.CreatedAt)
	return err
}

func (r *Repository) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE resumes SET status = $2 WHERE id = $1`, id, status)
	return err
}

func (r *Repository) SetParsed(ctx context.Context, id uuid.UUID, parsed json.RawMessage) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE resumes SET parsed_json = $2, status = $3 WHERE id = $1`,
		id, parsed, StatusParsed)
	return err
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Resume, error) {
	var res Resume
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, filename, file_url, status, COALESCE(parsed_json, '{}'::jsonb), created_at
		 FROM resumes WHERE id = $1`, id).
		Scan(&res.ID, &res.UserID, &res.Filename, &res.FileURL, &res.Status, &res.ParsedJSON, &res.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Resume{}, ErrNotFound
	}
	return res, err
}

func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]Resume, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, filename, file_url, status, COALESCE(parsed_json, '{}'::jsonb), created_at
		 FROM resumes WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Resume
	for rows.Next() {
		var res Resume
		if err := rows.Scan(&res.ID, &res.UserID, &res.Filename, &res.FileURL, &res.Status, &res.ParsedJSON, &res.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *Repository) CreateTailored(ctx context.Context, t Tailored) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tailored_resumes (id, resume_id, user_id, job_description, content_json, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		t.ID, t.ResumeID, t.UserID, t.JobDescription, t.ContentJSON, t.CreatedAt)
	return err
}
