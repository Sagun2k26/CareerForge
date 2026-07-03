package server

import (
	"context"
	"log/slog"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/questionbank"
	"github.com/sagun-patwari/ai-career-platform/internal/role"
)

// seed loads roles (from rolesPath) and questions (from questions.json beside
// it) into Postgres and the vector store. Both seeders are idempotent.
func seed(ctx context.Context, logger *slog.Logger, roleRepo *role.Repository, questionSvc *questionbank.Service, rolesPath string) error {
	n, err := role.Seed(ctx, roleRepo, rolesPath)
	if err != nil {
		return err
	}
	logger.Info("seeded roles", "count", n)

	roles, err := roleRepo.List(ctx)
	if err != nil {
		return err
	}
	byName := make(map[string]uuid.UUID, len(roles))
	for _, r := range roles {
		byName[r.Name] = r.ID
	}

	questionsPath := filepath.Join(filepath.Dir(rolesPath), "questions.json")
	qn, err := questionSvc.SeedFromFile(ctx, questionsPath, byName)
	if err != nil {
		return err
	}
	logger.Info("seeded questions", "count", qn)
	return nil
}
