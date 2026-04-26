package diff

import (
	"strings"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/matching"
)

// Tests for buildMatchingSummary

func TestBuildMatchingSummary_NoDiffs(t *testing.T) {
	fullSummary, compactSummary := buildSummary(nil, 20)
	if fullSummary != "No changes found" {
		t.Errorf("expected 'No changes found', got %q", fullSummary)
	}
	if compactSummary != "" {
		t.Errorf("expected no compact summary, got %q", compactSummary)
	}
}

func TestBuildMatchingSummary_EmptySlice(t *testing.T) {
	fullSummary, compactSummary := buildSummary([]matching.AppDiff{}, 20)
	if fullSummary != "No changes found" {
		t.Errorf("expected 'No changes found', got %q", fullSummary)
	}
	if compactSummary != "" {
		t.Errorf("expected no compact summary, got %q", compactSummary)
	}
}

func TestBuildMatchingSummary_OnlyAdded(t *testing.T) {
	diffs := []matching.AppDiff{
		{NewName: "app-1", Action: matching.ActionAdded, AddedLines: 10},
		{NewName: "app-2", Action: matching.ActionAdded, AddedLines: 5},
	}

	fullSummary, compactSummary := buildSummary(diffs, 20)

	if !strings.Contains(fullSummary, "Added (2):") {
		t.Errorf("expected 'Added (2):', got:\n%s", fullSummary)
	}
	if !strings.Contains(fullSummary, "+ app-1 (+10)") {
		t.Errorf("expected '+ app-1 (+10)', got:\n%s", fullSummary)
	}
	if !strings.Contains(fullSummary, "+ app-2 (+5)") {
		t.Errorf("expected '+ app-2 (+5)', got:\n%s", fullSummary)
	}
	if compactSummary != "" {
		t.Errorf("expected no compact summary, got:\n%s", compactSummary)
	}
	// Should NOT contain Deleted or Modified sections
	if strings.Contains(fullSummary, "Deleted") {
		t.Errorf("should not contain 'Deleted', got:\n%s", fullSummary)
	}
	if strings.Contains(fullSummary, "Modified") {
		t.Errorf("should not contain 'Modified', got:\n%s", fullSummary)
	}
}

func TestBuildMatchingSummary_OnlyDeleted(t *testing.T) {
	diffs := []matching.AppDiff{
		{OldName: "app-1", Action: matching.ActionDeleted, DeletedLines: 15},
	}

	fullSummary, compactSummary := buildSummary(diffs, 20)

	if !strings.Contains(fullSummary, "Deleted (1):") {
		t.Errorf("expected 'Deleted (1):', got:\n%s", fullSummary)
	}
	if !strings.Contains(fullSummary, "- app-1 (-15)") {
		t.Errorf("expected '- app-1 (-15)', got:\n%s", fullSummary)
	}
	if compactSummary != "" {
		t.Errorf("expected no compact summary, got:\n%s", compactSummary)
	}
}

func TestBuildMatchingSummary_OnlyModified(t *testing.T) {
	diffs := []matching.AppDiff{
		{OldName: "app-1", NewName: "app-1", Action: matching.ActionModified, AddedLines: 3, DeletedLines: 2},
	}

	fullSummary, compactSummary := buildSummary(diffs, 20)

	if !strings.Contains(fullSummary, "Modified (1):") {
		t.Errorf("expected 'Modified (1):', got:\n%s", fullSummary)
	}
	if !strings.Contains(fullSummary, "± app-1 (+3|-2)") {
		t.Errorf("expected '± app-1 (+3|-2)', got:\n%s", fullSummary)
	}
	if compactSummary != "" {
		t.Errorf("expected no compact summary, got:\n%s", compactSummary)
	}
}

func TestBuildMatchingSummary_MixedActions(t *testing.T) {
	diffs := []matching.AppDiff{
		{OldName: "deleted-app", Action: matching.ActionDeleted, DeletedLines: 20},
		{OldName: "mod-app", NewName: "mod-app", Action: matching.ActionModified, AddedLines: 5, DeletedLines: 3},
		{NewName: "new-app", Action: matching.ActionAdded, AddedLines: 12},
	}

	summary, details := buildSummary(diffs, 20)

	if !strings.Contains(summary, "Added (1):") {
		t.Errorf("expected 'Added (1):', got:\n%s", summary)
	}
	if !strings.Contains(summary, "Deleted (1):") {
		t.Errorf("expected 'Deleted (1):', got:\n%s", summary)
	}
	if !strings.Contains(summary, "Modified (1):") {
		t.Errorf("expected 'Modified (1):', got:\n%s", summary)
	}
	if details != "" {
		t.Errorf("expected no details, got:\n%s", details)
	}
}

func TestBuildMatchingSummary_RenamedApp(t *testing.T) {
	diffs := []matching.AppDiff{
		{OldName: "old-name", NewName: "new-name", Action: matching.ActionModified, AddedLines: 1},
	}

	summary, details := buildSummary(diffs, 20)

	// PrettyName for renamed app should show "old-name -> new-name"
	if !strings.Contains(summary, "± old-name -> new-name") {
		t.Errorf("expected renamed app in summary, got:\n%s", summary)
	}
	if details != "" {
		t.Errorf("expected no details, got:\n%s", details)
	}
}

func TestBuildMatchingSummary_NoChangeStats(t *testing.T) {
	// An app with 0 added and 0 deleted lines should show no stats
	diffs := []matching.AppDiff{
		{OldName: "app-1", NewName: "app-1", Action: matching.ActionModified},
	}

	summary, details := buildSummary(diffs, 20)

	// ChangeStats() returns "" when both are 0, so just the name
	if !strings.Contains(summary, "± app-1\n") {
		t.Errorf("expected 'app-1' without stats, got:\n%s", summary)
	}
	if details != "" {
		t.Errorf("expected no details, got:\n%s", details)
	}
}

func TestBuildMatchingSummary_CollapsesLargeSummary(t *testing.T) {
	diffs := []matching.AppDiff{
		{NewName: "new-app", Action: matching.ActionAdded, AddedLines: 12},
		{OldName: "deleted-app", Action: matching.ActionDeleted, DeletedLines: 20},
		{OldName: "mod-app", NewName: "mod-app", Action: matching.ActionModified, AddedLines: 5, DeletedLines: 3},
	}

	fullSummary, compactSummary := buildSummary(diffs, 2)

	// compactSummary should contain only counts
	expectedCompact := []string{
		"Total: 3 applications changed",
		"Added: 1",
		"Deleted: 1",
		"Modified: 1",
	}
	for _, expected := range expectedCompact {
		if !strings.Contains(compactSummary, expected) {
			t.Errorf("expected compact summary to contain %q, got:\n%s", expected, compactSummary)
		}
	}

	// compactSummary should not contain per-app details
	unexpectedCompact := []string{"+ new-app (+12)", "- deleted-app (-20)", "± mod-app (+5|-3)"}
	for _, unexpected := range unexpectedCompact {
		if strings.Contains(compactSummary, unexpected) {
			t.Errorf("expected compact summary to omit %q, got:\n%s", unexpected, compactSummary)
		}
	}

	// fullSummary should contain the full per-app details (format-agnostic)
	expectedFullSummary := []string{
		"Added (1):",
		"+ new-app (+12)",
		"Deleted (1):",
		"- deleted-app (-20)",
		"Modified (1):",
		"± mod-app (+5|-3)",
	}
	for _, expected := range expectedFullSummary {
		if !strings.Contains(fullSummary, expected) {
			t.Errorf("expected full summary to contain %q, got:\n%s", expected, fullSummary)
		}
	}

	// Should not contain any markdown/HTML formatting
	unexpectedFullSummary := []string{"<details>", "<summary>", "```"}
	for _, unexpected := range unexpectedFullSummary {
		if strings.Contains(fullSummary, unexpected) {
			t.Errorf("expected full summary to be format-agnostic, but found %q in:\n%s", unexpected, fullSummary)
		}
	}
}

func TestBuildMatchingSummary_ThresholdZeroAlwaysShowsInline(t *testing.T) {
	diffs := []matching.AppDiff{
		{NewName: "app-1", Action: matching.ActionAdded, AddedLines: 10},
		{NewName: "app-2", Action: matching.ActionAdded, AddedLines: 5},
	}

	fullSummary, compactSummary := buildSummary(diffs, 0)

	if !strings.Contains(fullSummary, "+ app-1 (+10)") || !strings.Contains(fullSummary, "+ app-2 (+5)") {
		t.Errorf("expected full summary, got:\n%s", fullSummary)
	}
	if compactSummary != "" {
		t.Errorf("expected no compact summary when threshold is zero, got:\n%s", compactSummary)
	}
}

// Tests for buildAppURLFromDiff

func TestBuildAppURLFromDiff(t *testing.T) {
	tests := []struct {
		name     string
		diff     matching.AppDiff
		baseURL  string
		expected string
	}{
		{
			name:     "empty base URL",
			diff:     matching.AppDiff{OldName: "my-app", NewName: "my-app"},
			baseURL:  "",
			expected: "",
		},
		{
			name:     "prefers old name",
			diff:     matching.AppDiff{OldName: "old-app", NewName: "new-app"},
			baseURL:  "https://argocd.example.com",
			expected: "https://argocd.example.com/applications/old-app",
		},
		{
			name:     "falls back to new name when no old name",
			diff:     matching.AppDiff{NewName: "new-app"},
			baseURL:  "https://argocd.example.com",
			expected: "https://argocd.example.com/applications/new-app",
		},
		{
			name:     "both names empty",
			diff:     matching.AppDiff{},
			baseURL:  "https://argocd.example.com",
			expected: "",
		},
		{
			name:     "trailing slash in base URL",
			diff:     matching.AppDiff{OldName: "my-app"},
			baseURL:  "https://argocd.example.com/",
			expected: "https://argocd.example.com/applications/my-app",
		},
		{
			name:     "multiple trailing slashes in base URL",
			diff:     matching.AppDiff{OldName: "my-app"},
			baseURL:  "https://argocd.example.com///",
			expected: "https://argocd.example.com/applications/my-app",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := buildAppURLFromDiff(tc.diff, tc.baseURL)
			if result != tc.expected {
				t.Errorf("buildAppURLFromDiff() = %q, want %q", result, tc.expected)
			}
		})
	}
}

// Tests for buildMatchingSections

func TestBuildMatchingSections_Empty(t *testing.T) {
	md, html := buildMatchingSections(nil, "")
	if len(md) != 0 {
		t.Errorf("expected 0 markdown sections, got %d", len(md))
	}
	if len(html) != 0 {
		t.Errorf("expected 0 html sections, got %d", len(html))
	}
}

func TestBuildMatchingSections_WithResources(t *testing.T) {
	diffs := []matching.AppDiff{
		{
			OldName:       "my-app",
			NewName:       "my-app",
			OldSourcePath: "/path/to/app",
			NewSourcePath: "/path/to/app",
			Action:        matching.ActionModified,
			Resources: []matching.ResourceDiff{
				{
					Kind:      "Deployment",
					Name:      "my-deploy",
					Namespace: "default",
					Content:   "-replicas: 1\n+replicas: 3\n",
				},
			},
		},
	}

	md, html := buildMatchingSections(diffs, "https://argocd.example.com")

	if len(md) != 1 {
		t.Fatalf("expected 1 markdown section, got %d", len(md))
	}
	if len(html) != 1 {
		t.Fatalf("expected 1 html section, got %d", len(html))
	}

	// Check markdown section
	if md[0].appName != "my-app" {
		t.Errorf("expected appName='my-app', got %q", md[0].appName)
	}
	if md[0].filePath != "/path/to/app" {
		t.Errorf("expected filePath='/path/to/app', got %q", md[0].filePath)
	}
	if md[0].appURL != "https://argocd.example.com/applications/my-app" {
		t.Errorf("expected appURL with my-app, got %q", md[0].appURL)
	}
	if len(md[0].resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(md[0].resources))
	}
	if md[0].resources[0].Header != "Deployment: default/my-deploy" {
		t.Errorf("expected resource header, got %q", md[0].resources[0].Header)
	}

	// HTML section should mirror markdown
	if html[0].appName != md[0].appName {
		t.Errorf("html and markdown appName should match")
	}
	if html[0].appURL != md[0].appURL {
		t.Errorf("html and markdown appURL should match")
	}
}

func TestBuildMatchingSections_SkippedResource(t *testing.T) {
	diffs := []matching.AppDiff{
		{
			NewName:       "my-app",
			NewSourcePath: "/path",
			Action:        matching.ActionModified,
			Resources: []matching.ResourceDiff{
				{
					Kind:      "Secret",
					Name:      "my-secret",
					Namespace: "default",
					IsSkipped: true,
				},
			},
		},
	}

	md, _ := buildMatchingSections(diffs, "")
	if len(md) != 1 || len(md[0].resources) != 1 {
		t.Fatalf("expected 1 section with 1 resource")
	}
	if !md[0].resources[0].IsSkipped {
		t.Error("expected resource to be marked as skipped")
	}
}

func TestBuildMatchingSections_NoURL(t *testing.T) {
	diffs := []matching.AppDiff{
		{
			NewName:       "my-app",
			NewSourcePath: "/path",
			Action:        matching.ActionAdded,
		},
	}

	md, _ := buildMatchingSections(diffs, "")
	if md[0].appURL != "" {
		t.Errorf("expected empty appURL when no argocdUIURL, got %q", md[0].appURL)
	}
}
