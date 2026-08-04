// Package middleware provides HTTP middleware.
package middleware

import (
	"context"
	"net/http"
	"strings"
)

// ContextKey is a custom type for context keys.
type ContextKey string

const (
	// AdminIDKey is the context key for admin ID.
	AdminIDKey ContextKey = "admin_id"
	// AdminUsernameKey is the context key for admin username.
	AdminUsernameKey ContextKey = "admin_username"
)

// AuthMiddleware validates JWT tokens and extracts admin info.
type AuthMiddleware struct {
	validateToken func(token string) (int, string, error)
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(validateToken func(token string) (int, string, error)) *AuthMiddleware {
	return &AuthMiddleware{
		validateToken: validateToken,
	}
}

// Handler returns the chi middleware handler.
func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, `{"error":"invalid authorization header format"}`, http.StatusUnauthorized)
			return
		}

		token := parts[1]
		adminID, username, err := m.validateToken(token)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		// Add admin info to context
		ctx := context.WithValue(r.Context(), AdminIDKey, adminID)
		ctx = context.WithValue(ctx, AdminUsernameKey, username)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAdminID extracts admin ID from context.
func GetAdminID(ctx context.Context) int {
	if id, ok := ctx.Value(AdminIDKey).(int); ok {
		return id
	}
	return 0
}

// GetAdminUsername extracts admin username from context.
func GetAdminUsername(ctx context.Context) string {
	if username, ok := ctx.Value(AdminUsernameKey).(string); ok {
		return username
	}
	return ""
}

// RequireAuth is a convenience wrapper for chi.Router.Use().
func RequireAuth(validateToken func(token string) (int, string, error)) func(http.Handler) http.Handler {
	m := NewAuthMiddleware(validateToken)
	return m.Handler
}
