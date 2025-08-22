// internal/service/auth_service.go
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
	"github.com/gurkanbulca/taskmaster/ent/generated/user"
	"github.com/gurkanbulca/taskmaster/pkg/auth"
)

type AuthService struct {
	authv1.UnimplementedAuthServiceServer
	client          *ent.Client
	tokenManager    *auth.TokenManager
	passwordManager *auth.PasswordManager
}

// NewAuthService creates a new authentication service
func NewAuthService(client *ent.Client, tokenManager *auth.TokenManager) *AuthService {
	return &AuthService{
		client:          client,
		tokenManager:    tokenManager,
		passwordManager: auth.NewPasswordManager(),
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

	return &authv1.RegisterResponse{
		User:         s.convertUserToProto(newUser),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	// Validate request
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	// Find user by email or username
	loginID := strings.ToLower(req.Email)
	foundUser, err := s.client.User.Query().
		Where(
			user.And(
				user.Or(
					user.EmailEQ(loginID),
					user.UsernameEQ(loginID),
				),
				user.IsActiveEQ(true),
			),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return nil, status.Error(codes.Internal, "failed to find user")
	}

	// Verify password
	if err := s.passwordManager.ComparePassword(foundUser.PasswordHash, req.Password); err != nil {
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

	// Update user with refresh token and last login
	foundUser, err = foundUser.Update().
		SetRefreshToken(refreshToken).
		SetRefreshTokenExpiresAt(time.Now().Add(7 * 24 * time.Hour)).
		SetLastLogin(time.Now()).
		Save(ctx)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update user")
	}

	return &authv1.LoginResponse{
		User:         s.convertUserToProto(foundUser),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
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
	userID, ok := ctx.Value("user_id").(string)
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

	return &authv1.GetMeResponse{
		User: s.convertUserToProto(foundUser),
	}, nil
}

// UpdateProfile updates the current user's profile
func (s *AuthService) UpdateProfile(ctx context.Context, req *authv1.UpdateProfileRequest) (*authv1.UpdateProfileResponse, error) {
	// Get user ID from context
	userID, ok := ctx.Value("user_id").(string)
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
	userID, ok := ctx.Value("user_id").(string)
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
		ClearRefreshToken().
		ClearRefreshTokenExpiresAt().
		Save(ctx)

	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update password")
	}

	return &emptypb.Empty{}, nil
}

// VerifyEmail verifies a user's email address
func (s *AuthService) VerifyEmail(ctx context.Context, req *authv1.VerifyEmailRequest) (*emptypb.Empty, error) {
	// TODO: Implement email verification with token
	// This would typically involve:
	// 1. Generating a verification token when user registers
	// 2. Sending an email with the token
	// 3. Verifying the token here and updating email_verified field

	return nil, status.Error(codes.Unimplemented, "email verification not implemented")
}

// RequestPasswordReset initiates a password reset process
func (s *AuthService) RequestPasswordReset(ctx context.Context, req *authv1.RequestPasswordResetRequest) (*emptypb.Empty, error) {
	// TODO: Implement password reset request
	// This would typically involve:
	// 1. Finding user by email
	// 2. Generating a reset token
	// 3. Sending an email with the reset link

	return nil, status.Error(codes.Unimplemented, "password reset request not implemented")
}

// ResetPassword resets a user's password using a reset token
func (s *AuthService) ResetPassword(ctx context.Context, req *authv1.ResetPasswordRequest) (*emptypb.Empty, error) {
	// TODO: Implement password reset
	// This would typically involve:
	// 1. Validating the reset token
	// 2. Updating the user's password
	// 3. Invalidating the reset token

	return nil, status.Error(codes.Unimplemented, "password reset not implemented")
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
		Id:            u.ID.String(),
		Email:         u.Email,
		Username:      u.Username,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Role:          convertRoleToProto(u.Role),
		IsActive:      u.IsActive,
		EmailVerified: u.EmailVerified,
		CreatedAt:     timestamppb.New(u.CreatedAt),
		UpdatedAt:     timestamppb.New(u.UpdatedAt),
	}

	if u.LastLogin != nil {
		proto.LastLogin = timestamppb.New(*u.LastLogin)
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
