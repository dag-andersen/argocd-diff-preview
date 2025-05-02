package argoapplication

import (
	"fmt"
	"sort"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/rs/zerolog/log"
)

// UniqueIds ensures all applications have unique IDs by adding suffixes to duplicates
func UniqueIds(apps []ArgoResource, branch *git.Branch) []ArgoResource {
	// Group applications by ID
	duplicateIds := make(map[string][]ArgoResource)
	for _, app := range apps {
		duplicateIds[app.Id] = append(duplicateIds[app.Id], app)
	}

	var newApps []ArgoResource
	duplicateCounter := 0

	// Process each group of applications
	for id, appsWithSameId := range duplicateIds {
		if len(appsWithSameId) > 1 {
			duplicateCounter++
			log.Debug().
				Str("branch", branch.Name).
				Msgf("Found %d duplicate applications with same name: %s", len(appsWithSameId), id)

			// Sort apps by filename for stable ordering
			sort.Slice(appsWithSameId, func(i, j int) bool {
				return appsWithSameId[i].FileName < appsWithSameId[j].FileName
			})

			// Rename each app with a suffix
			for i, app := range appsWithSameId {
				newId := fmt.Sprintf("%s-%d", id, i+1)

				// Create a copy of the app
				newApp := app
				newApp.Id = newId

				// Update the name in the YAML
				newApp.Yaml.SetName(newId)
				log.Debug().Str("branch", branch.Name).Str(newApp.Kind.ShortName(), newApp.GetLongName()).Msgf("Updated name in yaml to: %s", newId)

				newApps = append(newApps, newApp)
			}
		} else {
			// No duplicates, keep as is
			newApps = append(newApps, appsWithSameId[0])
		}
	}

	// sort newApps by filename
	sort.Slice(newApps, func(i, j int) bool {
		return newApps[i].Id < newApps[j].Id
	})

	if duplicateCounter > 0 {
		log.Info().
			Str("branch", branch.Name).
			Msgf("🔍 Found %d duplicate application names. Suffixing with -1, -2, -3, etc.", duplicateCounter)
		log.Info().Str("branch", branch.Name).Msgf("🤖 Applications after unique names: %v", len(newApps))
	}

	return newApps
}
