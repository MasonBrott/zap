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

type GeminiClient struct {
	client *genai.Client
	model  *genai.GenerativeModel
}

func NewGeminiClient(apiKey string) (*GeminiClient, error) {
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

func (g *GeminiClient) Close() {
	if g.client != nil {
		g.client.Close()
	}
}
