package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateGitDiff_HideDeletedAppDiffMessage(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, "base")
	targetPath := filepath.Join(tempDir, "target")

	baseApps := []AppInfo{
		{
			Id:          "app.yaml",
			Name:        "app",
			SourcePath:  "/path/app",
			FileContent: "kind: ConfigMap\nmetadata:\n  name: app\n",
		},
	}

	summary, markdownSections, htmlSections, err := generateGitDiff(basePath, targetPath, nil, 3, true, baseApps, nil, "")
	if err != nil {
		t.Fatalf("generateGitDiff failed: %v", err)
	}
	if summary == "No changes found" {
		t.Fatalf("expected changes for deleted app, got %q", summary)
	}
	if len(markdownSections) != 1 {
		t.Fatalf("expected 1 markdown section, got %d", len(markdownSections))
	}
	if len(htmlSections) != 1 {
		t.Fatalf("expected 1 html section, got %d", len(htmlSections))
	}
	if markdownSections[0].content != deletedAppDiffHiddenMessage {
		t.Fatalf("markdown content = %q, want %q", markdownSections[0].content, deletedAppDiffHiddenMessage)
	}
	if htmlSections[0].content != deletedAppDiffHiddenMessage {
		t.Fatalf("html content = %q, want %q", htmlSections[0].content, deletedAppDiffHiddenMessage)
	}
}

// TestGenerateGitDiff_HideDeletedAppDiff tests the hideDeletedAppDiff parameter behavior
func TestGenerateGitDiff_HideDeletedAppDiff(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "diff-test-hide-deleted-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	basePath := filepath.Join(tempDir, "base")
	targetPath := filepath.Join(tempDir, "target")

	appContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1`

	modifiedContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 2`

	// Base apps: has both app-to-delete and app-to-modify
	baseApps := []AppInfo{
		{
			Id:          "deleted-app.yaml",
			Name:        "app-to-delete",
			SourcePath:  "/path/to/deleted",
			FileContent: appContent,
		},
		{
			Id:          "modified-app.yaml",
			Name:        "app-to-modify",
			SourcePath:  "/path/to/modified",
			FileContent: appContent,
		},
	}

	// Target apps: only has app-to-modify (app-to-delete is deleted)
	targetApps := []AppInfo{
		{
			Id:          "modified-app.yaml",
			Name:        "app-to-modify",
			SourcePath:  "/path/to/modified",
			FileContent: modifiedContent,
		},
	}

	t.Run("hideDeletedAppDiff=false shows full diff for deleted apps", func(t *testing.T) {

		hideDeletedAppDiff := false

		summary, markdownSections, htmlSections, err := generateGitDiff(
			basePath, targetPath, nil, 3, hideDeletedAppDiff, baseApps, targetApps, "",
		)

		if err != nil {
			t.Fatalf("generateGitDiff failed: %v", err)
		}

		// Should have 2 sections: one deleted, one modified
		if len(markdownSections) != 2 {
			t.Errorf("Expected 2 sections, got %d", len(markdownSections))
		}
		if len(htmlSections) != 2 {
			t.Errorf("Expected 2 HTML sections, got %d", len(htmlSections))
		}

		// Summary should mention both deleted and modified
		if !strings.Contains(summary, "Deleted") {
			t.Errorf("Summary should contain 'Deleted', got: %s", summary)
		}
		if !strings.Contains(summary, "Modified") {
			t.Errorf("Summary should contain 'Modified', got: %s", summary)
		}

		// Find the deleted app section and verify it has diff content
		foundDeletedWithContent := false
		for _, section := range markdownSections {
			sectionContent, _ := section.build(10000)
			if strings.Contains(sectionContent, "app-to-delete") {
				if strings.Contains(sectionContent, "Application deleted") {
					// Should have actual diff content (minus lines showing the deleted content)
					if strings.Contains(sectionContent, "- apiVersion:") || strings.Contains(sectionContent, "-apiVersion:") {
						foundDeletedWithContent = true
					}
				}
			}
		}

		if !foundDeletedWithContent {
			t.Error("With hideDeletedAppDiff=false, deleted app should have diff content showing removed lines")
		}
	})

	t.Run("hideDeletedAppDiff=true hides diff content for deleted apps", func(t *testing.T) {
		hideDeletedAppDiff := true
		summary, markdownSections, htmlSections, err := generateGitDiff(
			basePath, targetPath, nil, 3, hideDeletedAppDiff, baseApps, targetApps, "",
		)

		if err != nil {
			t.Fatalf("generateGitDiff failed: %v", err)
		}

		// Should have 2 sections: one deleted (header only), one modified
		if len(markdownSections) != 2 {
			t.Errorf("Expected 2 sections, got %d", len(markdownSections))
		}
		if len(htmlSections) != 2 {
			t.Errorf("Expected 2 HTML sections, got %d", len(htmlSections))
		}

		// Summary should still mention both deleted and modified
		if !strings.Contains(summary, "Deleted") {
			t.Errorf("Summary should contain 'Deleted', got: %s", summary)
		}
		if !strings.Contains(summary, "Modified") {
			t.Errorf("Summary should contain 'Modified', got: %s", summary)
		}

		// Find the deleted app section and verify it shows the hidden message instead of full diff
		for _, section := range markdownSections {
			sectionContent, _ := section.build(10000)
			if strings.Contains(sectionContent, "app-to-delete") {
				// Should have the deletion header
				if !strings.Contains(sectionContent, "Application deleted") {
					t.Error("Deleted app should have deletion header")
				}
				// Should have the hidden message
				if !strings.Contains(sectionContent, deletedAppDiffHiddenMessage) {
					t.Errorf("With hideDeletedAppDiff=true, deleted app should show hidden message, got: %s", sectionContent)
				}
				// Should NOT have actual diff content (no minus lines showing removed content)
				if strings.Contains(sectionContent, "- apiVersion:") || strings.Contains(sectionContent, "-apiVersion:") {
					t.Error("With hideDeletedAppDiff=true, deleted app should NOT have diff content showing removed lines")
				}
			}
		}

		// Verify modified app still has its diff content
		foundModifiedWithContent := false
		for _, section := range markdownSections {
			sectionContent, _ := section.build(10000)
			if strings.Contains(sectionContent, "app-to-modify") {
				if strings.Contains(sectionContent, "replicas: 1") && strings.Contains(sectionContent, "replicas: 2") {
					foundModifiedWithContent = true
				}
			}
		}

		if !foundModifiedWithContent {
			t.Error("Modified app should still have its diff content")
		}
	})
}
