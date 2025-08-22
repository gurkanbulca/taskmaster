// ent/schema/task.go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Task holds the schema definition for the Task entity.
type Task struct {
	ent.Schema
}

// Fields of the Task.
func (Task) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),

		field.String("title").
			NotEmpty().
			Comment("Task title"),

		field.Text("description").
			Optional().
			Comment("Detailed description of the task"),

		field.Enum("status").
			Values("pending", "in_progress", "completed", "cancelled").
			Default("pending").
			Comment("Current status of the task"),

		field.Enum("priority").
			Values("low", "medium", "high", "critical").
			Default("medium").
			Comment("Priority level of the task"),

		field.String("assigned_to").
			Optional().
			Comment("Email or ID of the person assigned to this task"),

		field.Time("due_date").
			Optional().
			Nillable().
			Comment("When the task should be completed"),

		field.JSON("tags", []string{}).
			Optional().
			SchemaType(map[string]string{
				dialect.Postgres: "text[]",
			}).
			Comment("Tags for categorizing the task"),

		field.JSON("metadata", map[string]interface{}{}).
			Optional().
			Default(map[string]interface{}{}).
			Comment("Additional metadata for the task"),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("When the task was created"),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("When the task was last updated"),
	}
}

// Edges of the Task.
func (Task) Edges() []ent.Edge {
	return []ent.Edge{
		// Add this when you create User schema
		// edge.From("creator", User.Type).
		//     Ref("tasks").
		//     Unique(),

		// Self-referencing edge for subtasks
		edge.To("subtasks", Task.Type).
			From("parent").
			Unique(),

		// Comments edge (when you add Comment schema)
		// edge.To("comments", Comment.Type),
	}
}

// Indexes of the Task.
func (Task) Indexes() []ent.Index {
	return []ent.Index{
		// Index on status for filtering
		index.Fields("status"),

		// Index on priority for filtering
		index.Fields("priority"),

		// Index on assigned_to for filtering by assignee
		index.Fields("assigned_to"),

		// Composite index for common queries
		index.Fields("status", "priority"),

		// Index on created_at for sorting
		index.Fields("created_at"),
	}
}
