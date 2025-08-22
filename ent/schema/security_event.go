package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// SecurityEvent holds the schema definition for security events
type SecurityEvent struct {
	ent.Schema
}

// Fields of the SecurityEvent.
func (SecurityEvent) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.UUID("user_id", uuid.UUID{}).
			Comment("User who triggered the event"),

		field.Enum("event_type").
			Values(
				"login_success",
				"login_failed",
				"password_changed",
				"password_reset_requested",
				"password_reset_completed",
				"email_verification_sent",
				"email_verification_completed",
				"account_locked",
				"account_unlocked",
				"security_alert",
				"suspicious_activity",
			).
			Comment("Type of security event"),

		field.String("ip_address").
			Optional().
			Comment("IP address where event occurred"),

		field.String("user_agent").
			Optional().
			Comment("User agent string"),

		field.String("description").
			Optional().
			Comment("Human-readable description of the event"),

		field.JSON("metadata", map[string]interface{}{}).
			Optional().
			Default(map[string]interface{}{}).
			Comment("Additional event metadata"),

		field.Enum("severity").
			Values("low", "medium", "high", "critical").
			Default("low").
			Comment("Event severity level"),

		field.Bool("resolved").
			Default(false).
			Comment("Whether the security event has been resolved"),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("When the event occurred"),
	}
}

// Edges of the SecurityEvent.
func (SecurityEvent) Edges() []ent.Edge {
	return []ent.Edge{
		// Security event belongs to a user
		edge.From("user", User.Type).
			Ref("security_events").
			Unique().
			Required().
			Field("user_id"),
	}
}

// Indexes of the SecurityEvent.
func (SecurityEvent) Indexes() []ent.Index {
	return []ent.Index{
		// Index on user_id for user-specific queries
		index.Fields("user_id"),

		// Index on event_type for filtering
		index.Fields("event_type"),

		// Index on severity for security monitoring
		index.Fields("severity"),

		// Index on created_at for time-based queries
		index.Fields("created_at"),

		// Composite index for user event queries
		index.Fields("user_id", "event_type", "created_at"),

		// Index for unresolved security events
		index.Fields("resolved", "severity", "created_at"),
	}
}
