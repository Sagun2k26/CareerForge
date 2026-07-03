package interview

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
	r.Post("/", h.start)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Post("/{id}/answer", h.answer)
	return r
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	var body struct {
		RoleID string `json:"role_id"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	roleID, err := uuid.Parse(body.RoleID)
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid role_id"))
		return
	}
	res, err := h.svc.Start(r.Context(), userID, roleID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

func (h *Handler) answer(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid session id"))
		return
	}
	var body struct {
		Answer string `json:"answer"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	res, err := h.svc.Answer(r.Context(), userID, id, body.Answer)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid session id"))
		return
	}
	sess, err := h.svc.Get(r.Context(), userID, id)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sess)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	items, err := h.svc.List(r.Context(), userID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"sessions": items})
}
