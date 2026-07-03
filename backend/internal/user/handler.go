package user

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sagun-patwari/ai-career-platform/internal/auth"
	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
)

// Handler exposes user HTTP endpoints. Handlers stay thin: validate input, call
// the service, render the result.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// PublicRoutes returns routes that do not require authentication.
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	return r
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := httpx.Decode(r, &c); err != nil {
		httpx.Error(w, err)
		return
	}
	res, err := h.svc.Register(r.Context(), c.Email, c.Password)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if err := httpx.Decode(r, &c); err != nil {
		httpx.Error(w, err)
		return
	}
	res, err := h.svc.Login(r.Context(), c.Email, c.Password)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// Me returns the authenticated user; mounted under the protected router.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.UserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "unauthorized", "not authenticated"))
		return
	}
	u, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}
