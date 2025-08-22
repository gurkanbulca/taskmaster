// internal/middleware/auth.go - Updated with Stream method
package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/gurkanbulca/taskmaster/pkg/auth"
)

// UpdatedAuthInterceptor provides authentication middleware with metadata extraction
type UpdatedAuthInterceptor struct {
	tokenManager  *auth.TokenManager
	publicMethods map[string]bool
}

// NewUpdatedAuthInterceptor creates a new auth interceptor
func NewUpdatedAuthInterceptor(tokenManager *auth.TokenManager) *UpdatedAuthInterceptor {
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

	return &UpdatedAuthInterceptor{
		tokenManager:  tokenManager,
		publicMethods: publicMethods,
	}
}

// Unary returns a unary server interceptor for authentication
func (a *UpdatedAuthInterceptor) Unary() grpc.UnaryServerInterceptor {
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
func (a *UpdatedAuthInterceptor) Stream() grpc.StreamServerInterceptor {
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
func (a *UpdatedAuthInterceptor) authenticate(ctx context.Context) (context.Context, error) {
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

	// Add user information to context using new context keys
	ctx = context.WithValue(ctx, ContextKeyUserID, claims.UserID)
	ctx = context.WithValue(ctx, ContextKeyUserEmail, claims.Email)
	ctx = context.WithValue(ctx, ContextKeyUserRole, claims.Role)

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
