// internal/service/password_reset.go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/ent/generated/user"
	"github.com/gurkanbulca/taskmaster/pkg/auth"
	"github.com/gurkanbulca/taskmaster/pkg/email"
	"github.com/gurkanbulca/taskmaster/pkg/security"
)

const (
	// PasswordResetTokenLength is the length of password reset tokens
	PasswordResetTokenLength = 32
	// PasswordResetTokenDuration is how long reset tokens are valid
	PasswordResetTokenDuration = 1 * time.Hour
	// MaxPasswordResetAttempts is the maximum number of reset attempts per day
	MaxPasswordResetAttempts = 5
	// PasswordResetRateLimit is the minimum time between reset requests
	PasswordResetRateLimit = 15 * time.Minute
)

// PasswordResetService handles password reset logic
type PasswordResetService struct {
	client          *ent.Client
	emailService    email.EmailService
	passwordManager *auth.PasswordManager
	securityLogger  *SecurityLogger
}

// NewPasswordResetService creates a new password reset service
func NewPasswordResetService(client *ent.Client, emailService email.EmailService, passwordManager *auth.PasswordManager, securityLogger *SecurityLogger) *PasswordResetService {
	return &PasswordResetService{
		client:          client,
		emailService:    emailService,
		passwordManager: passwordManager,
		securityLogger:  securityLogger,
	}
}

// RequestPasswordReset initiates a password reset process
func (s *PasswordResetService) RequestPasswordReset(ctx context.Context, email string) error {
	if email == "" {
		return status.Error(codes.InvalidArgument, "email is required")
	}

	// Normalize email
	email = strings.ToLower(strings.TrimSpace(email))

	// Find user by email
	foundUser, err := s.client.User.Query().
		Where(
			user.And(
				user.EmailEQ(email),
				user.IsActiveEQ(true),
			),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			// Don't reveal whether user exists - return success for security
			// Log the attempt for monitoring
			if err := s.logSecurityEvent(ctx, uuid.Nil, "password_reset_attempted_invalid_email",
				fmt.Sprintf("Password reset attempted for non-existent email: %s", email),
				"medium", ipAddress, userAgent); err != nil {
				// Log error but continue
			}
			return nil
		}
		return status.Error(codes.Internal, "failed to find user")
	}

	// Check rate limiting - only allow one request per 15 minutes
	if foundUser.PasswordResetExpiresAt != nil {
		timeUntilNextRequest := foundUser.PasswordResetExpiresAt.Add(-PasswordResetTokenDuration).Add(PasswordResetRateLimit)
		if time.Now().Before(timeUntilNextRequest) {
			// Log the rate limit violation
			if err := s.securityService.LogUserSecurityEvent(ctx, foundUser.ID, security.EventTypeSuspiciousActivity,
				"Password reset request rate limited", security.SeverityMedium, ipAddress, userAgent); err != nil {
				// Log error but continue
			}
			return status.Error(codes.ResourceExhausted, "please wait before requesting another password reset")
		}
	}

	// Check daily attempts (reset attempts counter daily)
	if foundUser.PasswordResetAttempts >= MaxPasswordResetAttempts {
		// Check if it's been 24 hours since last attempt
		if foundUser.PasswordResetExpiresAt != nil && time.Since(*foundUser.PasswordResetExpiresAt) < 24*time.Hour {
			// Log the attempt limit violation
			if err := s.logUserSecurityEvent(ctx, foundUser.ID, "password_reset_attempts_exceeded",
				"Password reset attempts limit exceeded", "high", ipAddress, userAgent); err != nil {
				// Log error but continue
			}
			return status.Error(codes.ResourceExhausted, "maximum password reset attempts exceeded for today")
		}
		// Reset attempts if it's been 24 hours
		foundUser = foundUser.Update().SetPasswordResetAttempts(0).SaveX(ctx)
	}

	// Generate reset token
	token, err := s.generateResetToken()
	if err != nil {
		return status.Error(codes.Internal, "failed to generate reset token")
	}

	// Update user with reset token
	expiresAt := time.Now().Add(PasswordResetTokenDuration)
	updatedUser, err := foundUser.Update().
		SetPasswordResetToken(token).
		SetPasswordResetExpiresAt(expiresAt).
		AddPasswordResetAttempts(1).
		Save(ctx)

	if err != nil {
		return status.Error(codes.Internal, "failed to update user")
	}

	// Send password reset email
	if err := s.emailService.SendPasswordResetEmail(ctx, updatedUser, token); err != nil {
		// Log error but don't expose email system details
		if err := s.securityService.LogUserSecurityEvent(ctx, foundUser.ID, security.EventTypeSecurityAlert,
			"Failed to send password reset email", security.SeverityHigh, ipAddress, userAgent); err != nil {
			// Log error but continue
		}
		return status.Error(codes.Internal, "failed to send password reset email")
	}

	// Log successful password reset request
	if err := s.securityService.LogUserSecurityEvent(ctx, foundUser.ID, security.EventTypePasswordResetRequested,
		"Password reset email sent", security.SeverityLow, ipAddress, userAgent); err != nil {
		// Log error but don't fail the operation
	}

	return nil
}

// VerifyPasswordResetToken verifies if a password reset token is valid
func (s *PasswordResetService) VerifyPasswordResetToken(ctx context.Context, token string) (*PasswordResetTokenInfo, error) {
	if token == "" {
		return nil, status.Error(codes.InvalidArgument, "reset token is required")
	}

	// Find user by reset token
	foundUser, err := s.client.User.Query().
		Where(
			user.And(
				user.PasswordResetTokenEQ(token),
				user.IsActiveEQ(true),
			),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "invalid or expired reset token")
		}
		return nil, status.Error(codes.Internal, "failed to find user")
	}

	// Check if token is expired
	if foundUser.PasswordResetExpiresAt != nil && foundUser.PasswordResetExpiresAt.Before(time.Now()) {
		return nil, status.Error(codes.DeadlineExceeded, "reset token has expired")
	}

	// Return token info (with masked email for security)
	tokenInfo := &PasswordResetTokenInfo{
		IsValid:   true,
		Email:     maskEmail(foundUser.Email),
		ExpiresAt: foundUser.PasswordResetExpiresAt,
	}

	return tokenInfo, nil
}

// ResetPassword resets a user's password using a valid reset token
func (s *PasswordResetService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if token == "" {
		return status.Error(codes.InvalidArgument, "reset token is required")
	}
	if newPassword == "" {
		return status.Error(codes.InvalidArgument, "new password is required")
	}

	// Validate password strength
	if err := s.passwordManager.ValidatePassword(newPassword); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	// Find user by reset token
	foundUser, err := s.client.User.Query().
		Where(
			user.And(
				user.PasswordResetTokenEQ(token),
				user.IsActiveEQ(true),
			),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			// Log invalid token attempt
			if err := s.securityLogger.LogSystemFromContext(ctx, security.EventTypeSuspiciousActivity,
				"Invalid password reset token used", security.SeverityMedium); err != nil {
				// Log error but continue
			}
			return status.Error(codes.NotFound, "invalid or expired reset token")
		}
		return status.Error(codes.Internal, "failed to find user")
	}

	// Check if token is expired
	if foundUser.PasswordResetExpiresAt != nil && foundUser.PasswordResetExpiresAt.Before(time.Now()) {
		// Log expired token attempt
		if err := s.securityLogger.LogFromContext(ctx, foundUser.ID, security.EventTypeSuspiciousActivity,
			"Expired password reset token used", security.SeverityMedium); err != nil {
			// Log error but continue
		}
		return status.Error(codes.DeadlineExceeded, "reset token has expired")
	}

	// Hash new password
	hashedPassword, err := s.passwordManager.HashPassword(newPassword)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	// Update user with new password and clear reset token
	now := time.Now()
	_, err = foundUser.Update().
		SetPasswordHash(hashedPassword).
		SetPasswordChangedAt(now).
		SetPasswordResetAt(now).
		ClearPasswordResetToken().
		ClearPasswordResetExpiresAt().
		SetPasswordResetAttempts(0). // Reset attempts on successful reset
		ClearRefreshToken().         // Invalidate all existing sessions
		ClearRefreshTokenExpiresAt().
		SetFailedLoginAttempts(0). // Reset failed login attempts
		ClearAccountLockedUntil(). // Unlock account if it was locked
		Save(ctx)

	if err != nil {
		return status.Error(codes.Internal, "failed to reset password")
	}

	// Send password changed notification email
	if foundUser.SecurityNotificationsEnabled {
		if err := s.emailService.SendPasswordChangedNotification(ctx, foundUser); err != nil {
			// Log error but don't fail the operation
			if err := s.securityLogger.LogFromContext(ctx, foundUser.ID, security.EventTypeSecurityAlert,
				"Failed to send password changed notification", security.SeverityMedium); err != nil {
				// Log error but continue
			}
		}
	}

	// Log successful password reset
	if err := s.securityLogger.LogPasswordResetCompleted(ctx, foundUser.ID); err != nil {
		// Log error but don't fail the operation
	}

	return nil
}

// GetPasswordResetStatus returns the password reset status for a user
func (s *PasswordResetService) GetPasswordResetStatus(ctx context.Context, userID string) (*PasswordResetStatus, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Get user
	foundUser, err := s.client.User.Get(ctx, userUUID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	status := &PasswordResetStatus{
		Attempts:    foundUser.PasswordResetAttempts,
		MaxAttempts: MaxPasswordResetAttempts,
	}

	if foundUser.PasswordResetExpiresAt != nil {
		status.ExpiresAt = foundUser.PasswordResetExpiresAt
		status.IsExpired = foundUser.PasswordResetExpiresAt.Before(time.Now())
		status.HasActiveRequest = !status.IsExpired
	}

	// Check if user can request another reset
	if foundUser.PasswordResetExpiresAt != nil {
		timeUntilNextRequest := foundUser.PasswordResetExpiresAt.Add(-PasswordResetTokenDuration).Add(PasswordResetRateLimit)
		status.CanRequest = time.Now().After(timeUntilNextRequest) && foundUser.PasswordResetAttempts < MaxPasswordResetAttempts
	} else {
		status.CanRequest = foundUser.PasswordResetAttempts < MaxPasswordResetAttempts
	}

	if foundUser.PasswordResetAt != nil {
		status.LastResetAt = foundUser.PasswordResetAt
	}

	return status, nil
}

// generateResetToken generates a cryptographically secure reset token
func (s *PasswordResetService) generateResetToken() (string, error) {
	bytes := make([]byte, PasswordResetTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// maskEmail masks an email address for security display
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}

	username := parts[0]
	domain := parts[1]

	// Mask username but show first and last character if long enough
	if len(username) <= 2 {
		return strings.Repeat("*", len(username)) + "@" + domain
	}

	masked := string(username[0]) + strings.Repeat("*", len(username)-2) + string(username[len(username)-1])
	return masked + "@" + domain
}

// CleanupExpiredTokens removes expired password reset tokens
// This should be run periodically as a background job
func (s *PasswordResetService) CleanupExpiredTokens(ctx context.Context) error {
	_, err := s.client.User.Update().
		Where(
			user.And(
				user.PasswordResetTokenNotNil(),
				user.PasswordResetExpiresAtLT(time.Now()),
			),
		).
		ClearPasswordResetToken().
		ClearPasswordResetExpiresAt().
		Save(ctx)

	return err
}

// PasswordResetTokenInfo contains information about a password reset token
type PasswordResetTokenInfo struct {
	IsValid   bool       `json:"is_valid"`
	Email     string     `json:"email"` // Masked email
	ExpiresAt *time.Time `json:"expires_at"`
}

// PasswordResetStatus represents the password reset status for a user
type PasswordResetStatus struct {
	Attempts         int        `json:"attempts"`
	MaxAttempts      int        `json:"max_attempts"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	IsExpired        bool       `json:"is_expired"`
	HasActiveRequest bool       `json:"has_active_request"`
	CanRequest       bool       `json:"can_request"`
	LastResetAt      *time.Time `json:"last_reset_at,omitempty"`
}
