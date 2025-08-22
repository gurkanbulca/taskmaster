// pkg/auth/password.go
package auth

import (
	"errors"
	"fmt"
	"regexp"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrWeakPassword = errors.New("password does not meet requirements")
)

// PasswordManager handles password hashing and validation
type PasswordManager struct {
	minLength      int
	requireUpper   bool
	requireLower   bool
	requireNumber  bool
	requireSpecial bool
}

// NewPasswordManager creates a new password manager with default settings
func NewPasswordManager() *PasswordManager {
	return &PasswordManager{
		minLength:      8,
		requireUpper:   true,
		requireLower:   true,
		requireNumber:  true,
		requireSpecial: false,
	}
}

// HashPassword hashes a password using bcrypt
func (pm *PasswordManager) HashPassword(password string) (string, error) {
	// Validate password strength
	if err := pm.ValidatePassword(password); err != nil {
		return "", err
	}

	// Generate hash with cost of 12
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hashedBytes), nil
}

// ComparePassword compares a password with a hash
func (pm *PasswordManager) ComparePassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// ValidatePassword checks if a password meets the requirements
func (pm *PasswordManager) ValidatePassword(password string) error {
	if len(password) < pm.minLength {
		return fmt.Errorf("%w: minimum length is %d characters", ErrWeakPassword, pm.minLength)
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if pm.requireUpper && !hasUpper {
		return fmt.Errorf("%w: must contain at least one uppercase letter", ErrWeakPassword)
	}
	if pm.requireLower && !hasLower {
		return fmt.Errorf("%w: must contain at least one lowercase letter", ErrWeakPassword)
	}
	if pm.requireNumber && !hasNumber {
		return fmt.Errorf("%w: must contain at least one number", ErrWeakPassword)
	}
	if pm.requireSpecial && !hasSpecial {
		return fmt.Errorf("%w: must contain at least one special character", ErrWeakPassword)
	}

	return nil
}

// ValidateEmail validates an email address format
func ValidateEmail(email string) error {
	// Simple email regex pattern
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	if !emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}

	if len(email) > 255 {
		return errors.New("email address too long")
	}

	return nil
}

// ValidateUsername validates a username
func ValidateUsername(username string) error {
	if len(username) < 3 {
		return errors.New("username must be at least 3 characters")
	}

	if len(username) > 50 {
		return errors.New("username must not exceed 50 characters")
	}

	// Username can contain letters, numbers, underscore, and hyphen
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
	if !usernameRegex.MatchString(username) {
		return errors.New("username can only contain letters, numbers, underscore, and hyphen")
	}

	return nil
}
