// Package role manages target roles (the job a user is preparing for) including
// their required-skill lists and a sample job description. Roles are seeded from
// a JSON file on startup.
package role

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
)

// Role is a target job a user prepares for.
type Role struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	RequiredSkills []string  `json:"required_skills"`
	SampleJD       string    `json:"sample_jd"`
}

var ErrNotFound = errors.New("role not found")

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Upsert inserts or updates a role by name (used by the seeder).
func (r *Repository) Upsert(ctx context.Context, role Role) error {
	skills, _ := json.Marshal(role.RequiredSkills)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO roles (id, name, required_skills_json, sample_jd)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (name) DO UPDATE SET required_skills_json = EXCLUDED.required_skills_json,
		   sample_jd = EXCLUDED.sample_jd`,
		role.ID, role.Name, skills, role.SampleJD)
	return err
}

func (r *Repository) List(ctx context.Context) ([]Role, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, required_skills_json, sample_jd FROM roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, role)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Role, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, required_skills_json, sample_jd FROM roles WHERE id = $1`, id)
	role, err := scanRole(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Role{}, ErrNotFound
	}
	return role, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRole(s scanner) (Role, error) {
	var role Role
	var skills []byte
	if err := s.Scan(&role.ID, &role.Name, &skills, &role.SampleJD); err != nil {
		return Role{}, err
	}
	_ = json.Unmarshal(skills, &role.RequiredSkills)
	return role, nil
}

// Seed loads roles from a JSON file and upserts them.
func Seed(ctx context.Context, repo *Repository, path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var roles []Role
	if err := json.Unmarshal(raw, &roles); err != nil {
		return 0, err
	}
	for i := range roles {
		if roles[i].ID == uuid.Nil {
			roles[i].ID = uuid.New()
		}
		if err := repo.Upsert(ctx, roles[i]); err != nil {
			return 0, err
		}
	}
	return len(roles), nil
}

// Handler exposes read-only role endpoints.
type Handler struct{ repo *Repository }

func NewHandler(repo *Repository) *Handler { return &Handler{repo: repo} }

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	roles, err := h.repo.List(r.Context())
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"roles": roles})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid role id"))
		return
	}
	role, err := h.repo.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, role)
}
