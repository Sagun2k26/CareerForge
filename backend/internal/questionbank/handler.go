package questionbank

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
	r.Get("/", h.list)
	r.Get("/search", h.search)
	r.Get("/{id}/model-answer", h.modelAnswer)
	r.Post("/{id}/practiced", h.practiced)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	roleID, err := uuid.Parse(r.URL.Query().Get("role_id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "role_id is required"))
		return
	}
	topic := r.URL.Query().Get("topic")
	items, err := h.svc.ListByRole(r.Context(), userID, roleID, topic)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"questions": items})
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "missing_query", "q is required"))
		return
	}
	var roleID *uuid.UUID
	if rid := r.URL.Query().Get("role_id"); rid != "" {
		if id, err := uuid.Parse(rid); err == nil {
			roleID = &id
		}
	}
	items, err := h.svc.Search(r.Context(), q, roleID, 5)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"questions": items})
}

func (h *Handler) modelAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid question id"))
		return
	}
	roleName := r.URL.Query().Get("role_name")
	res, err := h.svc.ModelAnswer(r.Context(), id, roleName)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) practiced(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid question id"))
		return
	}
	var body struct {
		Practiced bool `json:"practiced"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	if err := h.svc.MarkPracticed(r.Context(), userID, id, body.Practiced); err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
