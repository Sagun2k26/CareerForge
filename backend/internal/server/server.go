// Package server wires the modular monolith together: it builds the dependency
// graph (db, llm, rag, workers, repositories, services, handlers) and mounts a
// single chi router with a shared middleware chain.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sagun-patwari/ai-career-platform/internal/analysis"
	"github.com/sagun-patwari/ai-career-platform/internal/auth"
	"github.com/sagun-patwari/ai-career-platform/internal/config"
	"github.com/sagun-patwari/ai-career-platform/internal/database"
	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
	"github.com/sagun-patwari/ai-career-platform/internal/interview"
	"github.com/sagun-patwari/ai-career-platform/internal/llm"
	"github.com/sagun-patwari/ai-career-platform/internal/questionbank"
	"github.com/sagun-patwari/ai-career-platform/internal/rag"
	"github.com/sagun-patwari/ai-career-platform/internal/resume"
	"github.com/sagun-patwari/ai-career-platform/internal/role"
	"github.com/sagun-patwari/ai-career-platform/internal/user"
	"github.com/sagun-patwari/ai-career-platform/internal/worker"
)

// Server holds the assembled application and its lifecycle resources.
type Server struct {
	Handler http.Handler
	pool    *pgxpool.Pool
	workers *worker.Pool
	logger  *slog.Logger
	llm     *llm.Client
}

// New builds the entire application from configuration.
func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Server, error) {
	// Infrastructure ---------------------------------------------------------
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(ctx, pool, cfg.MigrationDir); err != nil {
		pool.Close()
		return nil, err
	}

	llmClient := llm.FromConfig(cfg.LLMProvider, cfg.LLMModel, cfg.LLMAPIKey, cfg.LLMTimeout)
	logger.Info("llm provider", "provider", llmClient.Provider())

	ragEngine, err := rag.New(cfg.VectorDir, llmClient)
	if err != nil {
		pool.Close()
		return nil, err
	}

	workers := worker.NewPool(ctx, 4, 256, logger)

	jwt := auth.NewManager(cfg.JWTSecret, cfg.JWTTTL)

	// Repositories -----------------------------------------------------------
	userRepo := user.NewRepository(pool)
	roleRepo := role.NewRepository(pool)
	resumeRepo := resume.NewRepository(pool)
	analysisRepo := analysis.NewRepository(pool)
	questionRepo := questionbank.NewRepository(pool)
	interviewRepo := interview.NewRepository(pool)

	// Services ---------------------------------------------------------------
	userSvc := user.NewService(userRepo, jwt)
	resumeSvc, err := resume.NewService(resumeRepo, llmClient, workers, cfg.StorageDir)
	if err != nil {
		pool.Close()
		return nil, err
	}
	analysisSvc := analysis.NewService(analysisRepo, resumeRepo, roleRepo, llmClient)
	questionSvc := questionbank.NewService(questionRepo, ragEngine, llmClient)
	interviewSvc := interview.NewService(interviewRepo, roleRepo, ragEngine, llmClient)

	// Seed roles + questions -------------------------------------------------
	if err := seed(ctx, logger, roleRepo, questionSvc, cfg.SeedFile); err != nil {
		logger.Error("seeding failed (continuing)", "error", err)
	}

	// Handlers ---------------------------------------------------------------
	userH := user.NewHandler(userSvc)
	roleH := role.NewHandler(roleRepo)
	resumeH := resume.NewHandler(resumeSvc)
	analysisH := analysis.NewHandler(analysisSvc)
	questionH := questionbank.NewHandler(questionSvc)
	interviewH := interview.NewHandler(interviewSvc)

	// Router -----------------------------------------------------------------
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(90 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(newRateLimiter(300, time.Minute).middleware)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]any{"status": "ok", "llm": llmClient.Provider()})
	})

	r.Route("/api", func(api chi.Router) {
		// Public.
		api.Mount("/auth", userH.PublicRoutes())
		api.Mount("/roles", roleH.Routes())

		// Protected.
		api.Group(func(p chi.Router) {
			p.Use(jwt.Middleware)
			p.Get("/me", userH.Me)
			p.Mount("/resumes", resumeH.Routes())
			p.Mount("/analyses", analysisH.Routes())
			p.Mount("/questions", questionH.Routes())
			p.Mount("/interviews", interviewH.Routes())
		})
	})

	return &Server{Handler: r, pool: pool, workers: workers, logger: logger, llm: llmClient}, nil
}

// Close releases resources gracefully.
func (s *Server) Close() {
	s.workers.Shutdown()
	s.pool.Close()
}
