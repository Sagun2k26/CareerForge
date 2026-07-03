package user

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/sagun-patwari/ai-career-platform/internal/auth"
	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
)

// Service holds user business logic: registration, login and token issuance.
type Service struct {
	repo *Repository
	jwt  *auth.Manager
}

func NewService(repo *Repository, jwt *auth.Manager) *Service {
	return &Service{repo: repo, jwt: jwt}
}

// AuthResult is returned on successful register/login.
type AuthResult struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

func (s *Service) Register(ctx context.Context, email, password string) (AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return AuthResult{}, httpx.NewError(http.StatusBadRequest, "invalid_email", "a valid email is required")
	}
	if len(password) < 8 {
		return AuthResult{}, httpx.NewError(http.StatusBadRequest, "weak_password", "password must be at least 8 characters")
	}
	if _, err := s.repo.GetByEmail(ctx, email); err == nil {
		return AuthResult{}, httpx.NewError(http.StatusConflict, "email_taken", "an account with this email already exists")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}
	u, err := s.repo.Create(ctx, email, string(hash))
	if err != nil {
		return AuthResult{}, err
	}
	return s.issue(u)
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return AuthResult{}, httpx.NewError(http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return AuthResult{}, httpx.NewError(http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	}
	return s.issue(u)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) issue(u User) (AuthResult, error) {
	token, err := s.jwt.Issue(u.ID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{Token: token, User: u}, nil
}
