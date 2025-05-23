package duplicates

import (
	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RemoveDuplicates finds and filters out duplicate applications between base and target branches
func RemoveDuplicates(baseApps, targetApps []argoapplication.ArgoResource) ([]argoapplication.ArgoResource, []argoapplication.ArgoResource) {
	// Find duplicates
	var duplicateYaml []*unstructured.Unstructured
	for _, baseApp := range baseApps {
		for _, targetApp := range targetApps {
			if baseApp.Id == targetApp.Id && yamlEqual(baseApp.Yaml, targetApp.Yaml) {
				log.Debug().Str(baseApp.Kind.ShortName(), baseApp.Name).Msg("Skipping application because it has not changed")
				duplicateYaml = append(duplicateYaml, baseApp.Yaml)
				break
			}
		}
	}

	if len(duplicateYaml) == 0 {
		return baseApps, targetApps
	}

	// Remove duplicates and log stats
	baseAppsBefore := len(baseApps)
	targetAppsBefore := len(targetApps)

	// Actually filter out the duplicates using the helper function
	baseApps = filterDuplicates(baseApps, duplicateYaml)
	targetApps = filterDuplicates(targetApps, duplicateYaml)

	log.Info().Str("branch", string(git.Base)).Msgf(
		"🤖 Skipped %d Application[Sets] because they have not changed after patching",
		baseAppsBefore-len(baseApps),
	)

	log.Info().Str("branch", string(git.Target)).Msgf(
		"🤖 Skipped %d Application[Sets] because they have not changed after patching",
		targetAppsBefore-len(targetApps),
	)

	log.Info().Str("branch", string(git.Base)).Msgf(
		"🤖 Using the remaining %d Application[Sets]",
		len(baseApps),
	)

	log.Info().Str("branch", string(git.Target)).Msgf(
		"🤖 Using the remaining %d Application[Sets]",
		len(targetApps),
	)

	return baseApps, targetApps
}

func filterDuplicates(apps []argoapplication.ArgoResource, duplicates []*unstructured.Unstructured) []argoapplication.ArgoResource {
	var filtered []argoapplication.ArgoResource
	for _, app := range apps {
		isDuplicate := false
		for _, dup := range duplicates {
			if yamlEqual(app.Yaml, dup) {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			filtered = append(filtered, app)
		}
	}
	return filtered
}
