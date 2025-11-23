package argocd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

// portForwardToArgoCD sets up a port forward to the ArgoCD server if not already active
func (a *ArgoCDInstallation) portForwardToArgoCD() error {

	connection := a.ArgoCDApiConnection

	connection.portForwardMutex.Lock()

	// Check if port forward is already active
	if connection.portForwardActive {
		connection.portForwardMutex.Unlock()
		log.Debug().Msg("Port forward to ArgoCD server is already active, reusing existing connection")
		return nil
	}

	log.Debug().Msg("🔌 Setting up port forward to ArgoCD server...")

	// Create channels for coordination
	readyChan := make(chan struct{}, 1)
	stopChan := make(chan struct{}, 1)

	// Set up port forward to argocd-server service
	// Forward local port to pod port 8080 (the actual port the server listens on)
	// Note: The service exposes 443, but the pod itself listens on 8080
	// Discover the service by label "app.kubernetes.io/component=server"
	labelSelector := "app.kubernetes.io/component=server"
	serviceName, err := a.K8sClient.GetServiceNameByLabel(a.Namespace, labelSelector)
	if err != nil {
		connection.portForwardMutex.Unlock()
		return fmt.Errorf("failed to find ArgoCD server service with label %s: %w", labelSelector, err)
	}

	// Start the port forward
	log.Debug().Msgf("Starting port forward from localhost:%d to %s:%d in namespace %s", connection.portForwardLocalPort, serviceName, remotePort, a.Namespace)
	if err := a.K8sClient.PortForwardToService(a.Namespace, serviceName, connection.portForwardLocalPort, remotePort, readyChan, stopChan); err != nil {
		connection.portForwardMutex.Unlock()
		return fmt.Errorf("failed to set up port forward: %w", err)
	}

	// Mark port forward as active and store the stop channel BEFORE waiting
	// This prevents other goroutines from trying to create another port forward
	connection.portForwardActive = true
	connection.portForwardStopChan = stopChan
	connection.portForwardMutex.Unlock()

	// Wait for port forward to be ready (outside the mutex lock)
	log.Debug().Msg("Waiting for port forward to be ready...")

	// Add timeout to prevent hanging forever
	select {
	case <-readyChan:
		log.Debug().Msgf("🔌 Port forward ready: localhost:%d -> %s:%d", connection.portForwardLocalPort, serviceName, remotePort)
		return nil
	case <-time.After(30 * time.Second):
		// Reset state on timeout
		log.Warn().Msg("⚠️ Timeout waiting for port forward to be ready")
		connection.portForwardMutex.Lock()
		connection.portForwardActive = false
		if connection.portForwardStopChan != nil {
			close(connection.portForwardStopChan)
			connection.portForwardStopChan = nil
		}
		connection.portForwardMutex.Unlock()
		return fmt.Errorf("timeout waiting for port forward to be ready")
	}
}

// StopPortForward stops the port forward to ArgoCD server if it's active
func (a *ArgoCDInstallation) StopPortForward() {
	connection := a.ArgoCDApiConnection
	connection.portForwardMutex.Lock()
	defer connection.portForwardMutex.Unlock()

	if connection.portForwardActive && connection.portForwardStopChan != nil {
		log.Debug().Msg("Stopping port forward to ArgoCD server...")
		close(connection.portForwardStopChan)
		connection.portForwardActive = false
		connection.portForwardStopChan = nil
	}
}

// GetManifests returns the manifests for an application using the ArgoCD API
func (a *ArgoCDInstallation) GetManifestsFromAPI(appName string) (string, error) {
	// Ensure port forward is active
	if err := a.portForwardToArgoCD(); err != nil {
		return "", err
	}

	// Make API request to get manifests
	url := fmt.Sprintf("%s/api/v1/applications/%s/manifests", a.ArgoCDApiConnection.apiServerURL, appName)

	log.Debug().Msgf("Getting manifests for app '%s' from API: %s", appName, url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set authorization header with bearer token
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.ArgoCDApiConnection.authToken))

	// Create HTTP client with TLS config to handle redirects
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		// Check if app exists
		exists, _ := a.K8sClient.CheckIfResourceExists(ApplicationGVR, a.Namespace, appName)
		if !exists {
			log.Warn().Msgf("App %s does not exist", appName)
		}
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode == 404 {
		log.Warn().Msgf("App %s does not exist (404)", appName)
		return "", fmt.Errorf("application not found: %s", appName)
	}

	if resp.StatusCode != http.StatusOK {

		var response struct {
			Error string `json:"error"`
		}

		if err := json.Unmarshal(body, &response); err == nil {
			return "", fmt.Errorf("ArgoCD API returned error: %s", response.Error)
		}

		return "", fmt.Errorf("ArgoCD API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response to extract manifests
	// The API returns manifests as an array of JSON strings, not objects
	var manifestResponse struct {
		Manifests []string `json:"manifests"`
	}

	// if body is empty, it means the application is empty
	if strings.TrimSpace(string(body)) == "{}" {
		log.Warn().Msgf("⚠️ Application is empty: %s", appName)
		return "", nil
	}

	if err := json.Unmarshal(body, &manifestResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal manifests response: %w", err)
	}

	if len(manifestResponse.Manifests) == 0 {
		return "", fmt.Errorf("no manifests found for app %s", appName)
	}

	// Convert manifests to YAML format with --- separators
	// Each manifest is already a JSON string, we need to convert each to YAML
	var manifestsYAML strings.Builder
	for i, manifestStr := range manifestResponse.Manifests {
		// The manifest is a JSON string, convert it to YAML
		manifestYAML, err := yaml.JSONToYAML([]byte(manifestStr))
		if err != nil {
			return "", fmt.Errorf("failed to convert manifest %d to YAML: %w", i, err)
		}

		// Write separator between manifests (except for the first one)
		if i > 0 {
			manifestsYAML.WriteString("---\n")
		}

		// Write the YAML manifest
		manifestsYAML.Write(manifestYAML)
	}

	log.Debug().Msgf("Successfully retrieved %d manifests for app %s", len(manifestResponse.Manifests), appName)
	return manifestsYAML.String(), nil
}
