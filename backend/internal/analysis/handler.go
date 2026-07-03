package analysis

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/auth"
	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Patch("/{id}/progress", h.progress)
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
	a, err := h.svc.Analyze(r.Context(), userID, resumeID, roleID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, a)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	items, err := h.svc.List(r.Context(), userID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"analyses": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid analysis id"))
		return
	}
	a, err := h.svc.Get(r.Context(), userID, id)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func (h *Handler) progress(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid analysis id"))
		return
	}
	var body struct {
		ItemOrder int    `json:"item_order"`
		Status    string `json:"status"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	if err := h.svc.UpdateProgress(r.Context(), userID, id, body.ItemOrder, body.Status); err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
