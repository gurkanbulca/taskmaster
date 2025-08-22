// internal/service/security_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/ent/generated/securityevent"
	"github.com/gurkanbulca/taskmaster/pkg/security"
)

// SecurityService handles security event logging and management
type SecurityService struct {
	client *ent.Client
}

// NewSecurityService creates a new security service
func NewSecurityService(client *ent.Client) *SecurityService {
	return &SecurityService{
		client: client,
	}
}

// LogSecurityEvent logs a security event with proper type conversion
func (s *SecurityService) LogSecurityEvent(ctx context.Context, req *LogSecurityEventRequest) error {
	// Parse event type
	eventType, err := security.ParseEventType(req.EventType)
	if err != nil {
		return fmt.Errorf("invalid event type: %w", err)
	}

	// Parse severity
	severity, err := security.ParseSeverity(req.Severity)
	if err != nil {
		return fmt.Errorf("invalid severity: %w", err)
	}

	// Create security event
	create := s.client.SecurityEvent.Create().
		SetEventType(eventType).
		SetSeverity(severity)

	// Set user ID if provided
	if req.UserID != uuid.Nil {
		create = create.SetUserID(req.UserID)
	}

	// Set optional fields
	if req.Description != "" {
		create = create.SetDescription(req.Description)
	}
	if req.IPAddress != "" {
		create = create.SetIPAddress(req.IPAddress)
	}
	if req.UserAgent != "" {
		create = create.SetUserAgent(req.UserAgent)
	}
	if len(req.Metadata) > 0 {
		create = create.SetMetadata(req.Metadata)
	}

	_, err = create.Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to save security event: %w", err)
	}

	return nil
}

// LogUserSecurityEvent is a convenience method for logging user-specific events
func (s *SecurityService) LogUserSecurityEvent(ctx context.Context, userID uuid.UUID, eventType, description, severity, ipAddress, userAgent string) error {
	req := &LogSecurityEventRequest{
		UserID:      userID,
		EventType:   eventType,
		Description: description,
		Severity:    severity,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
	}
	return s.LogSecurityEvent(ctx, req)
}

// LogSystemSecurityEvent is a convenience method for logging system-wide events
func (s *SecurityService) LogSystemSecurityEvent(ctx context.Context, eventType, description, severity, ipAddress, userAgent string) error {
	req := &LogSecurityEventRequest{
		UserID:      uuid.Nil, // No specific user
		EventType:   eventType,
		Description: description,
		Severity:    severity,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
	}
	return s.LogSecurityEvent(ctx, req)
}

// GetSecurityEvents retrieves security events with filtering
func (s *SecurityService) GetSecurityEvents(ctx context.Context, req *GetSecurityEventsRequest) (*GetSecurityEventsResponse, error) {
	query := s.client.SecurityEvent.Query().
		WithUser() // Include user information

	// Apply filters
	if req.UserID != uuid.Nil {
		query = query.Where(securityevent.UserIDEQ(req.UserID))
	}

	if req.EventType != "" {
		eventType, err := security.ParseEventType(req.EventType)
		if err != nil {
			return nil, fmt.Errorf("invalid event type filter: %w", err)
		}
		query = query.Where(securityevent.EventTypeEQ(eventType))
	}

	if req.Severity != "" {
		severity, err := security.ParseSeverity(req.Severity)
		if err != nil {
			return nil, fmt.Errorf("invalid severity filter: %w", err)
		}
		query = query.Where(securityevent.SeverityEQ(severity))
	}

	if !req.FromDate.IsZero() {
		query = query.Where(securityevent.CreatedAtGTE(req.FromDate))
	}

	if !req.ToDate.IsZero() {
		query = query.Where(securityevent.CreatedAtLTE(req.ToDate))
	}

	if req.OnlyUnresolved {
		query = query.Where(securityevent.ResolvedEQ(false))
	}

	// Get total count
	totalCount, err := query.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count security events: %w", err)
	}

	// Apply pagination
	if req.Limit > 0 {
		query = query.Limit(req.Limit)
	}
	if req.Offset > 0 {
		query = query.Offset(req.Offset)
	}

	// Order by creation date (newest first)
	query = query.Order(ent.Desc(securityevent.FieldCreatedAt))

	// Execute query
	events, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get security events: %w", err)
	}

	return &GetSecurityEventsResponse{
		Events:     events,
		TotalCount: totalCount,
	}, nil
}

// ResolveSecurityEvent marks a security event as resolved
func (s *SecurityService) ResolveSecurityEvent(ctx context.Context, eventID uuid.UUID) error {
	_, err := s.client.SecurityEvent.UpdateOneID(eventID).
		SetResolved(true).
		Save(ctx)

	if err != nil {
		return fmt.Errorf("failed to resolve security event: %w", err)
	}

	return nil
}

// GetSecurityStats returns security statistics
func (s *SecurityService) GetSecurityStats(ctx context.Context, userID *uuid.UUID) (*SecurityStats, error) {
	query := s.client.SecurityEvent.Query()

	if userID != nil {
		query = query.Where(securityevent.UserIDEQ(*userID))
	}

	// Get total events
	totalEvents, err := query.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count total events: %w", err)
	}

	// Get unresolved events
	unresolvedEvents, err := query.Where(securityevent.ResolvedEQ(false)).Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count unresolved events: %w", err)
	}

	// Get high/critical severity events
	highSeverityEvents, err := query.Where(
		securityevent.SeverityIn(
			securityevent.SeverityHigh,
			securityevent.SeverityCritical,
		),
	).Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count high severity events: %w", err)
	}

	return &SecurityStats{
		TotalEvents:        totalEvents,
		UnresolvedEvents:   unresolvedEvents,
		HighSeverityEvents: highSeverityEvents,
	}, nil
}

// Request/Response types

// LogSecurityEventRequest represents a request to log a security event
type LogSecurityEventRequest struct {
	UserID      uuid.UUID              `json:"user_id"`
	EventType   string                 `json:"event_type"`
	Description string                 `json:"description"`
	Severity    string                 `json:"severity"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// GetSecurityEventsRequest represents a request to get security events
type GetSecurityEventsRequest struct {
	UserID         uuid.UUID `json:"user_id,omitempty"`
	EventType      string    `json:"event_type,omitempty"`
	Severity       string    `json:"severity,omitempty"`
	FromDate       time.Time `json:"from_date,omitempty"`
	ToDate         time.Time `json:"to_date,omitempty"`
	OnlyUnresolved bool      `json:"only_unresolved"`
	Limit          int       `json:"limit"`
	Offset         int       `json:"offset"`
}

// GetSecurityEventsResponse represents the response from getting security events
type GetSecurityEventsResponse struct {
	Events     []*ent.SecurityEvent `json:"events"`
	TotalCount int                  `json:"total_count"`
}

// SecurityStats represents security statistics
type SecurityStats struct {
	TotalEvents        int `json:"total_events"`
	UnresolvedEvents   int `json:"unresolved_events"`
	HighSeverityEvents int `json:"high_severity_events"`
}
