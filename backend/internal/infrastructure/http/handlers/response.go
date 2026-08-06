// Package handlers provides HTTP request handlers.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// HTTP response helpers - shared across all handlers

// OK sends HTTP 200 JSON response.
func OK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(v)
}

// Error sends HTTP error response.
func Error(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Respond sends JSON response with custom status.
func Respond(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}

// parseID extracts int ID from URL parameter.
func parseID(r *http.Request, param string) int {
	idStr := chi.URLParam(r, param)
	if len(idStr) == 0 {
		return 0
	}
	// Simple atoi
	result := 0
	for _, c := range idStr {
		if c < '0' || c > '9' {
			return 0
		}
		if result > 214748364 { // prevent overflow
			return 0
		}
		result = result*10 + int(c-'0')
	}
	return result
}

// SuccessResponse represents a standard success response.
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse represents a standard error response.
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// NewSuccessResponse creates a success response.
func NewSuccessResponse(data interface{}) SuccessResponse {
	return SuccessResponse{Success: true, Data: data}
}

// NewErrorResponse creates an error response.
func NewErrorResponse(err string) ErrorResponse {
	return ErrorResponse{Success: false, Error: err}
}

// RespondWithSuccess sends a success response.
func RespondWithSuccess(w http.ResponseWriter, status int, data interface{}) {
	Respond(w, status, NewSuccessResponse(data))
}

// RespondWithError sends an error response.
func RespondWithError(w http.ResponseWriter, status int, err string) {
	Respond(w, status, NewErrorResponse(err))
}

// BadRequest sends HTTP 400 response.
func BadRequest(w http.ResponseWriter, msg string) {
	Error(w, http.StatusBadRequest, msg)
}

// Created sends HTTP 201 response.
func Created(w http.ResponseWriter, v interface{}) {
	Respond(w, http.StatusCreated, v)
}

// NoContent sends HTTP 204 response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// NotFound sends HTTP 404 response.
func NotFound(w http.ResponseWriter, msg string) {
	Error(w, http.StatusNotFound, msg)
}

// InternalError sends HTTP 500 response.
func InternalError(w http.ResponseWriter, msg string) {
	Error(w, http.StatusInternalServerError, msg)
}
