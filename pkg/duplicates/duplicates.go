package duplicates

import (
	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
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

	log.Info().Msgf("🤖 Skipping identical applications between base and target branches")

	// Actually filter out the duplicates using the helper function
	baseApps = filterDuplicates(baseApps, duplicateYaml)
	targetApps = filterDuplicates(targetApps, duplicateYaml)

	if baseAppsBefore != len(baseApps) || targetAppsBefore != len(targetApps) {
		log.Info().Msgf(
			"🤖 Skipped %d Application[Sets] in base branch because it has an identical copy in the target branch",
			baseAppsBefore-len(baseApps),
		)
		log.Info().Msgf(
			"🤖 Keeping the remaining %d Application[Sets] for the base branch",
			len(baseApps),
		)

		log.Info().Msgf(
			"🤖 Skipped %d Application[Sets] in target branch because it has an identical copy in the base branch",
			targetAppsBefore-len(targetApps),
		)
		log.Info().Msgf(
			"🤖 Keeping the remaining %d Application[Sets] for the target branch",
			len(targetApps),
		)
	} else {
		log.Info().Msg("🤖 No identical applications found between base and target branches")
	}

	return baseApps, targetApps
}

func filterDuplicates(apps []argoapplication.ArgoResource, duplicates []*unstructured.Unstructured) []argoapplication.ArgoResource {
	log.Debug().Msgf("filtering %d Applications for duplicates", len(apps))

	// Create a set of duplicate YAML strings for O(1) lookup
	duplicateSet := make(map[string]bool)
	for _, dup := range duplicates {
		dupStr, err := yaml.Marshal(dup)
		if err != nil {
			log.Debug().Err(err).Msg("failed to marshal duplicate YAML, skipping")
			continue
		}
		duplicateSet[string(dupStr)] = true
	}

	var filtered []argoapplication.ArgoResource
	for _, app := range apps {
		appStr, err := yaml.Marshal(app.Yaml)
		if err != nil {
			log.Debug().Err(err).Str("app", app.Name).Msg("failed to marshal app YAML, including in results")
			filtered = append(filtered, app)
			continue
		}

		if !duplicateSet[string(appStr)] {
			filtered = append(filtered, app)
		}
	}

	log.Debug().Msgf("removed %d duplicates", len(apps)-len(filtered))
	return filtered
}
