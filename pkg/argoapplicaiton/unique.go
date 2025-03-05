package argoapplicaiton

import (
	"fmt"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
	yamlutil "github.com/argocd-diff-preview/argocd-diff-preview/pkg/yaml"
	"github.com/rs/zerolog/log"
)

// UniqueNames ensures that each application has a unique name by appending the branch name if necessary.
// It returns a new slice with unique names.
// UniqueNames ensures all applications have unique names by adding suffixes to duplicates
func UniqueNames(apps []ArgoResource, branch *types.Branch) []ArgoResource {
	// Group applications by name
	duplicateNames := make(map[string][]ArgoResource)
	for _, app := range apps {
		duplicateNames[app.Name] = append(duplicateNames[app.Name], app)
	}

	var newApps []ArgoResource
	duplicateCounter := 0

	// Process each group of applications
	for name, appsWithSameName := range duplicateNames {
		if len(appsWithSameName) > 1 {
			duplicateCounter++
			log.Debug().
				Str("branch", branch.Name).
				Msgf("Found %d duplicate applications with same name: %s", len(appsWithSameName), name)

			// Sort apps by their YAML representation for consistent ordering
			sortedApps := make([]ArgoResource, len(appsWithSameName))
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

	return newApps
}
