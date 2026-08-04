// Package response provides standardized HTTP response helpers.
package response

import (
	"encoding/json"
	"net/http"
)

// Response represents a standard API response.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Success creates a success response.
func Success(data interface{}) Response {
	return Response{Success: true, Data: data}
}

// Error creates an error response.
func Error(err string) Response {
	return Response{Success: false, Error: err}
}

// JSON sends a JSON response.
func JSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// OK sends HTTP 200.
func OK(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, Success(data))
}

// Created sends HTTP 201.
func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, Success(data))
}

// BadRequest sends HTTP 400.
func BadRequest(w http.ResponseWriter, err string) {
	JSON(w, http.StatusBadRequest, Error(err))
}

// Unauthorized sends HTTP 401.
func Unauthorized(w http.ResponseWriter, err string) {
	JSON(w, http.StatusUnauthorized, Error(err))
}

// Forbidden sends HTTP 403.
func Forbidden(w http.ResponseWriter, err string) {
	JSON(w, http.StatusForbidden, Error(err))
}

// NotFound sends HTTP 404.
func NotFound(w http.ResponseWriter, err string) {
	JSON(w, http.StatusNotFound, Error(err))
}

// InternalError sends HTTP 500.
func InternalError(w http.ResponseWriter, err string) {
	JSON(w, http.StatusInternalServerError, Error(err))
}
