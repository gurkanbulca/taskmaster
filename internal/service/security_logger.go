// internal/service/security_logger.go
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/gurkanbulca/taskmaster/internal/middleware"
	"github.com/gurkanbulca/taskmaster/pkg/security"
)

// SecurityLogger provides convenience methods for logging security events
type SecurityLogger struct {
	securityService *SecurityService
}

// NewSecurityLogger creates a new security logger
func NewSecurityLogger(securityService *SecurityService) *SecurityLogger {
	return &SecurityLogger{
		securityService: securityService,
	}
}

// LogFromContext logs a security event using context information
func (sl *SecurityLogger) LogFromContext(ctx context.Context, userID uuid.UUID, eventType, description, severity string) error {
	clientInfo := middleware.GetClientInfoFromContext(ctx)

	return sl.securityService.LogUserSecurityEvent(
		ctx,
		userID,
		eventType,
		description,
		severity,
		clientInfo.IPAddress,
		clientInfo.UserAgent,
	)
}

// LogSystemFromContext logs a system security event using context information
func (sl *SecurityLogger) LogSystemFromContext(ctx context.Context, eventType, description, severity string) error {
	clientInfo := middleware.GetClientInfoFromContext(ctx)

	return sl.securityService.LogSystemSecurityEvent(
		ctx,
		eventType,
		description,
		severity,
		clientInfo.IPAddress,
		clientInfo.UserAgent,
	)
}

// LogCurrentUserFromContext logs a security event for the current authenticated user
func (sl *SecurityLogger) LogCurrentUserFromContext(ctx context.Context, eventType, description, severity string) error {
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		// If no user in context, log as system event
		return sl.LogSystemFromContext(ctx, eventType, description, severity)
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		// If invalid user ID, log as system event with error
		return sl.LogSystemFromContext(ctx, security.EventTypeSecurityAlert,
			"Invalid user ID in context during security logging", security.SeverityMedium)
	}

	return sl.LogFromContext(ctx, userUUID, eventType, description, severity)
}

// Convenience methods for common security events

func (sl *SecurityLogger) LogLoginSuccess(ctx context.Context, userID uuid.UUID) error {
	return sl.LogFromContext(ctx, userID, security.EventTypeLoginSuccess,
		"User successfully logged in", security.SeverityLow)
}

func (sl *SecurityLogger) LogLoginFailed(ctx context.Context, email, reason string) error {
	return sl.LogSystemFromContext(ctx, security.EventTypeLoginFailed,
		"Login failed for "+email+": "+reason, security.SeverityMedium)
}

func (sl *SecurityLogger) LogPasswordChanged(ctx context.Context, userID uuid.UUID) error {
	return sl.LogFromContext(ctx, userID, security.EventTypePasswordChanged,
		"User password changed", security.SeverityLow)
}

func (sl *SecurityLogger) LogPasswordResetRequested(ctx context.Context, userID uuid.UUID) error {
	return sl.LogFromContext(ctx, userID, security.EventTypePasswordResetRequested,
		"Password reset requested", security.SeverityLow)
}

func (sl *SecurityLogger) LogPasswordResetCompleted(ctx context.Context, userID uuid.UUID) error {
	return sl.LogFromContext(ctx, userID, security.EventTypePasswordResetCompleted,
		"Password reset completed", security.SeverityLow)
}

func (sl *SecurityLogger) LogEmailVerificationSent(ctx context.Context, userID uuid.UUID) error {
	return sl.LogFromContext(ctx, userID, security.EventTypeEmailVerificationSent,
		"Email verification sent", security.SeverityLow)
}

func (sl *SecurityLogger) LogEmailVerificationCompleted(ctx context.Context, userID uuid.UUID) error {
	return sl.LogFromContext(ctx, userID, security.EventTypeEmailVerificationCompleted,
		"Email verification completed", security.SeverityLow)
}

func (sl *SecurityLogger) LogAccountLocked(ctx context.Context, userID uuid.UUID, reason string) error {
	return sl.LogFromContext(ctx, userID, security.EventTypeAccountLocked,
		"Account locked: "+reason, security.SeverityHigh)
}

func (sl *SecurityLogger) LogSuspiciousActivity(ctx context.Context, userID uuid.UUID, description string) error {
	return sl.LogFromContext(ctx, userID, security.EventTypeSuspiciousActivity,
		description, security.SeverityMedium)
}

func (sl *SecurityLogger) LogSecurityAlert(ctx context.Context, userID uuid.UUID, description string) error {
	return sl.LogFromContext(ctx, userID, security.EventTypeSecurityAlert,
		description, security.SeverityHigh)
}
