package argoapplication

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argocdpath "github.com/argoproj/argo-cd/v3/util/app/path"
	"github.com/dag-andersen/argocd-diff-preview/pkg/app_selector"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// RenderMode controls whether an application should be rendered
type RenderMode string

const (
	// RenderAlways means the application is always rendered, even if identical between branches
	RenderAlways RenderMode = "always"
	// RenderNever means the application is never rendered (same as ignore: true)
	RenderNever RenderMode = "never"
	// RenderChanged means the application is only rendered if it changed between branches (default)
	RenderChanged RenderMode = "changed"
)

const (
	AnnotationWatchPattern = "argocd-diff-preview/watch-pattern"
	AnnotationIgnore       = "argocd-diff-preview/ignore"
	AnnotationRender       = "argocd-diff-preview/render"
)

type ApplicationSelectionOptions struct {
	FileRegex                  *regexp.Regexp
	Selector                   []app_selector.Selector
	FilesChanged               []string
	IgnoreInvalidWatchPattern  bool
	WatchIfNoWatchPatternFound bool
}

const maxFilesChangedDisplay = 20

func formatFilesChanged(files []string) string {
	if len(files) == 0 {
		return ""
	}
	limit := min(len(files), maxFilesChangedDisplay)
	result := fmt.Sprintf("'%s'", strings.Join(files[:limit], "', '"))
	if len(files) > maxFilesChangedDisplay {
		result += fmt.Sprintf(" [%d more omitted]", len(files)-maxFilesChangedDisplay)
	}
	return result
}

func (appSelectionOptions ApplicationSelectionOptions) LogRules() {

	hasSelector := len(appSelectionOptions.Selector) > 0
	onlySelectAppsWithWatchPatterns := len(appSelectionOptions.FilesChanged) > 0 && !appSelectionOptions.WatchIfNoWatchPatternFound

	switch {
	case hasSelector && onlySelectAppsWithWatchPatterns:
		var selectorStrs []string
		for _, s := range appSelectionOptions.Selector {
			selectorStrs = append(selectorStrs, s.String())
		}
		log.Info().Msgf(
			"🤖 Will only select Application[Sets] that match '%s' and watch these files: %s",
			strings.Join(selectorStrs, ","),
			formatFilesChanged(appSelectionOptions.FilesChanged),
		)
	case hasSelector:
		var selectorStrs []string
		for _, s := range appSelectionOptions.Selector {
			selectorStrs = append(selectorStrs, s.String())
		}
		log.Info().Msgf(
			"🤖 Will only select Application[Sets] that match '%s'",
			strings.Join(selectorStrs, ","),
		)
	case onlySelectAppsWithWatchPatterns:
		log.Info().Msgf(
			"🤖 Will only select Application[Sets] that watch these files: %s",
			formatFilesChanged(appSelectionOptions.FilesChanged),
		)
	}
}

type ArgoSelection struct {
	SelectedApps []ArgoResource
	SkippedApps  []ArgoResource
}

func ApplicationSelection(
	apps []ArgoResource,
	appSelectionOptions ApplicationSelectionOptions,
) *ArgoSelection {
	var selectedApps []ArgoResource
	var skippedApps []ArgoResource
	for _, app := range apps {
		if app.Filter(appSelectionOptions) {
			selectedApps = append(selectedApps, app)
		} else {
			skippedApps = append(skippedApps, app)
		}
	}
	return &ArgoSelection{
		SelectedApps: selectedApps,
		SkippedApps:  skippedApps,
	}
}

// Filter checks if the application matches the given selectors and watches the given files
func (a *ArgoResource) Filter(
	appSelectionOptions ApplicationSelectionOptions,
) bool {
	// First check render mode annotation
	switch a.GetRenderMode() {
	case RenderNever:
		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s is not selected because: application is ignored because render mode is '%s'", a.Kind.ShortName(), RenderNever)
		return false
	case RenderAlways:
		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s is selected because: application is forced because render mode is '%s'", a.Kind.ShortName(), RenderAlways)
		return true
	}

	// Then check legacy ignore annotation
	selected, reason := a.filterByIgnoreAnnotation()
	if !selected {
		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s is not selected because: %s", a.Kind.ShortName(), reason)
		return false
	}

	// Then check selectors
	if len(appSelectionOptions.Selector) > 0 {
		selected, reason := a.filterBySelectors(appSelectionOptions.Selector)
		if !selected {
			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s is not selected because: %s", a.Kind.ShortName(), reason)
			return false
		}
	}

	// Then check files changed
	if len(appSelectionOptions.FilesChanged) > 0 {
		selected, reason := a.filterByFilesChanged(appSelectionOptions.FilesChanged, appSelectionOptions.IgnoreInvalidWatchPattern, appSelectionOptions.WatchIfNoWatchPatternFound)
		if !selected {
			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s is not selected because: %s", a.Kind.ShortName(), reason)
			return false
		}
		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s is selected because: %s", a.Kind.ShortName(), reason)
	}

	return true
}

func (a *ArgoResource) filterByIgnoreAnnotation() (bool, string) {

	// get annotations
	annotations, found, err := unstructured.NestedStringMap(a.Yaml.Object, "metadata", "annotations")
	if err != nil || !found || len(annotations) == 0 {
		return true, "no 'argocd-diff-preview/ignore' annotation found"
	}

	if value, exists := annotations[AnnotationIgnore]; exists && value == "true" {
		return false, fmt.Sprintf("application is ignored because of '%s: %s'", AnnotationIgnore, value)
	}
	return true, "application is not ignored"
}

// GetRenderMode returns the render mode for the application based on annotations
func (a *ArgoResource) GetRenderMode() RenderMode {
	if a.Yaml == nil {
		return RenderChanged
	}

	annotations, found, err := unstructured.NestedStringMap(a.Yaml.Object, "metadata", "annotations")
	if err != nil || !found || len(annotations) == 0 {
		return RenderChanged
	}

	if value, exists := annotations[AnnotationRender]; exists {
		mode := RenderMode(strings.ToLower(strings.TrimSpace(value)))
		switch mode {
		case RenderAlways, RenderNever, RenderChanged:
			return mode
		default:
			return RenderChanged
		}
	}

	return RenderChanged
}

// filterBySelectors checks if the application matches the given selectors
func (a *ArgoResource) filterBySelectors(selectors []app_selector.Selector) (bool, string) {
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
		if (s.Operator == app_selector.Eq && !matches) || (s.Operator == app_selector.Ne && matches) {
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

	effectiveWatchPattern, effectiveManifestGeneratePaths := a.effectiveWatchAnnotations(annotations)

	if effectiveWatchPattern == "" && effectiveManifestGeneratePaths == "" {
		return watchIfNoWatchPatternFound, "no effective watch-pattern or manifest-generate-paths annotation found"
	}

	if effectiveWatchPattern != "" {
		if selectedWatchPattern, reasonWatchPattern := a.filterByAnnotationWatchPattern(effectiveWatchPattern, filesChanged, ignoreInvalidWatchPattern); selectedWatchPattern {
			return true, reasonWatchPattern
		}
	}

	if effectiveManifestGeneratePaths != "" {
		if selectedManifestGeneratePaths, reasonManifestGeneratePaths := a.filterByManifestGeneratePaths(effectiveManifestGeneratePaths, filesChanged); selectedManifestGeneratePaths {
			return true, reasonManifestGeneratePaths
		}
	}

	return false, "files changed does not match watch-pattern or manifest-generate-paths"
}

func (a *ArgoResource) effectiveWatchAnnotations(annotations map[string]string) (watchPattern string, manifestGeneratePaths string) {
	watchPattern = strings.TrimSpace(annotations[AnnotationWatchPattern])

	// ApplicationSet does not support manifest-generate-paths, so we only check for it on Applications.
	if a.Kind == Application {
		manifestGeneratePaths = strings.TrimSpace(annotations[v1alpha1.AnnotationKeyManifestGeneratePaths])
	}

	return watchPattern, manifestGeneratePaths
}

func (a *ArgoResource) filterByAnnotationWatchPattern(watchPattern string, filesChanged []string, ignoreInvalidWatchPattern bool) (bool, string) {

	for pattern := range strings.SplitSeq(watchPattern, ",") {
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

		if slices.ContainsFunc(filesChanged, regex.MatchString) {
			return true, fmt.Sprintf("files changed matches watch-pattern '%s'", watchPattern)
		}
	}

	return false, fmt.Sprintf("no files changed match watch-pattern '%s'", watchPattern)
}

// filterByManifestGeneratePaths checks if the application manifest-generate-paths matches any of the changed files.
// It reuses Argo CD's own manifest-generate-paths helpers for Applications to avoid semantic drift.
func (a *ArgoResource) filterByManifestGeneratePaths(manifestGeneratePaths string, filesChanged []string) (bool, string) {

	if a.Kind != Application {
		return false, "manifest-generate-paths is ignored for non-Application resources"
	}

	app, err := a.asArgoCDApplication(manifestGeneratePaths)
	if err != nil {
		log.Debug().Err(err).Str(a.Kind.ShortName(), a.GetLongName()).Msg("Failed to convert Application before checking manifest-generate-paths")
		return false, fmt.Sprintf("failed to convert Application before checking manifest-generate-paths: %v", err)
	} else {
		sources := app.Spec.GetSources()
		if app.Spec.SourceHydrator != nil {
			sources = append(sources, app.Spec.SourceHydrator.GetDrySource())
		}

		for _, source := range sources {
			refreshPaths := argocdpath.GetSourceRefreshPaths(app, source)
			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("Paths to compare with files changed: %v", refreshPaths)
			if len(refreshPaths) > 0 && argocdpath.AppFilesHaveChanged(refreshPaths, filesChanged) {
				return true, fmt.Sprintf("files changed match manifest-generate-paths: '%s'", manifestGeneratePaths)
			}
		}

		return false, fmt.Sprintf("no files changed match manifest-generate-paths: '%s'", manifestGeneratePaths)
	}
}

func (a *ArgoResource) asArgoCDApplication(manifestGeneratePaths string) (*v1alpha1.Application, error) {
	if a.Yaml == nil {
		return nil, fmt.Errorf("no YAML found")
	}

	var app v1alpha1.Application
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(a.Yaml.Object, &app); err != nil {
		return nil, fmt.Errorf("failed to convert unstructured Application: %w", err)
	}

	if app.Annotations == nil {
		app.Annotations = map[string]string{}
	}
	app.Annotations[v1alpha1.AnnotationKeyManifestGeneratePaths] = manifestGeneratePaths

	return &app, nil
}
