package argocd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRemoteArgoCD(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		token    string
		wantURL  string
	}{
		{
			name:    "URL without trailing slash",
			url:     "https://argocd.example.com",
			token:   "test-token",
			wantURL: "https://argocd.example.com",
		},
		{
			name:    "URL with trailing slash",
			url:     "https://argocd.example.com/",
			token:   "test-token",
			wantURL: "https://argocd.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewRemoteArgoCD(tt.url, tt.token, false)
			assert.Equal(t, tt.wantURL, client.URL)
			assert.Equal(t, tt.token, client.Token)
			assert.NotNil(t, client.HTTPClient)
		})
	}
}

func TestRemoteArgoCD_ListApplications(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		assert.Equal(t, "/api/v1/applications", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Return mock response
		response := ApplicationListResponse{
			Items: []ApplicationItem{
				{
					Metadata: ApplicationMetadata{
						Name:      "app1",
						Namespace: "argocd",
					},
					Spec: ApplicationSpec{
						Destination: ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					},
				},
				{
					Metadata: ApplicationMetadata{
						Name:      "app2",
						Namespace: "argocd",
					},
					Spec: ApplicationSpec{
						Destination: ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "production",
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewRemoteArgoCD(server.URL, "test-token", false)
	apps, err := client.ListApplications()

	require.NoError(t, err)
	assert.Len(t, apps, 2)
	assert.Equal(t, "app1", apps[0].Metadata.Name)
	assert.Equal(t, "app2", apps[1].Metadata.Name)
}

func TestRemoteArgoCD_FetchManifests(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		assert.Equal(t, "/api/v1/applications/test-app/manifests", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return mock response with JSON manifests
		response := ManifestsResponse{
			Manifests: []string{
				`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test-deployment"}}`,
				`{"apiVersion":"v1","kind":"Service","metadata":{"name":"test-service"}}`,
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewRemoteArgoCD(server.URL, "test-token", false)
	manifests, err := client.FetchManifests("test-app")

	require.NoError(t, err)
	assert.Len(t, manifests, 2)
	assert.Equal(t, "Deployment", manifests[0].GetKind())
	assert.Equal(t, "test-deployment", manifests[0].GetName())
	assert.Equal(t, "Service", manifests[1].GetKind())
	assert.Equal(t, "test-service", manifests[1].GetName())
}

func TestRemoteArgoCD_FetchManifests_Error(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"application not found"}`))
	}))
	defer server.Close()

	client := NewRemoteArgoCD(server.URL, "test-token", false)
	_, err := client.FetchManifests("nonexistent-app")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestRemoteArgoCD_MatchApplicationsByName(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := ApplicationListResponse{
			Items: []ApplicationItem{
				{Metadata: ApplicationMetadata{Name: "app1"}},
				{Metadata: ApplicationMetadata{Name: "app2"}},
				{Metadata: ApplicationMetadata{Name: "app3"}},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewRemoteArgoCD(server.URL, "test-token", false)

	targetApps := []string{"app1", "app2", "new-app"}
	matches, newApps, err := client.MatchApplicationsByName(targetApps)

	require.NoError(t, err)

	// Should match app1 and app2
	assert.Len(t, matches, 2)
	assert.Equal(t, "app1", matches["app1"])
	assert.Equal(t, "app2", matches["app2"])

	// new-app should be identified as new
	assert.Len(t, newApps, 1)
	assert.Equal(t, "new-app", newApps[0])
}

func TestRemoteArgoCD_FetchLiveManifestsForApps(t *testing.T) {
	callCount := 0
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Return different manifests based on the app name
		response := ManifestsResponse{
			Manifests: []string{
				`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm-` + r.URL.Path + `"}}`,
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewRemoteArgoCD(server.URL, "test-token", false)
	results, err := client.FetchLiveManifestsForApps([]string{"app1", "app2"})

	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, 2, callCount)

	// Verify the results contain the expected app names
	appNames := make([]string, len(results))
	for i, r := range results {
		appNames[i] = r.Name
	}
	assert.Contains(t, appNames, "app1")
	assert.Contains(t, appNames, "app2")
}

func TestRemoteArgoCD_GetApplicationByName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/applications/test-app", r.URL.Path)

		app := ApplicationItem{
			Metadata: ApplicationMetadata{
				Name:      "test-app",
				Namespace: "argocd",
			},
			Spec: ApplicationSpec{
				Source: &ApplicationSource{
					RepoURL:        "https://github.com/example/repo",
					Path:           "manifests",
					TargetRevision: "main",
				},
				Destination: ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "default",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(app)
	}))
	defer server.Close()

	client := NewRemoteArgoCD(server.URL, "test-token", false)
	app, err := client.GetApplicationByName("test-app")

	require.NoError(t, err)
	assert.Equal(t, "test-app", app.Metadata.Name)
	assert.Equal(t, "https://github.com/example/repo", app.Spec.Source.RepoURL)
}
