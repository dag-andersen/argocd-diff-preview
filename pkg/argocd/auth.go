package argocd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// getInitialPassword retrieves the initial admin password from Kubernetes secret
func (a *ArgoCDInstallation) getInitialPassword() (string, error) {
	var err error
	var err_fallback error
	secret, err := a.K8sClient.GetSecretValue(a.Namespace, "argocd-initial-admin-secret", "password")
	if err != nil {
		log.Debug().Msgf("Failed to get password in 'argocd-initial-admin-secret'. Trying to get fallback password in 'argocd-cluster' secret.")
		secret, err_fallback = a.K8sClient.GetSecretValue(a.Namespace, "argocd-cluster", "admin.password")
		if err_fallback != nil {
			log.Error().Err(err).Msgf("❌ Failed to get secret 'argocd-initial-admin-secret'")
			log.Error().Err(err_fallback).Msgf("❌ Failed to get fallback secret 'argocd-cluster'")
			return "", fmt.Errorf("failed to get secret: %w", err)
		}
	}

	return secret, nil
}

func (a *ArgoCDInstallation) getTokenFromConfig() (string, error) {
	// Get the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Construct the path to the ArgoCD config file
	configPath := filepath.Join(homeDir, ".config", "argocd", "config")

	// Read the config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read ArgoCD config file: %w", err)
	}

	// Define the structure to parse the config
	type Context struct {
		Name   string `yaml:"name"`
		Server string `yaml:"server"`
		User   string `yaml:"user"`
	}

	type Server struct {
		Server          string `yaml:"server"`
		GRPCWebRootPath string `yaml:"grpc-web-root-path"`
		Insecure        bool   `yaml:"insecure,omitempty"`
		Core            bool   `yaml:"core,omitempty"`
	}

	type User struct {
		Name         string `yaml:"name"`
		AuthToken    string `yaml:"auth-token,omitempty"`
		RefreshToken string `yaml:"refresh-token,omitempty"`
	}

	type Config struct {
		Contexts       []Context `yaml:"contexts"`
		CurrentContext string    `yaml:"current-context"`
		PromptsEnabled bool      `yaml:"prompts-enabled"`
		Servers        []Server  `yaml:"servers"`
		Users          []User    `yaml:"users"`
	}

	// Parse the YAML config
	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return "", fmt.Errorf("failed to parse ArgoCD config: %w", err)
	}

	// Find the current context
	var currentContextUser string
	for _, ctx := range config.Contexts {
		if ctx.Name == config.CurrentContext {
			currentContextUser = ctx.User
			log.Debug().Msgf("Found current context '%s' with user '%s' in ArgoCD config at path: '%s'", config.CurrentContext, currentContextUser, configPath)
			break
		}
	}

	if currentContextUser == "" {
		return "", fmt.Errorf("current context '%s' not found in contexts in ArgoCD config at path: '%s'", config.CurrentContext, configPath)
	}

	// Find the user with matching name and get the auth token
	for _, user := range config.Users {
		if user.Name == currentContextUser {
			if user.AuthToken != "" {
				log.Debug().Msgf("Found auth token at path: '%s'", configPath)
				log.Info().Msg("🔑 Found auth token")
				return user.AuthToken, nil
			}
			return "", fmt.Errorf("user '%s' found but has no auth token in ArgoCD config at path: '%s'", user.Name, configPath)
		}
	}

	return "", fmt.Errorf("no auth token found in ArgoCD config at path: '%s' for user '%s'", configPath, currentContextUser)
}

// // getTokenFromApi retrieves an authentication token from the ArgoCD API (cached)
// func (a *ArgoCDInstallation) getTokenFromApi(username, password string) (string, error) {

// 	log.Info().Msg("🔑 Fetching new authentication token...")

// 	// Set up port forward to ArgoCD server
// 	if err := a.portForwardToArgoCD(); err != nil {
// 		return "", err
// 	}

// 	// Prepare the login request payload
// 	loginData := map[string]string{
// 		"username": username,
// 		"password": password,
// 	}

// 	jsonData, err := json.Marshal(loginData)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to marshal login data: %w", err)
// 	}

// 	// Make the HTTP request to ArgoCD API
// 	// Use plain HTTP since we're connecting directly to the pod's port 8080
// 	url := fmt.Sprintf("%s/api/v1/session", a.ArgoCDApiConnection.apiServerURL)

// 	log.Debug().Msgf("🌐 Making request to: %s", url)

// 	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create HTTP request: %w", err)
// 	}

// 	req.Header.Set("Content-Type", "application/json")

// 	// Create HTTP client with timeout and TLS config
// 	// ArgoCD redirects HTTP to HTTPS, so we need to handle both
// 	client := &http.Client{
// 		Timeout: 10 * time.Second,
// 		Transport: &http.Transport{
// 			TLSClientConfig: &tls.Config{
// 				InsecureSkipVerify: true, // Skip certificate verification for self-signed certs
// 			},
// 		},
// 	}

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to make HTTP request: %w", err)
// 	}
// 	defer func() { _ = resp.Body.Close() }()

// 	// Read the response body
// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to read response body: %w", err)
// 	}

// 	// Check if the request was successful
// 	if resp.StatusCode != http.StatusOK {
// 		return "", fmt.Errorf("ArgoCD API returned status %d: %s", resp.StatusCode, string(body))
// 	}

// 	// Parse the JSON response to extract the token
// 	var sessionResponse struct {
// 		Token string `json:"token"`
// 	}

// 	if err := json.Unmarshal(body, &sessionResponse); err != nil {
// 		return "", fmt.Errorf("failed to unmarshal response: %w", err)
// 	}

// 	if sessionResponse.Token == "" {
// 		return "", fmt.Errorf("token not found in response")
// 	}

// 	// Cache the token for future use
// 	a.ArgoCDApiConnection.authToken = sessionResponse.Token

// 	log.Info().Msg("🔑 Successfully obtained and cached ArgoCD token")
// 	return sessionResponse.Token, nil
// }

func (a *ArgoCDInstallation) updateToken() error {
	// Return cached token if available
	token, err := a.getTokenFromConfig()
	if err != nil {
		return fmt.Errorf("failed to get initial token: %w", err)
	}

	a.ArgoCDApiConnection.authToken = token
	return nil
}

// login performs login to ArgoCD using the CLI (legacy method)
func (a *ArgoCDInstallation) login() error {
	log.Info().Msgf("🦑 Logging in to Argo CD through CLI...")

	// Get initial admin password
	password, err := a.getInitialPassword()
	if err != nil {
		return err
	}

	username := "admin"

	maxAttempts := 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Debug().Msgf("Login attempt %d/%d to Argo CD...", attempt, maxAttempts)

		// Build login command arguments
		args := []string{"login", "--insecure", "--username", username, "--password", password}
		if a.LoginOptions != "" {
			args = append(args, strings.Fields(a.LoginOptions)...)
		}

		out, err := a.runArgocdCommand(args...)
		if err == nil {
			log.Debug().Msgf("Login successful on attempt %d. Output: %s", attempt, out)
			break
		}

		if attempt >= maxAttempts {
			log.Error().Err(err).Msgf("❌ Failed to login to Argo CD after %d attempts", maxAttempts)
			return fmt.Errorf("failed to login after %d attempts", maxAttempts)
		}

		log.Debug().Msgf("Waiting 1s before next login attempt (%d/%d)...", attempt+1, maxAttempts)
		log.Warn().Err(err).Msgf("Argo CD login attempt %d/%d failed.", attempt, maxAttempts)
		time.Sleep(1 * time.Second)
	}

	log.Debug().Msg("Verifying login by listing applications...")
	if _, errList := a.runArgocdCommand("app", "list"); errList != nil {
		log.Error().Err(errList).Msg("❌ Failed to list applications after login (verification step).")
		return fmt.Errorf("login verification failed (unable to list applications): %w", errList)
	}

	// Update the token
	if a.UseAPI() {
		if err := a.updateToken(); err != nil {
			return fmt.Errorf("failed to update token: %w", err)
		}

		// Not needed to login, but we ensure we have a port forward to ArgoCD
		if err := a.portForwardToArgoCD(); err != nil {
			return fmt.Errorf("failed to port forward to ArgoCD: %w", err)
		}
	}

	return nil
}

// OnlyLogin performs only the login step without installing ArgoCD
func (a *ArgoCDInstallation) OnlyLogin() (time.Duration, error) {
	startTime := time.Now()

	// Login to ArgoCD
	if err := a.login(); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to login: %w", err)
	}

	log.Info().Msg("🦑 Logged in to Argo CD successfully")

	// Check Argo CD CLI version vs Argo CD Server version
	if err := a.CheckArgoCDCLIVersionVsServerVersion(); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to check argocd cli version vs server version: %w", err)
	}

	return time.Since(startTime), nil
}
