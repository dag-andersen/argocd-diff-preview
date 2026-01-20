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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// RemoteArgoCD represents a connection to a remote ArgoCD instance
type RemoteArgoCD struct {
	URL        string
	Token      string
	HTTPClient *http.Client
}

// NewRemoteArgoCD creates a new RemoteArgoCD client
func NewRemoteArgoCD(url, token string, insecure bool) *RemoteArgoCD {
	// Normalize URL - remove trailing slash
	url = strings.TrimSuffix(url, "/")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Skip TLS verification if insecure mode is enabled
	if insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // User explicitly requested insecure mode
		}
		log.Debug().Msg("TLS certificate verification disabled for remote ArgoCD")
	}

	return &RemoteArgoCD{
		URL:        url,
		Token:      token,
		HTTPClient: client,
	}
}

// ApplicationListResponse represents the response from /api/v1/applications
type ApplicationListResponse struct {
	Items []ApplicationItem `json:"items"`
}

// ApplicationItem represents a single application in the list response
type ApplicationItem struct {
	Metadata ApplicationMetadata `json:"metadata"`
	Spec     ApplicationSpec     `json:"spec"`
}

// ApplicationMetadata contains application metadata
type ApplicationMetadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// ApplicationSpec contains application spec
type ApplicationSpec struct {
	Source      *ApplicationSource   `json:"source,omitempty"`
	Sources     []ApplicationSource  `json:"sources,omitempty"`
	Destination ApplicationDestination `json:"destination"`
}

// ApplicationSource contains source information
type ApplicationSource struct {
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path"`
	TargetRevision string `json:"targetRevision"`
}

// ApplicationDestination contains destination information
type ApplicationDestination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}

// ManifestsResponse represents the response from /api/v1/applications/{app}/manifests
type ManifestsResponse struct {
	Manifests []string `json:"manifests"`
}

// doRequest performs an authenticated HTTP request to the ArgoCD API
func (r *RemoteArgoCD) doRequest(method, path string) ([]byte, error) {
	url := fmt.Sprintf("%s%s", r.URL, path)

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.Token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// ListApplications returns all applications from the remote ArgoCD instance
func (r *RemoteArgoCD) ListApplications() ([]ApplicationItem, error) {
	log.Debug().Msgf("Listing applications from remote ArgoCD: %s", r.URL)

	body, err := r.doRequest("GET", "/api/v1/applications")
	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}

	var response ApplicationListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse applications response: %w", err)
	}

	log.Debug().Msgf("Found %d applications in remote ArgoCD", len(response.Items))
	return response.Items, nil
}

// FetchManifests retrieves the rendered manifests for an application
func (r *RemoteArgoCD) FetchManifests(appName string) ([]unstructured.Unstructured, error) {
	log.Debug().Msgf("Fetching manifests for application: %s", appName)

	path := fmt.Sprintf("/api/v1/applications/%s/manifests", appName)
	body, err := r.doRequest("GET", path)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifests for %s: %w", appName, err)
	}

	var response ManifestsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse manifests response: %w", err)
	}

	var manifests []unstructured.Unstructured
	for _, manifestStr := range response.Manifests {
		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(manifestStr), &obj.Object); err != nil {
			// Try JSON unmarshal as fallback
			if err := json.Unmarshal([]byte(manifestStr), &obj.Object); err != nil {
				log.Warn().Err(err).Msgf("Failed to parse manifest for %s, skipping", appName)
				continue
			}
		}
		manifests = append(manifests, obj)
	}

	log.Debug().Msgf("Fetched %d manifests for application: %s", len(manifests), appName)
	return manifests, nil
}

// LiveAppManifest represents fetched manifests for a live application
type LiveAppManifest struct {
	Name      string
	Manifests []unstructured.Unstructured
}

// FetchLiveManifestsForApps fetches manifests for multiple applications by name
func (r *RemoteArgoCD) FetchLiveManifestsForApps(appNames []string) ([]LiveAppManifest, error) {
	log.Info().Msgf("🌐 Fetching live manifests from remote ArgoCD for %d applications", len(appNames))

	var results []LiveAppManifest
	var fetchErrors []string

	for _, appName := range appNames {
		manifests, err := r.FetchManifests(appName)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to fetch manifests for %s", appName)
			fetchErrors = append(fetchErrors, appName)
			continue
		}

		results = append(results, LiveAppManifest{
			Name:      appName,
			Manifests: manifests,
		})
	}

	if len(fetchErrors) > 0 {
		log.Warn().Msgf("⚠️ Failed to fetch manifests for %d applications: %v", len(fetchErrors), fetchErrors)
	}

	log.Info().Msgf("🌐 Successfully fetched live manifests for %d applications", len(results))
	return results, nil
}

// GetApplicationByName returns a specific application by name
func (r *RemoteArgoCD) GetApplicationByName(appName string) (*ApplicationItem, error) {
	log.Debug().Msgf("Getting application by name: %s", appName)

	path := fmt.Sprintf("/api/v1/applications/%s", appName)
	body, err := r.doRequest("GET", path)
	if err != nil {
		return nil, fmt.Errorf("failed to get application %s: %w", appName, err)
	}

	var app ApplicationItem
	if err := json.Unmarshal(body, &app); err != nil {
		return nil, fmt.Errorf("failed to parse application response: %w", err)
	}

	return &app, nil
}

// MatchApplicationsByName matches target applications to live applications by exact name
// Returns a map of target app name -> live app name for apps that exist in both
func (r *RemoteArgoCD) MatchApplicationsByName(targetAppNames []string) (map[string]string, []string, error) {
	liveApps, err := r.ListApplications()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list live applications: %w", err)
	}

	// Build a set of live app names
	liveAppSet := make(map[string]bool)
	for _, app := range liveApps {
		liveAppSet[app.Metadata.Name] = true
	}

	// Match target apps to live apps
	matches := make(map[string]string)
	var newApps []string

	for _, targetName := range targetAppNames {
		if liveAppSet[targetName] {
			matches[targetName] = targetName
		} else {
			newApps = append(newApps, targetName)
		}
	}

	log.Info().Msgf("🔗 Matched %d applications to live state, %d new applications", len(matches), len(newApps))
	return matches, newApps, nil
}
