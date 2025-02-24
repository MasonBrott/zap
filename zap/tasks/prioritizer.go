package tasks

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"zap/gemini"

	tasksapi "google.golang.org/api/tasks/v1"
)

type Prioritizer struct {
	service *Service
	gemini  *gemini.GeminiClient
}

func NewPrioritizer(service *Service, geminiClient *gemini.GeminiClient) *Prioritizer {
	return &Prioritizer{
		service: service,
		gemini:  geminiClient,
	}
}

// taskWithPriority combines a task with its priority for sorting
type taskWithPriority struct {
	task     *tasksapi.Task
	priority float64
}

// ReorderTasksByPriority reorders tasks in the specified lists based on AI analysis
func (p *Prioritizer) ReorderTasksByPriority(ctx context.Context, targetLists []string) error {
	for _, listTitle := range targetLists {
		taskList, err := p.service.GetTaskListByTitle(listTitle)
		if err != nil {
			return fmt.Errorf("error finding task list %s: %v", listTitle, err)
		}

		tasks, err := p.service.ListTasks(taskList.Id)
		if err != nil {
			return fmt.Errorf("error fetching tasks for list %s: %v", listTitle, err)
		}

		// Filter out subtasks - only process top-level tasks
		var topLevelTasks []*tasksapi.Task
		for _, task := range tasks {
			if task.Parent == "" {
				topLevelTasks = append(topLevelTasks, task)
			}
		}

		// Skip if no top-level tasks in the list
		if len(topLevelTasks) == 0 {
			fmt.Printf("No top-level tasks found in list: %s\n", listTitle)
			continue
		}

		// Get priorities from Gemini
		priorities, err := p.gemini.AnalyzeAndPrioritizeTasks(ctx, topLevelTasks)
		if err != nil {
			return fmt.Errorf("error analyzing tasks for list %s: %v", listTitle, err)
		}

		// Sort priorities by position
		sort.Slice(priorities, func(i, j int) bool {
			return priorities[i].NewPosition < priorities[j].NewPosition
		})

		// Apply the new order
		var previousTaskID string
		for _, priority := range priorities {
			_, err := p.service.MoveTask(taskList.Id, priority.TaskID, previousTaskID)
			if err != nil {
				return fmt.Errorf("error moving task %s: %v", priority.TaskID, err)
			}
			previousTaskID = priority.TaskID
		}

		fmt.Printf("Successfully prioritized %d tasks in list: %s\n", len(priorities), listTitle)
	}

	return nil
}

// getPriorityForTask returns the priority value for a given task ID
func getPriorityForTask(taskID string, priorities []gemini.TaskPriority) float64 {
	for _, p := range priorities {
		if p.TaskID == taskID {
			return p.Priority
		}
	}
	return 0 // Default priority if not found
}

func shouldProcessList(listTitle string, targetLists []string) bool {
	listTitle = strings.ToLower(listTitle)
	for _, target := range targetLists {
		if strings.ToLower(target) == listTitle {
			return true
		}
	}
	return false
}
