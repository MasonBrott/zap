package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
	tasksapi "google.golang.org/api/tasks/v1"
)

type TaskPriority struct {
	TaskID      string  `json:"taskId"`
	Priority    float64 `json:"priority"`
	Explanation string  `json:"explanation"`
	NewPosition string  `json:"newPosition"`
}

type SubtaskSuggestion struct {
	ParentTaskID string   `json:"parentTaskId"`
	Subtasks     []string `json:"subtasks"`
	Rationale    string   `json:"rationale"`
}

type GeminiClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
	tasks  *tasksapi.Service
}

func NewGeminiClient(apiKey string, tasksService *tasksapi.Service) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %v", err)
	}

	model := client.GenerativeModel("gemini-pro")

	// Set response constraints
	model.SetTemperature(0.1) // Lower temperature for more consistent output
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
	}

	return &GeminiClient{
		client: client,
		model:  model,
		tasks:  tasksService,
	}, nil
}

func (g *GeminiClient) AnalyzeAndPrioritizeTasks(ctx context.Context, tasks []*tasksapi.Task) ([]TaskPriority, error) {
	// Convert tasks to a format suitable for Gemini analysis
	taskData := make([]map[string]interface{}, len(tasks))
	for i, task := range tasks {
		taskData[i] = map[string]interface{}{
			"id":       task.Id,
			"title":    task.Title,
			"due":      task.Due,
			"notes":    task.Notes,
			"position": task.Position,
		}
	}

	// Create the prompt for Gemini
	taskJSON, err := json.Marshal(taskData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task data: %v", err)
	}

	prompt := fmt.Sprintf(`You are a task prioritization assistant. Your job is to analyze the following tasks and return a JSON array of prioritized tasks.

Rules:
1. Analyze due dates - tasks with closer due dates get higher priority
2. Look for priority markers in titles like [HIGH], [URGENT], [P1]
3. Consider task complexity and dependencies from notes
4. Return ONLY a valid JSON array with no additional text or markdown formatting

Input tasks:
%s

Response format (strict JSON array):
[
  {
    "taskId": "task-id-1",
    "priority": 95.5,
    "explanation": "High priority due to urgent marker and close deadline",
    "newPosition": "00001"
  },
  ...
]

The priority should be a number between 0-100, with higher numbers indicating higher priority.
The newPosition should be a string of 5 digits, ordered from highest to lowest priority (00001 being highest).
Respond with ONLY the JSON array, no other text.`, string(taskJSON))

	// Send request to Gemini
	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	// Parse the response
	responseText := resp.Candidates[0].Content.Parts[0].(genai.Text)

	// Clean up the response text
	cleanJSON := strings.TrimSpace(string(responseText))
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	var priorities []TaskPriority
	if err := json.Unmarshal([]byte(cleanJSON), &priorities); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %v\nResponse was: %s", err, cleanJSON)
	}

	// Validate the response
	if len(priorities) != len(tasks) {
		return nil, fmt.Errorf("received incorrect number of priorities: got %d, want %d", len(priorities), len(tasks))
	}

	// Ensure all tasks have valid priorities and positions
	for i := range priorities {
		if priorities[i].Priority < 0 || priorities[i].Priority > 100 {
			priorities[i].Priority = 50 // Default to middle priority if invalid
		}
		if len(priorities[i].NewPosition) != 5 {
			priorities[i].NewPosition = fmt.Sprintf("%05d", i+1) // Generate position if invalid
		}
	}

	return priorities, nil
}

func (g *GeminiClient) SuggestSubtasks(ctx context.Context, tasks []*tasksapi.Task) ([]SubtaskSuggestion, error) {
	// Convert tasks to a format suitable for Gemini analysis
	taskData := make([]map[string]interface{}, len(tasks))
	for i, task := range tasks {
		taskData[i] = map[string]interface{}{
			"id":     task.Id,
			"title":  task.Title,
			"notes":  task.Notes,
			"parent": task.Parent, // Include parent info
		}
	}

	// Create the prompt for Gemini
	taskJSON, err := json.Marshal(taskData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task data: %v", err)
	}

	prompt := fmt.Sprintf(`You are a task breakdown assistant. Analyze the following tasks and suggest logical subtasks that would help complete each task effectively. Only suggest subtasks for top-level tasks (those without a parent).

Rules:
1. Break down complex tasks into 1-3 actionable subtasks
2. Ensure subtasks are specific and measurable
3. Consider any details or requirements mentioned in the task notes
4. Focus on practical implementation steps
5. Only suggest subtasks for tasks that don't already have a parent
6. Return ONLY a valid JSON array with no additional text

Input tasks:
%s

Response format (strict JSON array):
[
  {
    "parentTaskId": "task-id-1",
    "subtasks": [
      "Research existing solutions",
      "Design database schema",
      "Implement core functionality"
    ],
    "rationale": "Breaking down into research, design, and implementation phases for systematic approach"
  }
]

Respond with ONLY the JSON array, no other text.`, string(taskJSON))

	// Send request to Gemini
	resp, err := g.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	// Parse the response
	responseText := resp.Candidates[0].Content.Parts[0].(genai.Text)

	// Clean up the response text
	cleanJSON := strings.TrimSpace(string(responseText))
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	var suggestions []SubtaskSuggestion
	if err := json.Unmarshal([]byte(cleanJSON), &suggestions); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %v\nResponse was: %s", err, cleanJSON)
	}

	// Filter out tasks that already have parents
	var topLevelTasks []*tasksapi.Task
	for _, task := range tasks {
		if task.Parent == "" {
			topLevelTasks = append(topLevelTasks, task)
		}
	}

	// Validate the response
	if len(suggestions) != len(topLevelTasks) {
		return nil, fmt.Errorf("received incorrect number of suggestions: got %d, want %d", len(suggestions), len(topLevelTasks))
	}

	return suggestions, nil
}

func (g *GeminiClient) CreateSubtasks(ctx context.Context, taskListId string, suggestions []SubtaskSuggestion) error {
	for _, suggestion := range suggestions {
		// Get the parent task to ensure it exists and get its properties
		parentTask, err := g.tasks.Tasks.Get(taskListId, suggestion.ParentTaskID).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to get parent task %s: %v", suggestion.ParentTaskID, err)
		}

		// Create each subtask
		for _, subtaskTitle := range suggestion.Subtasks {
			subtask := &tasksapi.Task{
				Title:  subtaskTitle,
				Parent: suggestion.ParentTaskID, // Explicitly set the parent ID
				Notes:  fmt.Sprintf("Auto-generated subtask\nRationale: %s", suggestion.Rationale),
			}

			// If parent has a due date, inherit it for the subtask
			if parentTask.Due != "" {
				subtask.Due = parentTask.Due
			}

			// Insert the task with the parent relationship
			insertCall := g.tasks.Tasks.Insert(taskListId, subtask)
			insertCall.Parent(suggestion.ParentTaskID) // Set parent using the API call method
			_, err := insertCall.Context(ctx).Do()
			if err != nil {
				return fmt.Errorf("failed to create subtask '%s' for parent task %s: %v", subtaskTitle, suggestion.ParentTaskID, err)
			}
		}
	}

	return nil
}

// AnalyzeAndCreateSubtasks combines subtask suggestion and creation into a single operation
func (g *GeminiClient) AnalyzeAndCreateSubtasks(ctx context.Context, taskListId string, tasks []*tasksapi.Task) error {
	suggestions, err := g.SuggestSubtasks(ctx, tasks)
	if err != nil {
		return fmt.Errorf("failed to suggest subtasks: %v", err)
	}

	err = g.CreateSubtasks(ctx, taskListId, suggestions)
	if err != nil {
		return fmt.Errorf("failed to create subtasks: %v", err)
	}

	return nil
}

func (g *GeminiClient) Close() {
	if g.client != nil {
		g.client.Close()
	}
}
