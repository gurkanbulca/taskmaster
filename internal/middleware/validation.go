// internal/middleware/validation.go - Updated with Stream method
package middleware

import (
	"context"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	authv1 "github.com/gurkanbulca/taskmaster/api/proto/auth/v1/generated"
	taskv1 "github.com/gurkanbulca/taskmaster/api/proto/task/v1/generated"
)

// ValidationConfig holds validation configuration
type ValidationConfig struct {
	MinPasswordLength      int
	RequirePasswordUpper   bool
	RequirePasswordLower   bool
	RequirePasswordNumber  bool
	RequirePasswordSpecial bool
	MinUsernameLength      int
	MaxUsernameLength      int
	MaxEmailLength         int
	MaxNameLength          int
	MaxDescriptionLength   int
	MaxTitleLength         int
}

// DefaultValidationConfig returns default validation configuration
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		MinPasswordLength:      8,
		RequirePasswordUpper:   true,
		RequirePasswordLower:   true,
		RequirePasswordNumber:  true,
		RequirePasswordSpecial: false,
		MinUsernameLength:      3,
		MaxUsernameLength:      50,
		MaxEmailLength:         255,
		MaxNameLength:          100,
		MaxDescriptionLength:   5000,
		MaxTitleLength:         200,
	}
}

// EnhancedValidationInterceptor provides comprehensive request validation
type EnhancedValidationInterceptor struct {
	config *ValidationConfig
}

// NewEnhancedValidationInterceptor creates a new enhanced validation interceptor
func NewEnhancedValidationInterceptor(config *ValidationConfig) *EnhancedValidationInterceptor {
	if config == nil {
		config = DefaultValidationConfig()
	}
	return &EnhancedValidationInterceptor{
		config: config,
	}
}

// Unary returns a unary server interceptor for enhanced validation
func (v *EnhancedValidationInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Validate request based on method
		if err := v.validateRequest(req, info.FullMethod); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// Stream returns a stream server interceptor for enhanced validation
func (v *EnhancedValidationInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// For streaming endpoints, we typically validate the initial request
		// Since WatchTasks doesn't have complex validation needs, we just pass through
		// If you need to validate streaming requests, you would wrap the stream here

		return handler(srv, stream)
	}
}

// validateRequest validates different request types
func (v *EnhancedValidationInterceptor) validateRequest(req interface{}, method string) error {
	switch r := req.(type) {
	case *authv1.RegisterRequest:
		return v.validateRegisterRequest(r)
	case *authv1.LoginRequest:
		return v.validateLoginRequest(r)
	case *authv1.ChangePasswordRequest:
		return v.validateChangePasswordRequest(r)
	case *authv1.UpdateProfileRequest:
		return v.validateUpdateProfileRequest(r)
	case *authv1.RequestPasswordResetRequest:
		return v.validatePasswordResetRequest(r)
	case *authv1.ResetPasswordRequest:
		return v.validateResetPasswordRequest(r)
	case *authv1.VerifyEmailRequest:
		return v.validateVerifyEmailRequest(r)
	case *taskv1.CreateTaskRequest:
		return v.validateCreateTaskRequest(r)
	case *taskv1.UpdateTaskRequest:
		return v.validateUpdateTaskRequest(r)
	case *taskv1.GetTaskRequest:
		return v.validateGetTaskRequest(r)
	case *taskv1.DeleteTaskRequest:
		return v.validateDeleteTaskRequest(r)
	case *taskv1.ListTasksRequest:
		return v.validateListTasksRequest(r)
	}

	return nil
}

// Auth service validations

func (v *EnhancedValidationInterceptor) validateRegisterRequest(req *authv1.RegisterRequest) error {
	var errors []string

	// Email validation
	if err := v.validateEmail(req.Email); err != nil {
		errors = append(errors, fmt.Sprintf("email: %s", err.Error()))
	}

	// Username validation
	if err := v.validateUsername(req.Username); err != nil {
		errors = append(errors, fmt.Sprintf("username: %s", err.Error()))
	}

	// Password validation
	if err := v.validatePassword(req.Password); err != nil {
		errors = append(errors, fmt.Sprintf("password: %s", err.Error()))
	}

	// Name validation
	if req.FirstName != "" {
		if err := v.validateName(req.FirstName, "first_name"); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if req.LastName != "" {
		if err := v.validateName(req.LastName, "last_name"); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return status.Error(codes.InvalidArgument, strings.Join(errors, "; "))
	}

	return nil
}

func (v *EnhancedValidationInterceptor) validateLoginRequest(req *authv1.LoginRequest) error {
	var errors []string

	// Email/username validation
	if req.Email == "" {
		errors = append(errors, "email or username is required")
	} else if len(req.Email) > v.config.MaxEmailLength {
		errors = append(errors, fmt.Sprintf("email/username too long (max %d characters)", v.config.MaxEmailLength))
	}

	// Password validation
	if req.Password == "" {
		errors = append(errors, "password is required")
	}

	if len(errors) > 0 {
		return status.Error(codes.InvalidArgument, strings.Join(errors, "; "))
	}

	return nil
}

func (v *EnhancedValidationInterceptor) validateChangePasswordRequest(req *authv1.ChangePasswordRequest) error {
	var errors []string

	// Current password validation
	if req.CurrentPassword == "" {
		errors = append(errors, "current password is required")
	}

	// New password validation
	if err := v.validatePassword(req.NewPassword); err != nil {
		errors = append(errors, fmt.Sprintf("new password: %s", err.Error()))
	}

	// Ensure passwords are different
	if req.CurrentPassword == req.NewPassword {
		errors = append(errors, "new password must be different from current password")
	}

	if len(errors) > 0 {
		return status.Error(codes.InvalidArgument, strings.Join(errors, "; "))
	}

	return nil
}

func (v *EnhancedValidationInterceptor) validateUpdateProfileRequest(req *authv1.UpdateProfileRequest) error {
	var errors []string

	// Name validation
	if req.FirstName != "" {
		if err := v.validateName(req.FirstName, "first_name"); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if req.LastName != "" {
		if err := v.validateName(req.LastName, "last_name"); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Preferences validation
	if len(req.Preferences) > 50 {
		errors = append(errors, "too many preferences (max 50)")
	}

	for key, value := range req.Preferences {
		if len(key) > 100 {
			errors = append(errors, fmt.Sprintf("preference key '%s' too long (max 100 characters)", key))
		}
		if len(value) > 1000 {
			errors = append(errors, fmt.Sprintf("preference value for '%s' too long (max 1000 characters)", key))
		}
	}

	if len(errors) > 0 {
		return status.Error(codes.InvalidArgument, strings.Join(errors, "; "))
	}

	return nil
}

func (v *EnhancedValidationInterceptor) validatePasswordResetRequest(req *authv1.RequestPasswordResetRequest) error {
	if err := v.validateEmail(req.Email); err != nil {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("email: %s", err.Error()))
	}
	return nil
}

func (v *EnhancedValidationInterceptor) validateResetPasswordRequest(req *authv1.ResetPasswordRequest) error {
	var errors []string

	// Token validation
	if req.Token == "" {
		errors = append(errors, "reset token is required")
	} else if len(req.Token) < 32 || len(req.Token) > 128 {
		errors = append(errors, "invalid reset token format")
	}

	// Password validation
	if err := v.validatePassword(req.NewPassword); err != nil {
		errors = append(errors, fmt.Sprintf("new password: %s", err.Error()))
	}

	if len(errors) > 0 {
		return status.Error(codes.InvalidArgument, strings.Join(errors, "; "))
	}

	return nil
}

func (v *EnhancedValidationInterceptor) validateVerifyEmailRequest(req *authv1.VerifyEmailRequest) error {
	if req.Token == "" {
		return status.Error(codes.InvalidArgument, "verification token is required")
	}
	if len(req.Token) < 32 || len(req.Token) > 128 {
		return status.Error(codes.InvalidArgument, "invalid verification token format")
	}
	return nil
}

// Task service validations

func (v *EnhancedValidationInterceptor) validateCreateTaskRequest(req *taskv1.CreateTaskRequest) error {
	var errors []string

	// Title validation
	if req.Title == "" {
		errors = append(errors, "title is required")
	} else if len(req.Title) > v.config.MaxTitleLength {
		errors = append(errors, fmt.Sprintf("title too long (max %d characters)", v.config.MaxTitleLength))
	}

	// Description validation
	if len(req.Description) > v.config.MaxDescriptionLength {
		errors = append(errors, fmt.Sprintf("description too long (max %d characters)", v.config.MaxDescriptionLength))
	}

	// Priority validation
	if req.Priority == taskv1.Priority_PRIORITY_UNSPECIFIED {
		// Set default priority if not specified
		req.Priority = taskv1.Priority_PRIORITY_MEDIUM
	}

	// Tags validation
	if len(req.Tags) > 20 {
		errors = append(errors, "too many tags (max 20)")
	}

	for _, tag := range req.Tags {
		if len(tag) > 50 {
			errors = append(errors, "tag too long (max 50 characters)")
		}
		if strings.TrimSpace(tag) == "" {
			errors = append(errors, "empty tags are not allowed")
		}
	}

	// AssignedTo validation
	if req.AssignedTo != "" && len(req.AssignedTo) > v.config.MaxEmailLength {
		errors = append(errors, fmt.Sprintf("assigned_to too long (max %d characters)", v.config.MaxEmailLength))
	}

	if len(errors) > 0 {
		return status.Error(codes.InvalidArgument, strings.Join(errors, "; "))
	}

	return nil
}

func (v *EnhancedValidationInterceptor) validateUpdateTaskRequest(req *taskv1.UpdateTaskRequest) error {
	var errors []string

	// ID validation
	if req.Id == "" {
		errors = append(errors, "task ID is required")
	} else if !isValidUUID(req.Id) {
		errors = append(errors, "invalid task ID format")
	}

	// Title validation (if provided)
	if req.Title != "" && len(req.Title) > v.config.MaxTitleLength {
		errors = append(errors, fmt.Sprintf("title too long (max %d characters)", v.config.MaxTitleLength))
	}

	// Description validation (if provided)
	if len(req.Description) > v.config.MaxDescriptionLength {
		errors = append(errors, fmt.Sprintf("description too long (max %d characters)", v.config.MaxDescriptionLength))
	}

	// Tags validation (if provided)
	if len(req.Tags) > 20 {
		errors = append(errors, "too many tags (max 20)")
	}

	for _, tag := range req.Tags {
		if len(tag) > 50 {
			errors = append(errors, "tag too long (max 50 characters)")
		}
		if strings.TrimSpace(tag) == "" {
			errors = append(errors, "empty tags are not allowed")
		}
	}

	// AssignedTo validation (if provided)
	if req.AssignedTo != "" && len(req.AssignedTo) > v.config.MaxEmailLength {
		errors = append(errors, fmt.Sprintf("assigned_to too long (max %d characters)", v.config.MaxEmailLength))
	}

	if len(errors) > 0 {
		return status.Error(codes.InvalidArgument, strings.Join(errors, "; "))
	}

	return nil
}

func (v *EnhancedValidationInterceptor) validateGetTaskRequest(req *taskv1.GetTaskRequest) error {
	if req.Id == "" {
		return status.Error(codes.InvalidArgument, "task ID is required")
	}
	if !isValidUUID(req.Id) {
		return status.Error(codes.InvalidArgument, "invalid task ID format")
	}
	return nil
}

func (v *EnhancedValidationInterceptor) validateDeleteTaskRequest(req *taskv1.DeleteTaskRequest) error {
	if req.Id == "" {
		return status.Error(codes.InvalidArgument, "task ID is required")
	}
	if !isValidUUID(req.Id) {
		return status.Error(codes.InvalidArgument, "invalid task ID format")
	}
	return nil
}

func (v *EnhancedValidationInterceptor) validateListTasksRequest(req *taskv1.ListTasksRequest) error {
	if req.PageSize < 0 {
		return status.Error(codes.InvalidArgument, "page size cannot be negative")
	}
	if req.PageSize > 100 {
		return status.Error(codes.InvalidArgument, "page size cannot exceed 100")
	}
	if req.PageSize == 0 {
		req.PageSize = 10 // Set default
	}
	return nil
}

// Helper validation functions

func (v *EnhancedValidationInterceptor) validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}

	if len(email) > v.config.MaxEmailLength {
		return fmt.Errorf("email too long (max %d characters)", v.config.MaxEmailLength)
	}

	// Parse email to validate format
	_, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

func (v *EnhancedValidationInterceptor) validateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}

	if len(username) < v.config.MinUsernameLength {
		return fmt.Errorf("username too short (min %d characters)", v.config.MinUsernameLength)
	}

	if len(username) > v.config.MaxUsernameLength {
		return fmt.Errorf("username too long (max %d characters)", v.config.MaxUsernameLength)
	}

	// Username should only contain alphanumeric characters, underscores, and hyphens
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("username can only contain letters, numbers, underscores, and hyphens")
	}

	return nil
}

func (v *EnhancedValidationInterceptor) validatePassword(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}

	if len(password) < v.config.MinPasswordLength {
		return fmt.Errorf("password too short (min %d characters)", v.config.MinPasswordLength)
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	var requirements []string

	if v.config.RequirePasswordUpper && !hasUpper {
		requirements = append(requirements, "uppercase letter")
	}
	if v.config.RequirePasswordLower && !hasLower {
		requirements = append(requirements, "lowercase letter")
	}
	if v.config.RequirePasswordNumber && !hasNumber {
		requirements = append(requirements, "number")
	}
	if v.config.RequirePasswordSpecial && !hasSpecial {
		requirements = append(requirements, "special character")
	}

	if len(requirements) > 0 {
		return fmt.Errorf("password must contain at least one %s", strings.Join(requirements, ", "))
	}

	return nil
}

func (v *EnhancedValidationInterceptor) validateName(name, fieldName string) error {
	if len(name) > v.config.MaxNameLength {
		return fmt.Errorf("%s too long (max %d characters)", fieldName, v.config.MaxNameLength)
	}

	// Names should not contain special characters except spaces, hyphens, and apostrophes
	nameRegex := regexp.MustCompile(`^[a-zA-Z\s'-]+$`)
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("%s contains invalid characters", fieldName)
	}

	return nil
}

// isValidUUID checks if a string is a valid UUID format
func isValidUUID(s string) bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	return uuidRegex.MatchString(s)
}
