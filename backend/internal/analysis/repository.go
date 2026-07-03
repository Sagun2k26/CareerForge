// Package analysis compares a parsed resume against a target role to produce a
// skill-gap score and an ordered learning roadmap, and tracks the user's
// progress through that roadmap.
package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GapResult is the model's read on resume-vs-role fit.
type GapResult struct {
	GapScore      int      `json:"gap_score"`
	MatchedSkills []string `json:"matched_skills"`
	MissingSkills []string `json:"missing_skills"`
	Summary       string   `json:"summary"`
}

// RoadmapItem is one ordered step in the learning plan.
type RoadmapItem struct {
	Order     int      `json:"order"`
	Topic     string   `json:"topic"`
	Why       string   `json:"why"`
	Resources []string `json:"resources"`
	Milestone string   `json:"milestone"`
	Status    string   `json:"status,omitempty"` // filled from progress on read
}

// Analysis is a persisted gap analysis plus its roadmap.
type Analysis struct {
	ID            uuid.UUID     `json:"id"`
	UserID        uuid.UUID     `json:"user_id"`
	ResumeID      uuid.UUID     `json:"resume_id"`
	RoleID        uuid.UUID     `json:"role_id"`
	RoleName      string        `json:"role_name"`
	GapScore      int           `json:"gap_score"`
	MatchedSkills []string      `json:"matched_skills"`
	MissingSkills []string      `json:"missing_skills"`
	Summary       string        `json:"summary"`
	Roadmap       []RoadmapItem `json:"roadmap"`
	CreatedAt     time.Time     `json:"created_at"`
}

const (
	ProgressTodo string = "todo"
	ProgressDone string = "done"
)

var ErrNotFound = errors.New("analysis not found")

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Save persists the analysis and its roadmap in one transaction.
func (r *Repository) Save(ctx context.Context, a Analysis) error {
	matched, _ := json.Marshal(a.MatchedSkills)
	missing, _ := json.Marshal(a.MissingSkills)
	roadmap, _ := json.Marshal(a.Roadmap)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO analyses (id, user_id, resume_id, role_id, gap_score, matched_skills_json, missing_skills_json, summary, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		a.ID, a.UserID, a.ResumeID, a.RoleID, a.GapScore, matched, missing, a.Summary, a.CreatedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO roadmaps (id, analysis_id, items_json) VALUES ($1, $2, $3)`,
		uuid.New(), a.ID, roadmap); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Get loads an analysis, its roadmap and the user's progress merged in.
func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Analysis, error) {
	var a Analysis
	var matched, missing []byte
	err := r.pool.QueryRow(ctx,
		`SELECT a.id, a.user_id, a.resume_id, a.role_id, r.name, a.gap_score,
		        a.matched_skills_json, a.missing_skills_json, a.summary, a.created_at
		 FROM analyses a JOIN roles r ON r.id = a.role_id WHERE a.id = $1`, id).
		Scan(&a.ID, &a.UserID, &a.ResumeID, &a.RoleID, &a.RoleName, &a.GapScore,
			&matched, &missing, &a.Summary, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, ErrNotFound
	}
	if err != nil {
		return Analysis{}, err
	}
	_ = json.Unmarshal(matched, &a.MatchedSkills)
	_ = json.Unmarshal(missing, &a.MissingSkills)

	var items []byte
	if err := r.pool.QueryRow(ctx, `SELECT items_json FROM roadmaps WHERE analysis_id = $1`, id).Scan(&items); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, err
	}
	_ = json.Unmarshal(items, &a.Roadmap)

	// Merge progress.
	progress, err := r.progressMap(ctx, a.UserID, id)
	if err != nil {
		return Analysis{}, err
	}
	for i := range a.Roadmap {
		if status, ok := progress[a.Roadmap[i].Order]; ok {
			a.Roadmap[i].Status = status
		} else {
			a.Roadmap[i].Status = ProgressTodo
		}
	}
	return a, nil
}

// ListByUser returns lightweight analysis summaries for the dashboard.
func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]Analysis, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.role_id, r.name, a.gap_score, a.created_at
		 FROM analyses a JOIN roles r ON r.id = a.role_id
		 WHERE a.user_id = $1 ORDER BY a.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Analysis
	for rows.Next() {
		var a Analysis
		a.UserID = userID
		if err := rows.Scan(&a.ID, &a.RoleID, &a.RoleName, &a.GapScore, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) progressMap(ctx context.Context, userID, analysisID uuid.UUID) (map[int]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT item_order, status FROM progress WHERE user_id = $1 AND analysis_id = $2`, userID, analysisID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int]string)
	for rows.Next() {
		var order int
		var status string
		if err := rows.Scan(&order, &status); err != nil {
			return nil, err
		}
		out[order] = status
	}
	return out, rows.Err()
}

// SetProgress upserts the status of a single roadmap item.
func (r *Repository) SetProgress(ctx context.Context, userID, analysisID uuid.UUID, itemOrder int, status string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO progress (id, user_id, analysis_id, item_order, status, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (user_id, analysis_id, item_order)
		 DO UPDATE SET status = EXCLUDED.status, updated_at = EXCLUDED.updated_at`,
		uuid.New(), userID, analysisID, itemOrder, status, time.Now().UTC())
	return err
}
