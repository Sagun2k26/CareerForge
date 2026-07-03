package resume

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sagun-patwari/ai-career-platform/internal/auth"
	"github.com/sagun-patwari/ai-career-platform/internal/httpx"
)

const maxUpload = 10 << 20 // 10 MiB

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Routes returns the protected resume routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.upload)
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Post("/{id}/tailor", h.tailor)
	return r
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_upload", "could not parse upload"))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "missing_file", "expected a 'file' field"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxUpload))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	res, err := h.svc.Upload(r.Context(), userID, header.Filename, data)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, res)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	items, err := h.svc.List(r.Context(), userID)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"resumes": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid resume id"))
		return
	}
	res, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) tailor(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "invalid_id", "invalid resume id"))
		return
	}
	var body struct {
		JobDescription string `json:"job_description"`
	}
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, err)
		return
	}
	t, err := h.svc.Tailor(r.Context(), userID, id, body.JobDescription)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}
