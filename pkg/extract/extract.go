package extract

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplicaiton"
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
// by applying each application individually and capturing the resulting resources
func GetResourcesFromBothBranches(
	argocd *argocd.ArgoCDInstallation,
	baseBranch *types.Branch,
	targetBranch *types.Branch,
	timeout uint64,
	baseApps []argoapplicaiton.ArgoResource,
	targetApps []argoapplicaiton.ArgoResource,
	outputFolder string,
) error {
	log.Info().Msg("🚀 Starting pipeline extraction process")

	// Create destination folders
	baseDestFolder := fmt.Sprintf("%s/%s", outputFolder, baseBranch.Type())
	targetDestFolder := fmt.Sprintf("%s/%s", outputFolder, targetBranch.Type())
	if err := utils.CreateFolder(baseDestFolder); err != nil {
		return fmt.Errorf("failed to create base destination folder: %w", err)
	}
	if err := utils.CreateFolder(targetDestFolder); err != nil {
		return fmt.Errorf("failed to create target destination folder: %w", err)
	}

	// Process base branch applications
	log.Info().Msgf("🔍 Processing %d applications from base branch (%s)", len(baseApps), baseBranch.Name)
	baseManifests, err := processApplicationsPipeline(argocd, baseApps, timeout)
	if err != nil {
		return fmt.Errorf("failed to process base branch applications: %w", err)
	}

	// Write base manifests to files
	for appName, manifest := range baseManifests {
		outputPath := fmt.Sprintf("%s/%s", baseDestFolder, appName)
		if err := utils.WriteFile(outputPath, manifest); err != nil {
			return fmt.Errorf("failed to write manifest for %s: %w", appName, err)
		}
		log.Info().Str("app", appName).Str("path", outputPath).Msg("✅ Wrote manifest to file")
	}

	// Process target branch applications
	log.Info().Msgf("🔍 Processing %d applications from target branch (%s)", len(targetApps), targetBranch.Name)
	targetManifests, err := processApplicationsPipeline(argocd, targetApps, timeout)
	if err != nil {
		return fmt.Errorf("failed to process target branch applications: %w", err)
	}

	// Write target manifests to files
	for appName, manifest := range targetManifests {
		outputPath := fmt.Sprintf("%s/%s", targetDestFolder, appName)
		if err := utils.WriteFile(outputPath, manifest); err != nil {
			return fmt.Errorf("failed to write manifest for %s: %w", appName, err)
		}
		log.Info().Str("app", appName).Str("path", outputPath).Msg("✅ Wrote manifest to file")
	}

	return nil
}

// processApplicationsPipeline processes applications in a pipeline fashion:
// For each application: apply → wait for sync → extract manifests → delete
// Returns a map of application names to their manifests
func processApplicationsPipeline(
	argocd *argocd.ArgoCDInstallation,
	apps []argoapplicaiton.ArgoResource,
	timeout uint64,
) (map[string]string, error) {
	// Create worker pool
	const maxWorkers = 5
	type workItem struct {
		app argoapplicaiton.ArgoResource
	}
	type resultItem struct {
		appName   string
		manifests string
		err       error
	}

	workChan := make(chan workItem, len(apps))
	resultChan := make(chan resultItem, len(apps))

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Fill work channel with apps
	for _, app := range apps {
		workChan <- workItem{app: app}
	}
	close(workChan)

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workChan {
				// Process a single application
				manifests, err := processSingleApplication(ctx, argocd, work.app)
				resultChan <- resultItem{
					appName:   work.app.Name,
					manifests: manifests,
					err:       err,
				}
			}
		}()
	}

	// Wait for all workers to finish and close result channel
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	manifests := make(map[string]string)
	successCount := 0
	failedApps := make(map[string]string)

	for result := range resultChan {
		if result.err != nil {
			failedApps[result.appName] = result.err.Error()
			log.Warn().Str("app", result.appName).Err(result.err).Msg("❌ Failed to process application")
			continue
		}

		manifests[result.appName] = result.manifests
		successCount++
		log.Info().Str("app", result.appName).Msg("✅ Successfully processed application")
	}

	// Log summary
	log.Info().Msgf("📊 Processed %d/%d applications successfully", successCount, len(apps))
	if len(failedApps) > 0 {
		log.Error().Msgf("❌ Failed to process %d applications", len(failedApps))
		for app, errMsg := range failedApps {
			log.Error().Str("app", app).Msgf("Error: %s", errMsg)
		}
		return manifests, fmt.Errorf("failed to process %d applications", len(failedApps))
	}

	return manifests, nil
}

// processSingleApplication processes a single application through the pipeline:
// 1. Apply the application
// 2. Wait for it to sync
// 3. Extract the manifests
// 4. Delete the application
func processSingleApplication(
	ctx context.Context,
	argocd *argocd.ArgoCDInstallation,
	app argoapplicaiton.ArgoResource,
) (string, error) {
	appName := app.Name
	log.Debug().Str("app", appName).Msg("🔄 Starting pipeline for application")

	// Convert the app to YAML string
	appYaml, err := app.AsString()
	if err != nil {
		return "", fmt.Errorf("failed to convert app to YAML: %w", err)
	}

	// Apply the application
	log.Debug().Str("app", appName).Msg("📄 Applying application")
	if err := utils.KubectlApplyFromString(appYaml); err != nil {
		return "", fmt.Errorf("failed to apply application: %w", err)
	}

	//sleep for 3 seconds
	time.Sleep(3 * time.Second)

	// Wait for the application to sync or fail
	log.Debug().Str("app", appName).Msg("⏳ Waiting for application to sync")
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for application to sync")
		default:
			// Check application status
			status, conditions, err := getApplicationStatus(appName, argocd.Namespace)
			if err != nil {
				log.Warn().Str("app", appName).Err(err).Msg("Failed to get application status, retrying...")
				time.Sleep(2 * time.Second)
				continue
			}

			// Check for errors in conditions
			for _, condition := range conditions {
				if isErrorCondition(condition.Type) {
					msg := condition.Message
					if containsAny(msg, errorMessages) {
						// Application failed
						// Delete the application before returning
						_ = deleteApplication(appName, argocd.Namespace) // Best effort cleanup, ignore errors
						return "", fmt.Errorf("application failed: %s", msg)
					} else if containsAny(msg, timeoutMessages) {
						// Application timed out, try refreshing
						log.Info().Str("app", appName).Msg("🔄 Refreshing application due to timeout")
						if err := argocd.RefreshApp(appName); err != nil {
							log.Warn().Str("app", appName).Err(err).Msg("Failed to refresh application")
						}
					}
				}
			}

			// Check if the application is synced or out of sync
			if status == "Synced" || status == "OutOfSync" {
				// Extract manifests
				log.Debug().Str("app", appName).Msg("📋 Extracting manifests")
				manifests, err := argocd.GetManifests(appName)
				if err != nil {
					// Clean up before returning
					_ = deleteApplication(appName, argocd.Namespace) // Best effort cleanup
					return "", fmt.Errorf("failed to extract manifests: %w", err)
				}

				// Delete the application
				log.Debug().Str("app", appName).Msg("🗑️ Deleting application")
				if err := deleteApplication(appName, argocd.Namespace); err != nil {
					log.Warn().Str("app", appName).Err(err).Msg("Failed to delete application")
					// Continue anyway, since we got the manifests
				}

				return manifests, nil
			}

			// Wait before checking again
			time.Sleep(2 * time.Second)
		}
	}
}

// getApplicationStatus returns the sync status and conditions of an application
func getApplicationStatus(appName string, namespace string) (string, []struct {
	Type    string `yaml:"type"`
	Message string `yaml:"message"`
}, error) {
	// When getting a single resource by name, kubectl returns the resource directly
	// not as a list with items
	var appOutput struct {
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
	}

	cmd := fmt.Sprintf("kubectl get applications.argoproj.io %s -n %s -oyaml", appName, namespace)
	output, err := utils.RunCommand(cmd)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get application status: %w", err)
	}

	log.Debug().Str("app", appName).Msgf("Application status command output received (length: %d bytes)", len(output))

	if err := yaml.Unmarshal([]byte(output), &appOutput); err != nil {
		// Log the output to help with debugging
		log.Debug().Str("app", appName).Str("output", output).Msg("Failed to parse application output")
		return "", nil, fmt.Errorf("failed to parse application status: %w", err)
	}

	// Verify the application actually exists
	if appOutput.Metadata.Name == "" {
		log.Debug().Str("app", appName).Str("output", output).Msg("Application not found in output")
		return "", nil, fmt.Errorf("application not found or has no metadata")
	}

	status := appOutput.Status.Sync.Status
	conditions := appOutput.Status.Conditions

	return status, conditions, nil
}

// deleteApplication deletes an application
func deleteApplication(appName string, namespace string) error {
	cmd := fmt.Sprintf("kubectl delete applications.argoproj.io %s -n %s --cascade=foreground", appName, namespace)
	_, err := utils.RunCommand(cmd)
	return err
}

// Application represents an Argo CD application with its status
type Application struct {
	Name       string
	Status     string
	Conditions []struct {
		Type    string
		Message string
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
