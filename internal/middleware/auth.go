// internal/middleware/auth.go
package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/gurkanbulca/taskmaster/pkg/auth"
)

// AuthInterceptor provides authentication middleware
type AuthInterceptor struct {
	tokenManager  *auth.TokenManager
	publicMethods map[string]bool
}

// NewAuthInterceptor creates a new auth interceptor
func NewAuthInterceptor(tokenManager *auth.TokenManager) *AuthInterceptor {
	// Define which methods don't require authentication
	publicMethods := map[string]bool{
		"/auth.v1.AuthService/Register":             true,
		"/auth.v1.AuthService/Login":                true,
		"/auth.v1.AuthService/RefreshToken":         true,
		"/auth.v1.AuthService/VerifyEmail":          true,
		"/auth.v1.AuthService/RequestPasswordReset": true,
		"/auth.v1.AuthService/ResetPassword":        true,
		"/grpc.health.v1.Health/Check":              true,
		"/grpc.health.v1.Health/Watch":              true,
	}

	return &AuthInterceptor{
		tokenManager:  tokenManager,
		publicMethods: publicMethods,
	}
}

// Unary returns a unary server interceptor for authentication
func (a *AuthInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Check if method requires authentication
		if a.publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		// Extract and validate token
		newCtx, err := a.authenticate(ctx)
		if err != nil {
			return nil, err
		}

		return handler(newCtx, req)
	}
}

// Stream returns a stream server interceptor for authentication
func (a *AuthInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Check if method requires authentication
		if a.publicMethods[info.FullMethod] {
			return handler(srv, stream)
		}

		// Extract and validate token
		newCtx, err := a.authenticate(stream.Context())
		if err != nil {
			return err
		}

		// Wrap the stream with authenticated context
		wrappedStream := &authenticatedServerStream{
			ServerStream: stream,
			ctx:          newCtx,
		}

		return handler(srv, wrappedStream)
	}
}

// authenticate extracts and validates the JWT token from metadata
func (a *AuthInterceptor) authenticate(ctx context.Context) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Extract authorization header
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// Extract token from header
	token, err := auth.ExtractTokenFromHeader(authHeaders[0])
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	// Validate token
	claims, err := a.tokenManager.ValidateAccessToken(token)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	// Add user information to context
	ctx = context.WithValue(ctx, "user_id", claims.UserID)
	ctx = context.WithValue(ctx, "user_email", claims.Email)
	ctx = context.WithValue(ctx, "user_username", claims.Username)
	ctx = context.WithValue(ctx, "user_role", claims.Role)

	return ctx, nil
}

// authenticatedServerStream wraps grpc.ServerStream with authenticated context
type authenticatedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedServerStream) Context() context.Context {
	return s.ctx
}

// GetUserIDFromContext extracts user ID from context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value("user_id").(string)
	return userID, ok
}

// GetUserRoleFromContext extracts user role from context
func GetUserRoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value("user_role").(string)
	return role, ok
}

// RequireRole creates an interceptor that requires specific roles
func RequireRole(roles ...string) grpc.UnaryServerInterceptor {
	roleMap := make(map[string]bool)
	for _, role := range roles {
		roleMap[role] = true
	}

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		userRole, ok := GetUserRoleFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "user not authenticated")
		}

		if !roleMap[userRole] {
			return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
		}

		return handler(ctx, req)
	}
}
