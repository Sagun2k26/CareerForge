package analysisjob

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/auth"
	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	return r
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())

	var body struct {
		ResumeID string `json:"resume_id"`
		RoleID   string `json:"role_id"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}

	resumeID, err := uuid.Parse(body.ResumeID)
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid resume_id"))
		return
	}
	roleID, err := uuid.Parse(body.RoleID)
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid role_id"))
		return
	}

	job, err := h.svc.Enqueue(r.Context(), userID, resumeID, roleID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, job)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid analysis job id"))
		return
	}

	job, err := h.svc.Get(r.Context(), userID, id)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, job)
}
