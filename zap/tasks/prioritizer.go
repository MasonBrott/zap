package tasks

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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

// ReorderTasksByPriority fetches tasks from specified lists and reorders them based on Gemini analysis
func (p *Prioritizer) ReorderTasksByPriority(ctx context.Context, targetLists []string) error {
	// Get all task lists
	taskLists, err := p.service.ListTaskLists()
	if err != nil {
		return fmt.Errorf("failed to list task lists: %v", err)
	}

	// Filter and process target lists
	var allTasks []*tasksapi.Task
	taskListMap := make(map[string]*tasksapi.TaskList) // Map task list ID to task list
	taskToListMap := make(map[string]string)           // Map task ID to its task list ID

	for _, list := range taskLists {
		if !shouldProcessList(list.Title, targetLists) {
			continue
		}

		tasks, err := p.service.ListTasks(list.Id)
		if err != nil {
			return fmt.Errorf("failed to list tasks for %s: %v", list.Title, err)
		}

		// Store task list and create mappings
		taskListMap[list.Id] = list
		for _, task := range tasks {
			allTasks = append(allTasks, task)
			taskToListMap[task.Id] = list.Id
		}
	}

	if len(allTasks) == 0 {
		return fmt.Errorf("no tasks found in the specified lists")
	}

	// Get priority analysis from Gemini
	priorities, err := p.gemini.AnalyzeAndPrioritizeTasks(ctx, allTasks)
	if err != nil {
		return fmt.Errorf("failed to analyze tasks: %v", err)
	}

	// Group tasks by list and combine with priorities
	tasksByList := make(map[string][]taskWithPriority)
	for _, task := range allTasks {
		listID := taskToListMap[task.Id]
		priority := getPriorityForTask(task.Id, priorities)
		tasksByList[listID] = append(tasksByList[listID], taskWithPriority{
			task:     task,
			priority: priority,
		})
	}

	// Update each list separately
	for listID, tasksWithPriority := range tasksByList {
		// Sort tasks by priority (highest first)
		sort.Slice(tasksWithPriority, func(i, j int) bool {
			return tasksWithPriority[i].priority > tasksWithPriority[j].priority
		})

		// Move highest priority task to the top first
		highestPriorityTask := tasksWithPriority[0].task
		_, err := p.service.MoveTask(listID, highestPriorityTask.Id, "")
		if err != nil {
			return fmt.Errorf("failed to move task %s to top in list %s: %v",
				highestPriorityTask.Title, taskListMap[listID].Title, err)
		}

		// Move remaining tasks in order
		for i := 1; i < len(tasksWithPriority); i++ {
			currentTask := tasksWithPriority[i].task
			previousTask := tasksWithPriority[i-1].task

			_, err := p.service.MoveTask(listID, currentTask.Id, previousTask.Id)
			if err != nil {
				return fmt.Errorf("failed to move task %s after %s in list %s: %v",
					currentTask.Title, previousTask.Title, taskListMap[listID].Title, err)
			}

			// Add a small delay to ensure order is preserved
			time.Sleep(100 * time.Millisecond)
		}
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
