package extract

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"

	"github.com/dag-andersen/argocd-diff-preview/pkg/annotations"
	argocdPkg "github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
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
	baseManifest string,
	targetManifest string,
	debug bool,
) ([]ExtractedApp, []ExtractedApp, error) {
	// Apply base manifest directly from string with kubectl
	if _, err := argocd.K8sClient.ApplyManifestFromString(baseManifest, argocd.Namespace); err != nil {
		return nil, nil, fmt.Errorf("failed to apply base apps: %w", err)
	}

	log.Debug().Str("branch", baseBranch.Name).Msg("Applied manifest")

	baseApps, err := extractResourcesFromClusterAsApps(argocd, baseBranch, timeout, debug)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get resources: %w", err)
	}

	log.Debug().Str("branch", baseBranch.Name).Msg("Extracted manifests")

	// delete applications
	if err := argocd.K8sClient.DeleteArgoCDApplications(argocd.Namespace); err != nil {
		return nil, nil, fmt.Errorf("failed to delete applications: %w", err)
	}

	// apply target manifest
	if _, err := argocd.K8sClient.ApplyManifestFromString(targetManifest, argocd.Namespace); err != nil {
		return nil, nil, fmt.Errorf("failed to apply target apps: %w", err)
	}

	log.Debug().Str("branch", targetBranch.Name).Msg("Applied manifest")
	targetApps, err := extractResourcesFromClusterAsApps(argocd, targetBranch, timeout, debug)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get resources: %w", err)
	}

	log.Debug().Str("branch", targetBranch.Name).Msg("Extracted manifests")
	return baseApps, targetApps, nil
}

// extractResourcesFromClusterAsApps extracts resources from Argo CD for a specific branch as ExtractedApp structs
func extractResourcesFromClusterAsApps(
	argocd *argocdPkg.ArgoCDInstallation,
	branch *git.Branch,
	timeout uint64,
	debug bool,
) ([]ExtractedApp, error) {
	log.Info().Str("branch", branch.Name).Msg("🤖 Getting resources from branch")

	// Create a slice to store all extracted apps
	extractedApps := make([]ExtractedApp, 0)

	processedApps := make(map[string]bool)
	failedApps := make(map[string]string)
	startTime := time.Now()

	for {
		// Get all applications
		var yamlOutput struct {
			Items []struct {
				Metadata struct {
					Name string `yaml:"name"`
				} `yaml:"metadata"`
				Status struct {
					Sync struct {
						Status string `yaml:"status"`
					} `yaml:"sync"`
					Conditions []struct {
						Type    string `yaml:"type"`
						Message string `yaml:"message"`
					} `yaml:"conditions"`
				} `yaml:"status"`
			} `yaml:"items"`
		}

		output, err := argocd.K8sClient.GetArgoCDApplications(argocd.Namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get applications: %v", err)
		}

		if err := yaml.Unmarshal([]byte(output), &yamlOutput); err != nil {
			return nil, fmt.Errorf("failed to parse applications yaml: %v", err)
		}

		if len(yamlOutput.Items) == 0 || len(yamlOutput.Items) == len(processedApps) {
			break
		}

		var timedOutApps []string
		var otherErrors []struct{ name, msg string }
		appsLeft := 0

		if debug {
			if err := argocd.EnsureArgoCdIsReady(); err != nil {
				return nil, fmt.Errorf("failed to wait for deployments to be ready: %w", err)
			}
		}

		// Process each application
		for _, item := range yamlOutput.Items {
			name := item.Metadata.Name
			if processedApps[name] {
				continue
			}

			switch item.Status.Sync.Status {
			case "OutOfSync", "Synced":
				log.Debug().Str("name", name).Msg("Extracting manifests from Application")
				manifests, err := argocd.GetManifests(name)
				if err != nil {
					log.Error().Msgf("❌ Failed to get manifests for application: %s, error: %v", name, err)
					failedApps[name] = err.Error()
					continue
				}

				sourcePath, err := getApplicationSourcePath(argocd.K8sClient, argocd.Namespace, name)
				if err != nil {
					log.Error().Msgf("❌ Failed to get source path for application: %s, error: %v", name, err)
					sourcePath = "Unknown"
				}

				originalApplicationName, err := getOriginalApplicationName(argocd.K8sClient, argocd.Namespace, name)
				if err != nil {
					log.Error().Msgf("❌ Failed to get original application name for application: %s, error: %v", name, err)
					originalApplicationName = "Unknown"
				}

				log.Debug().Str("branch", branch.Name).Str("name", originalApplicationName).Str("id", name).Str("path", sourcePath).Msg("Extracted manifests from Application")

				// Create an ExtractedApp and add to our slice
				app := ExtractedApp{
					Id:         name,
					Name:       originalApplicationName,
					SourcePath: sourcePath,
					Manifest:   manifests,
				}
				extractedApps = append(extractedApps, app)
				processedApps[name] = true

			case "Unknown":
				for _, condition := range item.Status.Conditions {
					if isErrorCondition(condition.Type) {
						msg := condition.Message
						if containsAny(msg, errorMessages) {
							failedApps[name] = msg
						} else if containsAny(msg, timeoutMessages) {
							log.Warn().Msgf("⚠️ Application: %s timed out with error: %s", name, msg)
							timedOutApps = append(timedOutApps, name)
							otherErrors = append(otherErrors, struct{ name, msg string }{name, msg})
						} else {
							log.Error().Msgf("❌ Application: %s failed with error: %s", name, msg)
							otherErrors = append(otherErrors, struct{ name, msg string }{name, msg})
						}
					}
				}
			}
			appsLeft++
		}

		// Handle errors
		if len(failedApps) > 0 {
			for name, msg := range failedApps {
				log.Error().Msgf("❌ Failed to process application: %s with error: \n%s", name, msg)
			}
			return nil, fmt.Errorf("failed to process applications")
		}

		// Handle timeouts
		if time.Since(startTime).Seconds() > float64(timeout) {
			log.Error().Msgf("❌ Timed out after %d seconds", timeout)
			log.Info().Msgf("❌ Processed %d applications, but %d applications still remain",
				len(processedApps), appsLeft)
			if len(otherErrors) > 0 {
				log.Error().Msg("❌ Applications with 'ComparisonError' errors:")
				for _, err := range otherErrors {
					log.Error().Msgf("❌ %s, %s", err.name, err.msg)
				}
			}
			return nil, fmt.Errorf("timed out")
		}

		// Handle timed out apps
		if len(timedOutApps) > 0 {
			log.Info().Msgf("💤 %d Applications timed out", len(timedOutApps))
			for _, app := range timedOutApps {
				if err := argocd.RefreshApp(app); err != nil {
					log.Error().Msgf("⚠️ Failed to refresh application: %s with %v", app, err)
				} else {
					log.Info().Msgf("🔄 Refreshing application: %s", app)
				}
			}
		}

		// Sleep before next iteration
		time.Sleep(5 * time.Second)
	}

	log.Info().Str("branch", branch.Name).Msgf("🤖 Got all resources from %d applications in %s", len(processedApps), time.Since(startTime).Round(time.Second))

	return extractedApps, nil
}

func getApplicationSourcePath(k8sClient *utils.K8sClient, namespace string, appName string) (string, error) {
	return k8sClient.GetResourceAnnotation(argocdPkg.ApplicationGVR, namespace, appName, annotations.SourcePathKey)
}

func getOriginalApplicationName(k8sClient *utils.K8sClient, namespace string, appName string) (string, error) {
	return k8sClient.GetResourceAnnotation(argocdPkg.ApplicationGVR, namespace, appName, annotations.OriginalApplicationNameKey)
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
