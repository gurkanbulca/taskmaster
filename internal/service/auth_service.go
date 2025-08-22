// internal/service/auth_service.go - Complete with Security Event Retrieval
package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/gurkanbulca/taskmaster/api/proto/auth/v1/generated"
	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/ent/generated/securityevent"
	"github.com/gurkanbulca/taskmaster/ent/generated/user"
	"github.com/gurkanbulca/taskmaster/internal/config"
	"github.com/gurkanbulca/taskmaster/internal/middleware"
	"github.com/gurkanbulca/taskmaster/pkg/auth"
	"github.com/gurkanbulca/taskmaster/pkg/security"
)

type AuthService struct {
	authv1.UnimplementedAuthServiceServer
	client                   *ent.Client
	tokenManager             *auth.TokenManager
	passwordManager          *auth.PasswordManager
	emailVerificationService *EmailVerificationService
	passwordResetService     *PasswordResetService
	securityLogger           *SecurityLogger
	securityService          *SecurityService // Add security service for event retrieval
	securityConfig           config.SecurityConfig
}

// NewAuthService creates a new authentication service with configurable security settings
func NewAuthService(
	client *ent.Client,
	tokenManager *auth.TokenManager,
	emailVerificationService *EmailVerificationService,
	passwordResetService *PasswordResetService,
	securityLogger *SecurityLogger,
	securityConfig config.SecurityConfig,
) *AuthService {
	return &AuthService{
		client:                   client,
		tokenManager:             tokenManager,
		passwordManager:          auth.NewPasswordManager(),
		emailVerificationService: emailVerificationService,
		passwordResetService:     passwordResetService,
		securityLogger:           securityLogger,
		securityService:          NewSecurityService(client), // Initialize security service
		securityConfig:           securityConfig,
	}
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.RegisterResponse, error) {
	// Validate request
	if err := s.validateRegisterRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Check if user already exists
	exists, err := s.client.User.Query().
		Where(
			user.Or(
				user.EmailEQ(strings.ToLower(req.Email)),
				user.UsernameEQ(strings.ToLower(req.Username)),
			),
		).
		Exist(ctx)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to check user existence")
	}

	if exists {
		return nil, status.Error(codes.AlreadyExists, "user with this email or username already exists")
	}

	// Hash password
	hashedPassword, err := s.passwordManager.HashPassword(req.Password)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Create user
	newUser, err := s.client.User.Create().
		SetEmail(strings.ToLower(req.Email)).
		SetUsername(strings.ToLower(req.Username)).
		SetPasswordHash(hashedPassword).
		SetFirstName(req.FirstName).
		SetLastName(req.LastName).
		SetRole(user.RoleUser).
		SetIsActive(true).
		SetEmailVerified(false).
		SetPasswordChangedAt(time.Now()).
		SetEmailNotificationsEnabled(true).
		SetSecurityNotificationsEnabled(s.securityConfig.EnableSecurityNotifications).
		Save(ctx)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create user")
	}

	// Generate tokens
	accessToken, refreshToken, expiresIn, err := s.tokenManager.GenerateTokenPair(
		newUser.ID.String(),
		newUser.Email,
		newUser.Username,
		string(newUser.Role),
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate tokens")
	}

	// Update user with refresh token
	_, err = newUser.Update().
		SetRefreshToken(refreshToken).
		SetRefreshTokenExpiresAt(time.Now().Add(7 * 24 * time.Hour)).
		Save(ctx)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to save refresh token")
	}

	// Send verification email if requested or required
	emailVerificationRequired := false
	if req.SendVerificationEmail || s.securityConfig.RequireEmailVerification {
		if err := s.emailVerificationService.SendVerificationEmail(ctx, newUser.ID.String()); err != nil {
			// Log error but don't fail registration
			log.Printf("Failed to send verification email: %v", err)
		} else {
			emailVerificationRequired = true
		}
	}

	return &authv1.RegisterResponse{
		User:                      s.convertUserToProto(newUser),
		AccessToken:               accessToken,
		RefreshToken:              refreshToken,
		ExpiresIn:                 expiresIn,
		EmailVerificationRequired: emailVerificationRequired,
	}, nil
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	// Validate request
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	// Get client info from context
	clientInfo := middleware.GetClientInfoFromContext(ctx)

	// Find user by email or username
	loginID := strings.ToLower(req.Email)
	foundUser, err := s.client.User.Query().
		Where(
			user.Or(
				user.EmailEQ(loginID),
				user.UsernameEQ(loginID),
			),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			// Log failed login attempt
			if err := s.securityLogger.LogLoginFailed(ctx, loginID, "user not found"); err != nil {
				// Log error but continue
			}
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return nil, status.Error(codes.Internal, "failed to find user")
	}

	// Check if account is locked
	if foundUser.AccountLockedUntil != nil && foundUser.AccountLockedUntil.After(time.Now()) {
		return &authv1.LoginResponse{
			AccountLocked: true,
			LockedUntil:   timestamppb.New(*foundUser.AccountLockedUntil),
		}, status.Error(codes.PermissionDenied, fmt.Sprintf("account is locked until %s", foundUser.AccountLockedUntil.Format(time.RFC3339)))
	}

	// Check if account is active
	if !foundUser.IsActive {
		return nil, status.Error(codes.PermissionDenied, "account is deactivated")
	}

	// Verify password
	if err := s.passwordManager.ComparePassword(foundUser.PasswordHash, req.Password); err != nil {
		// Increment failed login attempts
		failedAttempts := foundUser.FailedLoginAttempts + 1
		update := foundUser.Update().SetFailedLoginAttempts(failedAttempts)

		// Lock account if max attempts exceeded (using configurable value)
		if failedAttempts >= s.securityConfig.MaxLoginAttempts {
			lockUntil := time.Now().Add(s.securityConfig.AccountLockoutDuration)
			update = update.SetAccountLockedUntil(lockUntil)

			// Log account locked event
			if err := s.securityLogger.LogAccountLocked(ctx, foundUser.ID,
				fmt.Sprintf("max login attempts (%d) exceeded", s.securityConfig.MaxLoginAttempts)); err != nil {
				// Log error but continue
			}

			// Save the update
			if _, err := update.Save(ctx); err != nil {
				log.Printf("Failed to update failed login attempts: %v", err)
			}

			// Return specific error for account lockout
			return &authv1.LoginResponse{
					AccountLocked: true,
					LockedUntil:   timestamppb.New(lockUntil),
				}, status.Error(codes.PermissionDenied,
					fmt.Sprintf("account locked due to %d failed login attempts. Try again after %s",
						s.securityConfig.MaxLoginAttempts,
						s.securityConfig.AccountLockoutDuration))
		} else {
			// Not locked yet, just update failed attempts
			if _, err := update.Save(ctx); err != nil {
				log.Printf("Failed to update failed login attempts: %v", err)
			}
		}

		// Log failed login
		if err := s.securityLogger.LogLoginFailed(ctx, loginID,
			fmt.Sprintf("invalid password (attempt %d of %d)", failedAttempts, s.securityConfig.MaxLoginAttempts)); err != nil {
			// Log error but continue
		}

		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	// Generate tokens
	accessToken, refreshToken, expiresIn, err := s.tokenManager.GenerateTokenPair(
		foundUser.ID.String(),
		foundUser.Email,
		foundUser.Username,
		string(foundUser.Role),
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate tokens")
	}

	// Update user with refresh token, last login, and reset failed attempts
	foundUser, err = foundUser.Update().
		SetRefreshToken(refreshToken).
		SetRefreshTokenExpiresAt(time.Now().Add(7 * 24 * time.Hour)).
		SetLastLogin(time.Now()).
		SetLastLoginIP(clientInfo.IPAddress).
		SetFailedLoginAttempts(0). // Reset failed attempts on successful login
		ClearAccountLockedUntil(). // Clear any existing lock
		Save(ctx)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update user")
	}

	// Log successful login
	if err := s.securityLogger.LogLoginSuccess(ctx, foundUser.ID); err != nil {
		// Log error but don't fail login
	}

	// Check if email verification is required
	emailVerificationRequired := !foundUser.EmailVerified && s.securityConfig.RequireEmailVerification

	return &authv1.LoginResponse{
		User:                      s.convertUserToProto(foundUser),
		AccessToken:               accessToken,
		RefreshToken:              refreshToken,
		ExpiresIn:                 expiresIn,
		EmailVerificationRequired: emailVerificationRequired,
		AccountLocked:             false,
	}, nil
}

// RefreshToken generates a new access token using a refresh token
func (s *AuthService) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	// Validate refresh token
	claims, err := s.tokenManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	// Find user and verify refresh token matches
	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid user ID in token")
	}

	foundUser, err := s.client.User.Query().
		Where(
			user.And(
				user.ID(userUUID),
				user.RefreshTokenEQ(req.RefreshToken),
				user.IsActiveEQ(true),
			),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		}
		return nil, status.Error(codes.Internal, "failed to find user")
	}

	// Check if refresh token is expired
	if foundUser.RefreshTokenExpiresAt != nil && foundUser.RefreshTokenExpiresAt.Before(time.Now()) {
		return nil, status.Error(codes.Unauthenticated, "refresh token expired")
	}

	// Check if session has timed out (using configurable session timeout)
	if foundUser.LastLogin != nil && time.Since(*foundUser.LastLogin) > s.securityConfig.SessionTimeoutDuration {
		// Clear refresh token
		if err := s.client.User.UpdateOneID(userUUID).
			ClearRefreshToken().
			ClearRefreshTokenExpiresAt().
			Exec(ctx); err != nil {
			log.Printf("Failed to clear expired refresh token: %v", err)
		}
		return nil, status.Error(codes.Unauthenticated, "session has timed out, please login again")
	}

	// Generate new token pair
	accessToken, refreshToken, expiresIn, err := s.tokenManager.GenerateTokenPair(
		foundUser.ID.String(),
		foundUser.Email,
		foundUser.Username,
		string(foundUser.Role),
	)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate tokens")
	}

	// Update refresh token
	_, err = foundUser.Update().
		SetRefreshToken(refreshToken).
		SetRefreshTokenExpiresAt(time.Now().Add(7 * 24 * time.Hour)).
		Save(ctx)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update refresh token")
	}

	return &authv1.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

// Logout invalidates the user's refresh token
func (s *AuthService) Logout(ctx context.Context, req *authv1.LogoutRequest) (*emptypb.Empty, error) {
	if req.RefreshToken == "" {
		return &emptypb.Empty{}, nil
	}

	// Validate refresh token to get user ID
	claims, err := s.tokenManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		// Even if token is invalid, we return success for logout
		return &emptypb.Empty{}, nil
	}

	// Parse the user ID
	userUUID, err := uuid.Parse(claims.UserID)
	if err != nil {
		// Invalid UUID, but still return success
		return &emptypb.Empty{}, nil
	}

	// Clear refresh token from database
	err = s.client.User.UpdateOneID(userUUID).
		ClearRefreshToken().
		ClearRefreshTokenExpiresAt().
		Exec(ctx)

	if err != nil && !ent.IsNotFound(err) {
		// Log error but still return success for logout
		log.Printf("Failed to clear refresh token for user %s: %v", claims.UserID, err)
	}

	return &emptypb.Empty{}, nil
}

// GetMe returns the current authenticated user's information
func (s *AuthService) GetMe(ctx context.Context, _ *emptypb.Empty) (*authv1.GetMeResponse, error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}

	// Find user
	foundUser, err := s.client.User.Get(ctx, uuid.MustParse(userID))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	// Get email verification status
	verificationStatus, err := s.emailVerificationService.GetVerificationStatus(ctx, userID)
	if err != nil {
		// Log error but don't fail the request
		log.Printf("Failed to get email verification status: %v", err)
	}

	response := &authv1.GetMeResponse{
		User: s.convertUserToProto(foundUser),
	}

	if verificationStatus != nil {
		response.EmailVerificationStatus = &authv1.EmailVerificationStatus{
			EmailVerified: verificationStatus.EmailVerified,
			Attempts:      int32(verificationStatus.Attempts),
			MaxAttempts:   int32(verificationStatus.MaxAttempts),
			IsExpired:     verificationStatus.IsExpired,
			CanResend:     verificationStatus.CanResend,
		}
		if verificationStatus.ExpiresAt != nil {
			response.EmailVerificationStatus.ExpiresAt = timestamppb.New(*verificationStatus.ExpiresAt)
		}
	}

	return response, nil
}

// UpdateProfile updates the current user's profile
func (s *AuthService) UpdateProfile(ctx context.Context, req *authv1.UpdateProfileRequest) (*authv1.UpdateProfileResponse, error) {
	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}

	// Build update query
	update := s.client.User.UpdateOneID(uuid.MustParse(userID))

	if req.FirstName != "" {
		update = update.SetFirstName(req.FirstName)
	}
	if req.LastName != "" {
		update = update.SetLastName(req.LastName)
	}
	if len(req.Preferences) > 0 {
		// Convert map[string]string to map[string]interface{}
		preferences := make(map[string]interface{})
		for k, v := range req.Preferences {
			preferences[k] = v
		}
		update = update.SetPreferences(preferences)
	}

	// Phase 2: Update notification settings
	update = update.
		SetEmailNotificationsEnabled(req.EmailNotificationsEnabled).
		SetSecurityNotificationsEnabled(req.SecurityNotificationsEnabled)

	// Execute update
	updatedUser, err := update.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to update profile")
	}

	return &authv1.UpdateProfileResponse{
		User: s.convertUserToProto(updatedUser),
	}, nil
}

// ChangePassword changes the current user's password
func (s *AuthService) ChangePassword(ctx context.Context, req *authv1.ChangePasswordRequest) (*emptypb.Empty, error) {
	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}

	// Validate request
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "current and new passwords are required")
	}

	// Find user
	foundUser, err := s.client.User.Get(ctx, uuid.MustParse(userID))
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to get user")
	}

	// Verify current password
	if err := s.passwordManager.ComparePassword(foundUser.PasswordHash, req.CurrentPassword); err != nil {
		return nil, status.Error(codes.InvalidArgument, "incorrect current password")
	}

	// Hash new password
	hashedPassword, err := s.passwordManager.HashPassword(req.NewPassword)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Update password and clear refresh token
	_, err = foundUser.Update().
		SetPasswordHash(hashedPassword).
		SetPasswordChangedAt(time.Now()).
		ClearRefreshToken().
		ClearRefreshTokenExpiresAt().
		Save(ctx)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update password")
	}

	// Log password change
	if err := s.securityLogger.LogPasswordChanged(ctx, foundUser.ID); err != nil {
		// Log error but don't fail
	}

	// Send notification email if requested and enabled
	if req.NotifyViaEmail && foundUser.SecurityNotificationsEnabled {
		// This would send an email notification about password change
		// Implementation depends on email service
	}

	return &emptypb.Empty{}, nil
}

// Phase 2: Email Verification Methods

// SendVerificationEmail sends a verification email to the authenticated user
func (s *AuthService) SendVerificationEmail(ctx context.Context, _ *authv1.SendVerificationEmailRequest) (*emptypb.Empty, error) {
	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}

	if err := s.emailVerificationService.SendVerificationEmail(ctx, userID); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// VerifyEmail verifies a user's email address using a token
func (s *AuthService) VerifyEmail(ctx context.Context, req *authv1.VerifyEmailRequest) (*emptypb.Empty, error) {
	if err := s.emailVerificationService.VerifyEmail(ctx, req.Token); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// ResendVerificationEmail resends the verification email
func (s *AuthService) ResendVerificationEmail(ctx context.Context, _ *authv1.ResendVerificationEmailRequest) (*emptypb.Empty, error) {
	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}

	if err := s.emailVerificationService.ResendVerificationEmail(ctx, userID); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// GetVerificationStatus returns the email verification status
func (s *AuthService) GetVerificationStatus(ctx context.Context, _ *authv1.GetVerificationStatusRequest) (*authv1.GetVerificationStatusResponse, error) {
	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}

	status, err := s.emailVerificationService.GetVerificationStatus(ctx, userID)
	if err != nil {
		return nil, err
	}

	response := &authv1.GetVerificationStatusResponse{
		Status: &authv1.EmailVerificationStatus{
			EmailVerified: status.EmailVerified,
			Attempts:      int32(status.Attempts),
			MaxAttempts:   int32(status.MaxAttempts),
			IsExpired:     status.IsExpired,
			CanResend:     status.CanResend,
		},
	}

	if status.ExpiresAt != nil {
		response.Status.ExpiresAt = timestamppb.New(*status.ExpiresAt)
	}

	return response, nil
}

// Phase 2: Password Reset Methods

// RequestPasswordReset initiates a password reset process
func (s *AuthService) RequestPasswordReset(ctx context.Context, req *authv1.RequestPasswordResetRequest) (*emptypb.Empty, error) {
	if err := s.passwordResetService.RequestPasswordReset(ctx, req.Email); err != nil {
		// For security, we might want to return success even if the email doesn't exist
		// to avoid revealing whether an email is registered
		return &emptypb.Empty{}, nil
	}

	return &emptypb.Empty{}, nil
}

// VerifyPasswordResetToken verifies if a password reset token is valid
func (s *AuthService) VerifyPasswordResetToken(ctx context.Context, req *authv1.VerifyPasswordResetTokenRequest) (*authv1.VerifyPasswordResetTokenResponse, error) {
	tokenInfo, err := s.passwordResetService.VerifyPasswordResetToken(ctx, req.Token)
	if err != nil {
		return nil, err
	}

	response := &authv1.VerifyPasswordResetTokenResponse{
		IsValid: tokenInfo.IsValid,
		Email:   tokenInfo.Email,
	}

	if tokenInfo.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(*tokenInfo.ExpiresAt)
	}

	return response, nil
}

// ResetPassword resets a user's password using a reset token
func (s *AuthService) ResetPassword(ctx context.Context, req *authv1.ResetPasswordRequest) (*emptypb.Empty, error) {
	if err := s.passwordResetService.ResetPassword(ctx, req.Token, req.NewPassword); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

// Phase 2: Security Methods - COMPLETE IMPLEMENTATION

// GetSecurityEvents returns security events for the authenticated user
func (s *AuthService) GetSecurityEvents(ctx context.Context, req *authv1.GetSecurityEventsRequest) (*authv1.GetSecurityEventsResponse, error) {
	// Get user ID from context
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user not authenticated")
	}

	// Get user role to check if they're admin
	userRole, _ := middleware.GetUserRoleFromContext(ctx)

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Build query
	query := s.client.SecurityEvent.Query()

	// If not admin, only show their own events
	if userRole != "admin" {
		query = query.Where(securityevent.UserIDEQ(userUUID))
	}

	// Apply filters
	if req.EventType != authv1.SecurityEventType_SECURITY_EVENT_TYPE_UNSPECIFIED {
		eventType := convertProtoEventTypeToString(req.EventType)
		if entEventType, err := security.ParseEventType(eventType); err == nil {
			query = query.Where(securityevent.EventTypeEQ(entEventType))
		}
	}

	// Apply date filters
	if req.FromDate != nil {
		query = query.Where(securityevent.CreatedAtGTE(req.FromDate.AsTime()))
	}
	if req.ToDate != nil {
		query = query.Where(securityevent.CreatedAtLTE(req.ToDate.AsTime()))
	}

	// Get total count
	totalCount, err := query.Count(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to count security events")
	}

	// Apply pagination
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// TODO: Implement proper pagination with page tokens
	// For now, use simple offset-based pagination
	offset := 0
	if req.PageToken != "" {
		// Parse page token (simplified - in production, use proper token encoding)
		fmt.Sscanf(req.PageToken, "offset:%d", &offset)
	}

	query = query.
		Limit(int(pageSize)).
		Offset(offset).
		Order(ent.Desc(securityevent.FieldCreatedAt))

	// Execute query
	events, err := query.All(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get security events")
	}

	// Convert to proto
	protoEvents := make([]*authv1.SecurityEvent, len(events))
	for i, event := range events {
		protoEvents[i] = s.convertSecurityEventToProto(event)
	}

	// Create next page token
	nextPageToken := ""
	if len(events) == int(pageSize) && offset+int(pageSize) < totalCount {
		nextPageToken = fmt.Sprintf("offset:%d", offset+int(pageSize))
	}

	return &authv1.GetSecurityEventsResponse{
		Events:        protoEvents,
		NextPageToken: nextPageToken,
		TotalCount:    int32(totalCount),
	}, nil
}

// UnlockAccount unlocks a user's account (admin only)
func (s *AuthService) UnlockAccount(ctx context.Context, req *authv1.UnlockAccountRequest) (*emptypb.Empty, error) {
	// Check if user is admin
	userRole, ok := middleware.GetUserRoleFromContext(ctx)
	if !ok || userRole != "admin" {
		return nil, status.Error(codes.PermissionDenied, "admin access required")
	}

	userUUID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Unlock the account
	err = s.client.User.UpdateOneID(userUUID).
		SetFailedLoginAttempts(0).
		ClearAccountLockedUntil().
		Exec(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to unlock account")
	}

	// Log the unlock event
	if err := s.securityLogger.LogFromContext(ctx, userUUID, security.EventTypeAccountUnlocked,
		"Account unlocked by admin", security.SeverityLow); err != nil {
		// Log error but don't fail
	}

	return &emptypb.Empty{}, nil
}

// Helper functions

func (s *AuthService) validateRegisterRequest(req *authv1.RegisterRequest) error {
	if err := auth.ValidateEmail(req.Email); err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}

	if err := auth.ValidateUsername(req.Username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}

	if req.Password == "" {
		return errors.New("password is required")
	}

	return nil
}

func (s *AuthService) convertUserToProto(u *ent.User) *authv1.User {
	proto := &authv1.User{
		Id:                           u.ID.String(),
		Email:                        u.Email,
		Username:                     u.Username,
		FirstName:                    u.FirstName,
		LastName:                     u.LastName,
		Role:                         convertRoleToProto(u.Role),
		IsActive:                     u.IsActive,
		EmailVerified:                u.EmailVerified,
		EmailNotificationsEnabled:    u.EmailNotificationsEnabled,
		SecurityNotificationsEnabled: u.SecurityNotificationsEnabled,
		FailedLoginAttempts:          int32(u.FailedLoginAttempts),
		CreatedAt:                    timestamppb.New(u.CreatedAt),
		UpdatedAt:                    timestamppb.New(u.UpdatedAt),
	}

	if u.LastLogin != nil {
		proto.LastLogin = timestamppb.New(*u.LastLogin)
	}

	if u.AccountLockedUntil != nil {
		proto.AccountLockedUntil = timestamppb.New(*u.AccountLockedUntil)
	}

	if u.PasswordChangedAt != nil {
		proto.PasswordChangedAt = timestamppb.New(*u.PasswordChangedAt)
	}

	return proto
}

func (s *AuthService) convertSecurityEventToProto(event *ent.SecurityEvent) *authv1.SecurityEvent {
	proto := &authv1.SecurityEvent{
		Id:          event.ID.String(),
		EventType:   convertStringEventTypeToProto(string(event.EventType)),
		Description: event.Description,
		IpAddress:   event.IPAddress,
		UserAgent:   event.UserAgent,
		Severity:    convertStringSeverityToProto(string(event.Severity)),
		Resolved:    event.Resolved,
		CreatedAt:   timestamppb.New(event.CreatedAt),
		Metadata:    make(map[string]string),
	}

	// Convert metadata from map[string]interface{} to map[string]string
	if event.Metadata != nil {
		for k, v := range event.Metadata {
			proto.Metadata[k] = fmt.Sprintf("%v", v)
		}
	}

	return proto
}

func convertRoleToProto(role user.Role) authv1.UserRole {
	switch role {
	case user.RoleAdmin:
		return authv1.UserRole_USER_ROLE_ADMIN
	case user.RoleManager:
		return authv1.UserRole_USER_ROLE_MANAGER
	case user.RoleUser:
		return authv1.UserRole_USER_ROLE_USER
	default:
		return authv1.UserRole_USER_ROLE_UNSPECIFIED
	}
}

func convertStringEventTypeToProto(eventType string) authv1.SecurityEventType {
	switch eventType {
	case "login_success":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_LOGIN_SUCCESS
	case "login_failed":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_LOGIN_FAILED
	case "password_changed":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_PASSWORD_CHANGED
	case "password_reset_requested":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_PASSWORD_RESET_REQUESTED
	case "password_reset_completed":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_PASSWORD_RESET_COMPLETED
	case "email_verification_sent":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_EMAIL_VERIFICATION_SENT
	case "email_verification_completed":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_EMAIL_VERIFICATION_COMPLETED
	case "account_locked":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_ACCOUNT_LOCKED
	case "account_unlocked":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_ACCOUNT_UNLOCKED
	case "security_alert":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_SECURITY_ALERT
	case "suspicious_activity":
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_SUSPICIOUS_ACTIVITY
	default:
		return authv1.SecurityEventType_SECURITY_EVENT_TYPE_UNSPECIFIED
	}
}

func convertProtoEventTypeToString(eventType authv1.SecurityEventType) string {
	switch eventType {
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_LOGIN_SUCCESS:
		return security.EventTypeLoginSuccess
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_LOGIN_FAILED:
		return security.EventTypeLoginFailed
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_PASSWORD_CHANGED:
		return security.EventTypePasswordChanged
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_PASSWORD_RESET_REQUESTED:
		return security.EventTypePasswordResetRequested
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_PASSWORD_RESET_COMPLETED:
		return security.EventTypePasswordResetCompleted
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_EMAIL_VERIFICATION_SENT:
		return security.EventTypeEmailVerificationSent
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_EMAIL_VERIFICATION_COMPLETED:
		return security.EventTypeEmailVerificationCompleted
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_ACCOUNT_LOCKED:
		return security.EventTypeAccountLocked
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_ACCOUNT_UNLOCKED:
		return security.EventTypeAccountUnlocked
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_SECURITY_ALERT:
		return security.EventTypeSecurityAlert
	case authv1.SecurityEventType_SECURITY_EVENT_TYPE_SUSPICIOUS_ACTIVITY:
		return security.EventTypeSuspiciousActivity
	default:
		return ""
	}
}

func convertStringSeverityToProto(severity string) authv1.SecurityEventSeverity {
	switch severity {
	case "low":
		return authv1.SecurityEventSeverity_SECURITY_EVENT_SEVERITY_LOW
	case "medium":
		return authv1.SecurityEventSeverity_SECURITY_EVENT_SEVERITY_MEDIUM
	case "high":
		return authv1.SecurityEventSeverity_SECURITY_EVENT_SEVERITY_HIGH
	case "critical":
		return authv1.SecurityEventSeverity_SECURITY_EVENT_SEVERITY_CRITICAL
	default:
		return authv1.SecurityEventSeverity_SECURITY_EVENT_SEVERITY_UNSPECIFIED
	}
}
