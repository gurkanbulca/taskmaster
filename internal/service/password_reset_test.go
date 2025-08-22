// internal/service/password_reset_test.go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/gurkanbulca/taskmaster/ent/generated/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gurkanbulca/taskmaster/ent/generated/enttest"
	"github.com/gurkanbulca/taskmaster/internal/middleware"
	"github.com/gurkanbulca/taskmaster/pkg/auth"
	"github.com/gurkanbulca/taskmaster/pkg/email"

	_ "github.com/mattn/go-sqlite3"
)

func TestPasswordResetService_RequestPasswordReset(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	passwordManager := auth.NewPasswordManager()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewPasswordResetService(client, mockEmailService, passwordManager, securityLogger)

	// Create test user
	testUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetUsername("testuser").
		SetPasswordHash("hash").
		SetIsActive(true).
		SetPasswordResetAttempts(0).
		Save(context.Background())
	require.NoError(t, err)

	// Add context with client info
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.ContextKeyIPAddress, "127.0.0.1")
	ctx = context.WithValue(ctx, middleware.ContextKeyUserAgent, "test-agent")

	tests := []struct {
		name         string
		email        string
		setupFunc    func()
		wantErr      bool
		expectedCode codes.Code
		checkEmail   bool
	}{
		{
			name:       "successful password reset request",
			email:      "test@example.com",
			wantErr:    false,
			checkEmail: true,
		},
		{
			name:         "empty email",
			email:        "",
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:    "non-existent email (should succeed for security)",
			email:   "nonexistent@example.com",
			wantErr: false, // Returns success to not reveal if email exists
		},
		{
			name:  "inactive user",
			email: "inactive@example.com",
			setupFunc: func() {
				client.User.Create().
					SetEmail("inactive@example.com").
					SetUsername("inactive").
					SetPasswordHash("hash").
					SetIsActive(false).
					Save(context.Background())
			},
			wantErr: false, // Returns success to not reveal account status
		},
		{
			name:  "rate limited",
			email: testUser.Email,
			setupFunc: func() {
				// Set recent password reset
				testUser.Update().
					SetPasswordResetExpiresAt(time.Now().Add(50 * time.Minute)). // Within rate limit window
					SetPasswordResetAttempts(1).
					Save(context.Background())
			},
			wantErr:      true,
			expectedCode: codes.ResourceExhausted,
		},
		{
			name:  "max attempts exceeded today",
			email: testUser.Email,
			setupFunc: func() {
				testUser.Update().
					SetPasswordResetAttempts(MaxPasswordResetAttempts).
					SetPasswordResetExpiresAt(time.Now().Add(-30 * time.Minute)). // Recent attempt
					Save(context.Background())
			},
			wantErr:      true,
			expectedCode: codes.ResourceExhausted,
		},
		{
			name:  "max attempts but over 24 hours ago",
			email: testUser.Email,
			setupFunc: func() {
				testUser.Update().
					SetPasswordResetAttempts(MaxPasswordResetAttempts).
					SetPasswordResetExpiresAt(time.Now().Add(-25 * time.Hour)). // Over 24 hours ago
					Save(context.Background())
			},
			wantErr:    false,
			checkEmail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset user state
			testUser.Update().
				SetPasswordResetAttempts(0).
				ClearPasswordResetToken().
				ClearPasswordResetExpiresAt().
				Save(context.Background())

			// Clear mock email service
			mockEmailService.Clear()

			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			err := service.RequestPasswordReset(ctx, tt.email)

			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)

				if tt.checkEmail {
					// Verify email was sent
					sentEmails := mockEmailService.GetSentEmails()
					assert.Greater(t, len(sentEmails), 0)

					if len(sentEmails) > 0 {
						lastEmail := mockEmailService.GetLastSentEmail()
						assert.Equal(t, tt.email, lastEmail.To)
						assert.Equal(t, "password_reset", lastEmail.Template)
					}

					// Verify user was updated with reset token
					updatedUser, err := client.User.Query().
						Where(user.EmailEQ(tt.email)).
						Only(context.Background())
					if err == nil {
						assert.NotEmpty(t, updatedUser.PasswordResetToken)
						assert.NotNil(t, updatedUser.PasswordResetExpiresAt)
						assert.Greater(t, updatedUser.PasswordResetAttempts, 0)
					}
				}
			}
		})
	}
}

func TestPasswordResetService_VerifyPasswordResetToken(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	passwordManager := auth.NewPasswordManager()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewPasswordResetService(client, mockEmailService, passwordManager, securityLogger)

	// Create test users with tokens
	validToken := "valid-reset-token-12345678901234567890123456"
	expiredToken := "expired-reset-token-123456789012345678901234"

	userWithValidToken, err := client.User.Create().
		SetEmail("valid@example.com").
		SetUsername("validuser").
		SetPasswordHash("hash").
		SetIsActive(true).
		SetPasswordResetToken(validToken).
		SetPasswordResetExpiresAt(time.Now().Add(30 * time.Minute)).
		Save(context.Background())
	require.NoError(t, err)

	_, err = client.User.Create().
		SetEmail("expired@example.com").
		SetUsername("expireduser").
		SetPasswordHash("hash").
		SetIsActive(true).
		SetPasswordResetToken(expiredToken).
		SetPasswordResetExpiresAt(time.Now().Add(-1 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name         string
		token        string
		wantErr      bool
		expectedCode codes.Code
		checkInfo    bool
	}{
		{
			name:      "valid token",
			token:     validToken,
			wantErr:   false,
			checkInfo: true,
		},
		{
			name:         "empty token",
			token:        "",
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "invalid token",
			token:        "invalid-token",
			wantErr:      true,
			expectedCode: codes.NotFound,
		},
		{
			name:         "expired token",
			token:        expiredToken,
			wantErr:      true,
			expectedCode: codes.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := service.VerifyPasswordResetToken(context.Background(), tt.token)

			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, info)
				assert.True(t, info.IsValid)

				if tt.checkInfo {
					// Email should be masked
					assert.Contains(t, info.Email, "@")
					assert.Contains(t, info.Email, "*")
					assert.NotEqual(t, userWithValidToken.Email, info.Email) // Should be masked
					assert.NotNil(t, info.ExpiresAt)
				}
			}
		})
	}
}

func TestPasswordResetService_ResetPassword(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	passwordManager := auth.NewPasswordManager()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewPasswordResetService(client, mockEmailService, passwordManager, securityLogger)

	// Create test user with reset token
	validToken := "valid-reset-token-12345678901234567890123456"
	oldPasswordHash, err := passwordManager.HashPassword("OldPassword123!")
	require.NoError(t, err)

	testUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetUsername("testuser").
		SetPasswordHash(oldPasswordHash).
		SetIsActive(true).
		SetPasswordResetToken(validToken).
		SetPasswordResetExpiresAt(time.Now().Add(30 * time.Minute)).
		SetPasswordResetAttempts(3).
		SetFailedLoginAttempts(5).
		SetAccountLockedUntil(time.Now().Add(1 * time.Hour)).
		SetRefreshToken("old-refresh-token").
		SetRefreshTokenExpiresAt(time.Now().Add(24 * time.Hour)).
		SetSecurityNotificationsEnabled(true).
		Save(context.Background())
	require.NoError(t, err)

	// Add context
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.ContextKeyIPAddress, "127.0.0.1")
	ctx = context.WithValue(ctx, middleware.ContextKeyUserAgent, "test-agent")

	tests := []struct {
		name         string
		token        string
		newPassword  string
		setupFunc    func()
		wantErr      bool
		expectedCode codes.Code
	}{
		{
			name:        "successful password reset",
			token:       validToken,
			newPassword: "NewSecurePassword456!",
			wantErr:     false,
		},
		{
			name:         "empty token",
			token:        "",
			newPassword:  "NewSecurePassword456!",
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "empty password",
			token:        validToken,
			newPassword:  "",
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "weak password",
			token:        validToken,
			newPassword:  "weak",
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "invalid token",
			token:        "invalid-token",
			newPassword:  "NewSecurePassword456!",
			wantErr:      true,
			expectedCode: codes.NotFound,
		},
		{
			name:        "expired token",
			token:       "expired-token-123",
			newPassword: "NewSecurePassword456!",
			setupFunc: func() {
				client.User.Create().
					SetEmail("expired@example.com").
					SetUsername("expired").
					SetPasswordHash("hash").
					SetIsActive(true).
					SetPasswordResetToken("expired-token-123").
					SetPasswordResetExpiresAt(time.Now().Add(-1 * time.Hour)).
					Save(context.Background())
			},
			wantErr:      true,
			expectedCode: codes.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			err := service.ResetPassword(ctx, tt.token, tt.newPassword)

			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)

				// Verify password was changed
				updatedUser, err := client.User.Get(context.Background(), testUser.ID)
				require.NoError(t, err)

				// Check new password works
				err = passwordManager.ComparePassword(updatedUser.PasswordHash, tt.newPassword)
				assert.NoError(t, err)

				// Check old password doesn't work
				err = passwordManager.ComparePassword(updatedUser.PasswordHash, "OldPassword123!")
				assert.Error(t, err)

				// Verify reset token was cleared
				assert.Empty(t, updatedUser.PasswordResetToken)
				assert.Nil(t, updatedUser.PasswordResetExpiresAt)
				assert.Equal(t, 0, updatedUser.PasswordResetAttempts)

				// Verify session was invalidated
				assert.Empty(t, updatedUser.RefreshToken)
				assert.Nil(t, updatedUser.RefreshTokenExpiresAt)

				// Verify account was unlocked
				assert.Equal(t, 0, updatedUser.FailedLoginAttempts)
				assert.Nil(t, updatedUser.AccountLockedUntil)

				// Verify timestamps were updated
				assert.NotNil(t, updatedUser.PasswordChangedAt)
				assert.NotNil(t, updatedUser.PasswordResetAt)

				// Verify notification email was sent
				if updatedUser.SecurityNotificationsEnabled {
					lastEmail := mockEmailService.GetLastSentEmail()
					assert.Equal(t, "password_changed", lastEmail.Template)
				}
			}
		})
	}
}

func TestPasswordResetService_GetPasswordResetStatus(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	passwordManager := auth.NewPasswordManager()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewPasswordResetService(client, mockEmailService, passwordManager, securityLogger)

	// Create test users with different states
	userWithActiveRequest, err := client.User.Create().
		SetEmail("active@example.com").
		SetUsername("activeuser").
		SetPasswordHash("hash").
		SetPasswordResetToken("active-token").
		SetPasswordResetExpiresAt(time.Now().Add(30 * time.Minute)).
		SetPasswordResetAttempts(2).
		SetPasswordResetAt(time.Now().Add(-1 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	userWithExpiredRequest, err := client.User.Create().
		SetEmail("expired@example.com").
		SetUsername("expireduser").
		SetPasswordHash("hash").
		SetPasswordResetToken("expired-token").
		SetPasswordResetExpiresAt(time.Now().Add(-1 * time.Hour)).
		SetPasswordResetAttempts(1).
		Save(context.Background())
	require.NoError(t, err)

	userNoRequest, err := client.User.Create().
		SetEmail("norequest@example.com").
		SetUsername("norequestuser").
		SetPasswordHash("hash").
		SetPasswordResetAttempts(0).
		Save(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name           string
		userID         string
		expectedStatus PasswordResetStatus
		wantErr        bool
	}{
		{
			name:   "user with active request",
			userID: userWithActiveRequest.ID.String(),
			expectedStatus: PasswordResetStatus{
				Attempts:         2,
				MaxAttempts:      MaxPasswordResetAttempts,
				IsExpired:        false,
				HasActiveRequest: true,
				CanRequest:       false, // Rate limited
			},
			wantErr: false,
		},
		{
			name:   "user with expired request",
			userID: userWithExpiredRequest.ID.String(),
			expectedStatus: PasswordResetStatus{
				Attempts:         1,
				MaxAttempts:      MaxPasswordResetAttempts,
				IsExpired:        true,
				HasActiveRequest: false,
				CanRequest:       true,
			},
			wantErr: false,
		},
		{
			name:   "user with no request",
			userID: userNoRequest.ID.String(),
			expectedStatus: PasswordResetStatus{
				Attempts:         0,
				MaxAttempts:      MaxPasswordResetAttempts,
				IsExpired:        false,
				HasActiveRequest: false,
				CanRequest:       true,
			},
			wantErr: false,
		},
		{
			name:    "invalid user ID",
			userID:  "invalid-uuid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := service.GetPasswordResetStatus(context.Background(), tt.userID)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, status)
				assert.Equal(t, tt.expectedStatus.Attempts, status.Attempts)
				assert.Equal(t, tt.expectedStatus.MaxAttempts, status.MaxAttempts)
				assert.Equal(t, tt.expectedStatus.IsExpired, status.IsExpired)
				assert.Equal(t, tt.expectedStatus.HasActiveRequest, status.HasActiveRequest)
				assert.Equal(t, tt.expectedStatus.CanRequest, status.CanRequest)
			}
		})
	}
}

func TestPasswordResetService_CleanupExpiredTokens(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	passwordManager := auth.NewPasswordManager()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewPasswordResetService(client, mockEmailService, passwordManager, securityLogger)

	// Create users with expired and valid tokens
	expiredUser1, err := client.User.Create().
		SetEmail("expired1@example.com").
		SetUsername("expired1").
		SetPasswordHash("hash").
		SetPasswordResetToken("expired-token-1").
		SetPasswordResetExpiresAt(time.Now().Add(-1 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	expiredUser2, err := client.User.Create().
		SetEmail("expired2@example.com").
		SetUsername("expired2").
		SetPasswordHash("hash").
		SetPasswordResetToken("expired-token-2").
		SetPasswordResetExpiresAt(time.Now().Add(-24 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	validUser, err := client.User.Create().
		SetEmail("valid@example.com").
		SetUsername("valid").
		SetPasswordHash("hash").
		SetPasswordResetToken("valid-token").
		SetPasswordResetExpiresAt(time.Now().Add(1 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	// Run cleanup
	err = service.CleanupExpiredTokens(context.Background())
	require.NoError(t, err)

	// Verify expired tokens were cleaned
	updatedExpired1, err := client.User.Get(context.Background(), expiredUser1.ID)
	require.NoError(t, err)
	assert.Empty(t, updatedExpired1.PasswordResetToken)
	assert.Nil(t, updatedExpired1.PasswordResetExpiresAt)

	updatedExpired2, err := client.User.Get(context.Background(), expiredUser2.ID)
	require.NoError(t, err)
	assert.Empty(t, updatedExpired2.PasswordResetToken)
	assert.Nil(t, updatedExpired2.PasswordResetExpiresAt)

	// Verify valid token was not cleaned
	updatedValid, err := client.User.Get(context.Background(), validUser.ID)
	require.NoError(t, err)
	assert.Equal(t, "valid-token", updatedValid.PasswordResetToken)
	assert.NotNil(t, updatedValid.PasswordResetExpiresAt)
}

func TestPasswordResetService_MaskEmail(t *testing.T) {
	tests := []struct {
		email    string
		expected string
	}{
		{
			email:    "user@example.com",
			expected: "u**r@example.com",
		},
		{
			email:    "a@example.com",
			expected: "*@example.com",
		},
		{
			email:    "ab@example.com",
			expected: "**@example.com",
		},
		{
			email:    "longusername@example.com",
			expected: "l**********e@example.com",
		},
		{
			email:    "invalid-email",
			expected: "invalid-email", // Returns as-is if not valid email format
		},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			masked := maskEmail(tt.email)
			assert.Equal(t, tt.expected, masked)
		})
	}
}
