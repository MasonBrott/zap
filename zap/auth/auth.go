package auth

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/tasks/v1"
)

// Config holds the service account configuration
type Config struct {
	credentialsPath string
	credentials     []byte
}

// NewConfig creates a new configuration from service account credentials file
func NewConfig(credentialsPath string) (*Config, error) {
	if _, err := os.Stat(credentialsPath); err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %v", err)
	}
	credentials, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %v", err)
	}
	return &Config{credentialsPath: credentialsPath, credentials: credentials}, nil
}

// CreateClient creates a new Tasks API client using service account credentials
func (c *Config) CreateClient(ctx context.Context) (*tasks.Service, error) {
	client, err := tasks.NewService(ctx, option.WithCredentialsFile(c.credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("unable to create tasks client: %v", err)
	}
	return client, nil
}

func (c *Config) CreateClientAsUser(ctx context.Context, userEmail string) (*tasks.Service, error) {
	config, err := google.JWTConfigFromJSON(c.credentials,
		"https://www.googleapis.com/auth/tasks",
	)
	if err != nil {
		return nil, fmt.Errorf("creating JWT config: %v", err)
	}

	// Set the subject (user to impersonate)
	config.Subject = userEmail

	client := config.Client(ctx)

	return tasks.NewService(ctx, option.WithHTTPClient(client))
}
