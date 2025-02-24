package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

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

// printTaskInfo prints detailed information about a task
func printTaskInfo(task *tasksapi.Task) {
	fmt.Println("\n----------------------------------------")
	fmt.Printf("Title: %s\n", task.Title)
	fmt.Printf("ID: %s\n", task.Id)
	fmt.Printf("Status: %s\n", task.Status)

	if task.Due != "" {
		due, _ := time.Parse(time.RFC3339, task.Due)
		fmt.Printf("Due: %s\n", due.Format("2006-01-02 15:04:05"))
	}

	if task.Notes != "" {
		fmt.Printf("Notes: %s\n", task.Notes)
	}

	if task.Completed != nil && *task.Completed != "" {
		completed, _ := time.Parse(time.RFC3339, *task.Completed)
		fmt.Printf("Completed: %s\n", completed.Format("2006-01-02 15:04:05"))
	}

	if task.Parent != "" {
		fmt.Printf("Parent Task ID: %s\n", task.Parent)
	}

	fmt.Printf("Position: %s\n", task.Position)
	fmt.Println("----------------------------------------")
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

	geminiClient, err := gemini.NewGeminiClient(geminiKey)
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

	// Display updated task lists
	taskLists, err := service.ListTaskLists()
	if err != nil {
		log.Fatal(err)
	}

	if len(taskLists) == 0 {
		fmt.Println("No task lists found.")
		return
	}

	// Display updated tasks
	for _, taskList := range taskLists {
		if !contains(targetLists, taskList.Title) {
			continue
		}

		tasks, err := service.ListTasks(taskList.Id)
		if err != nil {
			log.Printf("Error fetching tasks for list %s: %v", taskList.Title, err)
			continue
		}

		fmt.Printf("\n=== Updated Task List: %s ===\n", taskList.Title)
		if len(tasks) == 0 {
			fmt.Println("No tasks in this list")
			continue
		}

		for _, task := range tasks {
			printTaskInfo(task)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
