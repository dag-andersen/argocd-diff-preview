package argoapplicaiton

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	argocdsecurity "github.com/argoproj/argo-cd/v2/util/security"
	"github.com/dag-andersen/argocd-diff-preview/pkg/selector"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	AnnotationWatchPattern                = "argocd-diff-preview/watch-pattern"
	AnnotationIgnore                      = "argocd-diff-preview/ignore"
	AnnotationArgoCDManifestGeneratePaths = "argocd.argoproj.io/manifest-generate-paths"
)

// Filter checks if the application matches the given selectors and watches the given files
func (a *ArgoResource) Filter(
	selectors []selector.Selector,
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
func (a *ArgoResource) filterBySelectors(selectors []selector.Selector) bool {
	// Early return if no YAML
	if a.Yaml == nil {
		return false
	}

	// Get all labels directly from unstructured
	labels, found, err := unstructured.NestedStringMap(a.Yaml.Object, "metadata", "labels")
	if err != nil || !found || len(labels) == 0 {
		return false
	}

	// Check each selector against the labels
	for _, s := range selectors {
		labelValue, exists := labels[s.Key]
		if !exists {
			return false
		}

		matches := labelValue == s.Value
		if (s.Operator == selector.Eq && !matches) || (s.Operator == selector.Ne && matches) {
			return false
		}
	}

	return true
}

// filterByFilesChanged checks if the application watches any of the changed files
func (a *ArgoResource) filterByFilesChanged(filesChanged []string, ignoreInvalidWatchPattern bool) bool {
	if len(filesChanged) == 0 {
		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no files changed. Skipping")
		return false
	}

	// check if the application itself is in the list of files changed
	if slices.Contains(filesChanged, a.FileName) {
		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("application itself is in the list of files changed. Returning application")
		return true
	}

	log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("checking files changed: %v", filesChanged)

	// Get annotations directly from unstructured
	annotations, found, err := unstructured.NestedStringMap(a.Yaml.Object, "metadata", "annotations")
	if err != nil || !found || len(annotations) == 0 {
		return false
	}

	return a.filterByAnnotationWatchPattern(annotations, filesChanged, ignoreInvalidWatchPattern) || a.filterByManifestGeneratePaths(annotations, filesChanged)
}

func (a *ArgoResource) filterByAnnotationWatchPattern(annotations map[string]string, filesChanged []string, ignoreInvalidWatchPattern bool) bool {
	watchPattern, exists := annotations[AnnotationWatchPattern]
	if !exists || strings.TrimSpace(watchPattern) == "" {
		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no watch pattern annotation found. Skipping")
		return false
	}

	patternList := strings.TrimSpace(watchPattern)
	if patternList == "" {
		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no watch pattern value found. Skipping")
		return false
	}

	patterns := strings.Split(patternList, ",")

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)

		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("checking watch pattern: %s", pattern)

		if pattern == "" {
			log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("empty watch pattern found. Continuing")
			continue
		}

		regex, err := regexp.Compile(pattern)
		if err != nil {
			if !ignoreInvalidWatchPattern {
				log.Warn().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("⚠️ Invalid watch pattern '%s'", pattern)
				return false
			}
			log.Warn().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("⚠️ Ignoring invalid watch pattern '%s'", pattern)
			continue
		}

		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("watch pattern '%s' is valid. Checking files changed", pattern)

		for _, file := range filesChanged {
			if regex.MatchString(file) {
				log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("file '%s' matches watch pattern. Returning application", file)
				return true
			}
		}
	}

	log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no files changed match watch pattern. Skipping")
	return false
}

// filterByManifestGeneratePaths checks if the application manifest-generate-paths matches any of the changed files
// Mimics the behavior of the watch pattern from ArgoCD: https://github.com/argoproj/argo-cd/blob/master/util/app/path/path.go#L122-L151
func (a *ArgoResource) filterByManifestGeneratePaths(annotations map[string]string, filesChanged []string) bool {
	// Get manifest-generate-paths annotation
	manifestGeneratePaths, exists := annotations[AnnotationArgoCDManifestGeneratePaths]
	if !exists || strings.TrimSpace(manifestGeneratePaths) == "" {
		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no manifest-generate-paths annotation found. Skipping")
		return false
	}

	// Split the manifest paths by semicolon
	paths := strings.Split(manifestGeneratePaths, ";")

	if len(paths) == 0 {
		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no manifest-generate-paths found. Skipping")
		return false
	}

	var refreshPaths []string

	for _, path := range paths {
		// If manifest path is absolute, add it to the list of refresh paths
		if filepath.IsAbs(path) {
			refreshPaths = append(refreshPaths, filepath.Clean(path))
			continue
		}

		// If manifest path is relative, add the spec.source.path as base and make it absolute
		if sourcePath, found, err := unstructured.NestedString(a.Yaml.Object, "spec", "source", "path"); err == nil && found && len(sourcePath) > 0 {
			absPath := fmt.Sprintf("%s%s%s%s", string(filepath.Separator), sourcePath, string(filepath.Separator), path)
			refreshPaths = append(refreshPaths, filepath.Clean(absPath))
			continue
		}

		// If manifest path is relative and no spec.source.path is found, loop on each spec.sources[*].path and make it absolute
		// sources := yamlutil.GetYamlValue(a.Yaml, []string{"spec", "sources"})
		if sources, found, err := unstructured.NestedSlice(a.Yaml.Object, "spec", "sources"); err == nil && found && len(sources) > 0 {
			for _, src := range sources {
				log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("sourcePath: %v", src)
				if sourcePath, found, err := unstructured.NestedString(src.(map[string]any), "path"); err == nil && found && len(sourcePath) > 0 {
					absPath := fmt.Sprintf("%s%s%s%s", string(filepath.Separator), sourcePath, string(filepath.Separator), path)
					refreshPaths = append(refreshPaths, filepath.Clean(absPath))
				}
			}
		}
	}

	for _, f := range filesChanged {
		if !filepath.IsAbs(f) {
			f = string(filepath.Separator) + f
		}
		for _, item := range refreshPaths {
			if !filepath.IsAbs(item) {
				item = string(filepath.Separator) + item
			}
			if f == item {
				return true
			} else if _, err := argocdsecurity.EnforceToCurrentRoot(item, f); err == nil {
				return true
			} else if matched, err := filepath.Match(item, f); err == nil && matched {
				return true
			}
		}
	}

	log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no files changed match manifest-generate-paths. Skipping")
	return false
}
