// Package httpx contains small helpers for writing consistent JSON responses
// and decoding/validating request bodies. Keeping this in one place means every
// handler returns errors in the same shape.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ErrorResponse is the canonical error envelope returned by the API.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// APIError is an error that carries an HTTP status code.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Message }

// NewError builds an APIError.
func NewError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

// JSON writes v as a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// Error writes an error response, unwrapping APIError when possible.
func Error(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		JSON(w, apiErr.Status, ErrorResponse{Error: apiErr.Code, Message: apiErr.Message})
		return
	}
	JSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal_error", Message: err.Error()})
}

// Decode reads a JSON request body into dst, returning a 400 APIError on failure.
func Decode(r *http.Request, dst any) error {
	if r.Body == nil {
		return NewError(http.StatusBadRequest, "invalid_body", "request body is required")
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return NewError(http.StatusBadRequest, "invalid_body", err.Error())
	}
	return nil
}
