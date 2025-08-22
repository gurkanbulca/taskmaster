// internal/middleware/context_extractor.go
package middleware

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// ContextKeys for storing request metadata
type ContextKey string

const (
	ContextKeyIPAddress ContextKey = "ip_address"
	ContextKeyUserAgent ContextKey = "user_agent"
	ContextKeyUserID    ContextKey = "user_id"
	ContextKeyUserEmail ContextKey = "user_email"
	ContextKeyUserRole  ContextKey = "user_role"
)

// MetadataExtractorInterceptor extracts client metadata and adds it to context
type MetadataExtractorInterceptor struct{}

// NewMetadataExtractorInterceptor creates a new metadata extractor interceptor
func NewMetadataExtractorInterceptor() *MetadataExtractorInterceptor {
	return &MetadataExtractorInterceptor{}
}

// Unary returns a unary server interceptor for metadata extraction
func (m *MetadataExtractorInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract metadata and add to context
		enrichedCtx := m.enrichContext(ctx)
		return handler(enrichedCtx, req)
	}
}

// Stream returns a stream server interceptor for metadata extraction
func (m *MetadataExtractorInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Extract metadata and add to context
		enrichedCtx := m.enrichContext(stream.Context())

		// Wrap the stream with enriched context
		wrappedStream := &enrichedServerStream{
			ServerStream: stream,
			ctx:          enrichedCtx,
		}

		return handler(srv, wrappedStream)
	}
}

// enrichContext extracts IP address and user agent from the context
func (m *MetadataExtractorInterceptor) enrichContext(ctx context.Context) context.Context {
	// Extract IP address from peer info
	ipAddress := extractIPAddress(ctx)
	if ipAddress != "" {
		ctx = context.WithValue(ctx, ContextKeyIPAddress, ipAddress)
	}

	// Extract user agent from metadata
	userAgent := extractUserAgent(ctx)
	if userAgent != "" {
		ctx = context.WithValue(ctx, ContextKeyUserAgent, userAgent)
	}

	return ctx
}

// extractIPAddress extracts the client IP address from the context
func extractIPAddress(ctx context.Context) string {
	// Get peer information
	p, ok := peer.FromContext(ctx)
	if !ok {
		return ""
	}

	// Extract IP from the address
	addr := p.Addr.String()

	// Handle different address formats
	if tcpAddr, ok := p.Addr.(*net.TCPAddr); ok {
		return tcpAddr.IP.String()
	}

	// Fallback: try to parse the address string
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr // Return as-is if parsing fails
	}

	return host
}

// extractUserAgent extracts the user agent from gRPC metadata
func extractUserAgent(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// Check common user agent headers
	userAgentHeaders := []string{
		"user-agent",
		"grpc-user-agent",
		"x-user-agent",
	}

	for _, header := range userAgentHeaders {
		if values := md.Get(header); len(values) > 0 {
			return values[0]
		}
	}

	return ""
}

// enrichedServerStream wraps grpc.ServerStream with enriched context
type enrichedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *enrichedServerStream) Context() context.Context {
	return s.ctx
}

// Helper functions to extract values from context

// GetIPAddressFromContext extracts IP address from context
func GetIPAddressFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(ContextKeyIPAddress).(string); ok {
		return ip
	}
	return ""
}

// GetUserAgentFromContext extracts user agent from context
func GetUserAgentFromContext(ctx context.Context) string {
	if ua, ok := ctx.Value(ContextKeyUserAgent).(string); ok {
		return ua
	}
	return ""
}

// GetUserIDFromContext extracts user ID from context (updated to use new key)
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	if userID, ok := ctx.Value(ContextKeyUserID).(string); ok {
		return userID, true
	}
	// Fallback to old method for backward compatibility
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID, true
	}
	return "", false
}

// GetUserRoleFromContext extracts user role from context (updated to use new key)
func GetUserRoleFromContext(ctx context.Context) (string, bool) {
	if role, ok := ctx.Value(ContextKeyUserRole).(string); ok {
		return role, true
	}
	// Fallback to old method for backward compatibility
	if role, ok := ctx.Value("user_role").(string); ok {
		return role, true
	}
	return "", false
}

// GetUserEmailFromContext extracts user email from context
func GetUserEmailFromContext(ctx context.Context) (string, bool) {
	if email, ok := ctx.Value(ContextKeyUserEmail).(string); ok {
		return email, true
	}
	// Fallback to old method for backward compatibility
	if email, ok := ctx.Value("user_email").(string); ok {
		return email, true
	}
	return "", false
}

// GetClientInfo returns a struct with all client information
type ClientInfo struct {
	IPAddress string
	UserAgent string
	UserID    string
	UserEmail string
	UserRole  string
}

// GetClientInfoFromContext extracts all client information from context
func GetClientInfoFromContext(ctx context.Context) *ClientInfo {
	info := &ClientInfo{
		IPAddress: GetIPAddressFromContext(ctx),
		UserAgent: GetUserAgentFromContext(ctx),
	}

	if userID, ok := GetUserIDFromContext(ctx); ok {
		info.UserID = userID
	}

	if email, ok := GetUserEmailFromContext(ctx); ok {
		info.UserEmail = email
	}

	if role, ok := GetUserRoleFromContext(ctx); ok {
		info.UserRole = role
	}

	return info
}
