package argocd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// getInitialPassword retrieves the initial admin password from Kubernetes secret
func (a *ArgoCDInstallation) getInitialPassword() (string, error) {

	secret, err := a.K8sClient.GetSecretValue(a.Namespace, "argocd-initial-admin-secret", "password")
	if err != nil {
		log.Error().Msgf("❌ Failed to get secret: %s", err)
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	return secret, nil
}

// portForwardToArgoCD sets up a port forward to the ArgoCD server if not already active
func (a *ArgoCDInstallation) portForwardToArgoCD() error {
	a.portForwardMutex.Lock()

	// Check if port forward is already active
	if a.portForwardActive {
		a.portForwardMutex.Unlock()
		log.Debug().Msg("Port forward to ArgoCD server is already active, reusing existing connection")
		return nil
	}

	log.Info().Msg("🔌 Setting up port forward to ArgoCD server...")

	// Create channels for coordination
	readyChan := make(chan struct{}, 1)
	stopChan := make(chan struct{}, 1)

	// Set up port forward to argocd-server service
	// Forward local port to pod port 8080 (the actual port the server listens on)
	// Note: The service exposes 443, but the pod itself listens on 8080
	serviceName := "argocd-server"
	remotePort := 8080

	// Start the port forward
	log.Debug().Msgf("Starting port forward from localhost:%d to %s:%d in namespace %s", a.portForwardLocalPort, serviceName, remotePort, a.Namespace)
	err := a.K8sClient.PortForwardToService(a.Namespace, serviceName, a.portForwardLocalPort, remotePort, readyChan, stopChan)
	if err != nil {
		a.portForwardMutex.Unlock()
		return fmt.Errorf("failed to set up port forward: %w", err)
	}

	// Mark port forward as active and store the stop channel BEFORE waiting
	// This prevents other goroutines from trying to create another port forward
	a.portForwardActive = true
	a.portForwardStopChan = stopChan
	a.portForwardMutex.Unlock()

	// Wait for port forward to be ready (outside the mutex lock)
	log.Debug().Msg("Waiting for port forward to be ready...")

	// Add timeout to prevent hanging forever
	select {
	case <-readyChan:
		log.Info().Msgf("🔌 Port forward ready: localhost:%d -> %s:%d", a.portForwardLocalPort, serviceName, remotePort)
		return nil
	case <-time.After(30 * time.Second):
		// Reset state on timeout
		log.Warn().Msg("⚠️ Timeout waiting for port forward to be ready")
		a.portForwardMutex.Lock()
		a.portForwardActive = false
		if a.portForwardStopChan != nil {
			close(a.portForwardStopChan)
			a.portForwardStopChan = nil
		}
		a.portForwardMutex.Unlock()
		return fmt.Errorf("timeout waiting for port forward to be ready")
	}
}

// StopPortForward stops the port forward to ArgoCD server if it's active
func (a *ArgoCDInstallation) StopPortForward() {
	a.portForwardMutex.Lock()
	defer a.portForwardMutex.Unlock()

	if a.portForwardActive && a.portForwardStopChan != nil {
		log.Debug().Msg("Stopping port forward to ArgoCD server...")
		close(a.portForwardStopChan)
		a.portForwardActive = false
		a.portForwardStopChan = nil
	}
}

// getToken retrieves an authentication token from the ArgoCD API (cached)
func (a *ArgoCDInstallation) getToken(password string) (string, error) {

	// Return cached token if available
	if a.authToken != "" {
		log.Debug().Msg("Using cached authentication token")
		return a.authToken, nil
	}

	log.Info().Msg("🔑 Fetching new authentication token...")

	// Set up port forward to ArgoCD server
	if err := a.portForwardToArgoCD(); err != nil {
		return "", fmt.Errorf("failed to set up port forward: %w", err)
	}

	// Prepare the login request payload
	loginData := map[string]string{
		"username": "admin",
		"password": password,
	}

	jsonData, err := json.Marshal(loginData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal login data: %w", err)
	}

	// Make the HTTP request to ArgoCD API
	// Use plain HTTP since we're connecting directly to the pod's port 8080
	url := fmt.Sprintf("%s/api/v1/session", a.apiServerURL)

	log.Info().Msgf("🌐 Making request to: %s", url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Create HTTP client with timeout and TLS config
	// ArgoCD redirects HTTP to HTTPS, so we need to handle both
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Skip certificate verification for self-signed certs
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ArgoCD API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the JSON response to extract the token
	var sessionResponse struct {
		Token string `json:"token"`
	}

	if err := json.Unmarshal(body, &sessionResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if sessionResponse.Token == "" {
		return "", fmt.Errorf("token not found in response")
	}

	// Cache the token for future use
	a.authToken = sessionResponse.Token

	log.Info().Msg("🔑 Successfully obtained and cached ArgoCD token")
	return sessionResponse.Token, nil
}

func (a *ArgoCDInstallation) updateToken(password string) error {
	token, err := a.getToken(password)
	if err != nil {
		return fmt.Errorf("failed to get initial token: %w", err)
	}
	a.authToken = token
	return nil
}

// login performs login to ArgoCD using the CLI (legacy method)
func (a *ArgoCDInstallation) login() error {
	log.Info().Msgf("🦑 Logging in to Argo CD through CLI...")

	passwd := a.Password
	// If username is "admin" and password is empty, get the initial admin password
	if a.Username == "admin" && a.Password == "" {
		log.Debug().Msg("Using default admin credentials - retrieving initial password from secret")
		initialPassword, err := a.getInitialPassword()
		if err != nil {
			return err
		}
		passwd = initialPassword
	}

	// Update the token
	err := a.updateToken(passwd)
	if err != nil {
		return fmt.Errorf("failed to get initial token: %w", err)
	}

	maxAttempts := 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Debug().Msgf("Login attempt %d/%d to Argo CD...", attempt, maxAttempts)
		out, err := a.runArgocdCommand("login", "--insecure", "--username", a.Username, "--password", passwd)
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

	return time.Since(startTime), nil
}
