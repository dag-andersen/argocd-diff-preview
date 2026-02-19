package matching

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	"github.com/dag-andersen/argocd-diff-preview/pkg/resource_filter"
	"github.com/go-git/go-git/v5/utils/diff"
	"github.com/sergi/go-diff/diffmatchpatch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// ResourceDiff represents the diff output for a single resource within an application
type ResourceDiff struct {
	Kind         string
	Name         string
	Namespace    string
	Content      string // diff text (with +/-/space prefixes)
	AddedLines   int
	DeletedLines int
	IsSkipped    bool // true if resource matched an ignore rule
}

// Header returns a display header like "Kind/Name (namespace)" or "Kind/Name"
func (r *ResourceDiff) Header() string {
	if r.Namespace != "" {
		return fmt.Sprintf("%s/%s (%s)", r.Kind, r.Name, r.Namespace)
	}
	return fmt.Sprintf("%s/%s", r.Kind, r.Name)
}

// AppDiff represents the diff output for a single application pair
type AppDiff struct {
	OldName       string // Name in base branch (empty if added)
	NewName       string // Name in target branch (empty if deleted)
	OldSourcePath string // Source path in base branch
	NewSourcePath string // Source path in target branch
	Action        DiffAction
	Resources     []ResourceDiff // Per-resource diffs
	AddedLines    int
	DeletedLines  int
}

// DiffAction represents the type of change
type DiffAction int

const (
	ActionAdded DiffAction = iota
	ActionDeleted
	ActionModified
	ActionUnchanged
)

func (a DiffAction) String() string {
	switch a {
	case ActionAdded:
		return "added"
	case ActionDeleted:
		return "deleted"
	case ActionModified:
		return "modified"
	case ActionUnchanged:
		return "unchanged"
	default:
		return "unknown"
	}
}

// PrettyName returns a display name for the diff
func (d *AppDiff) PrettyName() string {
	switch {
	case d.NewName != "" && d.OldName != "" && d.NewName != d.OldName:
		return fmt.Sprintf("%s -> %s", d.OldName, d.NewName)
	case d.NewName != "":
		return d.NewName
	case d.OldName != "":
		return d.OldName
	default:
		return "Unknown"
	}
}

// PrettyPath returns a display path for the diff
func (d *AppDiff) PrettyPath() string {
	switch {
	case d.NewSourcePath != "" && d.OldSourcePath != "" && d.NewSourcePath != d.OldSourcePath:
		return fmt.Sprintf("%s -> %s", d.OldSourcePath, d.NewSourcePath)
	case d.NewSourcePath != "":
		return d.NewSourcePath
	case d.OldSourcePath != "":
		return d.OldSourcePath
	default:
		return "Unknown"
	}
}

// ChangeStats returns a formatted string showing +/- line counts
func (d *AppDiff) ChangeStats() string {
	switch {
	case d.AddedLines > 0 && d.DeletedLines > 0:
		return fmt.Sprintf(" (+%d|-%d)", d.AddedLines, d.DeletedLines)
	case d.AddedLines > 0:
		return fmt.Sprintf(" (+%d)", d.AddedLines)
	case d.DeletedLines > 0:
		return fmt.Sprintf(" (-%d)", d.DeletedLines)
	default:
		return ""
	}
}

// HasContent returns true if the diff has any resource content to show
func (d *AppDiff) HasContent() bool {
	return len(d.Resources) > 0
}

// GenerateAppDiffs uses similarity matching to generate diffs between base and target apps.
// This replaces the ID-based matching with content-based matching.
func GenerateAppDiffs(
	baseApps, targetApps []extract.ExtractedApp,
	contextLines uint,
	ignorePattern *string,
	ignoreResourceRules []resource_filter.IgnoreResourceRule,
) ([]AppDiff, error) {
	// Match apps by content similarity
	pairs := MatchApps(baseApps, targetApps)

	var diffs []AppDiff

	for _, pair := range pairs {
		appDiff, err := generateAppDiff(pair, contextLines, ignorePattern, ignoreResourceRules)
		if err != nil {
			return nil, fmt.Errorf("failed to generate diff for app pair: %w", err)
		}

		// Skip unchanged apps
		if appDiff.Action == ActionUnchanged {
			continue
		}

		diffs = append(diffs, appDiff)
	}

	// Sort diffs: deleted first, then modified, then added (like existing behavior)
	sort.SliceStable(diffs, func(i, j int) bool {
		if diffs[i].Action != diffs[j].Action {
			// Order: Deleted (1), Modified (2), Added (0) -> we want Deleted, Modified, Added
			// Map to sort order: Added=2, Deleted=0, Modified=1
			order := map[DiffAction]int{ActionDeleted: 0, ActionModified: 1, ActionAdded: 2}
			return order[diffs[i].Action] < order[diffs[j].Action]
		}
		if diffs[i].PrettyName() != diffs[j].PrettyName() {
			return diffs[i].PrettyName() < diffs[j].PrettyName()
		}
		return diffs[i].PrettyPath() < diffs[j].PrettyPath()
	})

	return diffs, nil
}

// generateAppDiff generates the diff for a single app pair
func generateAppDiff(pair Pair, contextLines uint, ignorePattern *string, ignoreResourceRules []resource_filter.IgnoreResourceRule) (AppDiff, error) {
	diff := AppDiff{}

	// Set names and paths
	if pair.Base != nil {
		diff.OldName = pair.Base.Name
		diff.OldSourcePath = pair.Base.SourcePath
	}
	if pair.Target != nil {
		diff.NewName = pair.Target.Name
		diff.NewSourcePath = pair.Target.SourcePath
	}

	// Determine action
	switch {
	case pair.Base == nil && pair.Target != nil:
		diff.Action = ActionAdded
	case pair.Base != nil && pair.Target == nil:
		diff.Action = ActionDeleted
	case pair.Base != nil && pair.Target != nil:
		diff.Action = ActionModified
	default:
		return diff, nil // Both nil - shouldn't happen
	}

	// Get changed resources
	changedResources := pair.ChangedResources()

	// If no changed resources and it was a modification, it's unchanged.
	// Added/deleted apps with zero resources should still be reported.
	if len(changedResources) == 0 {
		if diff.Action == ActionModified {
			diff.Action = ActionUnchanged
		}
		return diff, nil
	}

	// Build per-resource diffs
	resources, added, deleted, err := buildResourceDiffs(changedResources, contextLines, ignorePattern, ignoreResourceRules)
	if err != nil {
		return diff, err
	}

	// If all changes were filtered out by ignorePattern, mark as unchanged
	if len(resources) == 0 && diff.Action == ActionModified {
		diff.Action = ActionUnchanged
		return diff, nil
	}

	diff.Resources = resources
	diff.AddedLines = added
	diff.DeletedLines = deleted

	return diff, nil
}

// buildResourceDiffs generates per-resource diffs for all changed resources in an app
func buildResourceDiffs(
	resources []ResourcePair,
	contextLines uint,
	ignorePattern *string,
	ignoreResourceRules []resource_filter.IgnoreResourceRule,
) ([]ResourceDiff, int, int, error) {
	var result []ResourceDiff
	totalAdded := 0
	totalDeleted := 0

	// Sort resources by API version, kind, namespace, name - with CRDs always at the end.
	// This matches the sorting in ExtractedApp.sortManifests() for consistent output.
	sortedResources := make([]ResourcePair, len(resources))
	copy(sortedResources, resources)
	sort.SliceStable(sortedResources, func(i, j int) bool {
		ri := getResourceRef(&sortedResources[i])
		rj := getResourceRef(&sortedResources[j])

		// CRDs should always be at the end
		isCRD_I := ri.kind == "CustomResourceDefinition"
		isCRD_J := rj.kind == "CustomResourceDefinition"
		if isCRD_I != isCRD_J {
			return !isCRD_I
		}

		// Sort by apiVersion, then kind, then namespace, then name
		if ri.apiVersion != rj.apiVersion {
			return ri.apiVersion < rj.apiVersion
		}
		if ri.kind != rj.kind {
			return ri.kind < rj.kind
		}
		if ri.namespace != rj.namespace {
			return ri.namespace < rj.namespace
		}
		return ri.name < rj.name
	})

	for _, rp := range sortedResources {
		ref := getResourceRef(&rp)

		// Check if this resource matches any ignore rules.
		// If so, emit a skipped resource entry instead of the full diff.
		if len(ignoreResourceRules) > 0 && resourceMatchesIgnoreRules(&rp, ignoreResourceRules) {
			result = append(result, ResourceDiff{
				Kind:      ref.kind,
				Name:      ref.name,
				Namespace: ref.namespace,
				IsSkipped: true,
			})
			continue
		}

		// Generate diff for this resource pair
		diffResult, err := generateResourceDiff(rp, contextLines, ignorePattern)
		if err != nil {
			return nil, 0, 0, err
		}

		if diffResult.Content != "" {
			result = append(result, ResourceDiff{
				Kind:         ref.kind,
				Name:         ref.name,
				Namespace:    ref.namespace,
				Content:      diffResult.Content,
				AddedLines:   diffResult.AddedLines,
				DeletedLines: diffResult.DeletedLines,
			})
			totalAdded += diffResult.AddedLines
			totalDeleted += diffResult.DeletedLines
		}
	}

	return result, totalAdded, totalDeleted, nil
}

// generateResourceDiff generates diff for a single resource pair with ignore pattern support
func generateResourceDiff(rp ResourcePair, contextLines uint, ignorePattern *string) (DiffResult, error) {
	baseYAML, err := resourceToYAMLSorted(rp.Base)
	if err != nil {
		return DiffResult{}, fmt.Errorf("failed to marshal base resource: %w", err)
	}

	targetYAML, err := resourceToYAMLSorted(rp.Target)
	if err != nil {
		return DiffResult{}, fmt.Errorf("failed to marshal target resource: %w", err)
	}

	// Use the existing format logic from pkg/diff which handles ignore patterns
	return formatResourceDiff(baseYAML, targetYAML, contextLines, ignorePattern), nil
}

// resourceToYAMLSorted converts an unstructured resource to YAML string with sorted keys
func resourceToYAMLSorted(r *unstructured.Unstructured) (string, error) {
	if r == nil {
		return "", nil
	}

	yamlBytes, err := yaml.Marshal(r.Object)
	if err != nil {
		return "", err
	}

	return string(yamlBytes), nil
}

// resourceRef holds sort-relevant fields from a ResourcePair
type resourceRef struct {
	apiVersion string
	kind       string
	namespace  string
	name       string
}

// getResourceRef extracts sort-relevant fields from a ResourcePair,
// preferring the target resource if present (since it represents the new state).
// Falls back to base for deleted resources.
func getResourceRef(rp *ResourcePair) resourceRef {
	if rp.Target != nil {
		return resourceRef{
			apiVersion: rp.Target.GetAPIVersion(),
			kind:       rp.Target.GetKind(),
			namespace:  rp.Target.GetNamespace(),
			name:       rp.Target.GetName(),
		}
	}
	if rp.Base != nil {
		return resourceRef{
			apiVersion: rp.Base.GetAPIVersion(),
			kind:       rp.Base.GetKind(),
			namespace:  rp.Base.GetNamespace(),
			name:       rp.Base.GetName(),
		}
	}
	return resourceRef{}
}

// formatResourceDiff formats a diff between two YAML strings with ignore pattern support
func formatResourceDiff(baseYAML, targetYAML string, contextLines uint, ignorePattern *string) DiffResult {
	diffs := diff.Do(baseYAML, targetYAML)
	return formatDiffWithIgnore(diffs, contextLines, ignorePattern)
}

// Patterns that should always be ignored (same as pkg/diff/format.go)
var defaultIgnorePatterns = []string{
	"app.kubernetes.io/version: ",
	"helm.sh/chart: ",
	"checksum/config: ",
	"checksum/rules: ",
	"checksum/certs: ",
	"checksum/cmd-params: ",
	"checksum/cm: ",
	"checksum/config-maps: ",
	"checksum/secrets: ",
	"caBundle: ",
}

// formatDiffWithIgnore formats diffs with support for ignore patterns
func formatDiffWithIgnore(diffs []diffmatchpatch.Diff, contextLines uint, ignorePattern *string) DiffResult {
	// Process diffs into lines with metadata
	processedLines := processDiffLinesWithIgnore(diffs, ignorePattern)

	// Find indices of changed lines that should be shown
	var changedLines []int
	for i, line := range processedLines {
		if line.isChange && line.show {
			changedLines = append(changedLines, i)
		}
	}

	if len(changedLines) == 0 {
		return DiffResult{Content: "", AddedLines: 0, DeletedLines: 0}
	}

	// Build chunks of lines to include based on context
	chunks := buildChunks(changedLines, len(processedLines), contextLines)

	// Build output from chunks
	return buildOutputWithIgnore(chunks, processedLines)
}

// processedLineWithIgnore includes metadata for filtering
type processedLineWithIgnore struct {
	operation diffmatchpatch.Operation
	text      string
	isChange  bool
	show      bool
}

// processDiffLinesWithIgnore converts raw diffs into processedLine structs with ignore support
func processDiffLinesWithIgnore(diffs []diffmatchpatch.Diff, ignorePattern *string) []processedLineWithIgnore {
	var processedLines []processedLineWithIgnore

	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		isChange := d.Type != diffmatchpatch.DiffEqual

		for _, line := range lines {
			show := shouldShowLine(line, isChange, ignorePattern)
			processedLines = append(processedLines, processedLineWithIgnore{
				operation: d.Type,
				text:      line,
				isChange:  isChange,
				show:      show,
			})
		}
	}

	return processedLines
}

// shouldShowLine determines if a line should be shown in the diff output
func shouldShowLine(line string, isChange bool, ignorePattern *string) bool {
	if !isChange {
		return true
	}

	// Check custom ignore pattern
	if ignorePattern != nil && *ignorePattern != "" {
		if shouldIgnoreLine(line, *ignorePattern) {
			return false
		}
	}

	// Check default ignore patterns
	for _, pattern := range defaultIgnorePatterns {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), pattern) {
			return false
		}
	}

	return true
}

// shouldIgnoreLine checks if a line should be ignored based on regex pattern
func shouldIgnoreLine(line, pattern string) bool {
	matched, err := regexp.MatchString(pattern, line)
	if err != nil {
		// If regex fails, fall back to simple string matching
		return strings.Contains(line, pattern)
	}
	return matched
}

// buildOutputWithIgnore converts chunks into the final diff output string
func buildOutputWithIgnore(chunks []chunk, processedLines []processedLineWithIgnore) DiffResult {
	var buffer bytes.Buffer
	addedLines := 0
	deletedLines := 0

	for i, c := range chunks {
		for j := c.start; j <= c.end; j++ {
			line := processedLines[j]
			switch line.operation {
			case diffmatchpatch.DiffInsert:
				addedLines++
				buffer.WriteString("+" + line.text + "\n")
			case diffmatchpatch.DiffDelete:
				deletedLines++
				buffer.WriteString("-" + line.text + "\n")
			default:
				buffer.WriteString(" " + line.text + "\n")
			}
		}

		// Add separator if there's a next chunk
		if i < len(chunks)-1 {
			nextChunk := chunks[i+1]
			if skippedLines := nextChunk.start - c.end - 1; skippedLines > 0 {
				separator := fmt.Sprintf("@@ skipped %d lines (%d -> %d) @@", skippedLines, c.end+1, nextChunk.start-1)
				buffer.WriteString(separator + "\n")
			}
		}
	}

	return DiffResult{Content: buffer.String(), AddedLines: addedLines, DeletedLines: deletedLines}
}

// resourceMatchesIgnoreRules checks if either side of a ResourcePair matches any ignore rule
func resourceMatchesIgnoreRules(rp *ResourcePair, rules []resource_filter.IgnoreResourceRule) bool {
	if rp.Base != nil && resource_filter.MatchesAnyIgnoreRule(rp.Base, rules) {
		return true
	}
	if rp.Target != nil && resource_filter.MatchesAnyIgnoreRule(rp.Target, rules) {
		return true
	}
	return false
}
