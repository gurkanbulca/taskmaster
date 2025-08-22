// internal/service/test_helpers.go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gurkanbulca/taskmaster/ent/generated/securityevent"
	"github.com/gurkanbulca/taskmaster/pkg/security"
	"github.com/stretchr/testify/require"

	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/ent/generated/user"
	"github.com/gurkanbulca/taskmaster/pkg/auth"
)

// TestHelpers provides common test utilities
type TestHelpers struct {
	t               *testing.T
	client          *ent.Client
	passwordManager *auth.PasswordManager
}

// NewTestHelpers creates a new test helper instance
func NewTestHelpers(t *testing.T, client *ent.Client) *TestHelpers {
	return &TestHelpers{
		t:               t,
		client:          client,
		passwordManager: auth.NewPasswordManager(),
	}
}

// CreateTestUser creates a standard test user
func (h *TestHelpers) CreateTestUser(email, username, password string) *ent.User {
	hashedPassword, err := h.passwordManager.HashPassword(password)
	require.NoError(h.t, err)

	user, err := h.client.User.Create().
		SetEmail(email).
		SetUsername(username).
		SetPasswordHash(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetRole(user.RoleUser).
		SetIsActive(true).
		SetEmailVerified(false).
		Save(context.Background())
	require.NoError(h.t, err)

	return user
}

// CreateAdminUser creates an admin test user
func (h *TestHelpers) CreateAdminUser(email, username, password string) *ent.User {
	hashedPassword, err := h.passwordManager.HashPassword(password)
	require.NoError(h.t, err)

	user, err := h.client.User.Create().
		SetEmail(email).
		SetUsername(username).
		SetPasswordHash(hashedPassword).
		SetFirstName("Admin").
		SetLastName("User").
		SetRole(user.RoleAdmin).
		SetIsActive(true).
		SetEmailVerified(true).
		Save(context.Background())
	require.NoError(h.t, err)

	return user
}

// CreateManagerUser creates a manager test user
func (h *TestHelpers) CreateManagerUser(email, username, password string) *ent.User {
	hashedPassword, err := h.passwordManager.HashPassword(password)
	require.NoError(h.t, err)

	user, err := h.client.User.Create().
		SetEmail(email).
		SetUsername(username).
		SetPasswordHash(hashedPassword).
		SetFirstName("Manager").
		SetLastName("User").
		SetRole(user.RoleManager).
		SetIsActive(true).
		SetEmailVerified(true).
		Save(context.Background())
	require.NoError(h.t, err)

	return user
}

// CreateInactiveUser creates an inactive test user
func (h *TestHelpers) CreateInactiveUser(email, username, password string) *ent.User {
	hashedPassword, err := h.passwordManager.HashPassword(password)
	require.NoError(h.t, err)

	user, err := h.client.User.Create().
		SetEmail(email).
		SetUsername(username).
		SetPasswordHash(hashedPassword).
		SetFirstName("Inactive").
		SetLastName("User").
		SetRole(user.RoleUser).
		SetIsActive(false).
		SetEmailVerified(false).
		Save(context.Background())
	require.NoError(h.t, err)

	return user
}

// CreateVerifiedUser creates a user with verified email
func (h *TestHelpers) CreateVerifiedUser(email, username, password string) *ent.User {
	hashedPassword, err := h.passwordManager.HashPassword(password)
	require.NoError(h.t, err)

	user, err := h.client.User.Create().
		SetEmail(email).
		SetUsername(username).
		SetPasswordHash(hashedPassword).
		SetFirstName("Verified").
		SetLastName("User").
		SetRole(user.RoleUser).
		SetIsActive(true).
		SetEmailVerified(true).
		Save(context.Background())
	require.NoError(h.t, err)

	return user
}

// CreateSecurityEvent creates a test security event
func (h *TestHelpers) CreateSecurityEvent(userID string, eventType, severity string) *ent.SecurityEvent {
	userUUID, err := uuid.Parse(userID)
	require.NoError(h.t, err)

	parsedEventType, err := security.ParseEventType(eventType)
	require.NoError(h.t, err, "Failed to parse event type")

	// Convert string severity to Ent enum
	parsedSeverity, err := security.ParseSeverity(severity)
	require.NoError(h.t, err, "Failed to parse severity")

	event, err := h.client.SecurityEvent.Create().
		SetUserID(userUUID).
		SetEventType(parsedEventType).
		SetSeverity(parsedSeverity).
		SetDescription("Test event").
		SetIPAddress("127.0.0.1").
		SetUserAgent("test-agent").
		Save(context.Background())
	require.NoError(h.t, err)

	return event
}

// CleanupUser removes a test user and all related data
func (h *TestHelpers) CleanupUser(userID uuid.UUID) {
	// Delete security events
	h.client.SecurityEvent.Delete().
		Where(securityevent.UserIDEQ(userID)).
		Exec(context.Background())

	// Delete user
	h.client.User.DeleteOneID(userID).
		Exec(context.Background())
}

// AssertUserPasswordChanged verifies that a user's password was changed
func (h *TestHelpers) AssertUserPasswordChanged(userID uuid.UUID, newPassword string) {
	user, err := h.client.User.Get(context.Background(), userID)
	require.NoError(h.t, err)

	err = h.passwordManager.ComparePassword(user.PasswordHash, newPassword)
	require.NoError(h.t, err, "New password should work")
}

// AssertUserLocked verifies that a user account is locked
func (h *TestHelpers) AssertUserLocked(userID uuid.UUID) {
	user, err := h.client.User.Get(context.Background(), userID)
	require.NoError(h.t, err)

	require.NotNil(h.t, user.AccountLockedUntil, "Account should be locked")
	require.True(h.t, user.AccountLockedUntil.After(time.Now()), "Lock should be in the future")
}

// AssertUserUnlocked verifies that a user account is not locked
func (h *TestHelpers) AssertUserUnlocked(userID uuid.UUID) {
	user, err := h.client.User.Get(context.Background(), userID)
	require.NoError(h.t, err)

	require.Nil(h.t, user.AccountLockedUntil, "Account should not be locked")
	require.Equal(h.t, 0, user.FailedLoginAttempts, "Failed attempts should be reset")
}
