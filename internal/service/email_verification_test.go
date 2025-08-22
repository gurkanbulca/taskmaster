// internal/service/email_verification_test.go
package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gurkanbulca/taskmaster/ent/generated/enttest"
	"github.com/gurkanbulca/taskmaster/pkg/email"

	_ "github.com/mattn/go-sqlite3"
)

func TestEmailVerificationService_SendVerificationEmail(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewEmailVerificationService(client, mockEmailService, securityLogger)

	// Create test user
	testUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetUsername("testuser").
		SetPasswordHash("hash").
		SetEmailVerified(false).
		SetEmailVerificationAttempts(0).
		Save(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name         string
		userID       string
		setupFunc    func()
		wantErr      bool
		expectedCode codes.Code
	}{
		{
			name:    "successful send",
			userID:  testUser.ID.String(),
			wantErr: false,
		},
		{
			name:         "invalid user ID",
			userID:       "invalid-uuid",
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "non-existent user",
			userID:       "00000000-0000-0000-0000-000000000000",
			wantErr:      true,
			expectedCode: codes.NotFound,
		},
		{
			name:   "already verified email",
			userID: testUser.ID.String(),
			setupFunc: func() {
				testUser.Update().
					SetEmailVerified(true).
					Save(context.Background())
			},
			wantErr:      true,
			expectedCode: codes.FailedPrecondition,
		},
		{
			name:   "max attempts exceeded",
			userID: testUser.ID.String(),
			setupFunc: func() {
				testUser.Update().
					SetEmailVerified(false).
					SetEmailVerificationAttempts(MaxEmailVerificationAttempts).
					Save(context.Background())
			},
			wantErr:      true,
			expectedCode: codes.ResourceExhausted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset user state
			testUser.Update().
				SetEmailVerified(false).
				SetEmailVerificationAttempts(0).
				ClearEmailVerificationToken().
				ClearEmailVerificationExpiresAt().
				Save(context.Background())

			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			err := service.SendVerificationEmail(context.Background(), tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)

				// Verify email was sent
				sentEmails := mockEmailService.GetSentEmails()
				assert.Greater(t, len(sentEmails), 0)
				lastEmail := mockEmailService.GetLastSentEmail()
				assert.Equal(t, testUser.Email, lastEmail.To)
				assert.Equal(t, "verification", lastEmail.Template)

				// Verify user was updated
				updatedUser, err := client.User.Get(context.Background(), testUser.ID)
				require.NoError(t, err)
				assert.NotEmpty(t, updatedUser.EmailVerificationToken)
				assert.NotNil(t, updatedUser.EmailVerificationExpiresAt)
				assert.Equal(t, 1, updatedUser.EmailVerificationAttempts)
			}
		})
	}
}

func TestEmailVerificationService_VerifyEmail(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewEmailVerificationService(client, mockEmailService, securityLogger)

	// Create test user with verification token
	validToken := "valid-verification-token-12345678901234567890"
	expiredToken := "expired-verification-token-123456789012345"

	testUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetUsername("testuser").
		SetPasswordHash("hash").
		SetEmailVerified(false).
		SetEmailVerificationToken(validToken).
		SetEmailVerificationExpiresAt(time.Now().Add(24 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	// Create user with expired token
	expiredUser, err := client.User.Create().
		SetEmail("expired@example.com").
		SetUsername("expireduser").
		SetPasswordHash("hash").
		SetEmailVerified(false).
		SetEmailVerificationToken(expiredToken).
		SetEmailVerificationExpiresAt(time.Now().Add(-1 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name         string
		token        string
		wantErr      bool
		expectedCode codes.Code
	}{
		{
			name:    "successful verification",
			token:   validToken,
			wantErr: false,
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
			err := service.VerifyEmail(context.Background(), tt.token)

			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)

				// Verify user email is verified
				updatedUser, err := client.User.Get(context.Background(), testUser.ID)
				require.NoError(t, err)
				assert.True(t, updatedUser.EmailVerified)
				assert.Empty(t, updatedUser.EmailVerificationToken)
				assert.Nil(t, updatedUser.EmailVerificationExpiresAt)
				assert.Equal(t, 0, updatedUser.EmailVerificationAttempts)

				// Verify welcome email was sent
				lastEmail := mockEmailService.GetLastSentEmail()
				assert.Equal(t, "welcome", lastEmail.Template)
			}
		})
	}

	// Verify expired user wasn't modified
	unchangedUser, err := client.User.Get(context.Background(), expiredUser.ID)
	require.NoError(t, err)
	assert.False(t, unchangedUser.EmailVerified)
	assert.Equal(t, expiredToken, unchangedUser.EmailVerificationToken)
}

func TestEmailVerificationService_ResendVerificationEmail(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewEmailVerificationService(client, mockEmailService, securityLogger)

	// Create test user
	testUser, err := client.User.Create().
		SetEmail("test@example.com").
		SetUsername("testuser").
		SetPasswordHash("hash").
		SetEmailVerified(false).
		SetEmailVerificationAttempts(1).
		SetEmailVerificationToken("old-token").
		SetEmailVerificationExpiresAt(time.Now().Add(-23 * time.Hour)). // More than 1 hour ago
		Save(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name         string
		userID       string
		setupFunc    func()
		wantErr      bool
		expectedCode codes.Code
	}{
		{
			name:    "successful resend",
			userID:  testUser.ID.String(),
			wantErr: false,
		},
		{
			name:         "invalid user ID",
			userID:       "invalid-uuid",
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:   "already verified",
			userID: testUser.ID.String(),
			setupFunc: func() {
				testUser.Update().
					SetEmailVerified(true).
					Save(context.Background())
			},
			wantErr:      true,
			expectedCode: codes.FailedPrecondition,
		},
		{
			name:   "rate limited",
			userID: testUser.ID.String(),
			setupFunc: func() {
				testUser.Update().
					SetEmailVerified(false).
					SetEmailVerificationExpiresAt(time.Now().Add(23 * time.Hour)). // Less than 1 hour ago
					Save(context.Background())
			},
			wantErr:      true,
			expectedCode: codes.ResourceExhausted,
		},
		{
			name:   "max attempts exceeded",
			userID: testUser.ID.String(),
			setupFunc: func() {
				testUser.Update().
					SetEmailVerified(false).
					SetEmailVerificationAttempts(MaxEmailVerificationAttempts).
					SetEmailVerificationExpiresAt(time.Now().Add(-2 * time.Hour)).
					Save(context.Background())
			},
			wantErr:      true,
			expectedCode: codes.ResourceExhausted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset user state
			testUser.Update().
				SetEmailVerified(false).
				SetEmailVerificationAttempts(1).
				SetEmailVerificationToken("old-token").
				SetEmailVerificationExpiresAt(time.Now().Add(-23 * time.Hour)).
				Save(context.Background())

			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			err := service.ResendVerificationEmail(context.Background(), tt.userID)

			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)

				// Verify new token was generated
				updatedUser, err := client.User.Get(context.Background(), testUser.ID)
				require.NoError(t, err)
				assert.NotEqual(t, "old-token", updatedUser.EmailVerificationToken)
				assert.NotNil(t, updatedUser.EmailVerificationExpiresAt)
				assert.Equal(t, 2, updatedUser.EmailVerificationAttempts)

				// Verify email was sent
				lastEmail := mockEmailService.GetLastSentEmail()
				assert.Equal(t, testUser.Email, lastEmail.To)
				assert.Equal(t, "verification", lastEmail.Template)
			}
		})
	}
}

func TestEmailVerificationService_GetVerificationStatus(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewEmailVerificationService(client, mockEmailService, securityLogger)

	// Create test users with different states
	verifiedUser, err := client.User.Create().
		SetEmail("verified@example.com").
		SetUsername("verifieduser").
		SetPasswordHash("hash").
		SetEmailVerified(true).
		Save(context.Background())
	require.NoError(t, err)

	unverifiedUser, err := client.User.Create().
		SetEmail("unverified@example.com").
		SetUsername("unverifieduser").
		SetPasswordHash("hash").
		SetEmailVerified(false).
		SetEmailVerificationAttempts(2).
		SetEmailVerificationToken("token").
		SetEmailVerificationExpiresAt(time.Now().Add(10 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	expiredUser, err := client.User.Create().
		SetEmail("expired@example.com").
		SetUsername("expireduser").
		SetPasswordHash("hash").
		SetEmailVerified(false).
		SetEmailVerificationAttempts(1).
		SetEmailVerificationToken("expired-token").
		SetEmailVerificationExpiresAt(time.Now().Add(-1 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	tests := []struct {
		name           string
		userID         string
		expectedStatus EmailVerificationStatus
		wantErr        bool
	}{
		{
			name:   "verified user",
			userID: verifiedUser.ID.String(),
			expectedStatus: EmailVerificationStatus{
				EmailVerified: true,
				Attempts:      0,
				MaxAttempts:   MaxEmailVerificationAttempts,
				IsExpired:     false,
				CanResend:     false,
			},
			wantErr: false,
		},
		{
			name:   "unverified user",
			userID: unverifiedUser.ID.String(),
			expectedStatus: EmailVerificationStatus{
				EmailVerified: false,
				Attempts:      2,
				MaxAttempts:   MaxEmailVerificationAttempts,
				IsExpired:     false,
				CanResend:     false, // Can't resend yet (rate limited)
			},
			wantErr: false,
		},
		{
			name:   "expired token",
			userID: expiredUser.ID.String(),
			expectedStatus: EmailVerificationStatus{
				EmailVerified: false,
				Attempts:      1,
				MaxAttempts:   MaxEmailVerificationAttempts,
				IsExpired:     true,
				CanResend:     true,
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
			status, err := service.GetVerificationStatus(context.Background(), tt.userID)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, status)
				assert.Equal(t, tt.expectedStatus.EmailVerified, status.EmailVerified)
				assert.Equal(t, tt.expectedStatus.Attempts, status.Attempts)
				assert.Equal(t, tt.expectedStatus.MaxAttempts, status.MaxAttempts)
				assert.Equal(t, tt.expectedStatus.IsExpired, status.IsExpired)
				assert.Equal(t, tt.expectedStatus.CanResend, status.CanResend)
			}
		})
	}
}

func TestEmailVerificationService_CleanupExpiredTokens(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewEmailVerificationService(client, mockEmailService, securityLogger)

	// Create users with expired and valid tokens
	expiredUser1, err := client.User.Create().
		SetEmail("expired1@example.com").
		SetUsername("expired1").
		SetPasswordHash("hash").
		SetEmailVerificationToken("expired-token-1").
		SetEmailVerificationExpiresAt(time.Now().Add(-1 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	expiredUser2, err := client.User.Create().
		SetEmail("expired2@example.com").
		SetUsername("expired2").
		SetPasswordHash("hash").
		SetEmailVerificationToken("expired-token-2").
		SetEmailVerificationExpiresAt(time.Now().Add(-24 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	validUser, err := client.User.Create().
		SetEmail("valid@example.com").
		SetUsername("valid").
		SetPasswordHash("hash").
		SetEmailVerificationToken("valid-token").
		SetEmailVerificationExpiresAt(time.Now().Add(1 * time.Hour)).
		Save(context.Background())
	require.NoError(t, err)

	// Run cleanup
	err = service.CleanupExpiredTokens(context.Background())
	require.NoError(t, err)

	// Verify expired tokens were cleaned
	updatedExpired1, err := client.User.Get(context.Background(), expiredUser1.ID)
	require.NoError(t, err)
	assert.Empty(t, updatedExpired1.EmailVerificationToken)
	assert.Nil(t, updatedExpired1.EmailVerificationExpiresAt)

	updatedExpired2, err := client.User.Get(context.Background(), expiredUser2.ID)
	require.NoError(t, err)
	assert.Empty(t, updatedExpired2.EmailVerificationToken)
	assert.Nil(t, updatedExpired2.EmailVerificationExpiresAt)

	// Verify valid token was not cleaned
	updatedValid, err := client.User.Get(context.Background(), validUser.ID)
	require.NoError(t, err)
	assert.Equal(t, "valid-token", updatedValid.EmailVerificationToken)
	assert.NotNil(t, updatedValid.EmailVerificationExpiresAt)
}

func TestEmailVerificationService_TokenGeneration(t *testing.T) {
	// Setup
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)

	service := NewEmailVerificationService(client, mockEmailService, securityLogger)

	// Test token generation
	token1, err := service.generateVerificationToken()
	require.NoError(t, err)
	assert.Len(t, token1, EmailVerificationTokenLength*2) // Hex encoding doubles the length

	token2, err := service.generateVerificationToken()
	require.NoError(t, err)
	assert.Len(t, token2, EmailVerificationTokenLength*2)

	// Tokens should be unique
	assert.NotEqual(t, token1, token2)
}
