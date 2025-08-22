// ent/schema/user.go - Updated for Phase 2
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.String("email").
			NotEmpty().
			Unique().
			Comment("User email address"),

		field.String("username").
			NotEmpty().
			Unique().
			MinLen(3).
			MaxLen(50).
			Comment("Unique username"),

		field.String("password_hash").
			NotEmpty().
			Sensitive(). // Won't be included in logs
			Comment("Hashed password"),

		field.String("first_name").
			Optional().
			Default("").
			MaxLen(100).
			Comment("User's first name"),

		field.String("last_name").
			Optional().
			Default("").
			MaxLen(100).
			Comment("User's last name"),

		field.Enum("role").
			Values("user", "admin", "manager").
			Default("user").
			Comment("User role for authorization"),

		field.Bool("is_active").
			Default(true).
			Comment("Whether the user account is active"),

		field.Bool("email_verified").
			Default(false).
			Comment("Whether email is verified"),

		// Email Verification - Phase 2
		field.String("email_verification_token").
			Optional().
			Sensitive().
			Comment("Token for email verification"),

		field.Time("email_verification_expires_at").
			Optional().
			Nillable().
			Comment("Email verification token expiration"),

		field.Int("email_verification_attempts").
			Default(0).
			Comment("Number of email verification attempts"),

		// Password Reset - Phase 2
		field.String("password_reset_token").
			Optional().
			Sensitive().
			Comment("Token for password reset"),

		field.Time("password_reset_expires_at").
			Optional().
			Nillable().
			Comment("Password reset token expiration"),

		field.Time("password_reset_at").
			Optional().
			Nillable().
			Comment("Last password reset timestamp"),

		field.Int("password_reset_attempts").
			Default(0).
			Comment("Number of password reset attempts"),

		// Security - Phase 2
		field.Int("failed_login_attempts").
			Default(0).
			Comment("Number of consecutive failed login attempts"),

		field.Time("account_locked_until").
			Optional().
			Nillable().
			Comment("Account lockout expiration"),

		field.Time("last_login").
			Optional().
			Nillable().
			Comment("Last successful login timestamp"),

		field.String("last_login_ip").
			Optional().
			Comment("IP address of last login"),

		field.Time("password_changed_at").
			Optional().
			Nillable().
			Comment("When password was last changed"),

		// JWT Tokens
		field.String("refresh_token").
			Optional().
			Sensitive().
			Comment("Current refresh token"),

		field.Time("refresh_token_expires_at").
			Optional().
			Nillable().
			Comment("Refresh token expiration"),

		// User Preferences
		field.JSON("preferences", map[string]interface{}{}).
			Optional().
			Default(map[string]interface{}{}).
			Comment("User preferences and settings"),

		// Notification Settings - Phase 2
		field.Bool("email_notifications_enabled").
			Default(true).
			Comment("Whether email notifications are enabled"),

		field.Bool("security_notifications_enabled").
			Default(true).
			Comment("Whether security email notifications are enabled"),

		field.JSON("notification_preferences", map[string]interface{}{}).
			Optional().
			Default(map[string]interface{}{}).
			Comment("Detailed notification preferences"),

		// Timestamps
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("When the user was created"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("When the user was last updated"),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		// A user can create many tasks
		edge.To("created_tasks", Task.Type).
			Comment("Tasks created by this user"),

		// A user can be assigned to many tasks
		edge.To("assigned_tasks", Task.Type).
			Comment("Tasks assigned to this user"),

		// Security events - Phase 2
		edge.To("security_events", SecurityEvent.Type).
			Comment("Security events related to this user"),
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		// Unique index on email
		index.Fields("email").
			Unique(),

		// Unique index on username
		index.Fields("username").
			Unique(),

		// Index for login queries (email + active status)
		index.Fields("email", "is_active"),

		// Index for role-based queries
		index.Fields("role", "is_active"),

		// Index for email verification
		index.Fields("email_verification_token").
			Unique(),

		// Index for password reset
		index.Fields("password_reset_token").
			Unique(),

		// Index for account security
		index.Fields("account_locked_until"),

		// Index for created_at sorting
		index.Fields("created_at"),

		// Composite index for security queries
		index.Fields("email", "failed_login_attempts"),
	}
}
