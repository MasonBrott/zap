package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/tasks/v1"
)

const (
	// LocalRedirectURL is the redirect URL for local development
	LocalRedirectURL = "http://localhost:8085"
)

// Config holds the OAuth configuration
type Config struct {
	config *oauth2.Config
}

// WaitForCallback starts a local server and waits for the OAuth callback
func (c *Config) WaitForCallback(ctx context.Context) (string, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Parse the redirect URL to get the port
	u, err := url.Parse(LocalRedirectURL)
	if err != nil {
		return "", fmt.Errorf("invalid redirect URL: %v", err)
	}

	server := &http.Server{Addr: u.Host}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no code in callback")
			w.Write([]byte("Authorization failed. You can close this window."))
			return
		}

		codeChan <- code
		w.Write([]byte("Authorization successful! You can close this window."))

		// Shutdown the server after sending the response
		go server.Shutdown(ctx)
	})

	// Start the server
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case code := <-codeChan:
		return code, nil
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// NewConfig creates a new OAuth configuration from credentials file
func NewConfig(credentialsPath string) (*Config, error) {
	credBytes, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %v", err)
	}

	config, err := google.ConfigFromJSON(credBytes, tasks.TasksReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %v", err)
	}

	// Set the redirect URL and add offline access
	config.RedirectURL = LocalRedirectURL

	return &Config{config: config}, nil
}

// GetAuthURL returns the URL for OAuth authorization
func (c *Config) GetAuthURL() string {
	return c.config.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// Exchange exchanges the authorization code for a token and returns an authenticated client
func (c *Config) Exchange(ctx context.Context, authCode string) (*oauth2.Config, *oauth2.Token, error) {
	token, err := c.config.Exchange(ctx, authCode)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to exchange authorization code: %v", err)
	}

	return c.config, token, nil
}
