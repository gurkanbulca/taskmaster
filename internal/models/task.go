package models

import (
	"database/sql"
	"time"

	"github.com/lib/pq"
)

// Task status constants
const (
	TaskStatusPending    = "pending"
	TaskStatusInProgress = "in_progress"
	TaskStatusCompleted  = "completed"
	TaskStatusCancelled  = "cancelled"
)

// Priority constants
const (
	PriorityLow      = "low"
	PriorityMedium   = "medium"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

type Task struct {
	ID          string         `db:"id"`
	Title       string         `db:"title"`
	Description string         `db:"description"`
	Status      string         `db:"status"`
	Priority    string         `db:"priority"`
	AssignedTo  sql.NullString `db:"assigned_to"`
	DueDate     sql.NullTime   `db:"due_date"`
	Tags        pq.StringArray `db:"tags"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}
