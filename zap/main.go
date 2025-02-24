package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"zap/auth"
	"zap/gemini"
	"zap/tasks"

	tasksapi "google.golang.org/api/tasks/v1"
)

// TaskData holds the task list and its tasks
type TaskData struct {
	TaskList *tasksapi.TaskList
	Tasks    []*tasksapi.Task
}

func main() {
	// Parse command line flags
	userEmail := flag.String("u", "", "User email to impersonate")
	flag.Parse()

	if *userEmail == "" {
		log.Fatal("User email is required. Use -u flag to specify the email address.")
	}

	ctx := context.Background()

	// Initialize service account configuration
	authConfig, err := auth.NewConfig("credentials.json")
	if err != nil {
		log.Fatal(err)
	}

	// Create the tasks service using service account with user impersonation
	taskService, err := authConfig.CreateClientAsUser(ctx, *userEmail)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the Tasks service wrapper
	service, err := tasks.NewService(ctx, taskService)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize Gemini client
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is not set")
	}

	geminiClient, err := gemini.NewGeminiClient(geminiKey, taskService, "gemini-2.0-flash-thinking-exp-01-21")
	if err != nil {
		log.Fatal(err)
	}
	defer geminiClient.Close()

	// Create prioritizer
	prioritizer := tasks.NewPrioritizer(service, geminiClient)

	// Prioritize tasks in Backlog and In Progress lists
	targetLists := []string{"Backlog", "In Progress"}
	fmt.Printf("Analyzing and prioritizing tasks in lists: %v\n", targetLists)

	if err := prioritizer.ReorderTasksByPriority(ctx, targetLists); err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nTask prioritization completed successfully!")

	// Automatically create subtasks for tasks in target lists
	fmt.Printf("\nAnalyzing and creating subtasks for tasks in lists: %v\n", targetLists)
	for _, listTitle := range targetLists {
		taskList, err := service.GetTaskListByTitle(listTitle)
		if err != nil {
			log.Printf("Error finding task list %s: %v", listTitle, err)
			continue
		}

		tasks, err := service.ListTasks(taskList.Id)
		if err != nil {
			log.Printf("Error fetching tasks for list %s: %v", listTitle, err)
			continue
		}

		// Skip if no tasks in the list
		if len(tasks) == 0 {
			fmt.Printf("No tasks found in list: %s\n", listTitle)
			continue
		}

		// Count top-level tasks and tasks with subtasks
		topLevelCount := 0
		hasSubtasksCount := 0
		for _, task := range tasks {
			if task.Parent == "" {
				topLevelCount++
				// Check if this task has any subtasks
				for _, t := range tasks {
					if t.Parent == task.Id {
						hasSubtasksCount++
						break
					}
				}
			}
		}

		fmt.Printf("\nIn list '%s':\n", listTitle)
		fmt.Printf("- Found %d top-level tasks\n", topLevelCount)
		fmt.Printf("- %d tasks already have subtasks\n", hasSubtasksCount)
		fmt.Printf("- Will generate subtasks for %d tasks\n", topLevelCount-hasSubtasksCount)

		// Create subtasks using Gemini
		err = geminiClient.AnalyzeAndCreateSubtasks(ctx, taskList.Id, tasks)
		if err != nil {
			if err.Error() == "no tasks found that need subtasks" {
				fmt.Printf("All tasks in list '%s' already have subtasks. Skipping.\n", listTitle)
				continue
			}
			log.Printf("Error creating subtasks for list %s: %v", listTitle, err)
			continue
		}
		fmt.Printf("Successfully created subtasks for list: %s\n", listTitle)
	}

	fmt.Println("\nSubtask creation completed successfully!")

	// Display updated task lists
	taskLists, err := service.ListTaskLists()
	if err != nil {
		log.Fatal(err)
	}

	if len(taskLists) == 0 {
		fmt.Println("No task lists found.")
		return
	}
}
