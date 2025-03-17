package argoapplicaiton

import (
	"regexp"
	"slices"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/types"
	yamlutil "github.com/dag-andersen/argocd-diff-preview/pkg/yaml"
	"github.com/rs/zerolog/log"
)

const (
	AnnotationWatchPattern = "argocd-diff-preview/watch-pattern"
	AnnotationIgnore       = "argocd-diff-preview/ignore"
)

// Filter checks if the application matches the given selectors and watches the given files
func (a *ArgoResource) Filter(
	selectors []types.Selector,
	filesChanged []string,
	ignoreInvalidWatchPattern bool,
) *ArgoResource {
	// First check selectors
	if len(selectors) > 0 {
		if !a.filterBySelectors(selectors) {
			return nil
		}
	}

	// Then check files changed
	if len(filesChanged) > 0 {
		if !a.filterByFilesChanged(filesChanged, ignoreInvalidWatchPattern) {
			return nil
		}
	}

	return a
}

// filterBySelectors checks if the application matches the given selectors
func (a *ArgoResource) filterBySelectors(selectors []types.Selector) bool {
	metadata := yamlutil.GetYamlValue(a.Yaml, []string{"metadata"})
	if metadata == nil {
		return false
	}

	labels := yamlutil.GetYamlValue(metadata, []string{"labels"})
	if labels == nil {
		return false
	}

	for _, selector := range selectors {
		labelValue := yamlutil.GetYamlValue(labels, []string{selector.Key})
		if labelValue == nil {
			return false
		}

		matches := labelValue.Value == selector.Value
		if (selector.Operator == types.Eq && !matches) || (selector.Operator == types.Ne && matches) {
			return false
		}
	}

	return true
}

// filterByFilesChanged checks if the application watches any of the changed files
func (a *ArgoResource) filterByFilesChanged(filesChanged []string, ignoreInvalidWatchPattern bool) bool {

	if len(filesChanged) == 0 {
		log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("no files changed. Skipping")
		return false
	}

	// check if the application itself is in the list of files changed
	if slices.Contains(filesChanged, a.FileName) {
		log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("application itself is in the list of files changed. Returning application")
		return true
	}

	log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("checking files changed: %v", filesChanged)

	metadata := yamlutil.GetYamlValue(a.Yaml, []string{"metadata"})
	if metadata == nil {
		return false
	}

	annotations := yamlutil.GetYamlValue(metadata, []string{"annotations"})
	if annotations == nil {
		return false
	}

	watchPattern := yamlutil.GetYamlValue(annotations, []string{AnnotationWatchPattern})
	if watchPattern == nil {
		log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("no watch pattern annotation found. Skipping")
		return false
	}

	patternList := strings.TrimSpace(watchPattern.Value)
	if patternList == "" {
		log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("no watch pattern value found. Skipping")
		return false
	}

	patterns := strings.Split(patternList, ",")

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)

		log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("checking watch pattern: %s", pattern)

		if pattern == "" {
			log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("empty watch pattern found. Continuing")
			continue
		}

		regex, err := regexp.Compile(pattern)
		if err != nil {
			if !ignoreInvalidWatchPattern {
				log.Warn().Msgf("⚠️ Invalid watch pattern '%s' in file: %s", pattern, a.FileName)
				return false
			}
			log.Warn().Msgf("⚠️ Ignoring invalid watch pattern '%s' in file: %s", pattern, a.FileName)
			return true
		}

		log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("watch pattern '%s' is valid. Checking files changed", pattern)

		for _, file := range filesChanged {
			if regex.MatchString(file) {
				log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("file '%s' matches watch pattern. Returning application", file)
				return true
			}
		}
	}

	log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("no files changed match watch pattern. Skipping")
	return false
}
