// Package httpx contains helpers for consistent JSON responses and decoding.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Message }

func NewError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func Error(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		JSON(w, apiErr.Status, ErrorResponse{Error: apiErr.Code, Message: apiErr.Message})
		return
	}
	slog.Error("internal request error", "error", err)
	JSON(w, http.StatusInternalServerError, ErrorResponse{
		Error:   "internal_error",
		Message: "Something went wrong. Please try again.",
	})
}

func Decode(r *http.Request, dst any) error {
	if r.Body == nil {
		return NewError(http.StatusBadRequest, "invalid_body", "request body is required")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return NewError(http.StatusBadRequest, "invalid_body", "invalid request")
	}
	return nil
}
