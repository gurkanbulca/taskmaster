// internal/service/auth_service_test.go
package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/gurkanbulca/taskmaster/api/proto/auth/v1/generated"
	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/ent/generated/enttest"
	"github.com/gurkanbulca/taskmaster/ent/generated/user"
	"github.com/gurkanbulca/taskmaster/internal/config"
	"github.com/gurkanbulca/taskmaster/internal/middleware"
	"github.com/gurkanbulca/taskmaster/pkg/auth"
	"github.com/gurkanbulca/taskmaster/pkg/email"

	_ "github.com/mattn/go-sqlite3"
)

// Test helpers
func setupTestDB(t *testing.T) *ent.Client {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	return client
}

func createTestUser(t *testing.T, client *ent.Client) *ent.User {
	passwordManager := auth.NewPasswordManager()
	hashedPassword, err := passwordManager.HashPassword("TestPass123!")
	require.NoError(t, err)

	user, err := client.User.Create().
		SetEmail("test@example.com").
		SetUsername("testuser").
		SetPasswordHash(hashedPassword).
		SetFirstName("Test").
		SetLastName("User").
		SetRole(user.RoleUser).
		SetIsActive(true).
		SetEmailVerified(false).
		Save(context.Background())
	require.NoError(t, err)

	return user
}

func createTestSecurityConfig() config.SecurityConfig {
	return config.SecurityConfig{
		MaxLoginAttempts:             3,
		AccountLockoutDuration:       15 * time.Minute,
		MaxEmailVerificationAttempts: 5,
		MaxPasswordResetAttempts:     5,
		PasswordResetRateLimit:       15 * time.Minute,
		EnableSecurityNotifications:  true,
		RequireEmailVerification:     false,
		SessionTimeoutDuration:       30 * 24 * time.Hour,
	}
}

// Test AuthService
func TestAuthService_Register(t *testing.T) {
	tests := []struct {
		name          string
		request       *authv1.RegisterRequest
		setupFunc     func(*ent.Client)
		wantErr       bool
		expectedCode  codes.Code
		expectedEmail string
	}{
		{
			name: "successful registration",
			request: &authv1.RegisterRequest{
				Email:     "newuser@example.com",
				Username:  "newuser",
				Password:  "SecurePass123!",
				FirstName: "New",
				LastName:  "User",
			},
			wantErr:       false,
			expectedEmail: "newuser@example.com",
		},
		{
			name: "duplicate email",
			request: &authv1.RegisterRequest{
				Email:     "test@example.com",
				Username:  "newuser2",
				Password:  "SecurePass123!",
				FirstName: "New",
				LastName:  "User",
			},
			setupFunc: func(client *ent.Client) {
				createTestUser(t, client)
			},
			wantErr:      true,
			expectedCode: codes.AlreadyExists,
		},
		{
			name: "invalid email format",
			request: &authv1.RegisterRequest{
				Email:     "invalid-email",
				Username:  "newuser3",
				Password:  "SecurePass123!",
				FirstName: "New",
				LastName:  "User",
			},
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "weak password",
			request: &authv1.RegisterRequest{
				Email:     "weak@example.com",
				Username:  "weakuser",
				Password:  "weak",
				FirstName: "Weak",
				LastName:  "User",
			},
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "empty username",
			request: &authv1.RegisterRequest{
				Email:     "nouser@example.com",
				Username:  "",
				Password:  "SecurePass123!",
				FirstName: "No",
				LastName:  "Username",
			},
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			client := setupTestDB(t)
			defer client.Close()

			if tt.setupFunc != nil {
				tt.setupFunc(client)
			}

			tokenManager := auth.NewTokenManager(
				"test-access-secret",
				"test-refresh-secret",
				15*time.Minute,
				7*24*time.Hour,
			)

			mockEmailService := email.NewMockEmailService()
			securityService := NewSecurityService(client)
			securityLogger := NewSecurityLogger(securityService)
			emailVerificationService := NewEmailVerificationService(client, mockEmailService, securityLogger)
			passwordResetService := NewPasswordResetService(client, mockEmailService, auth.NewPasswordManager(), securityLogger)

			authService := NewAuthService(
				client,
				tokenManager,
				emailVerificationService,
				passwordResetService,
				securityLogger,
				createTestSecurityConfig(),
			)

			// Execute
			resp, err := authService.Register(context.Background(), tt.request)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.expectedEmail, resp.User.Email)
				assert.NotEmpty(t, resp.AccessToken)
				assert.NotEmpty(t, resp.RefreshToken)
				assert.Greater(t, resp.ExpiresIn, int64(0))
			}
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	tests := []struct {
		name         string
		request      *authv1.LoginRequest
		setupFunc    func(*ent.Client) *ent.User
		wantErr      bool
		expectedCode codes.Code
	}{
		{
			name: "successful login with email",
			request: &authv1.LoginRequest{
				Email:    "test@example.com",
				Password: "TestPass123!",
			},
			setupFunc: func(client *ent.Client) *ent.User {
				return createTestUser(t, client)
			},
			wantErr: false,
		},
		{
			name: "successful login with username",
			request: &authv1.LoginRequest{
				Email:    "testuser",
				Password: "TestPass123!",
			},
			setupFunc: func(client *ent.Client) *ent.User {
				return createTestUser(t, client)
			},
			wantErr: false,
		},
		{
			name: "invalid password",
			request: &authv1.LoginRequest{
				Email:    "test@example.com",
				Password: "WrongPassword123!",
			},
			setupFunc: func(client *ent.Client) *ent.User {
				return createTestUser(t, client)
			},
			wantErr:      true,
			expectedCode: codes.Unauthenticated,
		},
		{
			name: "non-existent user",
			request: &authv1.LoginRequest{
				Email:    "nonexistent@example.com",
				Password: "TestPass123!",
			},
			wantErr:      true,
			expectedCode: codes.Unauthenticated,
		},
		{
			name: "inactive account",
			request: &authv1.LoginRequest{
				Email:    "inactive@example.com",
				Password: "TestPass123!",
			},
			setupFunc: func(client *ent.Client) *ent.User {
				passwordManager := auth.NewPasswordManager()
				hashedPassword, _ := passwordManager.HashPassword("TestPass123!")

				user, _ := client.User.Create().
					SetEmail("inactive@example.com").
					SetUsername("inactiveuser").
					SetPasswordHash(hashedPassword).
					SetIsActive(false).
					Save(context.Background())
				return user
			},
			wantErr:      true,
			expectedCode: codes.PermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			client := setupTestDB(t)
			defer client.Close()

			var testUser *ent.User
			if tt.setupFunc != nil {
				testUser = tt.setupFunc(client)
			}

			tokenManager := auth.NewTokenManager(
				"test-access-secret",
				"test-refresh-secret",
				15*time.Minute,
				7*24*time.Hour,
			)

			mockEmailService := email.NewMockEmailService()
			securityService := NewSecurityService(client)
			securityLogger := NewSecurityLogger(securityService)
			emailVerificationService := NewEmailVerificationService(client, mockEmailService, securityLogger)
			passwordResetService := NewPasswordResetService(client, mockEmailService, auth.NewPasswordManager(), securityLogger)

			authService := NewAuthService(
				client,
				tokenManager,
				emailVerificationService,
				passwordResetService,
				securityLogger,
				createTestSecurityConfig(),
			)

			// Add context with client info
			ctx := context.Background()
			ctx = context.WithValue(ctx, middleware.ContextKeyIPAddress, "127.0.0.1")
			ctx = context.WithValue(ctx, middleware.ContextKeyUserAgent, "test-agent")

			// Execute
			resp, err := authService.Login(ctx, tt.request)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.NotEmpty(t, resp.AccessToken)
				assert.NotEmpty(t, resp.RefreshToken)
				assert.False(t, resp.AccountLocked)

				// Verify user was updated with login info
				if testUser != nil {
					updatedUser, err := client.User.Get(ctx, testUser.ID)
					require.NoError(t, err)
					assert.NotNil(t, updatedUser.LastLogin)
					assert.Equal(t, "127.0.0.1", updatedUser.LastLoginIP)
				}
			}
		})
	}
}

func TestAuthService_AccountLockout(t *testing.T) {
	// Setup
	client := setupTestDB(t)
	defer client.Close()

	// Create test user
	testUser := createTestUser(t, client)

	tokenManager := auth.NewTokenManager(
		"test-access-secret",
		"test-refresh-secret",
		15*time.Minute,
		7*24*time.Hour,
	)

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)
	emailVerificationService := NewEmailVerificationService(client, mockEmailService, securityLogger)
	passwordResetService := NewPasswordResetService(client, mockEmailService, auth.NewPasswordManager(), securityLogger)

	// Create auth service with max 3 login attempts
	securityConfig := createTestSecurityConfig()
	securityConfig.MaxLoginAttempts = 3
	securityConfig.AccountLockoutDuration = 5 * time.Minute

	authService := NewAuthService(
		client,
		tokenManager,
		emailVerificationService,
		passwordResetService,
		securityLogger,
		securityConfig,
	)

	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.ContextKeyIPAddress, "127.0.0.1")

	// Test multiple failed login attempts
	for i := 1; i <= 3; i++ {
		req := &authv1.LoginRequest{
			Email:    testUser.Email,
			Password: "WrongPassword123!",
		}

		resp, err := authService.Login(ctx, req)
		require.Error(t, err)

		if i < 3 {
			// Should not be locked yet
			st, _ := status.FromError(err)
			assert.Equal(t, codes.Unauthenticated, st.Code())
		} else {
			// Should be locked after 3rd attempt
			st, _ := status.FromError(err)
			assert.Equal(t, codes.PermissionDenied, st.Code())
			assert.Contains(t, st.Message(), "account locked")

			// Response should indicate account is locked
			if resp != nil {
				assert.True(t, resp.AccountLocked)
				assert.NotNil(t, resp.LockedUntil)
			}
		}
	}

	// Verify account is locked in database
	updatedUser, err := client.User.Get(ctx, testUser.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, updatedUser.FailedLoginAttempts)
	assert.NotNil(t, updatedUser.AccountLockedUntil)
	assert.True(t, updatedUser.AccountLockedUntil.After(time.Now()))

	// Try to login with correct password while locked
	req := &authv1.LoginRequest{
		Email:    testUser.Email,
		Password: "TestPass123!",
	}
	_, err = authService.Login(ctx, req)
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
	assert.Contains(t, st.Message(), "account is locked")
}

func TestAuthService_RefreshToken(t *testing.T) {
	// Setup
	client := setupTestDB(t)
	defer client.Close()

	testUser := createTestUser(t, client)

	tokenManager := auth.NewTokenManager(
		"test-access-secret",
		"test-refresh-secret",
		15*time.Minute,
		7*24*time.Hour,
	)

	// Generate initial tokens
	_, refreshToken, _, err := tokenManager.GenerateTokenPair(
		testUser.ID.String(),
		testUser.Email,
		testUser.Username,
		string(testUser.Role),
	)
	require.NoError(t, err)

	// Save refresh token to user
	testUser, err = testUser.Update().
		SetRefreshToken(refreshToken).
		SetRefreshTokenExpiresAt(time.Now().Add(7 * 24 * time.Hour)).
		SetLastLogin(time.Now()).
		Save(context.Background())
	require.NoError(t, err)

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)
	emailVerificationService := NewEmailVerificationService(client, mockEmailService, securityLogger)
	passwordResetService := NewPasswordResetService(client, mockEmailService, auth.NewPasswordManager(), securityLogger)

	authService := NewAuthService(
		client,
		tokenManager,
		emailVerificationService,
		passwordResetService,
		securityLogger,
		createTestSecurityConfig(),
	)

	tests := []struct {
		name         string
		refreshToken string
		setupFunc    func()
		wantErr      bool
		expectedCode codes.Code
	}{
		{
			name:         "successful token refresh",
			refreshToken: refreshToken,
			wantErr:      false,
		},
		{
			name:         "invalid refresh token",
			refreshToken: "invalid-token",
			wantErr:      true,
			expectedCode: codes.Unauthenticated,
		},
		{
			name:         "empty refresh token",
			refreshToken: "",
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "expired refresh token",
			refreshToken: refreshToken,
			setupFunc: func() {
				// Set refresh token as expired
				testUser.Update().
					SetRefreshTokenExpiresAt(time.Now().Add(-1 * time.Hour)).
					Save(context.Background())
			},
			wantErr:      true,
			expectedCode: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			req := &authv1.RefreshTokenRequest{
				RefreshToken: tt.refreshToken,
			}

			resp, err := authService.RefreshToken(context.Background(), req)

			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.NotEmpty(t, resp.AccessToken)
				assert.NotEmpty(t, resp.RefreshToken)
				assert.Greater(t, resp.ExpiresIn, int64(0))
			}
		})
	}
}

func TestAuthService_GetMe(t *testing.T) {
	// Setup
	client := setupTestDB(t)
	defer client.Close()

	testUser := createTestUser(t, client)

	tokenManager := auth.NewTokenManager(
		"test-access-secret",
		"test-refresh-secret",
		15*time.Minute,
		7*24*time.Hour,
	)

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)
	emailVerificationService := NewEmailVerificationService(client, mockEmailService, securityLogger)
	passwordResetService := NewPasswordResetService(client, mockEmailService, auth.NewPasswordManager(), securityLogger)

	authService := NewAuthService(
		client,
		tokenManager,
		emailVerificationService,
		passwordResetService,
		securityLogger,
		createTestSecurityConfig(),
	)

	tests := []struct {
		name         string
		setupContext func() context.Context
		wantErr      bool
		expectedCode codes.Code
	}{
		{
			name: "successful get me",
			setupContext: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, middleware.ContextKeyUserID, testUser.ID.String())
				return ctx
			},
			wantErr: false,
		},
		{
			name: "no user in context",
			setupContext: func() context.Context {
				return context.Background()
			},
			wantErr:      true,
			expectedCode: codes.Unauthenticated,
		},
		{
			name: "invalid user ID in context",
			setupContext: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, middleware.ContextKeyUserID, "invalid-uuid")
				return ctx
			},
			wantErr:      true,
			expectedCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()

			resp, err := authService.GetMe(ctx, nil)

			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, testUser.Email, resp.User.Email)
				assert.Equal(t, testUser.Username, resp.User.Username)
			}
		})
	}
}

func TestAuthService_ChangePassword(t *testing.T) {
	// Setup
	client := setupTestDB(t)
	defer client.Close()

	testUser := createTestUser(t, client)

	tokenManager := auth.NewTokenManager(
		"test-access-secret",
		"test-refresh-secret",
		15*time.Minute,
		7*24*time.Hour,
	)

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)
	emailVerificationService := NewEmailVerificationService(client, mockEmailService, securityLogger)
	passwordResetService := NewPasswordResetService(client, mockEmailService, auth.NewPasswordManager(), securityLogger)

	authService := NewAuthService(
		client,
		tokenManager,
		emailVerificationService,
		passwordResetService,
		securityLogger,
		createTestSecurityConfig(),
	)

	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, testUser.ID.String())

	tests := []struct {
		name         string
		request      *authv1.ChangePasswordRequest
		wantErr      bool
		expectedCode codes.Code
	}{
		{
			name: "successful password change",
			request: &authv1.ChangePasswordRequest{
				CurrentPassword: "TestPass123!",
				NewPassword:     "NewSecurePass456!",
			},
			wantErr: false,
		},
		{
			name: "incorrect current password",
			request: &authv1.ChangePasswordRequest{
				CurrentPassword: "WrongPassword123!",
				NewPassword:     "NewSecurePass456!",
			},
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "weak new password",
			request: &authv1.ChangePasswordRequest{
				CurrentPassword: "TestPass123!",
				NewPassword:     "weak",
			},
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "empty passwords",
			request: &authv1.ChangePasswordRequest{
				CurrentPassword: "",
				NewPassword:     "",
			},
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := authService.ChangePassword(ctx, tt.request)

			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)

				// Verify password was changed
				updatedUser, err := client.User.Get(ctx, testUser.ID)
				require.NoError(t, err)

				// Try to verify with new password
				passwordManager := auth.NewPasswordManager()
				err = passwordManager.ComparePassword(updatedUser.PasswordHash, tt.request.NewPassword)
				assert.NoError(t, err)

				// Verify refresh token was cleared
				assert.Empty(t, updatedUser.RefreshToken)
			}
		})
	}
}

func TestAuthService_UpdateProfile(t *testing.T) {
	// Setup
	client := setupTestDB(t)
	defer client.Close()

	testUser := createTestUser(t, client)

	tokenManager := auth.NewTokenManager(
		"test-access-secret",
		"test-refresh-secret",
		15*time.Minute,
		7*24*time.Hour,
	)

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)
	emailVerificationService := NewEmailVerificationService(client, mockEmailService, securityLogger)
	passwordResetService := NewPasswordResetService(client, mockEmailService, auth.NewPasswordManager(), securityLogger)

	authService := NewAuthService(
		client,
		tokenManager,
		emailVerificationService,
		passwordResetService,
		securityLogger,
		createTestSecurityConfig(),
	)

	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, testUser.ID.String())

	req := &authv1.UpdateProfileRequest{
		FirstName: "Updated",
		LastName:  "Name",
		Preferences: map[string]string{
			"theme":    "dark",
			"language": "en",
		},
		EmailNotificationsEnabled:    true,
		SecurityNotificationsEnabled: false,
	}

	resp, err := authService.UpdateProfile(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "Updated", resp.User.FirstName)
	assert.Equal(t, "Name", resp.User.LastName)
	assert.True(t, resp.User.EmailNotificationsEnabled)
	assert.False(t, resp.User.SecurityNotificationsEnabled)

	// Verify in database
	updatedUser, err := client.User.Get(ctx, testUser.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", updatedUser.FirstName)
	assert.Equal(t, "Name", updatedUser.LastName)
	assert.True(t, updatedUser.EmailNotificationsEnabled)
	assert.False(t, updatedUser.SecurityNotificationsEnabled)
}

func TestAuthService_GetSecurityEvents(t *testing.T) {
	// Setup
	client := setupTestDB(t)
	defer client.Close()

	testUser := createTestUser(t, client)
	adminUser, err := client.User.Create().
		SetEmail("admin@example.com").
		SetUsername("admin").
		SetPasswordHash("hash").
		SetRole(user.RoleAdmin).
		SetIsActive(true).
		Save(context.Background())
	require.NoError(t, err)

	// Create some security events
	for i := 0; i < 5; i++ {
		_, err = client.SecurityEvent.Create().
			SetUserID(testUser.ID).
			SetEventType("login_success").
			SetDescription(fmt.Sprintf("Event %d", i)).
			SetSeverity("low").
			SetIPAddress("127.0.0.1").
			Save(context.Background())
		require.NoError(t, err)
	}

	// Create events for admin user
	for i := 0; i < 3; i++ {
		_, err = client.SecurityEvent.Create().
			SetUserID(adminUser.ID).
			SetEventType("login_failed").
			SetDescription(fmt.Sprintf("Admin event %d", i)).
			SetSeverity("medium").
			SetIPAddress("192.168.1.1").
			Save(context.Background())
		require.NoError(t, err)
	}

	tokenManager := auth.NewTokenManager(
		"test-access-secret",
		"test-refresh-secret",
		15*time.Minute,
		7*24*time.Hour,
	)

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)
	emailVerificationService := NewEmailVerificationService(client, mockEmailService, securityLogger)
	passwordResetService := NewPasswordResetService(client, mockEmailService, auth.NewPasswordManager(), securityLogger)

	authService := NewAuthService(
		client,
		tokenManager,
		emailVerificationService,
		passwordResetService,
		securityLogger,
		createTestSecurityConfig(),
	)

	tests := []struct {
		name          string
		userID        string
		userRole      string
		request       *authv1.GetSecurityEventsRequest
		expectedCount int
	}{
		{
			name:     "regular user sees only own events",
			userID:   testUser.ID.String(),
			userRole: "user",
			request: &authv1.GetSecurityEventsRequest{
				PageSize: 10,
			},
			expectedCount: 5,
		},
		{
			name:     "admin sees all events",
			userID:   adminUser.ID.String(),
			userRole: "admin",
			request: &authv1.GetSecurityEventsRequest{
				PageSize: 10,
			},
			expectedCount: 8,
		},
		{
			name:     "filter by event type",
			userID:   adminUser.ID.String(),
			userRole: "admin",
			request: &authv1.GetSecurityEventsRequest{
				PageSize:  10,
				EventType: authv1.SecurityEventType_SECURITY_EVENT_TYPE_LOGIN_SUCCESS,
			},
			expectedCount: 5,
		},
		{
			name:     "pagination",
			userID:   adminUser.ID.String(),
			userRole: "admin",
			request: &authv1.GetSecurityEventsRequest{
				PageSize: 3,
			},
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = context.WithValue(ctx, middleware.ContextKeyUserID, tt.userID)
			ctx = context.WithValue(ctx, middleware.ContextKeyUserRole, tt.userRole)

			resp, err := authService.GetSecurityEvents(ctx, tt.request)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Len(t, resp.Events, tt.expectedCount)

			if tt.name == "pagination" {
				assert.NotEmpty(t, resp.NextPageToken)
			}

			// Verify total count
			if tt.name == "regular user sees only own events" {
				assert.Equal(t, int32(5), resp.TotalCount)
			} else if tt.name == "admin sees all events" {
				assert.Equal(t, int32(8), resp.TotalCount)
			}
		})
	}
}

func TestAuthService_UnlockAccount(t *testing.T) {
	// Setup
	client := setupTestDB(t)
	defer client.Close()

	// Create locked user
	lockedUser := createTestUser(t, client)
	lockTime := time.Now().Add(1 * time.Hour)
	lockedUser, err := lockedUser.Update().
		SetFailedLoginAttempts(5).
		SetAccountLockedUntil(lockTime).
		Save(context.Background())
	require.NoError(t, err)

	// Create admin user
	adminUser, err := client.User.Create().
		SetEmail("admin@example.com").
		SetUsername("admin").
		SetPasswordHash("hash").
		SetRole(user.RoleAdmin).
		SetIsActive(true).
		Save(context.Background())
	require.NoError(t, err)

	tokenManager := auth.NewTokenManager(
		"test-access-secret",
		"test-refresh-secret",
		15*time.Minute,
		7*24*time.Hour,
	)

	mockEmailService := email.NewMockEmailService()
	securityService := NewSecurityService(client)
	securityLogger := NewSecurityLogger(securityService)
	emailVerificationService := NewEmailVerificationService(client, mockEmailService, securityLogger)
	passwordResetService := NewPasswordResetService(client, mockEmailService, auth.NewPasswordManager(), securityLogger)

	authService := NewAuthService(
		client,
		tokenManager,
		emailVerificationService,
		passwordResetService,
		securityLogger,
		createTestSecurityConfig(),
	)

	tests := []struct {
		name         string
		userRole     string
		request      *authv1.UnlockAccountRequest
		wantErr      bool
		expectedCode codes.Code
	}{
		{
			name:     "admin can unlock account",
			userRole: "admin",
			request: &authv1.UnlockAccountRequest{
				UserId: lockedUser.ID.String(),
			},
			wantErr: false,
		},
		{
			name:     "non-admin cannot unlock",
			userRole: "user",
			request: &authv1.UnlockAccountRequest{
				UserId: lockedUser.ID.String(),
			},
			wantErr:      true,
			expectedCode: codes.PermissionDenied,
		},
		{
			name:     "invalid user ID",
			userRole: "admin",
			request: &authv1.UnlockAccountRequest{
				UserId: "invalid-uuid",
			},
			wantErr:      true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:     "non-existent user",
			userRole: "admin",
			request: &authv1.UnlockAccountRequest{
				UserId: uuid.New().String(),
			},
			wantErr:      true,
			expectedCode: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = context.WithValue(ctx, middleware.ContextKeyUserID, adminUser.ID.String())
			ctx = context.WithValue(ctx, middleware.ContextKeyUserRole, tt.userRole)

			_, err := authService.UnlockAccount(ctx, tt.request)

			if tt.wantErr {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			} else {
				require.NoError(t, err)

				// Verify account was unlocked
				unlockedUser, err := client.User.Get(ctx, lockedUser.ID)
				require.NoError(t, err)
				assert.Equal(t, 0, unlockedUser.FailedLoginAttempts)
				assert.Nil(t, unlockedUser.AccountLockedUntil)
			}
		})
	}
}
