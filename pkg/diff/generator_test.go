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
	// Check that blocks contain the hidden message
	mdContent := formatBlocksToMarkdown(markdownSections[0].blocks)
	if !strings.Contains(mdContent, deletedAppDiffHiddenMessage) {
		t.Fatalf("markdown blocks should contain %q, got %q", deletedAppDiffHiddenMessage, mdContent)
	}
	// For HTML, we need to check the blocks directly
	if len(htmlSections[0].blocks) != 1 || htmlSections[0].blocks[0].Content != deletedAppDiffHiddenMessage {
		t.Fatalf("html blocks should contain %q", deletedAppDiffHiddenMessage)
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
				// Should have actual diff content (minus lines showing the deleted content)
				if strings.Contains(sectionContent, "- apiVersion:") || strings.Contains(sectionContent, "-apiVersion:") {
					foundDeletedWithContent = true
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

// TestGenerateGitDiff_ResourceKindChange tests diff output when a resource changes kind
// (e.g., ConfigMap → Secret).
func TestGenerateGitDiff_ResourceKindChange(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, "base")
	targetPath := filepath.Join(tempDir, "target")

	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  key: value`

	targetContent := `apiVersion: v1
kind: Secret
metadata:
  name: my-config
  namespace: default
type: Opaque
stringData:
  key: value`

	// Expected: header shows the kind transformation
	expectedOutput := `#### ConfigMap → Secret/my-config (default)
` + "```diff" + `
 apiVersion: v1
-kind: ConfigMap
+kind: Secret
 metadata:
   name: my-config
   namespace: default
-data:
+type: Opaque
+stringData:
   key: value
` + "```"

	baseApps := []AppInfo{{Id: "app.yaml", Name: "my-app", SourcePath: "/path/app", FileContent: baseContent}}
	targetApps := []AppInfo{{Id: "app.yaml", Name: "my-app", SourcePath: "/path/app", FileContent: targetContent}}

	_, markdownSections, _, err := generateGitDiff(basePath, targetPath, nil, 10, false, baseApps, targetApps, "")
	if err != nil {
		t.Fatalf("generateGitDiff failed: %v", err)
	}

	if len(markdownSections) != 1 {
		t.Fatalf("expected 1 markdown section, got %d", len(markdownSections))
	}

	actualOutput := strings.TrimSpace(formatBlocksToMarkdown(markdownSections[0].blocks))
	expectedOutput = strings.TrimSpace(expectedOutput)

	if actualOutput != expectedOutput {
		t.Errorf("Output mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedOutput, actualOutput)
	}
}

// TestGenerateGitDiff_ResourceDeletedWithContentMoved tests diff output when a resource
// is deleted but some of its content is moved to another resource.
// We split on all "---" separators, so each resource gets its own block.
func TestGenerateGitDiff_ResourceDeletedWithContentMoved(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, "base")
	targetPath := filepath.Join(tempDir, "target")

	baseContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  key: value
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: other-config
  namespace: default
data:
  keyOne: "1"
  keyTwo: "2"
  keyThree: "3"`

	targetContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  keyOne: "1"
  keyTwo: "2"
  keyThree: "3"`

	// Current behavior: all resources in one block with only the first resource header
	expectedOutput := `#### ConfigMap/my-config (default)
` + "```diff" + `
 apiVersion: v1
 kind: ConfigMap
 metadata:
   name: my-config
   namespace: default
 data:
-  key: value
-apiVersion: v1
-kind: ConfigMap
-metadata:
-  name: other-config
-  namespace: default
-data:
   keyOne: "1"
   keyTwo: "2"
   keyThree: "3"
` + "```"

	baseApps := []AppInfo{{Id: "app.yaml", Name: "my-app", SourcePath: "/path/app", FileContent: baseContent}}
	targetApps := []AppInfo{{Id: "app.yaml", Name: "my-app", SourcePath: "/path/app", FileContent: targetContent}}

	_, markdownSections, _, err := generateGitDiff(basePath, targetPath, nil, 10, false, baseApps, targetApps, "")
	if err != nil {
		t.Fatalf("generateGitDiff failed: %v", err)
	}

	if len(markdownSections) != 1 {
		t.Fatalf("expected 1 markdown section, got %d", len(markdownSections))
	}

	actualOutput := strings.TrimSpace(formatBlocksToMarkdown(markdownSections[0].blocks))
	expectedOutput = strings.TrimSpace(expectedOutput)

	if actualOutput != expectedOutput {
		t.Errorf("Output mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedOutput, actualOutput)
	}
}

// TestGenerateGitDiff_NewFileWithMultipleResources tests that a new file with multiple
// resources (separated by ---) produces separate resource blocks with proper headers.
func TestGenerateGitDiff_NewFileWithMultipleResources(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, "base")
	targetPath := filepath.Join(tempDir, "target")

	// New file with 3 resources
	targetContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 1
---
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: default
spec:
  type: ClusterIP
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-app-config
  namespace: default
data:
  key: value`

	// Each resource should get its own block with a header
	expectedOutput := `#### Deployment/my-app (default)
` + "```diff" + `
+apiVersion: apps/v1
+kind: Deployment
+metadata:
+  name: my-app
+  namespace: default
+spec:
+  replicas: 1
` + "```" + `
#### Service/my-app (default)
` + "```diff" + `
+apiVersion: v1
+kind: Service
+metadata:
+  name: my-app
+  namespace: default
+spec:
+  type: ClusterIP
` + "```" + `
#### ConfigMap/my-app-config (default)
` + "```diff" + `
+apiVersion: v1
+kind: ConfigMap
+metadata:
+  name: my-app-config
+  namespace: default
+data:
+  key: value
` + "```"

	baseApps := []AppInfo{} // No base apps - this is a new file
	targetApps := []AppInfo{{Id: "app.yaml", Name: "my-app", SourcePath: "/path/app", FileContent: targetContent}}

	_, markdownSections, _, err := generateGitDiff(basePath, targetPath, nil, 10, false, baseApps, targetApps, "")
	if err != nil {
		t.Fatalf("generateGitDiff failed: %v", err)
	}

	if len(markdownSections) != 1 {
		t.Fatalf("expected 1 markdown section, got %d", len(markdownSections))
	}

	actualOutput := strings.TrimSpace(formatBlocksToMarkdown(markdownSections[0].blocks))
	expectedOutput = strings.TrimSpace(expectedOutput)

	if actualOutput != expectedOutput {
		t.Errorf("Output mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedOutput, actualOutput)
	}
}

// TestGenerateGitDiff_DeletedFileWithMultipleResources tests that a deleted file
// with multiple resources produces separate blocks with proper headers for each.
func TestGenerateGitDiff_DeletedFileWithMultipleResources(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, "base")
	targetPath := filepath.Join(tempDir, "target")

	// Deleted file with 2 resources
	baseContent := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 1
---
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: default
spec:
  type: ClusterIP`

	// Current behavior: all resources in one block with only the first resource header
	expectedOutput := `#### Deployment/my-app (default)
` + "```diff" + `
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  name: my-app
-  namespace: default
-spec:
-  replicas: 1
-apiVersion: v1
-kind: Service
-metadata:
-  name: my-app
-  namespace: default
-spec:
-  type: ClusterIP
` + "```"

	baseApps := []AppInfo{{Id: "app.yaml", Name: "my-app", SourcePath: "/path/app", FileContent: baseContent}}
	targetApps := []AppInfo{} // App is deleted

	_, markdownSections, _, err := generateGitDiff(basePath, targetPath, nil, 10, false, baseApps, targetApps, "")
	if err != nil {
		t.Fatalf("generateGitDiff failed: %v", err)
	}

	if len(markdownSections) != 1 {
		t.Fatalf("expected 1 markdown section, got %d", len(markdownSections))
	}

	actualOutput := strings.TrimSpace(formatBlocksToMarkdown(markdownSections[0].blocks))
	expectedOutput = strings.TrimSpace(expectedOutput)

	if actualOutput != expectedOutput {
		t.Errorf("Output mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedOutput, actualOutput)
	}
}
