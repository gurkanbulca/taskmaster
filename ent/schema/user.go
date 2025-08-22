// ent/schema/user.go
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

		field.Time("last_login").
			Optional().
			Nillable().
			Comment("Last login timestamp"),

		field.String("refresh_token").
			Optional().
			Sensitive().
			Comment("Current refresh token"),

		field.Time("refresh_token_expires_at").
			Optional().
			Nillable().
			Comment("Refresh token expiration"),

		field.JSON("preferences", map[string]interface{}{}).
			Optional().
			Default(map[string]interface{}{}).
			Comment("User preferences"),

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

		// Index for created_at sorting
		index.Fields("created_at"),
	}
}
