// internal/service/task_service_ent.go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	taskv1 "github.com/gurkanbulca/taskmaster/api/proto/task/v1/generated"
	ent "github.com/gurkanbulca/taskmaster/ent/generated"
	"github.com/gurkanbulca/taskmaster/internal/repository"
)

type TaskService struct {
	taskv1.UnimplementedTaskServiceServer
	repo *repository.EntTaskRepository
}

func NewTaskService(repo *repository.EntTaskRepository) *TaskService {
	return &TaskService{
		repo: repo,
	}
}

// CreateTask creates a new task
func (s *TaskService) CreateTask(ctx context.Context, req *taskv1.CreateTaskRequest) (*taskv1.CreateTaskResponse, error) {
	// Validate request
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	// Prepare input
	input := &repository.TaskInput{
		Title:       req.Title,
		Description: req.Description,
		Status:      "pending",
		Priority:    convertPriorityToString(req.Priority),
		Tags:        req.Tags,
		Metadata:    make(map[string]interface{}),
	}

	if req.AssignedTo != "" {
		input.AssignedTo = &req.AssignedTo
	}

	if req.DueDate != nil {
		dueDate := req.DueDate.AsTime()
		input.DueDate = &dueDate
	}

	// Create task
	task, err := s.repo.Create(ctx, input)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create task: %v", err)
	}

	return &taskv1.CreateTaskResponse{
		Task: convertEntTaskToProto(task),
	}, nil
}

// GetTask retrieves a task by ID
func (s *TaskService) GetTask(ctx context.Context, req *taskv1.GetTaskRequest) (*taskv1.GetTaskResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Parse UUID
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task ID format")
	}

	// Get task
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "task not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get task: %v", err)
	}

	return &taskv1.GetTaskResponse{
		Task: convertEntTaskToProto(task),
	}, nil
}

// ListTasks retrieves a list of tasks
func (s *TaskService) ListTasks(ctx context.Context, req *taskv1.ListTasksRequest) (*taskv1.ListTasksResponse, error) {
	// Set default page size
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Build filter
	filter := repository.ListFilter{
		Limit:  int(pageSize),
		Offset: 0, // Calculate from page token if needed
	}

	if req.Status != taskv1.TaskStatus_TASK_STATUS_UNSPECIFIED {
		status := convertStatusToString(req.Status)
		filter.Status = &status
	}

	if req.Priority != taskv1.Priority_PRIORITY_UNSPECIFIED {
		priority := convertPriorityToString(req.Priority)
		filter.Priority = &priority
	}

	// Get tasks
	tasks, totalCount, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list tasks: %v", err)
	}

	// Convert to proto
	protoTasks := make([]*taskv1.Task, len(tasks))
	for i, task := range tasks {
		protoTasks[i] = convertEntTaskToProto(task)
	}

	return &taskv1.ListTasksResponse{
		Tasks:         protoTasks,
		NextPageToken: "", // Implement pagination token logic
		TotalCount:    int32(totalCount),
	}, nil
}

// UpdateTask updates an existing task
func (s *TaskService) UpdateTask(ctx context.Context, req *taskv1.UpdateTaskRequest) (*taskv1.UpdateTaskResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Parse UUID
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task ID format")
	}

	// Build update input
	input := &repository.TaskUpdateInput{}

	if req.Title != "" {
		input.Title = &req.Title
	}
	if req.Description != "" {
		input.Description = &req.Description
	}
	if req.Status != taskv1.TaskStatus_TASK_STATUS_UNSPECIFIED {
		status := convertStatusToString(req.Status)
		input.Status = &status
	}
	if req.Priority != taskv1.Priority_PRIORITY_UNSPECIFIED {
		priority := convertPriorityToString(req.Priority)
		input.Priority = &priority
	}
	if req.AssignedTo != "" {
		input.AssignedTo = &req.AssignedTo
	}
	if req.DueDate != nil {
		dueDate := req.DueDate.AsTime()
		input.DueDate = &dueDate
	}
	if len(req.Tags) > 0 {
		input.Tags = req.Tags
	}

	// Update task
	task, err := s.repo.Update(ctx, id, input)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "task not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to update task: %v", err)
	}

	return &taskv1.UpdateTaskResponse{
		Task: convertEntTaskToProto(task),
	}, nil
}

// DeleteTask deletes a task
func (s *TaskService) DeleteTask(ctx context.Context, req *taskv1.DeleteTaskRequest) (*emptypb.Empty, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	// Parse UUID
	id, err := uuid.Parse(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task ID format")
	}

	// Delete task
	if err := s.repo.Delete(ctx, id); err != nil {
		if ent.IsNotFound(err) {
			return nil, status.Error(codes.NotFound, "task not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to delete task: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// WatchTasks streams task events
func (s *TaskService) WatchTasks(req *taskv1.WatchTasksRequest, stream taskv1.TaskService_WatchTasksServer) error {
	// This is a simplified implementation
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			event := &taskv1.TaskEvent{
				EventType: taskv1.TaskEvent_EVENT_TYPE_UPDATED,
				Timestamp: timestamppb.Now(),
			}

			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

// Helper functions

func convertEntTaskToProto(task *ent.Task) *taskv1.Task {
	proto := &taskv1.Task{
		Id:          task.ID.String(),
		Title:       task.Title,
		Description: task.Description,
		Status:      convertStringToStatus(string(task.Status)),
		Priority:    convertStringToPriority(string(task.Priority)),
		CreatedAt:   timestamppb.New(task.CreatedAt),
		UpdatedAt:   timestamppb.New(task.UpdatedAt),
		Tags:        task.Tags,
	}

	if task.AssignedTo != "" {
		proto.AssignedTo = task.AssignedTo
	}

	if task.DueDate != nil {
		proto.DueDate = timestamppb.New(*task.DueDate)
	}

	if task.Metadata != nil {
		proto.Metadata = make(map[string]string)
		for k, v := range task.Metadata {
			proto.Metadata[k] = fmt.Sprintf("%v", v)
		}
	}

	return proto
}

func convertStatusToString(status taskv1.TaskStatus) string {
	switch status {
	case taskv1.TaskStatus_TASK_STATUS_PENDING:
		return "pending"
	case taskv1.TaskStatus_TASK_STATUS_IN_PROGRESS:
		return "in_progress"
	case taskv1.TaskStatus_TASK_STATUS_COMPLETED:
		return "completed"
	case taskv1.TaskStatus_TASK_STATUS_CANCELLED:
		return "cancelled"
	default:
		return "pending"
	}
}

func convertStringToStatus(status string) taskv1.TaskStatus {
	switch status {
	case "pending":
		return taskv1.TaskStatus_TASK_STATUS_PENDING
	case "in_progress":
		return taskv1.TaskStatus_TASK_STATUS_IN_PROGRESS
	case "completed":
		return taskv1.TaskStatus_TASK_STATUS_COMPLETED
	case "cancelled":
		return taskv1.TaskStatus_TASK_STATUS_CANCELLED
	default:
		return taskv1.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

func convertPriorityToString(priority taskv1.Priority) string {
	switch priority {
	case taskv1.Priority_PRIORITY_LOW:
		return "low"
	case taskv1.Priority_PRIORITY_MEDIUM:
		return "medium"
	case taskv1.Priority_PRIORITY_HIGH:
		return "high"
	case taskv1.Priority_PRIORITY_CRITICAL:
		return "critical"
	default:
		return "medium"
	}
}

func convertStringToPriority(priority string) taskv1.Priority {
	switch priority {
	case "low":
		return taskv1.Priority_PRIORITY_LOW
	case "medium":
		return taskv1.Priority_PRIORITY_MEDIUM
	case "high":
		return taskv1.Priority_PRIORITY_HIGH
	case "critical":
		return taskv1.Priority_PRIORITY_CRITICAL
	default:
		return taskv1.Priority_PRIORITY_UNSPECIFIED
	}
}
