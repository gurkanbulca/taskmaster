// internal/middleware/validation.go
package middleware

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Validator interface that request messages can implement
type Validator interface {
	Validate() error
}

// ValidationInterceptor provides request validation middleware
type ValidationInterceptor struct{}

// NewValidationInterceptor creates a new validation interceptor
func NewValidationInterceptor() *ValidationInterceptor {
	return &ValidationInterceptor{}
}

// Unary returns a unary server interceptor for validation
func (v *ValidationInterceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Check if request implements Validator interface
		if validator, ok := req.(Validator); ok {
			if err := validator.Validate(); err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		}

		return handler(ctx, req)
	}
}

// Stream returns a stream server interceptor for validation
func (v *ValidationInterceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Wrap the stream to validate incoming messages
		wrappedStream := &validatingServerStream{
			ServerStream: stream,
		}

		return handler(srv, wrappedStream)
	}
}

// validatingServerStream wraps grpc.ServerStream to validate messages
type validatingServerStream struct {
	grpc.ServerStream
}

func (s *validatingServerStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	// Check if message implements Validator interface
	if validator, ok := m.(Validator); ok {
		if err := validator.Validate(); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
	}

	return nil
}

// Common validation functions

// ValidatePageSize validates pagination page size
func ValidatePageSize(pageSize int32) error {
	if pageSize < 0 {
		return fmt.Errorf("page size cannot be negative")
	}
	if pageSize > 100 {
		return fmt.Errorf("page size cannot exceed 100")
	}
	return nil
}

// ValidateUUID validates a UUID string
func ValidateUUID(id string) error {
	if id == "" {
		return fmt.Errorf("ID is required")
	}
	// Simple UUID format check (can be enhanced)
	if len(id) != 36 {
		return fmt.Errorf("invalid ID format")
	}
	return nil
}

// ValidateRequired validates that a string field is not empty
func ValidateRequired(field, fieldName string) error {
	if field == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

// ValidateMaxLength validates maximum string length
func ValidateMaxLength(field string, maxLength int, fieldName string) error {
	if len(field) > maxLength {
		return fmt.Errorf("%s cannot exceed %d characters", fieldName, maxLength)
	}
	return nil
}

// ValidateMinLength validates minimum string length
func ValidateMinLength(field string, minLength int, fieldName string) error {
	if len(field) < minLength {
		return fmt.Errorf("%s must be at least %d characters", fieldName, minLength)
	}
	return nil
}
