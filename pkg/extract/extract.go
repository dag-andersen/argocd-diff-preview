package extract

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/types"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"gopkg.in/yaml.v3"
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

// GetResourcesFromBothBranches extracts resources from both base and target branches
// by applying their manifests to the cluster and capturing the resulting resources
func GetResourcesFromBothBranches(
	argocd *argocd.ArgoCDInstallation,
	baseBranch *types.Branch,
	targetBranch *types.Branch,
	timeout uint64,
	inputFolder string,
	outputFolder string,
) error {
	// Apply files to cluster with kubectl
	baseAppsPath := fmt.Sprintf("%s/%s.yaml", inputFolder, baseBranch.FolderName())
	if err := utils.KubectlApply(baseAppsPath); err != nil {
		return fmt.Errorf("failed to apply base apps: %w", err)
	}

	if err := extractResourcesFromCluster(argocd, baseBranch, timeout, outputFolder); err != nil {
		return fmt.Errorf("failed to get resources: %w", err)
	}

	// delete applications
	if err := utils.DeleteApplications(); err != nil {
		return fmt.Errorf("failed to delete applications: %w", err)
	}

	// apply target apps
	targetAppsPath := fmt.Sprintf("%s/%s.yaml", inputFolder, targetBranch.FolderName())
	if err := utils.KubectlApply(targetAppsPath); err != nil {
		return fmt.Errorf("failed to apply target apps: %w", err)
	}

	if err := extractResourcesFromCluster(argocd, targetBranch, timeout, outputFolder); err != nil {
		return fmt.Errorf("failed to get resources: %w", err)
	}

	return nil
}

// extractResourcesFromCluster extracts resources from Argo CD for a specific branch
func extractResourcesFromCluster(
	argocd *argocd.ArgoCDInstallation,
	branch *types.Branch,
	timeout uint64,
	outputFolder string,
) error {
	log.Info().Str("branch", branch.Name).Msg("🤖 Getting resources from branch")

	destinationFolder := fmt.Sprintf("%s/%s", outputFolder, branch.Type())

	// Create destination folder
	if err := utils.CreateFolder(destinationFolder); err != nil {
		return fmt.Errorf("failed to create destination folder: %w", err)
	}

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

		cmd := "kubectl get applications -A -oyaml"
		output, err := utils.RunCommand(cmd)
		if err != nil {
			return fmt.Errorf("failed to get applications: %v", err)
		}

		if err := yaml.Unmarshal([]byte(output), &yamlOutput); err != nil {
			return fmt.Errorf("failed to parse applications yaml: %v", err)
		}

		if len(yamlOutput.Items) == 0 || len(yamlOutput.Items) == len(processedApps) {
			break
		}

		var timedOutApps []string
		var otherErrors []struct{ name, msg string }
		appsLeft := 0

		// Process each application
		for _, item := range yamlOutput.Items {
			name := item.Metadata.Name
			if processedApps[name] {
				continue
			}

			switch item.Status.Sync.Status {
			case "OutOfSync", "Synced":
				log.Debug().Msgf("Getting manifests for application: %s", name)
				manifests, err := argocd.GetManifests(name)
				if err != nil {
					log.Error().Msgf("❌ Failed to get manifests for application: %s, error: %v", name, err)
					failedApps[name] = err.Error()
					continue
				}

				if err := utils.WriteFile(fmt.Sprintf("%s/%s", destinationFolder, name), manifests); err != nil {
					return fmt.Errorf("failed to write manifests: %v", err)
				}

				processedApps[name] = true

			case "Unknown":
				for _, condition := range item.Status.Conditions {
					if isErrorCondition(condition.Type) {
						msg := condition.Message
						if containsAny(msg, errorMessages) {
							failedApps[name] = msg
						} else if containsAny(msg, timeoutMessages) {
							log.Warn().Msgf("Application: %s timed out with error: %s", name, msg)
							timedOutApps = append(timedOutApps, name)
							otherErrors = append(otherErrors, struct{ name, msg string }{name, msg})
						} else {
							log.Error().Msgf("Application: %s failed with error: %s", name, msg)
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
			return fmt.Errorf("failed to process applications")
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
			return fmt.Errorf("timed out")
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

	log.Info().Str("branch", branch.Name).Msgf("🤖 Got all resources from %d applications", len(processedApps))
	log.Info().Str("branch", branch.Name).Msgf("💾 Writing resources to: '%s/<app_name>'", destinationFolder)

	return nil
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
