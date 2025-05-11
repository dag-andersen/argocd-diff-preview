package extract

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	argocdPkg "github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
)

// Error and timeout messages that we look for in application status
var errorMessages = []string{
	"helm template .",
	"authentication required",
	"authentication failed",
	"path does not exist",
	"error converting YAML to JSON",
	"Unknown desc = `helm template .",
	"Unknown desc = `kustomize build",
	"Unknown desc = Unable to resolve",
	"is not a valid chart repository or cannot be reached",
	"Unknown desc = repository not found",
	"to a commit SHA",
}

var timeoutMessages = []string{
	"Client.Timeout",
	"failed to get git client for repo",
	"rpc error: code = Unknown desc = Get \"https",
	"i/o timeout",
	"Could not resolve host: github.com",
	":8081: connect: connection refused",
	"Temporary failure in name resolution",
	"=git-upload-pack",
}

// contains a app name, source path, and extracted manifest
type ExtractedApp struct {
	Id         string
	Name       string
	SourcePath string
	Manifest   string
}

// GetResourcesFromBothBranches extracts resources from both base and target branches
// by applying their manifests to the cluster and capturing the resulting resources
func GetResourcesFromBothBranches(
	argocd *argocdPkg.ArgoCDInstallation,
	baseBranch *git.Branch,
	targetBranch *git.Branch,
	timeout uint64,
	baseApps []argoapplication.ArgoResource,
	targetApps []argoapplication.ArgoResource,
) ([]ExtractedApp, []ExtractedApp, error) {

	extractedBasedApps, err := getResourcesFromApps(argocd, baseBranch, baseApps, timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get resources: %w", err)
	}
	log.Debug().Str("branch", baseBranch.Name).Msg("Extracted manifests")

	// delete applications
	if err := argocd.K8sClient.DeleteArgoCDApplications(argocd.Namespace); err != nil {
		return nil, nil, fmt.Errorf("failed to delete applications: %w", err)
	}

	log.Debug().Str("branch", targetBranch.Name).Msg("Applied manifest")
	extractedTargetApps, err := getResourcesFromApps(argocd, targetBranch, targetApps, timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get resources: %w", err)
	}
	log.Debug().Str("branch", targetBranch.Name).Msg("Extracted manifests")

	return extractedBasedApps, extractedTargetApps, nil
}

// getResourcesFromApps extracts resources from Argo CD for a specific branch as ExtractedApp structs
func getResourcesFromApps(
	argocd *argocdPkg.ArgoCDInstallation,
	branch *git.Branch,
	apps []argoapplication.ArgoResource,
	timeout uint64,
) ([]ExtractedApp, error) {
	log.Info().Str("branch", branch.Name).Msg("🤖 Getting Applications from branch")

	// ensure that no apps have the same name. Fail if they do
	appNames := make(map[string]bool)
	for _, app := range apps {
		if appNames[app.Id] {
			return nil, fmt.Errorf("duplicate app name: %s - Please open an issue on GitHub", app.Id)
		}
		appNames[app.Id] = true
	}

	// Process apps in parallel
	results := make(chan struct {
		app ExtractedApp
		err error
	}, len(apps))

	for _, app := range apps {
		go func(app argoapplication.ArgoResource) {
			result, err := getResourcesFromApp(argocd, app, timeout)
			results <- struct {
				app ExtractedApp
				err error
			}{app: result, err: err}
		}(app)
	}

	// Collect results
	extractedApps := make([]ExtractedApp, 0, len(apps))
	var firstError error

	for i := 0; i < len(apps); i++ {
		result := <-results
		if result.err != nil {
			if firstError == nil {
				firstError = result.err
			}
			log.Error().Err(result.err).Msg("Failed to extract app")
			continue
		}
		extractedApps = append(extractedApps, result.app)
	}

	if firstError != nil {
		return nil, firstError
	}

	log.Info().Str("branch", branch.Name).Msgf("🤖 Got all resources from %d applications", len(extractedApps))

	return extractedApps, nil
}

// getResourcesFromApp extracts a single application from the cluster
func getResourcesFromApp(argocd *argocdPkg.ArgoCDInstallation, app argoapplication.ArgoResource, timeout uint64) (ExtractedApp, error) {
	// Apply the application manifest first
	unstructured, err := app.AsUnstructured()
	if err != nil {
		return ExtractedApp{}, fmt.Errorf("failed to convert app to unstructured: %w", err)
	}

	if err := argocd.K8sClient.ApplyManifest(unstructured, "string", argocd.Namespace); err != nil {
		return ExtractedApp{}, fmt.Errorf("failed to apply manifest for application %s: %w", app.GetLongName(), err)
	}

	log.Debug().Str("name", app.GetLongName()).Msg("Applied manifest for application")

	startTime := time.Now()
	var result ExtractedApp

	for {
		// Check if we've exceeded timeout
		if time.Since(startTime).Seconds() > float64(timeout) {
			return result, fmt.Errorf("timed out waiting for application %s", app.GetLongName())
		}

		// Get application status
		output, err := argocd.K8sClient.GetArgoCDApplication(argocd.Namespace, app.Id)
		if err != nil {
			return result, fmt.Errorf("failed to get application %s: %w", app.GetLongName(), err)
		}

		var appStatus struct {
			Status struct {
				Sync struct {
					Status string `yaml:"status"`
				} `yaml:"sync"`
				Conditions []struct {
					Type    string `yaml:"type"`
					Message string `yaml:"message"`
				} `yaml:"conditions"`
			} `yaml:"status"`
		}

		if err := yaml.Unmarshal([]byte(output), &appStatus); err != nil {
			return result, fmt.Errorf("failed to parse application yaml for %s: %w", app.GetLongName(), err)
		}

		switch appStatus.Status.Sync.Status {
		case "OutOfSync", "Synced":
			log.Debug().Str("name", app.GetLongName()).Msg("Extracting manifests from Application")
			manifests, exists, err := argocd.GetManifests(app.Id)
			if !exists {
				return result, fmt.Errorf("application %s does not exist", app.GetLongName())
			}

			if err != nil {
				return result, fmt.Errorf("failed to get manifests for application %s: %w", app.GetLongName(), err)
			}

			log.Debug().Str("name", app.GetLongName()).Msg("Extracted manifests from Application")

			return ExtractedApp{
				Id:         app.Id,
				Name:       app.Name,
				SourcePath: app.FileName,
				Manifest:   manifests,
			}, nil

		case "Unknown":
			for _, condition := range appStatus.Status.Conditions {
				if isErrorCondition(condition.Type) {
					msg := condition.Message
					if containsAny(msg, errorMessages) {
						return result, fmt.Errorf("application %s failed: %s", app.Name, msg)
					} else if containsAny(msg, timeoutMessages) {
						log.Warn().Str("App", app.GetLongName()).Msgf("⚠️ Application timed out with error: %s", msg)
						if err := argocd.RefreshApp(app.Id); err != nil {
							log.Error().Err(err).Str("App", app.GetLongName()).Msg("⚠️ Failed to refresh application")
						} else {
							log.Info().Str("App", app.GetLongName()).Msg("🔄 Refreshed application")
						}
					} else {
						log.Error().Str("App", app.GetLongName()).Msgf("❌ Application failed with error: %s", msg)
						return result, fmt.Errorf("application %s failed: %s", app.Name, msg)
					}
				}
			}
		}

		// Sleep before next iteration
		time.Sleep(5 * time.Second)
	}
}

func isErrorCondition(condType string) bool {
	return condType != "" && containsIgnoreCase(condType, "error")
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if s != "" && strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
