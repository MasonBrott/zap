package tasks

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/tasks/v1"
)

// Service handles Google Tasks operations
type Service struct {
	service *tasks.Service
}

// NewService creates a new Tasks service with the provided OAuth config and token
func NewService(ctx context.Context, config *oauth2.Config, token *oauth2.Token) (*Service, error) {
	client := config.Client(ctx, token)

	service, err := tasks.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to create tasks service: %v", err)
	}

	return &Service{service: service}, nil
}

// ListTaskLists retrieves all task lists for the authenticated user
func (s *Service) ListTaskLists() ([]*tasks.TaskList, error) {
	tasklists, err := s.service.Tasklists.List().Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve task lists: %v", err)
	}

	return tasklists.Items, nil
}

// GetTaskList retrieves a specific task list by ID
func (s *Service) GetTaskList(taskListID string) (*tasks.TaskList, error) {
	taskList, err := s.service.Tasklists.Get(taskListID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve task list: %v", err)
	}

	return taskList, nil
}

// ListTasks retrieves all tasks in a specific task list
func (s *Service) ListTasks(taskListID string) ([]*tasks.Task, error) {
	tasks, err := s.service.Tasks.List(taskListID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve tasks: %v", err)
	}

	return tasks.Items, nil
}

// NewTask creates a new task struct with common fields
func NewTask(title string) *tasks.Task {
	return &tasks.Task{
		Title:  title,
		Status: "needsAction",
	}
}

// UpdateTask updates an existing task in a specific task list
func (s *Service) UpdateTask(taskListID string, taskID string, task *tasks.Task) (*tasks.Task, error) {
	updatedTask, err := s.service.Tasks.Update(taskListID, taskID, task).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to update task: %v", err)
	}
	return updatedTask, nil
}

// MarkTaskComplete marks a task as completed
func (s *Service) MarkTaskComplete(taskListID string, taskID string) (*tasks.Task, error) {
	task, err := s.service.Tasks.Get(taskListID, taskID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get task: %v", err)
	}

	task.Status = "completed"
	return s.UpdateTask(taskListID, taskID, task)
}

// MarkTaskIncomplete marks a task as not completed
func (s *Service) MarkTaskIncomplete(taskListID string, taskID string) (*tasks.Task, error) {
	task, err := s.service.Tasks.Get(taskListID, taskID).Do()
	if err != nil {
		return nil, fmt.Errorf("unable to get task: %v", err)
	}

	task.Status = "needsAction"
	return s.UpdateTask(taskListID, taskID, task)
}
