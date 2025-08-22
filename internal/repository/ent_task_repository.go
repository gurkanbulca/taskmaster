// internal/repository/ent_task_repository.go
package repository

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/ent/generated/predicate"
	"github.com/gurkanbulca/taskmaster/ent/generated/task"
	"github.com/gurkanbulca/taskmaster/ent/generated/user"
)

type EntTaskRepository struct {
	client *ent.Client
}

func NewEntTaskRepository(client *ent.Client) *EntTaskRepository {
	return &EntTaskRepository{
		client: client,
	}
}

func (r *EntTaskRepository) Create(ctx context.Context, t *TaskInput) (*ent.Task, error) {
	create := r.client.Task.
		Create().
		SetTitle(t.Title).
		SetDescription(t.Description).
		SetStatus(task.Status(t.Status)).
		SetPriority(task.Priority(t.Priority)).
		SetNillableAssignedTo(t.AssignedTo).
		SetNillableDueDate(t.DueDate)

	// Handle tags - ensure it's not nil
	if t.Tags != nil && len(t.Tags) > 0 {
		create = create.SetTags(t.Tags)
	} else {
		create = create.SetTags([]string{}) // Set empty array instead of nil
	}

	// Handle metadata
	if t.Metadata != nil {
		create = create.SetMetadata(t.Metadata)
	} else {
		create = create.SetMetadata(map[string]interface{}{})
	}

	// Set creator if provided
	if t.CreatorID != "" {
		creatorUUID, err := uuid.Parse(t.CreatorID)
		if err != nil {
			return nil, fmt.Errorf("invalid creator ID: %w", err)
		}
		create = create.SetCreatorID(creatorUUID)
	}

	// Set assignee if provided
	if t.AssigneeID != "" {
		assigneeUUID, err := uuid.Parse(t.AssigneeID)
		if err != nil {
			return nil, fmt.Errorf("invalid assignee ID: %w", err)
		}
		create = create.SetAssigneeID(assigneeUUID)
	}

	return create.Save(ctx)
}

func (r *EntTaskRepository) CreateWithCreator(ctx context.Context, t *TaskInput, creatorID string) (*ent.Task, error) {
	creatorUUID, err := uuid.Parse(creatorID)
	if err != nil {
		return nil, fmt.Errorf("invalid creator ID: %w", err)
	}

	create := r.client.Task.
		Create().
		SetTitle(t.Title).
		SetDescription(t.Description).
		SetStatus(task.Status(t.Status)).
		SetPriority(task.Priority(t.Priority)).
		SetNillableAssignedTo(t.AssignedTo).
		SetNillableDueDate(t.DueDate).
		SetCreatorID(creatorUUID)

	// Handle tags - ensure it's not nil
	if t.Tags != nil && len(t.Tags) > 0 {
		create = create.SetTags(t.Tags)
	} else {
		create = create.SetTags([]string{}) // Set empty array instead of nil
	}

	// Handle metadata
	if t.Metadata != nil {
		create = create.SetMetadata(t.Metadata)
	} else {
		create = create.SetMetadata(map[string]interface{}{})
	}

	// Set assignee if provided
	if t.AssigneeID != "" {
		assigneeUUID, err := uuid.Parse(t.AssigneeID)
		if err != nil {
			return nil, fmt.Errorf("invalid assignee ID: %w", err)
		}
		create = create.SetAssigneeID(assigneeUUID)
	}

	return create.Save(ctx)
}

func (r *EntTaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*ent.Task, error) {
	return r.client.Task.
		Query().
		Where(task.ID(id)).
		Only(ctx)
}

func (r *EntTaskRepository) GetByIDWithCreator(ctx context.Context, id uuid.UUID) (*ent.Task, error) {
	return r.client.Task.
		Query().
		Where(task.ID(id)).
		WithCreator().
		WithAssignee().
		Only(ctx)
}

func (r *EntTaskRepository) List(ctx context.Context, filter ListFilter) ([]*ent.Task, int, error) {
	query := r.client.Task.Query()

	// Apply filters
	var predicates []predicate.Task

	if filter.Status != nil {
		predicates = append(predicates, task.StatusEQ(task.Status(*filter.Status)))
	}

	if filter.Priority != nil {
		predicates = append(predicates, task.PriorityEQ(task.Priority(*filter.Priority)))
	}

	if filter.AssignedTo != nil {
		predicates = append(predicates, task.AssignedToEQ(*filter.AssignedTo))
	}

	// Filter by user ID (either creator or assignee)
	if filter.UserID != nil {
		userUUID, err := uuid.Parse(*filter.UserID)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid user ID: %w", err)
		}

		predicates = append(predicates, task.Or(
			task.HasCreatorWith(user.ID(userUUID)),
			task.HasAssigneeWith(user.ID(userUUID)),
		))
	}

	// Filter by creator ID specifically
	if filter.CreatorID != nil {
		creatorUUID, err := uuid.Parse(*filter.CreatorID)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid creator ID: %w", err)
		}
		predicates = append(predicates, task.HasCreatorWith(user.ID(creatorUUID)))
	}

	if filter.Search != "" {
		// Search in title and description
		predicates = append(predicates, task.Or(
			task.TitleContainsFold(filter.Search),
			task.DescriptionContainsFold(filter.Search),
		))
	}

	// Apply predicates
	if len(predicates) > 0 {
		query = query.Where(predicates...)
	}

	// Get total count before pagination
	totalCount, err := query.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	// Apply sorting
	switch filter.SortBy {
	case "created_at":
		if filter.SortOrder == "asc" {
			query = query.Order(ent.Asc(task.FieldCreatedAt))
		} else {
			query = query.Order(ent.Desc(task.FieldCreatedAt))
		}
	case "updated_at":
		if filter.SortOrder == "asc" {
			query = query.Order(ent.Asc(task.FieldUpdatedAt))
		} else {
			query = query.Order(ent.Desc(task.FieldUpdatedAt))
		}
	case "due_date":
		if filter.SortOrder == "asc" {
			query = query.Order(ent.Asc(task.FieldDueDate))
		} else {
			query = query.Order(ent.Desc(task.FieldDueDate))
		}
	case "priority":
		// Custom order for priority
		query = query.Order(func(s *sql.Selector) {
			s.OrderExpr(sql.ExprP(
				"CASE priority WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 WHEN 'low' THEN 4 END",
			))
		})
	default:
		query = query.Order(ent.Desc(task.FieldCreatedAt))
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Include creator and assignee information
	if filter.WithRelations {
		query = query.WithCreator().WithAssignee()
	}

	// Execute query
	tasks, err := query.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("query tasks: %w", err)
	}

	return tasks, totalCount, nil
}

func (r *EntTaskRepository) Update(ctx context.Context, id uuid.UUID, input *TaskUpdateInput) (*ent.Task, error) {
	update := r.client.Task.UpdateOneID(id)

	if input.Title != nil {
		update = update.SetTitle(*input.Title)
	}
	if input.Description != nil {
		update = update.SetDescription(*input.Description)
	}
	if input.Status != nil {
		update = update.SetStatus(task.Status(*input.Status))
	}
	if input.Priority != nil {
		update = update.SetPriority(task.Priority(*input.Priority))
	}
	if input.AssignedTo != nil {
		if *input.AssignedTo == "" {
			update = update.ClearAssignedTo().ClearAssignee()
		} else {
			update = update.SetAssignedTo(*input.AssignedTo)
			// Optionally set assignee relation if it's a valid user ID
			if assigneeUUID, err := uuid.Parse(*input.AssignedTo); err == nil {
				update = update.SetAssigneeID(assigneeUUID)
			}
		}
	}
	if input.DueDate != nil {
		update = update.SetDueDate(*input.DueDate)
	}
	if input.Tags != nil {
		update = update.SetTags(input.Tags)
	}
	if input.Metadata != nil {
		update = update.SetMetadata(input.Metadata)
	}

	return update.Save(ctx)
}

func (r *EntTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.client.Task.
		DeleteOneID(id).
		Exec(ctx)
}

// Batch operations
func (r *EntTaskRepository) CreateBatch(ctx context.Context, inputs []*TaskInput, creatorID string) ([]*ent.Task, error) {
	creatorUUID, err := uuid.Parse(creatorID)
	if err != nil {
		return nil, fmt.Errorf("invalid creator ID: %w", err)
	}

	builders := make([]*ent.TaskCreate, len(inputs))

	for i, input := range inputs {
		builder := r.client.Task.
			Create().
			SetTitle(input.Title).
			SetDescription(input.Description).
			SetStatus(task.Status(input.Status)).
			SetPriority(task.Priority(input.Priority)).
			SetNillableAssignedTo(input.AssignedTo).
			SetNillableDueDate(input.DueDate).
			SetTags(input.Tags).
			SetMetadata(input.Metadata).
			SetCreatorID(creatorUUID)

		if input.AssigneeID != "" {
			if assigneeUUID, err := uuid.Parse(input.AssigneeID); err == nil {
				builder = builder.SetAssigneeID(assigneeUUID)
			}
		}

		builders[i] = builder
	}

	return r.client.Task.CreateBulk(builders...).Save(ctx)
}

// Transaction example
func (r *EntTaskRepository) UpdateStatusBatch(ctx context.Context, ids []uuid.UUID, status string) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}

	for _, id := range ids {
		if err := tx.Task.UpdateOneID(id).SetStatus(task.Status(status)).Exec(ctx); err != nil {
			return rollback(tx, fmt.Errorf("update task %s: %w", id, err))
		}
	}

	return tx.Commit()
}

// Helper function for transaction rollback
func rollback(tx *ent.Tx, err error) error {
	if rerr := tx.Rollback(); rerr != nil {
		err = fmt.Errorf("%w: %v", err, rerr)
	}
	return err
}

// Types for repository input
type TaskInput struct {
	Title       string
	Description string
	Status      string
	Priority    string
	AssignedTo  *string
	AssigneeID  string // User ID for assignee relation
	CreatorID   string // User ID for creator relation
	DueDate     *time.Time
	Tags        []string
	Metadata    map[string]interface{}
}

type TaskUpdateInput struct {
	Title       *string
	Description *string
	Status      *string
	Priority    *string
	AssignedTo  *string
	AssigneeID  *string // User ID for assignee relation
	DueDate     *time.Time
	Tags        []string
	Metadata    map[string]interface{}
}

type ListFilter struct {
	Status        *string
	Priority      *string
	AssignedTo    *string
	UserID        *string // Filter by user (either creator or assignee)
	CreatorID     *string // Filter by creator specifically
	Tags          []string
	Search        string
	SortBy        string
	SortOrder     string
	Limit         int
	Offset        int
	WithRelations bool // Include creator and assignee information
}
