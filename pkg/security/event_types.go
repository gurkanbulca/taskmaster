// pkg/security/event_types.go
package security

import (
	"fmt"

	"github.com/gurkanbulca/taskmaster/ent/generated/securityevent"
)

// EventType constants for string-based event type handling
const (
	EventTypeLoginSuccess               = "login_success"
	EventTypeLoginFailed                = "login_failed"
	EventTypePasswordChanged            = "password_changed"
	EventTypePasswordResetRequested     = "password_reset_requested"
	EventTypePasswordResetCompleted     = "password_reset_completed"
	EventTypeEmailVerificationSent      = "email_verification_sent"
	EventTypeEmailVerificationCompleted = "email_verification_completed"
	EventTypeAccountLocked              = "account_locked"
	EventTypeAccountUnlocked            = "account_unlocked"
	EventTypeSecurityAlert              = "security_alert"
	EventTypeSuspiciousActivity         = "suspicious_activity"
)

// Severity constants for string-based severity handling
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// ParseEventType converts string event type to Ent enum
func ParseEventType(eventType string) (securityevent.EventType, error) {
	switch eventType {
	case EventTypeLoginSuccess:
		return securityevent.EventTypeLoginSuccess, nil
	case EventTypeLoginFailed:
		return securityevent.EventTypeLoginFailed, nil
	case EventTypePasswordChanged:
		return securityevent.EventTypePasswordChanged, nil
	case EventTypePasswordResetRequested:
		return securityevent.EventTypePasswordResetRequested, nil
	case EventTypePasswordResetCompleted:
		return securityevent.EventTypePasswordResetCompleted, nil
	case EventTypeEmailVerificationSent:
		return securityevent.EventTypeEmailVerificationSent, nil
	case EventTypeEmailVerificationCompleted:
		return securityevent.EventTypeEmailVerificationCompleted, nil
	case EventTypeAccountLocked:
		return securityevent.EventTypeAccountLocked, nil
	case EventTypeAccountUnlocked:
		return securityevent.EventTypeAccountUnlocked, nil
	case EventTypeSecurityAlert:
		return securityevent.EventTypeSecurityAlert, nil
	case EventTypeSuspiciousActivity:
		return securityevent.EventTypeSuspiciousActivity, nil
	default:
		return "", fmt.Errorf("unknown event type: %s", eventType)
	}
}

// ParseSeverity converts string severity to Ent enum
func ParseSeverity(severity string) (securityevent.Severity, error) {
	switch severity {
	case SeverityLow:
		return securityevent.SeverityLow, nil
	case SeverityMedium:
		return securityevent.SeverityMedium, nil
	case SeverityHigh:
		return securityevent.SeverityHigh, nil
	case SeverityCritical:
		return securityevent.SeverityCritical, nil
	default:
		return "", fmt.Errorf("unknown severity: %s", severity)
	}
}

// EventTypeToString converts Ent enum to string
func EventTypeToString(eventType securityevent.EventType) string {
	switch eventType {
	case securityevent.EventTypeLoginSuccess:
		return EventTypeLoginSuccess
	case securityevent.EventTypeLoginFailed:
		return EventTypeLoginFailed
	case securityevent.EventTypePasswordChanged:
		return EventTypePasswordChanged
	case securityevent.EventTypePasswordResetRequested:
		return EventTypePasswordResetRequested
	case securityevent.EventTypePasswordResetCompleted:
		return EventTypePasswordResetCompleted
	case securityevent.EventTypeEmailVerificationSent:
		return EventTypeEmailVerificationSent
	case securityevent.EventTypeEmailVerificationCompleted:
		return EventTypeEmailVerificationCompleted
	case securityevent.EventTypeAccountLocked:
		return EventTypeAccountLocked
	case securityevent.EventTypeAccountUnlocked:
		return EventTypeAccountUnlocked
	case securityevent.EventTypeSecurityAlert:
		return EventTypeSecurityAlert
	case securityevent.EventTypeSuspiciousActivity:
		return EventTypeSuspiciousActivity
	default:
		return "unknown"
	}
}

// SeverityToString converts Ent enum to string
func SeverityToString(severity securityevent.Severity) string {
	switch severity {
	case securityevent.SeverityLow:
		return SeverityLow
	case securityevent.SeverityMedium:
		return SeverityMedium
	case securityevent.SeverityHigh:
		return SeverityHigh
	case securityevent.SeverityCritical:
		return SeverityCritical
	default:
		return "unknown"
	}
}

// ValidEventTypes returns all valid event type strings
func ValidEventTypes() []string {
	return []string{
		EventTypeLoginSuccess,
		EventTypeLoginFailed,
		EventTypePasswordChanged,
		EventTypePasswordResetRequested,
		EventTypePasswordResetCompleted,
		EventTypeEmailVerificationSent,
		EventTypeEmailVerificationCompleted,
		EventTypeAccountLocked,
		EventTypeAccountUnlocked,
		EventTypeSecurityAlert,
		EventTypeSuspiciousActivity,
	}
}

// ValidSeverities returns all valid severity strings
func ValidSeverities() []string {
	return []string{
		SeverityLow,
		SeverityMedium,
		SeverityHigh,
		SeverityCritical,
	}
}

// IsValidEventType checks if the event type string is valid
func IsValidEventType(eventType string) bool {
	_, err := ParseEventType(eventType)
	return err == nil
}

// IsValidSeverity checks if the severity string is valid
func IsValidSeverity(severity string) bool {
	_, err := ParseSeverity(severity)
	return err == nil
}
