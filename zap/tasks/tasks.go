package tasks

import (
	"context"
	"fmt"

	tasksapi "google.golang.org/api/tasks/v1"
)

// Service handles Google Tasks operations
type Service struct {
	service *tasksapi.Service
}

// NewService creates a new Tasks service with the provided service client
func NewService(ctx context.Context, service *tasksapi.Service) (*Service, error) {
	if service == nil {
		return nil, fmt.Errorf("service cannot be nil")
	}
	return &Service{service: service}, nil
}

// ListTaskLists retrieves all task lists for the authenticated user
func (s *Service) ListTaskLists() ([]*tasksapi.TaskList, error) {
	tasklists, err := s.service.Tasklists.List().Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve task lists: %v", err)
	}

	return tasklists.Items, nil
}

// GetTaskList retrieves a specific task list by ID
func (s *Service) GetTaskList(taskListID string) (*tasksapi.TaskList, error) {
	taskList, err := s.service.Tasklists.Get(taskListID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve task list: %v", err)
	}

	return taskList, nil
}

// ListTasks retrieves all tasks in a specific task list
func (s *Service) ListTasks(taskListID string) ([]*tasksapi.Task, error) {
	tasks, err := s.service.Tasks.List(taskListID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve tasks: %v", err)
	}

	return tasks.Items, nil
}

// NewTask creates a new task struct with common fields
func NewTask(title string) *tasksapi.Task {
	return &tasksapi.Task{
		Title:  title,
		Status: "needsAction",
	}
}

// UpdateTask updates an existing task in a specific task list
func (s *Service) UpdateTask(taskListID string, taskID string, task *tasksapi.Task) (*tasksapi.Task, error) {
	updatedTask, err := s.service.Tasks.Update(taskListID, taskID, task).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to update task: %v", err)
	}
	return updatedTask, nil
}

// MoveTask moves a task to a new position in the list
func (s *Service) MoveTask(taskListID string, taskID string, previousTaskID string) (*tasksapi.Task, error) {
	moveCall := s.service.Tasks.Move(taskListID, taskID)
	if previousTaskID != "" {
		moveCall = moveCall.Previous(previousTaskID)
	}

	movedTask, err := moveCall.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to move task: %v", err)
	}
	return movedTask, nil
}

// MarkTaskComplete marks a task as completed
func (s *Service) MarkTaskComplete(taskListID string, taskID string) (*tasksapi.Task, error) {
	task, err := s.service.Tasks.Get(taskListID, taskID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get task: %v", err)
	}

	task.Status = "completed"
	return s.UpdateTask(taskListID, taskID, task)
}

// MarkTaskIncomplete marks a task as not completed
func (s *Service) MarkTaskIncomplete(taskListID string, taskID string) (*tasksapi.Task, error) {
	task, err := s.service.Tasks.Get(taskListID, taskID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get task: %v", err)
	}

	task.Status = "needsAction"
	return s.UpdateTask(taskListID, taskID, task)
}
