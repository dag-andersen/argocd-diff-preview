package diff

// ResourceSection represents a single resource's diff within an app section.
// This is the pkg/diff view of matching.ResourceDiff, avoiding import cycles.
type ResourceSection struct {
	Header    string // e.g. "Kind/Name (namespace)"
	Content   string // diff text with +/-/space prefixes
	IsSkipped bool   // true if resource matched an ignore rule
}
