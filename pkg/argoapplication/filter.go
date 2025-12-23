package argoapplication

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	argocdsecurity "github.com/argoproj/argo-cd/v3/util/security"
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

// IgnoredApp represents an app that was ignored via annotation, tracking both ID and source file
type IgnoredApp struct {
	Id       string
	FileName string
}

// FilterResult contains the filtered apps and any apps that were ignored via annotation
type FilterResult struct {
	Apps        []ArgoResource
	IgnoredApps []IgnoredApp // Apps filtered out due to argocd-diff-preview/ignore annotation
}

// RemoveIgnoredApps filters out apps from baseApps that match any app in ignoredApps.
// Matching is done by both ID and FileName to avoid filtering apps with same name from different sources.
func RemoveIgnoredApps(baseApps []ArgoResource, ignoredApps []IgnoredApp, branchName string) []ArgoResource {
	if len(ignoredApps) == 0 {
		return baseApps
	}

	ignoredSet := make(map[string]struct{}, len(ignoredApps))
	for _, ignored := range ignoredApps {
		key := fmt.Sprintf("%s|%s", ignored.Id, ignored.FileName)
		ignoredSet[key] = struct{}{}
	}

	var filtered []ArgoResource
	for _, app := range baseApps {
		key := fmt.Sprintf("%s|%s", app.Id, app.FileName)
		if _, exists := ignoredSet[key]; !exists {
			filtered = append(filtered, app)
		} else {
			log.Debug().Str("branch", branchName).Msgf(
				"Skipping %s '%s' because it is ignored on target branch",
				app.Kind.ShortName(), app.GetLongName(),
			)
		}
	}
	return filtered
}

func FilterAllWithLogging(apps []ArgoResource, filterOptions FilterOptions, branch *git.Branch) FilterResult {
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
	result := FilterAll(apps, filterOptions)

	// Log filtering results
	if numberOfAppsBeforeFiltering != len(result.Apps) {
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Found %d Application[Sets] before filtering",
			numberOfAppsBeforeFiltering,
		)
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Found %d Application[Sets] after filtering",
			len(result.Apps),
		)
	} else {
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Found %d Application[Sets]",
			numberOfAppsBeforeFiltering,
		)
	}

	return result
}

func FilterAll(
	apps []ArgoResource,
	filterOptions FilterOptions,
) FilterResult {
	var filteredApps []ArgoResource
	var ignoredApps []IgnoredApp
	for _, app := range apps {
		selected, ignoredByAnnotation := app.Filter(filterOptions)
		if selected {
			filteredApps = append(filteredApps, app)
		} else if ignoredByAnnotation {
			ignoredApps = append(ignoredApps, IgnoredApp{Id: app.Id, FileName: app.FileName})
		}
	}
	return FilterResult{
		Apps:        filteredApps,
		IgnoredApps: ignoredApps,
	}
}

// Filter checks if the application matches the given selectors and watches the given files.
// Returns (selected bool, ignoredByAnnotation bool) where ignoredByAnnotation is true
// if the app was filtered out specifically due to the argocd-diff-preview/ignore annotation.
func (a *ArgoResource) Filter(
	filterOptions FilterOptions,
) (bool, bool) {

	// First check selected annotation
	selected, reason := a.filterByIgnoreAnnotation()
	if !selected {
		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s is not selected because: %s", a.Kind.ShortName(), reason)
		return false, true // ignoredByAnnotation = true
	}

	// Then check selectors
	if len(filterOptions.Selector) > 0 {
		selected, reason := a.filterBySelectors(filterOptions.Selector)
		if !selected {
			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s is not selected because: %s", a.Kind.ShortName(), reason)
			return false, false
		}
	}

	// Then check files changed
	if len(filterOptions.FilesChanged) > 0 {
		selected, reason := a.filterByFilesChanged(filterOptions.FilesChanged, filterOptions.IgnoreInvalidWatchPattern, filterOptions.WatchIfNoWatchPatternFound)
		if !selected {
			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s is not selected because: %s", a.Kind.ShortName(), reason)
			return false, false
		}
		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s is selected because: %s", a.Kind.ShortName(), reason)
	}

	return true, false
}

func (a *ArgoResource) filterByIgnoreAnnotation() (bool, string) {
	annotations, found, err := unstructured.NestedStringMap(a.Yaml.Object, "metadata", "annotations")
	if err != nil || !found || len(annotations) == 0 {
		return true, "no 'argocd-diff-preview/ignore' annotation found"
	}

	if value, exists := annotations[annotationIgnore]; exists && value == "true" {
		return false, fmt.Sprintf("application is ignored because of '%s: %s'", annotationIgnore, value)
	}
	return true, "application is not ignored"
}

// filterBySelectors checks if the application matches the given selectors
func (a *ArgoResource) filterBySelectors(selectors []selector.Selector) (bool, string) {
	// Early return if no YAML
	if a.Yaml == nil {
		return false, "no YAML found"
	}

	// Get all labels directly from unstructured
	labels, found, err := unstructured.NestedStringMap(a.Yaml.Object, "metadata", "labels")
	if err != nil || !found || len(labels) == 0 {
		return false, "no labels found"
	}

	// Check each selector against the labels
	for _, s := range selectors {
		labelValue, exists := labels[s.Key]
		if !exists {
			return false, "label not found"
		}

		matches := labelValue == s.Value
		if (s.Operator == selector.Eq && !matches) || (s.Operator == selector.Ne && matches) {
			return false, fmt.Sprintf("label does not match selector: '%s'", s.String())
		}
	}

	return true, "labels matches selectors"
}

// filterByFilesChanged checks if the application watches any of the changed files and returns a reason for the selection
func (a *ArgoResource) filterByFilesChanged(filesChanged []string, ignoreInvalidWatchPattern bool, watchIfNoWatchPatternFound bool) (bool, string) {
	if len(filesChanged) == 0 {
		return false, "no files changed"
	}

	// check if the application itself is in the list of files changed
	if slices.Contains(filesChanged, a.FileName) {
		return true, "application itself is in the list of files changed"
	}

	// Get annotations directly from unstructured
	annotations, found, err := unstructured.NestedStringMap(a.Yaml.Object, "metadata", "annotations")
	if err != nil || !found || len(annotations) == 0 {
		return watchIfNoWatchPatternFound, "no watch-pattern or manifest-generate-paths annotation found"
	}

	watchPattern, watchPatternExists := annotations[annotationWatchPattern]
	manifestGeneratePaths, manifestGeneratePathsExists := annotations[annotationArgoCDManifestGeneratePaths]

	// Check if we effectively have no watch patterns (either no annotation or empty/whitespace-only values)
	effectiveWatchPattern := strings.TrimSpace(watchPattern)
	effectiveManifestGeneratePaths := strings.TrimSpace(manifestGeneratePaths)

	if (!watchPatternExists || effectiveWatchPattern == "") && (!manifestGeneratePathsExists || effectiveManifestGeneratePaths == "") {
		return watchIfNoWatchPatternFound, "no effective watch-pattern or manifest-generate-paths annotation found"
	}

	if selectedWatchPattern, reasonWatchPattern := a.filterByAnnotationWatchPattern(effectiveWatchPattern, filesChanged, ignoreInvalidWatchPattern); selectedWatchPattern {
		return true, reasonWatchPattern
	}

	if selectedManifestGeneratePaths, reasonManifestGeneratePaths := a.filterByManifestGeneratePaths(effectiveManifestGeneratePaths, filesChanged); selectedManifestGeneratePaths {
		return true, reasonManifestGeneratePaths
	}

	return false, "files changed does not match watch-pattern or manifest-generate-paths"
}

func (a *ArgoResource) filterByAnnotationWatchPattern(watchPattern string, filesChanged []string, ignoreInvalidWatchPattern bool) (bool, string) {

	patternsList := strings.Split(watchPattern, ",")

	for _, pattern := range patternsList {
		pattern = strings.TrimSpace(pattern)

		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("Checking if files changed matches watch-pattern: %s", pattern)

		if pattern == "" {
			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("empty watch-pattern found. Continuing")
			continue
		}

		regex, err := regexp.Compile(pattern)
		if err != nil {
			if !ignoreInvalidWatchPattern {
				log.Warn().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("⚠️ Invalid watch-pattern '%s'", pattern)
				return false, fmt.Sprintf("invalid watch-pattern '%s'", pattern)
			}
			log.Warn().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("⚠️ Ignoring invalid watch-pattern '%s'", pattern)
			continue
		}

		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("watch-pattern '%s' is valid. Checking if files changed matches watch-pattern", pattern)

		for _, file := range filesChanged {
			if regex.MatchString(file) {
				return true, fmt.Sprintf("files changed matches watch-pattern '%s'", watchPattern)
			}
		}
	}

	return false, fmt.Sprintf("no files changed match watch-pattern '%s'", watchPattern)
}

// filterByManifestGeneratePaths checks if the application manifest-generate-paths matches any of the changed files
// Mimics the behavior of the watch pattern from ArgoCD: https://github.com/argoproj/argo-cd/blob/master/util/app/path/path.go#L122-L151
func (a *ArgoResource) filterByManifestGeneratePaths(manifestGeneratePaths string, filesChanged []string) (bool, string) {

	// Split the manifest paths by semicolon
	paths := strings.Split(manifestGeneratePaths, ";")

	if len(paths) == 0 {
		return false, fmt.Sprintf("no '%s' annotation found", annotationArgoCDManifestGeneratePaths)
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
				log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("sourcePath: %v", src)
				if sourcePath, found, err := unstructured.NestedString(src.(map[string]any), "path"); err == nil && found && len(sourcePath) > 0 {
					absPath := fmt.Sprintf("%s%s%s%s", string(filepath.Separator), sourcePath, string(filepath.Separator), path)
					refreshPaths = append(refreshPaths, filepath.Clean(absPath))
				}
			}
		}
	}

	log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("Paths to compare with files changed: %v", refreshPaths)

	for _, f := range filesChanged {
		if !filepath.IsAbs(f) {
			f = string(filepath.Separator) + f
		}
		for _, item := range refreshPaths {
			if !filepath.IsAbs(item) {
				item = string(filepath.Separator) + item
			}
			if f == item {
				return true, fmt.Sprintf("file '%s' matches manifest-generate-paths: '%s'", f, manifestGeneratePaths)
			} else if _, err := argocdsecurity.EnforceToCurrentRoot(item, f); err == nil {
				return true, fmt.Sprintf("file '%s' matches manifest-generate-paths: '%s'", f, manifestGeneratePaths)
			} else if matched, err := filepath.Match(item, f); err == nil && matched {
				return true, fmt.Sprintf("file '%s' matches manifest-generate-paths: '%s'", f, manifestGeneratePaths)
			}
		}
	}

	return false, fmt.Sprintf("no files changed match manifest-generate-paths: '%s'", manifestGeneratePaths)
}
