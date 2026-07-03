// Package interview runs a stateful mock interview: the model asks questions one
// at a time, each answer is scored, and the full session is persisted so the
// user can resume later.
package interview

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Turn is one message in the interview transcript.
type Turn struct {
	Role    string `json:"role"` // "interviewer" | "candidate"
	Content string `json:"content"`
}

// Score is the evaluation of a single answer.
type Score struct {
	Question    string `json:"question"`
	Answer      string `json:"answer"`
	Clarity     int    `json:"clarity"`
	Correctness int    `json:"correctness"`
	Depth       int    `json:"depth"`
	Feedback    string `json:"feedback"`
}

const (
	StatusActive   = "active"
	StatusComplete = "complete"
)

// Session is a full mock-interview session.
type Session struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	RoleID     uuid.UUID `json:"role_id"`
	RoleName   string    `json:"role_name"`
	Status     string    `json:"status"`
	Transcript []Turn    `json:"transcript"`
	Scores     []Score   `json:"scores"`
	CreatedAt  time.Time `json:"created_at"`
}

var ErrNotFound = errors.New("interview session not found")

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Create(ctx context.Context, s Session) error {
	transcript, _ := json.Marshal(s.Transcript)
	scores, _ := json.Marshal(s.Scores)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO interview_sessions (id, user_id, role_id, status, transcript_json, scores_json, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		s.ID, s.UserID, s.RoleID, s.Status, transcript, scores, s.CreatedAt)
	return err
}

func (r *Repository) Update(ctx context.Context, s Session) error {
	transcript, _ := json.Marshal(s.Transcript)
	scores, _ := json.Marshal(s.Scores)
	_, err := r.pool.Exec(ctx,
		`UPDATE interview_sessions SET status = $2, transcript_json = $3, scores_json = $4 WHERE id = $1`,
		s.ID, s.Status, transcript, scores)
	return err
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Session, error) {
	var s Session
	var transcript, scores []byte
	err := r.pool.QueryRow(ctx,
		`SELECT i.id, i.user_id, i.role_id, r.name, i.status, i.transcript_json, i.scores_json, i.created_at
		 FROM interview_sessions i JOIN roles r ON r.id = i.role_id WHERE i.id = $1`, id).
		Scan(&s.ID, &s.UserID, &s.RoleID, &s.RoleName, &s.Status, &transcript, &scores, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, err
	}
	_ = json.Unmarshal(transcript, &s.Transcript)
	_ = json.Unmarshal(scores, &s.Scores)
	return s, nil
}

func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]Session, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT i.id, i.role_id, r.name, i.status, i.created_at
		 FROM interview_sessions i JOIN roles r ON r.id = i.role_id
		 WHERE i.user_id = $1 ORDER BY i.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var s Session
		s.UserID = userID
		if err := rows.Scan(&s.ID, &s.RoleID, &s.RoleName, &s.Status, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
