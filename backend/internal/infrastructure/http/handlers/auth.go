// Package handlers provides HTTP request handlers.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"ojs-monitor/backend/internal/application/usecase/auth"
	infraauth "ojs-monitor/backend/internal/infrastructure/auth"
	"ojs-monitor/backend/internal/domain/models"
)

// AuthHandler handles authentication HTTP requests.
type AuthHandler struct {
	authUC      *auth.UseCase
	authService *infraauth.Service
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(authUC *auth.UseCase, authService *infraauth.Service) *AuthHandler {
	return &AuthHandler{
		authUC:      authUC,
		authService: authService,
	}
}

// Login handles user authentication.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		Error(w, http.StatusBadRequest, "Username and password required")
		return
	}

	// Authenticate
	admin, err := h.authUC.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		if err == auth.ErrInvalidCredentials || err == auth.ErrUserNotFound {
			Error(w, http.StatusUnauthorized, "Invalid username or password")
			return
		}
		Error(w, http.StatusInternalServerError, "Authentication failed")
		return
	}

	// Generate token
	token, err := h.authService.GenerateToken(admin.ID, admin.Username)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Log activity
	h.authUC.LogActivity(r.Context(), admin.ID, "login", "system")

	OK(w, LoginResponse{
		Token:    token,
		Username: admin.Username,
	})
}

// GetAuditLogs returns audit logs.
func (h *AuthHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs, err := h.authUC.GetAuditLogs(r.Context(), limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "Failed to get audit logs")
		return
	}

	OK(w, logs)
}

// LoginRequest represents a login request payload.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a successful login response.
type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

// AdminResponse represents an admin user in responses.
type AdminResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

// ToAdminResponse converts Admin model to response.
func ToAdminResponse(a *models.Admin) AdminResponse {
	return AdminResponse{
		ID:       a.ID,
		Username: a.Username,
	}
}
