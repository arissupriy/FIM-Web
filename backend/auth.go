package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// jwtSecret is loaded from JWT_SECRET env variable.
// IMPORTANT: Must be set in production! Default is insecure.
var jwtSecret = []byte(getEnv("JWT_SECRET", "INSECURE-DEV-ONLY-CHANGE-ME"))

// GenerateToken creates a new JWT token for a given admin ID.
func GenerateToken(adminID int, username string) (string, error) {
	claims := jwt.MapClaims{
		"admin_id": adminID,
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// AuthMiddleware protects routes requiring authentication.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			respondError(w, http.StatusUnauthorized, "Missing or invalid token")
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			respondError(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			respondError(w, http.StatusUnauthorized, "Invalid token claims")
			return
		}

		// Safe type assertion for admin_id (prevents panic on malformed token)
		adminIDFloat, ok := claims["admin_id"].(float64)
		if !ok {
			respondError(w, http.StatusUnauthorized, "Invalid token claims: admin_id missing or wrong type")
			return
		}
		adminID := int(adminIDFloat)
		ctx := context.WithValue(r.Context(), "admin_id", adminID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
