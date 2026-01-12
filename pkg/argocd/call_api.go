package argocd

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

// grpcGatewayError represents the error format returned by gRPC-gateway
// when ArgoCD API calls fail. This matches the structure from
// github.com/grpc-ecosystem/grpc-gateway/runtime.
type grpcGatewayError struct {
	Error   string `json:"error,omitempty"`
	Code    int32  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// parseAPIError attempts to parse an error response body into a human-readable message.
// It handles the gRPC-gateway error format used by ArgoCD's API.
func parseAPIError(body []byte, statusCode int) error {
	var errResp grpcGatewayError
	if err := json.Unmarshal(body, &errResp); err == nil {
		// Prefer "message" field (more detailed), fall back to "error" field
		if errResp.Message != "" {
			return fmt.Errorf("ArgoCD API error (code %d): %s", errResp.Code, errResp.Message)
		}
		if errResp.Error != "" {
			return fmt.Errorf("ArgoCD API error: %s", errResp.Error)
		}
	}
	// Fallback to raw response if parsing fails
	return fmt.Errorf("ArgoCD API returned status %d: %s", statusCode, string(body))
}

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

// loginViaAPI performs login to ArgoCD using the HTTP API
func (a *ArgoCDInstallation) loginViaAPI() error {
	log.Info().Msg("🦑 Logging in to Argo CD via API...")

	// Get initial admin password
	password, err := a.getInitialPassword()
	if err != nil {
		return err
	}

	username := "admin"

	// Set up port forward to ArgoCD server
	if err := a.portForwardToArgoCD(); err != nil {
		return fmt.Errorf("failed to set up port forward: %w", err)
	}

	// Prepare the login request payload
	loginData := map[string]string{
		"username": username,
		"password": password,
	}

	jsonData, err := json.Marshal(loginData)
	if err != nil {
		return fmt.Errorf("failed to marshal login data: %w", err)
	}

	// Make the HTTP request to ArgoCD API
	url := fmt.Sprintf("%s/api/v1/session", a.ArgoCDApiConnection.apiServerURL)

	log.Debug().Msgf("Making login request to: %s", url)

	// Create HTTP client with timeout and TLS config
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	maxAttempts := 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Debug().Msgf("Login attempt %d/%d to Argo CD via API...", attempt, maxAttempts)

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			if attempt >= maxAttempts {
				return fmt.Errorf("failed to make HTTP request after %d attempts: %w", maxAttempts, err)
			}
			log.Warn().Err(err).Msgf("Login attempt %d/%d failed, retrying...", attempt, maxAttempts)
			time.Sleep(1 * time.Second)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			if attempt >= maxAttempts {
				return fmt.Errorf("login failed after %d attempts: %w", maxAttempts, parseAPIError(body, resp.StatusCode))
			}
			log.Warn().Msgf("Login attempt %d/%d failed with status %d, retrying...", attempt, maxAttempts, resp.StatusCode)
			time.Sleep(1 * time.Second)
			continue
		}

		// Parse the JSON response to extract the token
		var sessionResponse struct {
			Token string `json:"token"`
		}

		if err := json.Unmarshal(body, &sessionResponse); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}

		if sessionResponse.Token == "" {
			return fmt.Errorf("token not found in response")
		}

		// Cache the token for future use
		a.ArgoCDApiConnection.authToken = sessionResponse.Token

		log.Debug().Msgf("Login successful on attempt %d", attempt)
		log.Info().Msg("🔑 Successfully obtained ArgoCD token via API")
		return nil
	}

	return fmt.Errorf("failed to login after %d attempts", maxAttempts)
}

// getManifestsAPI returns the manifests for an application using the ArgoCD API.
// It uses the /manifests endpoint which fetches and renders manifests directly from
// the source (Git/Helm) without requiring cluster sync permissions.
// This is preferred over /managed-resources for diff preview because:
// 1. It works in locked-down clusters without cluster-level RBAC
// 2. It returns freshly rendered manifests from the source
// 3. It doesn't require the application to have been synced first
func (a *ArgoCDInstallation) getManifestsAPI(appName string) (string, bool, error) {
	// Ensure port forward is active
	if err := a.portForwardToArgoCD(); err != nil {
		return "", false, err
	}

	// Use /manifests endpoint - fetches and renders manifests directly from Git/Helm
	// This works even in locked-down mode where /managed-resources would fail
	url := fmt.Sprintf("%s/api/v1/applications/%s/manifests", a.ArgoCDApiConnection.apiServerURL, appName)

	log.Debug().Msgf("Getting manifests for app '%s' from API: %s", appName, url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", false, fmt.Errorf("failed to create HTTP request: %w", err)
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
		return "", exists, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode == 404 {
		log.Warn().Msgf("App %s does not exist (404)", appName)
		return "", false, fmt.Errorf("application not found: %s", appName)
	}

	if resp.StatusCode != http.StatusOK {
		return "", true, parseAPIError(body, resp.StatusCode)
	}

	// Parse JSON response using the official ArgoCD ManifestResponse type
	var manifestsResponse apiclient.ManifestResponse

	// if body is empty or just {}, the application has no manifests
	if strings.TrimSpace(string(body)) == "{}" {
		log.Debug().Msgf("Application has no manifests: %s", appName)
		return "", true, nil
	}

	if err := json.Unmarshal(body, &manifestsResponse); err != nil {
		return "", true, fmt.Errorf("failed to unmarshal manifests response: %w", err)
	}

	if len(manifestsResponse.Manifests) == 0 {
		log.Debug().Msgf("No manifests found for app %s", appName)
		return "", true, nil
	}

	// Convert each JSON manifest to YAML format with --- separators
	var manifestsYAML strings.Builder
	for i, manifestJSON := range manifestsResponse.Manifests {
		// Convert JSON to YAML
		manifestYAML, err := yaml.JSONToYAML([]byte(manifestJSON))
		if err != nil {
			return "", true, fmt.Errorf("failed to convert manifest %d to YAML: %w", i, err)
		}

		// Write separator between manifests (except for the first one)
		if i > 0 {
			manifestsYAML.WriteString("---\n")
		}

		// Write the YAML manifest
		manifestsYAML.Write(manifestYAML)
	}

	log.Debug().Msgf("Successfully retrieved %d manifests for app %s (revision: %s, sourceType: %s)",
		len(manifestsResponse.Manifests), appName, manifestsResponse.Revision, manifestsResponse.SourceType)
	return manifestsYAML.String(), true, nil
}

// appsetGenerateAPI generates applications from an ApplicationSet using the ArgoCD API
func (a *ArgoCDInstallation) appsetGenerateAPI(appSetPath string) (string, error) {
	// Ensure port forward is active
	if err := a.portForwardToArgoCD(); err != nil {
		return "", err
	}

	// Read the ApplicationSet file
	appSetBytes, err := os.ReadFile(appSetPath)
	if err != nil {
		return "", fmt.Errorf("failed to read ApplicationSet file: %w", err)
	}

	// Parse the ApplicationSet YAML to JSON for the API request
	var appSetObj map[string]any
	if err := yaml.Unmarshal(appSetBytes, &appSetObj); err != nil {
		return "", fmt.Errorf("failed to parse ApplicationSet YAML: %w", err)
	}

	// Wrap the ApplicationSet in the request structure
	requestBody := map[string]any{
		"applicationSet": appSetObj,
	}

	// Convert to JSON for the API request
	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Make API request to generate applications
	url := fmt.Sprintf("%s/api/v1/applicationsets/generate", a.ArgoCDApiConnection.apiServerURL)

	log.Debug().Msgf("Generating ApplicationSet from API: %s", url)

	req, err := http.NewRequest("POST", url, strings.NewReader(string(requestJSON)))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.ArgoCDApiConnection.authToken))
	req.Header.Set("Content-Type", "application/json")

	// Create HTTP client with TLS config
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", parseAPIError(body, resp.StatusCode)
	}

	// Parse JSON response to extract generated applications
	var generateResponse struct {
		Applications []json.RawMessage `json:"applications"`
	}

	if err := json.Unmarshal(body, &generateResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal generate response: %w", err)
	}

	if len(generateResponse.Applications) == 0 {
		log.Debug().Msg("No applications generated from ApplicationSet")
		return "", nil
	}

	// Convert applications to a YAML array format (matching CLI output format)
	// The CLI outputs applications as a YAML array starting with "- apiVersion: ..."
	var apps []map[string]any
	for i, appJSON := range generateResponse.Applications {
		var app map[string]any
		if err := json.Unmarshal(appJSON, &app); err != nil {
			return "", fmt.Errorf("failed to unmarshal application %d: %w", i, err)
		}

		// Backfill apiVersion and kind because the API doesn't return these fields
		// This matches what the argocd CLI does in cmd/argocd/commands/applicationset.go
		if _, ok := app["apiVersion"]; !ok {
			app["apiVersion"] = "argoproj.io/v1alpha1"
		}
		if _, ok := app["kind"]; !ok {
			app["kind"] = "Application"
		}

		apps = append(apps, app)
	}

	// Convert to YAML array format
	appsYAML, err := yaml.Marshal(apps)
	if err != nil {
		return "", fmt.Errorf("failed to marshal applications to YAML: %w", err)
	}

	log.Debug().Msgf("Successfully generated %d applications from ApplicationSet", len(generateResponse.Applications))
	return string(appsYAML), nil
}

// 	curl -X PUT \
//   -H "Authorization: Bearer $ARGOCD_TOKEN" \
//   -H "Content-Type: application/json" \
//   https://argocd.example.com/api/v1/projects/default \
//   -d '{
//     "project": {
//       "metadata": {
//         "name": "default"
//       },
//       "spec": {
//         "sourceNamespaces": ["*"],
//         ... other existing spec fields ...
//       }
//     }
//   }'

func addSourceNamespaceToDefaultAppProjectAPI(a *ArgoCDInstallation) error {
	// Ensure port forward is active
	if err := a.portForwardToArgoCD(); err != nil {
		return fmt.Errorf("failed to set up port forward: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/projects/default", a.ArgoCDApiConnection.apiServerURL)

	log.Debug().Msg("Getting current default AppProject configuration")

	// Create HTTP client with TLS config
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	// GET the current project
	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %w", err)
	}
	getReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.ArgoCDApiConnection.authToken))

	getResp, err := client.Do(getReq)
	if err != nil {
		return fmt.Errorf("failed to get default project: %w", err)
	}
	defer func() { _ = getResp.Body.Close() }()

	getBody, err := io.ReadAll(getResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read GET response body: %w", err)
	}

	if getResp.StatusCode != http.StatusOK {
		return parseAPIError(getBody, getResp.StatusCode)
	}

	// Parse the current project into typed struct
	var project v1alpha1.AppProject
	if err := json.Unmarshal(getBody, &project); err != nil {
		return fmt.Errorf("failed to unmarshal project: %w", err)
	}

	// Update sourceNamespaces to allow all namespaces
	project.Spec.SourceNamespaces = []string{"*"}

	log.Debug().Msgf("Updating default AppProject with sourceNamespaces: [*] (resourceVersion: %s)", project.ResourceVersion)

	// Wrap the project in the request structure expected by the API
	// The ArgoCD API expects: {"project": <AppProject>}
	requestBody := map[string]any{
		"project": project,
	}

	// Marshal the request body
	updatedProjectJSON, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal updated project: %w", err)
	}

	// PUT the updated project
	putReq, err := http.NewRequest("PUT", url, bytes.NewBuffer(updatedProjectJSON))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
	}
	putReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.ArgoCDApiConnection.authToken))
	putReq.Header.Set("Content-Type", "application/json")

	putResp, err := client.Do(putReq)
	if err != nil {
		return fmt.Errorf("failed to update default project: %w", err)
	}
	defer func() { _ = putResp.Body.Close() }()

	putBody, err := io.ReadAll(putResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read PUT response body: %w", err)
	}

	if putResp.StatusCode != http.StatusOK {
		return parseAPIError(putBody, putResp.StatusCode)
	}

	log.Debug().Msg("Successfully updated default AppProject with sourceNamespaces")
	return nil
}
