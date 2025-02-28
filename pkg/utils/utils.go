package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
	yamlutil "github.com/argocd-diff-preview/argocd-diff-preview/pkg/yaml"
	"github.com/rs/zerolog/log"
)

const (
	dirMode = os.ModePerm // 0755 - read/write/execute for owner, read/execute for group and others
)

// RunCommand executes a command and returns its output
func RunCommand(cmd string) (string, error) {
	command := exec.Command("sh", "-c", cmd)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %s: %w", string(output), err)
	}
	return string(output), nil
}

// WriteFile writes content to a file
func WriteFile(path string, content string) error {
	// Ensure content ends with a newline
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}

	return nil
}

// Create folder (clear its content if it exists)
func CreateFolder(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to delete folder: %v", err)
	}
	return os.MkdirAll(path, dirMode)
}

// WriteApplications writes applications to YAML files in the specified folder
func WriteApplications(
	apps []types.ArgoResource,
	branch *types.Branch,
	folder string,
) error {
	filePath := fmt.Sprintf("%s/%s.yaml", folder, branch.FolderName())
	log.Info().Msgf("💾 Writing %d Applications from '%s' to ./%s",
		len(apps), branch.Name, filePath)

	yaml := types.ApplicationsToString(apps)
	if err := WriteFile(filePath, yaml); err != nil {
		return fmt.Errorf("failed to write %s apps: %w", branch.Type(), err)
	}

	return nil
}

// UniqueNames ensures all applications have unique names by adding suffixes to duplicates
func UniqueNames(apps []types.ArgoResource, branch *types.Branch) []types.ArgoResource {
	// Group applications by name
	duplicateNames := make(map[string][]types.ArgoResource)
	for _, app := range apps {
		duplicateNames[app.Name] = append(duplicateNames[app.Name], app)
	}

	var newApps []types.ArgoResource
	duplicateCounter := 0

	// Process each group of applications
	for name, appsWithSameName := range duplicateNames {
		if len(appsWithSameName) > 1 {
			duplicateCounter++
			log.Debug().
				Str("branch", branch.Name).
				Msgf("Found %d duplicate applications with same name: %s", len(appsWithSameName), name)

			// Sort apps by their YAML representation for consistent ordering
			sortedApps := make([]types.ArgoResource, len(appsWithSameName))
			copy(sortedApps, appsWithSameName)

			// Sort by YAML string representation
			for i := 0; i < len(sortedApps); i++ {
				for j := i + 1; j < len(sortedApps); j++ {
					yamlI, _ := sortedApps[i].AsString()
					yamlJ, _ := sortedApps[j].AsString()
					if yamlI > yamlJ {
						sortedApps[i], sortedApps[j] = sortedApps[j], sortedApps[i]
					}
				}
			}

			// Rename each app with a suffix
			for i, app := range sortedApps {
				newName := fmt.Sprintf("%s-%d", name, i+1)

				// Create a copy of the app
				newApp := app
				newApp.Name = newName

				// Update the name in the YAML
				yamlutil.SetYamlValue(newApp.Yaml, []string{"metadata", "name"}, newName)
				log.Debug().Str("branch", branch.Name).Msgf("updated name in yaml: %s", newName)

				newApps = append(newApps, newApp)
			}
		} else {
			// No duplicates, keep as is
			newApps = append(newApps, appsWithSameName[0])
		}
	}

	if duplicateCounter > 0 {
		log.Info().
			Str("branch", branch.Name).
			Msgf("🔍 Found %d duplicate application names. Suffixing with -1, -2, -3, etc.", duplicateCounter)
		log.Info().Str("branch", branch.Name).Msgf("🤖 Applications after unique names: %v", len(newApps))
	}

	sanityCheckFailed := false

	// sanity check. Check that all names are unique
	log.Debug().Str("branch", branch.Name).Msgf("sanity checking unique names 1")
	names := make(map[string]bool)
	for _, app := range newApps {
		if names[app.Name] {
			log.Error().Str("branch", branch.Name).Msgf("Duplicate application name: %s", app.Name)
			sanityCheckFailed = true
		}
		names[app.Name] = true
	}
	log.Debug().Str("branch", branch.Name).Msgf("sanity checking passed 1")

	// sanity check by checking the yaml
	log.Debug().Str("branch", branch.Name).Msgf("sanity checking unique names 2")
	namesInYaml := make(map[string]bool)
	for _, app := range newApps {
		name := yamlutil.GetYamlValue(app.Yaml, []string{"metadata", "name"}).Value
		if namesInYaml[name] {
			log.Error().Str("branch", branch.Name).Msgf("Duplicate application name in yaml: %s", name)
			sanityCheckFailed = true
		}
		namesInYaml[name] = true
	}

	log.Debug().Str("branch", branch.Name).Msgf("sanity checking passed 2")
	if sanityCheckFailed {
		log.Error().Str("branch", branch.Name).Msgf("Sanity check failed")
		os.Exit(1)
	}
	return newApps
}
