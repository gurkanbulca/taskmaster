// internal/service/email_verification.go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/ent/generated/user"
	"github.com/gurkanbulca/taskmaster/pkg/email"
)

const (
	// EmailVerificationTokenLength is the length of email verification tokens
	EmailVerificationTokenLength = 32
	// EmailVerificationTokenDuration is how long verification tokens are valid
	EmailVerificationTokenDuration = 24 * time.Hour
	// MaxEmailVerificationAttempts is the maximum number of verification attempts
	MaxEmailVerificationAttempts = 5
)

// EmailVerificationService handles email verification logic
type EmailVerificationService struct {
	client         *ent.Client
	emailService   email.EmailService
	securityLogger *SecurityLogger
}

// NewEmailVerificationService creates a new email verification service
func NewEmailVerificationService(client *ent.Client, emailService email.EmailService, securityLogger *SecurityLogger) *EmailVerificationService {
	return &EmailVerificationService{
		client:         client,
		emailService:   emailService,
		securityLogger: securityLogger,
	}
}

// SendVerificationEmail sends a verification email to the user
func (s *EmailVerificationService) SendVerificationEmail(ctx context.Context, userID string) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Get user
	foundUser, err := s.client.User.Get(ctx, userUUID)
	if err != nil {
		if ent.IsNotFound(err) {
			return status.Error(codes.NotFound, "user not found")
		}
		return status.Error(codes.Internal, "failed to get user")
	}

	// Check if email is already verified
	if foundUser.EmailVerified {
		return status.Error(codes.FailedPrecondition, "email is already verified")
	}

	// Check verification attempts
	if foundUser.EmailVerificationAttempts >= MaxEmailVerificationAttempts {
		return status.Error(codes.ResourceExhausted, "maximum verification attempts exceeded")
	}

	// Generate verification token
	token, err := s.generateVerificationToken()
	if err != nil {
		return status.Error(codes.Internal, "failed to generate verification token")
	}

	// Update user with verification token
	expiresAt := time.Now().Add(EmailVerificationTokenDuration)
	updatedUser, err := foundUser.Update().
		SetEmailVerificationToken(token).
		SetEmailVerificationExpiresAt(expiresAt).
		AddEmailVerificationAttempts(1).
		Save(ctx)

	if err != nil {
		return status.Error(codes.Internal, "failed to update user")
	}

	// Send verification email
	if err := s.emailService.SendVerificationEmail(ctx, updatedUser, token); err != nil {
		// Log error but don't return it to avoid exposing email system details
		// In production, you'd want to log this properly
		return status.Error(codes.Internal, "failed to send verification email")
	}

	// Log security event
	if err := s.securityLogger.LogEmailVerificationSent(ctx, foundUser.ID); err != nil {
		// Log error but don't fail the operation
	}

	return nil
}

// VerifyEmail verifies an email using the provided token
func (s *EmailVerificationService) VerifyEmail(ctx context.Context, token string) error {
	if token == "" {
		return status.Error(codes.InvalidArgument, "verification token is required")
	}

	// Find user by verification token
	foundUser, err := s.client.User.Query().
		Where(
			user.And(
				user.EmailVerificationTokenEQ(token),
				user.EmailVerifiedEQ(false),
			),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return status.Error(codes.NotFound, "invalid or expired verification token")
		}
		return status.Error(codes.Internal, "failed to find user")
	}

	// Check if token is expired
	if foundUser.EmailVerificationExpiresAt != nil && foundUser.EmailVerificationExpiresAt.Before(time.Now()) {
		return status.Error(codes.DeadlineExceeded, "verification token has expired")
	}

	// Mark email as verified and clear verification token
	_, err = foundUser.Update().
		SetEmailVerified(true).
		ClearEmailVerificationToken().
		ClearEmailVerificationExpiresAt().
		SetEmailVerificationAttempts(0). // Reset attempts on successful verification
		Save(ctx)

	if err != nil {
		return status.Error(codes.Internal, "failed to verify email")
	}

	// Send welcome email
	if err := s.emailService.SendWelcomeEmail(ctx, foundUser); err != nil {
		// Log error but don't fail the verification
		// The email is verified successfully even if welcome email fails
	}

	// Log security event
	if err := s.securityLogger.LogEmailVerificationCompleted(ctx, foundUser.ID); err != nil {
		// Log error but don't fail the verification
	}

	return nil
}

// ResendVerificationEmail resends the verification email
func (s *EmailVerificationService) ResendVerificationEmail(ctx context.Context, userID string) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Get user
	foundUser, err := s.client.User.Get(ctx, userUUID)
	if err != nil {
		if ent.IsNotFound(err) {
			return status.Error(codes.NotFound, "user not found")
		}
		return status.Error(codes.Internal, "failed to get user")
	}

	// Check if email is already verified
	if foundUser.EmailVerified {
		return status.Error(codes.FailedPrecondition, "email is already verified")
	}

	// Check rate limiting (can only resend once per hour)
	if foundUser.EmailVerificationExpiresAt != nil {
		timeUntilNextResend := foundUser.EmailVerificationExpiresAt.Add(-EmailVerificationTokenDuration).Add(1 * time.Hour)
		if time.Now().Before(timeUntilNextResend) {
			return status.Error(codes.ResourceExhausted, "please wait before requesting another verification email")
		}
	}

	// Check verification attempts
	if foundUser.EmailVerificationAttempts >= MaxEmailVerificationAttempts {
		return status.Error(codes.ResourceExhausted, "maximum verification attempts exceeded")
	}

	// Generate new verification token
	token, err := s.generateVerificationToken()
	if err != nil {
		return status.Error(codes.Internal, "failed to generate verification token")
	}

	// Update user with new verification token
	expiresAt := time.Now().Add(EmailVerificationTokenDuration)
	updatedUser, err := foundUser.Update().
		SetEmailVerificationToken(token).
		SetEmailVerificationExpiresAt(expiresAt).
		AddEmailVerificationAttempts(1).
		Save(ctx)

	if err != nil {
		return status.Error(codes.Internal, "failed to update user")
	}

	// Send verification email
	if err := s.emailService.SendVerificationEmail(ctx, updatedUser, token); err != nil {
		return status.Error(codes.Internal, "failed to send verification email")
	}

	// Log security event
	if err := s.securityLogger.LogEmailVerificationSent(ctx, foundUser.ID); err != nil {
		// Log error but don't fail the operation
	}

	return nil
}

// GetVerificationStatus returns the email verification status for a user
func (s *EmailVerificationService) GetVerificationStatus(ctx context.Context, userID string) (*EmailVerificationStatus, error) {
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

	verificationStatus := &EmailVerificationStatus{
		EmailVerified: foundUser.EmailVerified,
		Attempts:      foundUser.EmailVerificationAttempts,
		MaxAttempts:   MaxEmailVerificationAttempts,
	}

	if foundUser.EmailVerificationExpiresAt != nil {
		verificationStatus.ExpiresAt = foundUser.EmailVerificationExpiresAt
		verificationStatus.IsExpired = foundUser.EmailVerificationExpiresAt.Before(time.Now())
	}

	verificationStatus.CanResend = !foundUser.EmailVerified &&
		foundUser.EmailVerificationAttempts < MaxEmailVerificationAttempts &&
		(foundUser.EmailVerificationExpiresAt == nil ||
			time.Now().After(foundUser.EmailVerificationExpiresAt.Add(-EmailVerificationTokenDuration).Add(1*time.Hour)))

	return verificationStatus, nil
}

// generateVerificationToken generates a cryptographically secure verification token
func (s *EmailVerificationService) generateVerificationToken() (string, error) {
	bytes := make([]byte, EmailVerificationTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// EmailVerificationStatus represents the email verification status
type EmailVerificationStatus struct {
	EmailVerified bool       `json:"email_verified"`
	Attempts      int        `json:"attempts"`
	MaxAttempts   int        `json:"max_attempts"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IsExpired     bool       `json:"is_expired"`
	CanResend     bool       `json:"can_resend"`
}

// CleanupExpiredTokens removes expired email verification tokens
// This should be run periodically as a background job
func (s *EmailVerificationService) CleanupExpiredTokens(ctx context.Context) error {
	_, err := s.client.User.Update().
		Where(
			user.And(
				user.EmailVerificationTokenNotNil(),
				user.EmailVerificationExpiresAtLT(time.Now()),
			),
		).
		ClearEmailVerificationToken().
		ClearEmailVerificationExpiresAt().
		Save(ctx)

	return err
}
