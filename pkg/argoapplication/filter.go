package argoapplication

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	argocdsecurity "github.com/argoproj/argo-cd/v2/util/security"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/selector"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	annotationWatchPattern                = "argocd-diff-preview/watch-pattern"
	annotationIgnore                      = "argocd-diff-preview/ignore"
	annotationArgoCDManifestGeneratePaths = "argocd.argoproj.io/manifest-generate-paths"
)

type FilterOptions struct {
	FileRegex                  *string
	Selector                   []selector.Selector
	FilesChanged               []string
	IgnoreInvalidWatchPattern  bool
	WatchIfNoWatchPatternFound bool
}

func FilterAllWithLogging(apps []ArgoResource, filterOptions FilterOptions, branch *git.Branch) []ArgoResource {
	// Log selector and files changed info
	switch {
	case len(filterOptions.Selector) > 0 && len(filterOptions.FilesChanged) > 0:
		var selectorStrs []string
		for _, s := range filterOptions.Selector {
			selectorStrs = append(selectorStrs, s.String())
		}
		log.Info().Msgf(
			"🤖 Will only run on Applications that match '%s' and watch these files: '%s'",
			strings.Join(selectorStrs, ","),
			strings.Join(filterOptions.FilesChanged, "', '"),
		)
	case len(filterOptions.Selector) > 0:
		var selectorStrs []string
		for _, s := range filterOptions.Selector {
			selectorStrs = append(selectorStrs, s.String())
		}
		log.Info().Msgf(
			"🤖 Will only run on Applications that match '%s'",
			strings.Join(selectorStrs, ","),
		)
	case len(filterOptions.FilesChanged) > 0:
		log.Info().Msgf(
			"🤖 Will only run on Applications that watch these files: '%s'",
			strings.Join(filterOptions.FilesChanged, "', '"),
		)
	}

	numberOfAppsBeforeFiltering := len(apps)

	// Filter applications
	filteredApps := FilterAll(apps, filterOptions)

	// Log filtering results
	if numberOfAppsBeforeFiltering != len(filteredApps) {
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Found %d Application[Sets] before filtering",
			numberOfAppsBeforeFiltering,
		)
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Found %d Application[Sets] after filtering",
			len(filteredApps),
		)
	} else {
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Found %d Application[Sets]",
			numberOfAppsBeforeFiltering,
		)
	}

	return filteredApps
}

func FilterAll(
	apps []ArgoResource,
	filterOptions FilterOptions,
) []ArgoResource {
	var filteredApps []ArgoResource
	for _, app := range apps {
		if app.Filter(filterOptions) {
			filteredApps = append(filteredApps, app)
		}
	}
	return filteredApps
}

// Filter checks if the application matches the given selectors and watches the given files
func (a *ArgoResource) Filter(
	filterOptions FilterOptions,
) bool {

	// First check ignore annotation
	if !a.filterByIgnoreAnnotation() {
		return false
	}

	// Then check selectors
	if len(filterOptions.Selector) > 0 {
		if !a.filterBySelectors(filterOptions.Selector) {
			return false
		}
	}

	// Then check files changed
	if len(filterOptions.FilesChanged) > 0 {
		if !a.filterByFilesChanged(filterOptions.FilesChanged, filterOptions.IgnoreInvalidWatchPattern, filterOptions.WatchIfNoWatchPatternFound) {
			return false
		}
	}

	return true
}

func (a *ArgoResource) filterByIgnoreAnnotation() bool {

	// get annotations
	annotations, found, err := unstructured.NestedStringMap(a.Yaml.Object, "metadata", "annotations")
	if err != nil || !found || len(annotations) == 0 {
		return true
	}

	if value, exists := annotations[annotationIgnore]; exists && value == "true" {
		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("application is ignored because of `argocd-diff-preview/ignore: %s`. Skipping", value)
		return false
	}
	return true
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
func (a *ArgoResource) filterByFilesChanged(filesChanged []string, ignoreInvalidWatchPattern bool, watchIfNoWatchPatternFound bool) bool {
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
		log.Debug().Str("patchType", "filter").Err(err).Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no annotations found")
		return watchIfNoWatchPatternFound
	}

	watchPattern, watchPatternExists := annotations[annotationWatchPattern]
	manifestGeneratePaths, manifestGeneratePathsExists := annotations[annotationArgoCDManifestGeneratePaths]

	// Check if we effectively have no watch patterns (either no annotation or empty/whitespace-only values)
	effectiveWatchPattern := strings.TrimSpace(watchPattern)
	effectiveManifestGeneratePaths := strings.TrimSpace(manifestGeneratePaths)

	if (!watchPatternExists || effectiveWatchPattern == "") && (!manifestGeneratePathsExists || effectiveManifestGeneratePaths == "") {
		if watchIfNoWatchPatternFound {
			log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no effective watch pattern or manifest-generate-paths annotation found. Selecting Application")
		} else {
			log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no effective watch pattern or manifest-generate-paths annotation found. Skipping application")
		}
		return watchIfNoWatchPatternFound
	}

	filter := a.filterByAnnotationWatchPattern(effectiveWatchPattern, filesChanged, ignoreInvalidWatchPattern, watchIfNoWatchPatternFound) ||
		a.filterByManifestGeneratePaths(effectiveManifestGeneratePaths, filesChanged)

	if !filter {
		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("Skipping application. Does not match watch pattern or manifest-generate-paths")
	}
	return filter
}

func (a *ArgoResource) filterByAnnotationWatchPattern(watchPattern string, filesChanged []string, ignoreInvalidWatchPattern bool, watchIfNoWatchPatternFound bool) bool {

	patternsList := strings.Split(watchPattern, ",")

	for _, pattern := range patternsList {
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

	log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no files changed match watch pattern")
	return false
}

// filterByManifestGeneratePaths checks if the application manifest-generate-paths matches any of the changed files
// Mimics the behavior of the watch pattern from ArgoCD: https://github.com/argoproj/argo-cd/blob/master/util/app/path/path.go#L122-L151
func (a *ArgoResource) filterByManifestGeneratePaths(manifestGeneratePaths string, filesChanged []string) bool {

	// Split the manifest paths by semicolon
	paths := strings.Split(manifestGeneratePaths, ";")

	if len(paths) == 0 {
		log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no manifest-generate-paths found")
		return false
	}

	var refreshPaths []string

	for _, path := range paths {
		// trim whitespace
		path = strings.TrimSpace(path)

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

	log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("Paths to compare with files changed: %v", refreshPaths)

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

	log.Debug().Str("patchType", "filter").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no files changed match manifest-generate-paths")
	return false
}
