package duplicates

import (
	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// RemoveDuplicates finds and filters out duplicate applications between base and target branches
func RemoveDuplicates(baseApps, targetApps *argoapplication.ArgoSelection) (*argoapplication.ArgoSelection, *argoapplication.ArgoSelection) {
	// Find duplicates
	var duplicateYaml []*unstructured.Unstructured
	for _, baseApp := range baseApps.SelectedApps {
		for _, targetApp := range targetApps.SelectedApps {
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
	baseAppsBefore := len(baseApps.SelectedApps)
	targetAppsBefore := len(targetApps.SelectedApps)

	// Actually filter out the duplicates using the helper function
	baseApps = filterDuplicates(baseApps, duplicateYaml)
	targetApps = filterDuplicates(targetApps, duplicateYaml)

	log.Info().Str("branch", string(git.Base)).Msgf(
		"🤖 Skipped %d Application[Sets] because they have not changed after patching",
		baseAppsBefore-len(baseApps.SelectedApps),
	)

	log.Info().Str("branch", string(git.Target)).Msgf(
		"🤖 Skipped %d Application[Sets] because they have not changed after patching",
		targetAppsBefore-len(targetApps.SelectedApps),
	)

	log.Info().Str("branch", string(git.Base)).Msgf(
		"🤖 Using the remaining %d Application[Sets]",
		len(baseApps.SelectedApps),
	)

	log.Info().Str("branch", string(git.Target)).Msgf(
		"🤖 Using the remaining %d Application[Sets]",
		len(targetApps.SelectedApps),
	)

	return baseApps, targetApps
}

func filterDuplicates(apps *argoapplication.ArgoSelection, duplicates []*unstructured.Unstructured) *argoapplication.ArgoSelection {
	log.Debug().Msgf("filtering %d Applications for duplicates", len(apps.SelectedApps))

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

	var selectedApps []argoapplication.ArgoResource
	skippedApps := apps.SkippedApps
	for _, app := range apps.SelectedApps {
		appStr, err := yaml.Marshal(app.Yaml)
		if err != nil {
			log.Debug().Err(err).Str("app", app.Name).Msg("failed to marshal app YAML, including in results")
			selectedApps = append(selectedApps, app)
			continue
		}

		if !duplicateSet[string(appStr)] {
			selectedApps = append(selectedApps, app)
		} else {
			skippedApps = append(skippedApps, app)
		}
	}

	log.Debug().Msgf("removed %d duplicates", len(apps.SelectedApps)-len(selectedApps))
	return &argoapplication.ArgoSelection{
		SelectedApps: selectedApps,
		SkippedApps:  skippedApps,
	}
}
