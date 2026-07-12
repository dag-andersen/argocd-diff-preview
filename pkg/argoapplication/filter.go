package argoapplication

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argocdsecurity "github.com/argoproj/argo-cd/v3/util/security"
	"github.com/dag-andersen/argocd-diff-preview/pkg/app_selector"
	"github.com/dag-andersen/argocd-diff-preview/pkg/repository"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	annotationWatchPattern = "argocd-diff-preview/watch-pattern"
	annotationIgnore       = "argocd-diff-preview/ignore"
	annotationRender       = "argocd-diff-preview/render"
)

type ApplicationSelectionOptions struct {
	FileRegex                  *regexp.Regexp
	Selector                   []app_selector.Selector
	FilesChanged               []string
	IgnoreInvalidWatchPattern  bool
	WatchIfNoWatchPatternFound bool
	// InferAppDependencies enables deriving each application's local-repo file dependencies
	// (spec.source.path and helm.valueFiles) so it is rendered only when those files change.
	InferAppDependencies bool
	// RepoSelector identifies the repository under diff; used to decide which sources are local.
	RepoSelector repository.Selector
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

	if appSelectionOptions.InferAppDependencies {
		log.Info().Msg("🤖 Inferring application dependencies from spec.source.path and helm.valueFiles")
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
		// When inferring dependencies, ApplicationSets bypass the files-changed filter so they are
		// always generated; their generated Applications are then filtered by inferred dependencies
		// (and by their own watch-pattern/manifest-generate-paths annotations) in the re-filter step.
		if appSelectionOptions.InferAppDependencies && a.Kind == ApplicationSet {
			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("%s bypasses files-changed filter because dependency inference is enabled; generated Applications are filtered instead", a.Kind.ShortName())
			return true
		}
		selected, reason := a.filterByFilesChanged(appSelectionOptions.FilesChanged, appSelectionOptions.IgnoreInvalidWatchPattern, appSelectionOptions.WatchIfNoWatchPatternFound, appSelectionOptions.InferAppDependencies, appSelectionOptions.RepoSelector)
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

	if value, exists := annotations[annotationIgnore]; exists && value == "true" {
		return false, fmt.Sprintf("application is ignored because of '%s: %s'", annotationIgnore, value)
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

	if value, exists := annotations[annotationRender]; exists {
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
func (a *ArgoResource) filterByFilesChanged(filesChanged []string, ignoreInvalidWatchPattern bool, watchIfNoWatchPatternFound bool, inferAppDependencies bool, repoSelector repository.Selector) (bool, string) {
	if len(filesChanged) == 0 {
		return false, "no files changed"
	}

	// check if the application itself is in the list of files changed
	if slices.Contains(filesChanged, a.FileName) {
		return true, "application itself is in the list of files changed"
	}

	// Inferred local-repo dependency paths (only when enabled).
	var inferredPaths []string
	if inferAppDependencies {
		inferredPaths = a.inferLocalWatchPaths(repoSelector)
	}

	// Read the watch-pattern / manifest-generate-paths annotations (if any).
	var effectiveWatchPattern, effectiveManifestGeneratePaths string
	if annotations, found, err := unstructured.NestedStringMap(a.Yaml.Object, "metadata", "annotations"); err == nil && found && len(annotations) > 0 {
		effectiveWatchPattern = strings.TrimSpace(annotations[annotationWatchPattern])
		effectiveManifestGeneratePaths = strings.TrimSpace(annotations[v1alpha1.AnnotationKeyManifestGeneratePaths])
	}

	hasWatchPattern := effectiveWatchPattern != ""
	hasManifestGeneratePaths := effectiveManifestGeneratePaths != ""
	hasInferredPaths := len(inferredPaths) > 0

	// If there is nothing to match against (no annotations and no inferred dependencies),
	// fall back to the watch-if-no-watch-pattern-found setting.
	if !hasWatchPattern && !hasManifestGeneratePaths && !hasInferredPaths {
		return watchIfNoWatchPatternFound, "no watch-pattern, manifest-generate-paths annotation, or inferred dependencies found"
	}

	if hasWatchPattern {
		if selected, reason := a.filterByAnnotationWatchPattern(effectiveWatchPattern, filesChanged, ignoreInvalidWatchPattern); selected {
			return true, reason
		}
	}

	if hasManifestGeneratePaths {
		if selected, reason := a.filterByManifestGeneratePaths(effectiveManifestGeneratePaths, filesChanged); selected {
			return true, reason
		}
	}

	if hasInferredPaths && anyFileChangedUnderPaths(filesChanged, inferredPaths) {
		return true, "files changed match inferred application dependencies"
	}

	return false, "files changed does not match watch-pattern, manifest-generate-paths, or inferred dependencies"
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

// filterByManifestGeneratePaths checks if the application manifest-generate-paths matches any of the changed files
// Mimics the behavior of the watch pattern from ArgoCD: https://github.com/argoproj/argo-cd/blob/master/util/app/path/path.go#L122-L151
func (a *ArgoResource) filterByManifestGeneratePaths(manifestGeneratePaths string, filesChanged []string) (bool, string) {

	// Split the manifest paths by semicolon
	paths := strings.Split(manifestGeneratePaths, ";")

	if len(paths) == 0 {
		return false, fmt.Sprintf("no '%s' annotation found", v1alpha1.AnnotationKeyManifestGeneratePaths)
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

	if anyFileChangedUnderPaths(filesChanged, refreshPaths) {
		return true, fmt.Sprintf("files changed match manifest-generate-paths: '%s'", manifestGeneratePaths)
	}

	return false, fmt.Sprintf("no files changed match manifest-generate-paths: '%s'", manifestGeneratePaths)
}

// anyFileChangedUnderPaths reports whether any changed file equals, is contained within, or
// glob-matches any of the given paths. Paths may be directories (containment match) or specific
// files (exact match); all entries and changed files are treated as repo-root-relative.
func anyFileChangedUnderPaths(filesChanged []string, paths []string) bool {
	for _, f := range filesChanged {
		if !filepath.IsAbs(f) {
			f = string(filepath.Separator) + f
		}
		f = filepath.Clean(f)
		for _, item := range paths {
			if !filepath.IsAbs(item) {
				item = string(filepath.Separator) + item
			}
			item = filepath.Clean(item)
			if f == item {
				return true
			} else if _, err := argocdsecurity.EnforceToCurrentRoot(item, f); err == nil {
				return true
			} else if matched, err := filepath.Match(item, f); err == nil && matched {
				return true
			}
		}
	}
	return false
}

// inferLocalWatchPaths derives the repo-relative paths (source directories and Helm value files)
// that an application depends on within the local repository, based on its sources. It returns
// nil when the application has no local dependencies (e.g. only remote sources). The result is
// used as an implicit watch-pattern so the application is rendered only when one of these files
// changes. It is source-type agnostic (Helm, Kustomize, plain directory, jsonnet, ...).
func (a *ArgoResource) inferLocalWatchPaths(repoSelector repository.Selector) []string {
	if a.Yaml == nil {
		return nil
	}

	var specPath []string
	switch a.Kind {
	case Application:
		specPath = []string{"spec"}
	case ApplicationSet:
		specPath = []string{"spec", "template", "spec"}
	default:
		return nil
	}

	specMap, found, _ := unstructured.NestedMap(a.Yaml.Object, specPath...)
	if !found {
		return nil
	}

	// Collect sources (single source and multi-source forms).
	var sources []map[string]any
	if source, ok := specMap["source"].(map[string]any); ok {
		sources = append(sources, source)
	}
	if rawSources, ok := specMap["sources"].([]any); ok {
		for _, s := range rawSources {
			if source, ok := s.(map[string]any); ok {
				sources = append(sources, source)
			}
		}
	}
	if len(sources) == 0 {
		return nil
	}

	// Build a map of ref name -> ref source base path, for sources pointing at the local repo.
	// Ref sources supply the files referenced by "$ref/..." Helm value files.
	type refInfo struct {
		local bool
		path  string
	}
	refs := map[string]refInfo{}
	for _, source := range sources {
		ref, _ := source["ref"].(string)
		if ref == "" {
			continue
		}
		repoURL, _ := source["repoURL"].(string)
		path, _ := source["path"].(string)
		refs[ref] = refInfo{local: repoSelector.Matches(repoURL), path: path}
	}

	var paths []string
	for _, source := range sources {
		repoURL, _ := source["repoURL"].(string)
		_, hasChart := source["chart"]
		sourcePath, _ := source["path"].(string)
		ref, _ := source["ref"].(string)
		localSource := repoSelector.Matches(repoURL)

		// A ref-only source (ref set, no path) produces no manifests; it only supplies files
		// for "$ref/..." value-file references, so it contributes no directory watch.
		refOnly := ref != "" && sourcePath == ""

		// Directory watch: a local, non-chart, non-ref-only source contributes its path.
		if localSource && !hasChart && !refOnly {
			paths = append(paths, normalizeWatchPath(sourcePath))
		}

		// Value-files watch: resolve helm.valueFiles to repo-relative file paths. This runs
		// regardless of whether this source is a remote chart, so a remote chart whose values
		// come from a local "$ref" source is still tracked.
		helm, ok := source["helm"].(map[string]any)
		if !ok {
			continue
		}
		valueFiles, ok := helm["valueFiles"].([]any)
		if !ok {
			continue
		}
		for _, vfRaw := range valueFiles {
			vf, ok := vfRaw.(string)
			if !ok || strings.TrimSpace(vf) == "" {
				continue
			}
			if strings.HasPrefix(vf, "$") {
				refName, refPath, ok := splitRefPath(vf)
				if !ok {
					continue
				}
				info, known := refs[refName]
				if !known || !info.local {
					continue
				}
				// Ref sources usually have no path (repo root); join is robust either way.
				paths = append(paths, normalizeWatchPath(filepath.Join(info.path, refPath)))
				continue
			}
			// Plain relative value file: meaningful only for a local source, resolved relative
			// to that source's path.
			if localSource {
				paths = append(paths, normalizeWatchPath(filepath.Join(sourcePath, vf)))
			}
		}
	}

	return paths
}

// normalizeWatchPath cleans a repo-relative path, mapping empty/"." to the repo root (".").
func normalizeWatchPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "." {
		return "."
	}
	return filepath.Clean(p)
}

// splitRefPath splits a "$refName/path/to/file" Helm value-file reference into the ref name and
// the path within that ref source. Returns ok=false when there is no path component.
func splitRefPath(vf string) (refName string, refPath string, ok bool) {
	if !strings.HasPrefix(vf, "$") {
		return "", "", false
	}
	name, path, found := strings.Cut(vf[1:], "/")
	if !found || name == "" || path == "" {
		return "", "", false
	}
	return name, path, true
}
