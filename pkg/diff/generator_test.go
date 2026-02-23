package diff

import (
	"strings"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/matching"
)

// Tests for buildMatchingSummary

func TestBuildMatchingSummary_NoDiffs(t *testing.T) {
	result := buildSummary(nil)
	if result != "No changes found" {
		t.Errorf("expected 'No changes found', got %q", result)
	}
}

func TestBuildMatchingSummary_EmptySlice(t *testing.T) {
	result := buildSummary([]matching.AppDiff{})
	if result != "No changes found" {
		t.Errorf("expected 'No changes found', got %q", result)
	}
}

func TestBuildMatchingSummary_OnlyAdded(t *testing.T) {
	diffs := []matching.AppDiff{
		{NewName: "app-1", Action: matching.ActionAdded, AddedLines: 10},
		{NewName: "app-2", Action: matching.ActionAdded, AddedLines: 5},
	}

	result := buildSummary(diffs)

	if !strings.Contains(result, "Added (2):") {
		t.Errorf("expected 'Added (2):', got:\n%s", result)
	}
	if !strings.Contains(result, "+ app-1 (+10)") {
		t.Errorf("expected '+ app-1 (+10)', got:\n%s", result)
	}
	if !strings.Contains(result, "+ app-2 (+5)") {
		t.Errorf("expected '+ app-2 (+5)', got:\n%s", result)
	}
	// Should NOT contain Deleted or Modified sections
	if strings.Contains(result, "Deleted") {
		t.Errorf("should not contain 'Deleted', got:\n%s", result)
	}
	if strings.Contains(result, "Modified") {
		t.Errorf("should not contain 'Modified', got:\n%s", result)
	}
}

func TestBuildMatchingSummary_OnlyDeleted(t *testing.T) {
	diffs := []matching.AppDiff{
		{OldName: "app-1", Action: matching.ActionDeleted, DeletedLines: 15},
	}

	result := buildSummary(diffs)

	if !strings.Contains(result, "Deleted (1):") {
		t.Errorf("expected 'Deleted (1):', got:\n%s", result)
	}
	if !strings.Contains(result, "- app-1 (-15)") {
		t.Errorf("expected '- app-1 (-15)', got:\n%s", result)
	}
}

func TestBuildMatchingSummary_OnlyModified(t *testing.T) {
	diffs := []matching.AppDiff{
		{OldName: "app-1", NewName: "app-1", Action: matching.ActionModified, AddedLines: 3, DeletedLines: 2},
	}

	result := buildSummary(diffs)

	if !strings.Contains(result, "Modified (1):") {
		t.Errorf("expected 'Modified (1):', got:\n%s", result)
	}
	if !strings.Contains(result, "± app-1 (+3|-2)") {
		t.Errorf("expected '± app-1 (+3|-2)', got:\n%s", result)
	}
}

func TestBuildMatchingSummary_MixedActions(t *testing.T) {
	diffs := []matching.AppDiff{
		{OldName: "deleted-app", Action: matching.ActionDeleted, DeletedLines: 20},
		{OldName: "mod-app", NewName: "mod-app", Action: matching.ActionModified, AddedLines: 5, DeletedLines: 3},
		{NewName: "new-app", Action: matching.ActionAdded, AddedLines: 12},
	}

	result := buildSummary(diffs)

	if !strings.Contains(result, "Added (1):") {
		t.Errorf("expected 'Added (1):', got:\n%s", result)
	}
	if !strings.Contains(result, "Deleted (1):") {
		t.Errorf("expected 'Deleted (1):', got:\n%s", result)
	}
	if !strings.Contains(result, "Modified (1):") {
		t.Errorf("expected 'Modified (1):', got:\n%s", result)
	}
}

func TestBuildMatchingSummary_RenamedApp(t *testing.T) {
	diffs := []matching.AppDiff{
		{OldName: "old-name", NewName: "new-name", Action: matching.ActionModified, AddedLines: 1},
	}

	result := buildSummary(diffs)

	// PrettyName for renamed app should show "old-name -> new-name"
	if !strings.Contains(result, "± old-name -> new-name") {
		t.Errorf("expected renamed app in summary, got:\n%s", result)
	}
}

func TestBuildMatchingSummary_NoChangeStats(t *testing.T) {
	// An app with 0 added and 0 deleted lines should show no stats
	diffs := []matching.AppDiff{
		{OldName: "app-1", NewName: "app-1", Action: matching.ActionModified},
	}

	result := buildSummary(diffs)

	// ChangeStats() returns "" when both are 0, so just the name
	if !strings.Contains(result, "± app-1\n") {
		t.Errorf("expected 'app-1' without stats, got:\n%s", result)
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
	if md[0].resources[0].Header != "Deployment: my-deploy (default)" {
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
