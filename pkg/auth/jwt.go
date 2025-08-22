// pkg/auth/jwt.go
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken      = errors.New("invalid token")
	ErrExpiredToken      = errors.New("token has expired")
	ErrInvalidClaims     = errors.New("invalid token claims")
	ErrInvalidSigningKey = errors.New("invalid signing key")
)

// TokenManager manages JWT tokens
type TokenManager struct {
	accessSecret    []byte
	refreshSecret   []byte
	accessDuration  time.Duration
	refreshDuration time.Duration
	issuer          string
}

// NewTokenManager creates a new token manager
func NewTokenManager(accessSecret, refreshSecret string, accessDuration, refreshDuration time.Duration) *TokenManager {
	return &TokenManager{
		accessSecret:    []byte(accessSecret),
		refreshSecret:   []byte(refreshSecret),
		accessDuration:  accessDuration,
		refreshDuration: refreshDuration,
		issuer:          "taskmaster",
	}
}

// CustomClaims represents the custom JWT claims
type CustomClaims struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Type     string `json:"type"` // "access" or "refresh"
	jwt.RegisteredClaims
}

// GenerateTokenPair generates both access and refresh tokens
func (tm *TokenManager) GenerateTokenPair(userID, email, username, role string) (accessToken, refreshToken string, expiresIn int64, err error) {
	// Generate access token
	accessToken, err = tm.generateToken(userID, email, username, role, "access", tm.accessSecret, tm.accessDuration)
	if err != nil {
		return "", "", 0, fmt.Errorf("generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err = tm.generateToken(userID, email, username, role, "refresh", tm.refreshSecret, tm.refreshDuration)
	if err != nil {
		return "", "", 0, fmt.Errorf("generate refresh token: %w", err)
	}

	expiresIn = int64(tm.accessDuration.Seconds())
	return accessToken, refreshToken, expiresIn, nil
}

// generateToken creates a JWT token with custom claims
func (tm *TokenManager) generateToken(userID, email, username, role, tokenType string, secret []byte, duration time.Duration) (string, error) {
	now := time.Now()

	claims := CustomClaims{
		UserID:   userID,
		Email:    email,
		Username: username,
		Role:     role,
		Type:     tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			Issuer:    tm.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateAccessToken validates an access token and returns the claims
func (tm *TokenManager) ValidateAccessToken(tokenString string) (*CustomClaims, error) {
	return tm.validateToken(tokenString, "access", tm.accessSecret)
}

// ValidateRefreshToken validates a refresh token and returns the claims
func (tm *TokenManager) ValidateRefreshToken(tokenString string) (*CustomClaims, error) {
	return tm.validateToken(tokenString, "refresh", tm.refreshSecret)
}

// validateToken validates a token and returns the custom claims
func (tm *TokenManager) validateToken(tokenString, expectedType string, secret []byte) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// Verify token type
	if claims.Type != expectedType {
		return nil, fmt.Errorf("invalid token type: expected %s, got %s", expectedType, claims.Type)
	}

	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, ErrExpiredToken
	}

	return claims, nil
}

// RefreshAccessToken generates a new access token from a valid refresh token
func (tm *TokenManager) RefreshAccessToken(refreshToken string) (string, int64, error) {
	// Validate refresh token
	claims, err := tm.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", 0, fmt.Errorf("validate refresh token: %w", err)
	}

	// Generate new access token
	accessToken, err := tm.generateToken(
		claims.UserID,
		claims.Email,
		claims.Username,
		claims.Role,
		"access",
		tm.accessSecret,
		tm.accessDuration,
	)
	if err != nil {
		return "", 0, fmt.Errorf("generate access token: %w", err)
	}

	expiresIn := int64(tm.accessDuration.Seconds())
	return accessToken, expiresIn, nil
}

// ExtractTokenFromHeader extracts the token from the Authorization header
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return "", errors.New("invalid authorization header format")
	}
	return authHeader[7:], nil
}
