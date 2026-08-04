// Package auth provides JWT authentication infrastructure.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config holds authentication configuration.
type Config struct {
	Secret     []byte
	Expiration time.Duration
}

// DefaultConfig returns default auth configuration.
func DefaultConfig() Config {
	return Config{
		Secret:     []byte("super-secret-ojs-monitor-key"),
		Expiration: 24 * time.Hour,
	}
}

// Claims represents JWT claims.
type Claims struct {
	AdminID   int    `json:"admin_id"`
	Username  string `json:"username"`
	jwt.RegisteredClaims
}

// Service provides authentication operations.
type Service struct {
	config Config
}

// New creates a new auth service.
func New(cfg Config) *Service {
	if len(cfg.Secret) == 0 {
		cfg.Secret = DefaultConfig().Secret
	}
	if cfg.Expiration == 0 {
		cfg.Expiration = DefaultConfig().Expiration
	}
	return &Service{config: cfg}
}

// GenerateToken creates a new JWT token for a given admin ID.
func (s *Service) GenerateToken(adminID int, username string) (string, error) {
	claims := &Claims{
		AdminID:  adminID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.config.Expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.config.Secret)
}

// ValidateToken validates a JWT token and returns claims.
func (s *Service) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.config.Secret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// ExtractAdminID extracts admin ID from a valid token.
func (s *Service) ExtractAdminID(tokenStr string) (int, string, error) {
	claims, err := s.ValidateToken(tokenStr)
	if err != nil {
		return 0, "", err
	}
	return claims.AdminID, claims.Username, nil
}

// ValidateTokenFunc returns a function compatible with middleware.RequireAuth.
func (s *Service) ValidateTokenFunc() func(token string) (int, string, error) {
	return func(token string) (int, string, error) {
		return s.ExtractAdminID(token)
	}
}

// GlobalService is the default auth service.
var GlobalService = New(DefaultConfig())

// GenerateToken is a convenience wrapper for GlobalService.
func GenerateToken(adminID int, username string) (string, error) {
	return GlobalService.GenerateToken(adminID, username)
}

// ValidateToken is a convenience wrapper for GlobalService.
func ValidateToken(tokenStr string) (*Claims, error) {
	return GlobalService.ValidateToken(tokenStr)
}

// ExtractAdminID is a convenience wrapper for GlobalService.
func ExtractAdminID(tokenStr string) (int, string, error) {
	return GlobalService.ExtractAdminID(tokenStr)
}
