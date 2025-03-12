package extract

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	baseManifest string,
	targetManifest string,
	outputFolder string,
) error {
	// Apply base manifest directly from string with kubectl
	if err := utils.KubectlApplyFromString(baseManifest); err != nil {
		return fmt.Errorf("failed to apply base apps: %w", err)
	}

	// sleep for 3 seconds
	time.Sleep(3 * time.Second)

	if err := extractResourcesFromCluster(argocd, baseBranch, timeout, outputFolder); err != nil {
		return fmt.Errorf("failed to get resources: %w", err)
	}

	// delete applications
	if err := utils.DeleteApplications(); err != nil {
		return fmt.Errorf("failed to delete applications: %w", err)
	}

	// apply target manifest
	if err := utils.KubectlApplyFromString(targetManifest); err != nil {
		return fmt.Errorf("failed to apply target apps: %w", err)
	}

	// sleep for 3 seconds
	time.Sleep(3 * time.Second)

	if err := extractResourcesFromCluster(argocd, targetBranch, timeout, outputFolder); err != nil {
		return fmt.Errorf("failed to get resources: %w", err)
	}

	return nil
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

// extractResourcesFromCluster extracts resources from Argo CD for a specific branch
func extractResourcesFromCluster(
	argocd *argocd.ArgoCDInstallation,
	branch *types.Branch,
	timeout uint64,
	outputFolder string,
) error {
	log.Info().Msg("🤖 Getting resources from branch")

	destinationFolder := fmt.Sprintf("%s/%s", outputFolder, branch.Type())

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Create channels for communication
	appChan := make(chan Application)
	resultChan := make(chan struct {
		name      string
		err       error
		manifests string
	})

	// Start a goroutine to continuously fetch applications
	go fetchApplications(ctx, appChan)

	// Start worker pool to process applications
	var wg sync.WaitGroup
	const maxWorkers = 5 // Adjust based on your needs

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			processApplications(ctx, argocd, appChan, resultChan)
		}()
	}

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results as they come in
	processedApps := make(map[string]bool)
	failedApps := make(map[string]string)

	for result := range resultChan {
		if result.err != nil {
			failedApps[result.name] = result.err.Error()
			continue
		}

		if err := utils.WriteFile(fmt.Sprintf("%s/%s", destinationFolder, result.name), result.manifests); err != nil {
			return fmt.Errorf("failed to write manifests: %v", err)
		}

		processedApps[result.name] = true
	}

	// Check for timeout
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			log.Error().Msgf("❌ Timed out after %d seconds", timeout)
			log.Info().Msgf("❌ Processed %d applications, but some applications still remain",
				len(processedApps))
			return fmt.Errorf("timed out")
		}
	default:
		// Context not done, all good
	}

	// Handle errors
	if len(failedApps) > 0 {
		for name, msg := range failedApps {
			log.Error().Msgf("❌ Failed to process application: %s with error: \n%s", name, msg)
		}
		return fmt.Errorf("failed to process applications")
	}

	log.Info().Str("branch", branch.Name).Msgf("🤖 Got all resources from %d applications", len(processedApps))
	log.Info().Str("branch", branch.Name).Msgf("💾 Writing resources to: '%s/<app_name>'", destinationFolder)

	return nil
}

// fetchApplications continuously fetches applications and sends them to the channel
func fetchApplications(ctx context.Context, appChan chan<- Application) {
	defer close(appChan)

	processedApps := make(map[string]bool)

	log.Debug().Msg("Fetching applications")

	for {
		select {
		case <-ctx.Done():
			return
		default:
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

			log.Debug().Msgf("Getting applications from cluster")

			cmd := "kubectl get applications -A -oyaml"
			output, err := utils.RunCommand(cmd)
			if err != nil {
				log.Error().Err(err).Msg("Failed to get applications")
				time.Sleep(5 * time.Second)
				continue
			}

			if err := yaml.Unmarshal([]byte(output), &yamlOutput); err != nil {
				log.Error().Err(err).Msg("Failed to parse applications yaml")
				time.Sleep(5 * time.Second)
				continue
			}

			log.Debug().Msgf("Got %d applications from cluster", len(yamlOutput.Items))

			// Send applications to channel
			newAppsFound := false
			for _, item := range yamlOutput.Items {
				name := item.Metadata.Name
				if processedApps[name] {
					continue
				}

				newAppsFound = true
				appChan <- Application{
					Name:   name,
					Status: item.Status.Sync.Status,
					Conditions: []struct {
						Type    string
						Message string
					}(item.Status.Conditions),
				}
				processedApps[name] = true
			}

			log.Debug().Msgf("Processed %d applications", len(processedApps))

			// If no new apps and all apps processed, we're done
			if !newAppsFound && len(yamlOutput.Items) == len(processedApps) {
				log.Debug().Msg("No new applications found and all applications processed")
				return
			}

			// Sleep before next poll
			time.Sleep(2 * time.Second)
		}
	}
}

// processApplications processes applications from the channel
func processApplications(
	ctx context.Context,
	argocd *argocd.ArgoCDInstallation,
	appChan <-chan Application,
	resultChan chan<- struct {
		name      string
		err       error
		manifests string
	},
) {
	for {
		select {
		case <-ctx.Done():
			return
		case app, ok := <-appChan:
			if !ok {
				return // Channel closed
			}

			// Process the application
			switch app.Status {
			case "OutOfSync", "Synced":
				log.Debug().Msgf("Getting manifests for application: %s", app.Name)
				manifests, err := argocd.GetManifests(app.Name)
				resultChan <- struct {
					name      string
					err       error
					manifests string
				}{
					name:      app.Name,
					err:       err,
					manifests: manifests,
				}

			case "Unknown":
				var err error
				for _, condition := range app.Conditions {
					if isErrorCondition(condition.Type) {
						msg := condition.Message
						if containsAny(msg, errorMessages) {
							err = fmt.Errorf("application error: %s", msg)
						} else if containsAny(msg, timeoutMessages) {
							log.Info().Msgf("Application: %s timed out with error: %s", app.Name, msg)
							// Refresh the app and let it be picked up again
							if refreshErr := argocd.RefreshApp(app.Name); refreshErr != nil {
								log.Error().Err(refreshErr).Msgf("Failed to refresh application: %s", app.Name)
							} else {
								log.Info().Msgf("🔄 Refreshing application: %s", app.Name)
							}
							err = fmt.Errorf("application timeout: %s", msg)
						} else {
							err = fmt.Errorf("application unknown error: %s", msg)
						}
					}
				}

				if err != nil {
					resultChan <- struct {
						name      string
						err       error
						manifests string
					}{
						name:      app.Name,
						err:       err,
						manifests: "",
					}
				}
			}
		}
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
