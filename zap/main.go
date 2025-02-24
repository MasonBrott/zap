package main

import (
	"context"
	"fmt"
	"log"

	"zap/auth"
	"zap/tasks"
)

func main() {
	ctx := context.Background()

	// Initialize OAuth configuration
	authConfig, err := auth.NewConfig("credentials.json")
	if err != nil {
		log.Fatal(err)
	}

	// Get authorization URL and prompt user
	authURL := authConfig.GetAuthURL()
	fmt.Printf("Opening the following URL in your browser: \n%v\n", authURL)
	fmt.Println("Waiting for OAuth callback...")

	// Wait for the callback with the authorization code
	authCode, err := authConfig.WaitForCallback(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Exchange the authorization code for tokens
	config, token, err := authConfig.Exchange(ctx, authCode)
	if err != nil {
		log.Fatal(err)
	}

	// Create the tasks service
	taskService, err := tasks.NewService(ctx, config, token)
	if err != nil {
		log.Fatal(err)
	}

	// List all task lists
	taskLists, err := taskService.ListTaskLists()
	if err != nil {
		log.Fatal(err)
	}

	// Print the task lists
	if len(taskLists) == 0 {
		fmt.Println("No task lists found.")
		return
	}

	fmt.Println("Task lists:")
	for _, taskList := range taskLists {
		fmt.Printf("- %s\n", taskList.Title)
	}

}
