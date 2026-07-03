// Package questionbank serves role-specific interview prep questions. Questions
// are stored in Postgres and embedded into the vector store so users can browse
// by role/topic or search semantically, mark questions practiced, and request
// LLM-generated model answers.
package questionbank

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Question is a single prep question.
type Question struct {
	ID          uuid.UUID `json:"id"`
	RoleID      uuid.UUID `json:"role_id"`
	Topic       string    `json:"topic"`
	Prompt      string    `json:"prompt"`
	ModelAnswer string    `json:"model_answer,omitempty"`
	Practiced   bool      `json:"practiced"`
}

const (
	StatusPracticed = "practiced"
)

var ErrNotFound = errors.New("question not found")

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Upsert(ctx context.Context, q Question) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO questions (id, role_id, topic, prompt, model_answer)
		 VALUES ($1,$2,$3,$4,NULLIF($5,''))
		 ON CONFLICT (id) DO UPDATE SET topic = EXCLUDED.topic, prompt = EXCLUDED.prompt`,
		q.ID, q.RoleID, q.Topic, q.Prompt, q.ModelAnswer)
	return err
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Question, error) {
	var q Question
	var model *string
	err := r.pool.QueryRow(ctx,
		`SELECT id, role_id, topic, prompt, model_answer FROM questions WHERE id = $1`, id).
		Scan(&q.ID, &q.RoleID, &q.Topic, &q.Prompt, &model)
	if errors.Is(err, pgx.ErrNoRows) {
		return Question{}, ErrNotFound
	}
	if model != nil {
		q.ModelAnswer = *model
	}
	return q, err
}

func (r *Repository) GetMany(ctx context.Context, ids []uuid.UUID) ([]Question, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, role_id, topic, prompt, model_answer FROM questions WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanQuestions(rows)
}

// ListByRole returns questions for a role, optionally filtered by topic, with
// the given user's practiced flags merged in.
func (r *Repository) ListByRole(ctx context.Context, userID, roleID uuid.UUID, topic string) ([]Question, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT q.id, q.role_id, q.topic, q.prompt, q.model_answer,
		        COALESCE(p.status = $3, false) AS practiced
		 FROM questions q
		 LEFT JOIN question_progress p ON p.question_id = q.id AND p.user_id = $1
		 WHERE q.role_id = $2 AND ($4 = '' OR q.topic = $4)
		 ORDER BY q.topic, q.prompt`,
		userID, roleID, StatusPracticed, topic)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanQuestionsWithPracticed(rows)
}

func (r *Repository) SaveModelAnswer(ctx context.Context, id uuid.UUID, answer string) error {
	_, err := r.pool.Exec(ctx, `UPDATE questions SET model_answer = $2 WHERE id = $1`, id, answer)
	return err
}

func (r *Repository) SetPracticed(ctx context.Context, userID, questionID uuid.UUID, practiced bool) error {
	status := ""
	if practiced {
		status = StatusPracticed
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO question_progress (id, user_id, question_id, status, last_practiced)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (user_id, question_id)
		 DO UPDATE SET status = EXCLUDED.status, last_practiced = EXCLUDED.last_practiced`,
		uuid.New(), userID, questionID, status, time.Now().UTC())
	return err
}

func (r *Repository) CountAll(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM questions`).Scan(&n)
	return n, err
}

func scanQuestions(rows pgx.Rows) ([]Question, error) {
	var out []Question
	for rows.Next() {
		var q Question
		var model *string
		if err := rows.Scan(&q.ID, &q.RoleID, &q.Topic, &q.Prompt, &model); err != nil {
			return nil, err
		}
		if model != nil {
			q.ModelAnswer = *model
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func scanQuestionsWithPracticed(rows pgx.Rows) ([]Question, error) {
	var out []Question
	for rows.Next() {
		var q Question
		var model *string
		if err := rows.Scan(&q.ID, &q.RoleID, &q.Topic, &q.Prompt, &model, &q.Practiced); err != nil {
			return nil, err
		}
		if model != nil {
			q.ModelAnswer = *model
		}
		out = append(out, q)
	}
	return out, rows.Err()
}
