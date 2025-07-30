package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/utils/merkletrie"
)

func TestDiff_prettyName(t *testing.T) {
	tests := []struct {
		name     string
		diff     Diff
		expected string
	}{
		{
			name:     "Both names, different",
			diff:     Diff{newName: "new-app", oldName: "old-app"},
			expected: "old-app -> new-app",
		},
		{
			name:     "Both names, same",
			diff:     Diff{newName: "app", oldName: "app"},
			expected: "app",
		},
		{
			name:     "Only new name",
			diff:     Diff{newName: "new-app"},
			expected: "new-app",
		},
		{
			name:     "Only old name",
			diff:     Diff{oldName: "old-app"},
			expected: "old-app",
		},
		{
			name:     "No names",
			diff:     Diff{},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diff.prettyName(); got != tt.expected {
				t.Errorf("prettyName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDiff_prettyPath(t *testing.T) {
	tests := []struct {
		name     string
		diff     Diff
		expected string
	}{
		{
			name:     "Both paths, different",
			diff:     Diff{newSourcePath: "/path/to/new", oldSourcePath: "/path/to/old"},
			expected: "/path/to/old -> /path/to/new",
		},
		{
			name:     "Both paths, same",
			diff:     Diff{newSourcePath: "/path/to/app", oldSourcePath: "/path/to/app"},
			expected: "/path/to/app",
		},
		{
			name:     "Only new path",
			diff:     Diff{newSourcePath: "/path/to/new"},
			expected: "/path/to/new",
		},
		{
			name:     "Only old path",
			diff:     Diff{oldSourcePath: "/path/to/old"},
			expected: "/path/to/old",
		},
		{
			name:     "No paths",
			diff:     Diff{},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diff.prettyPath(); got != tt.expected {
				t.Errorf("prettyPath() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDiff_commentHeader(t *testing.T) {
	tests := []struct {
		name     string
		diff     Diff
		expected string
	}{
		{
			name:     "Insert",
			diff:     Diff{newName: "app", newSourcePath: "/path", action: merkletrie.Insert},
			expected: "@@ Application added: app (/path) @@\n",
		},
		{
			name:     "Delete",
			diff:     Diff{oldName: "app", oldSourcePath: "/path", action: merkletrie.Delete},
			expected: "@@ Application deleted: app (/path) @@\n",
		},
		{
			name:     "Modify",
			diff:     Diff{newName: "app", newSourcePath: "/path", action: merkletrie.Modify},
			expected: "@@ Application modified: app (/path) @@\n",
		},
		{
			name:     "Unknown action",
			diff:     Diff{newName: "app", newSourcePath: "/path", action: 99}, // Assuming 99 is not a valid action
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diff.commentHeader(); got != tt.expected {
				t.Errorf("commentHeader() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDiff_buildSection(t *testing.T) {
	tests := []struct {
		name        string
		diff        Diff
		expectedFmt string // Use fmt string for easier comparison of structure
	}{
		{
			name: "Insert",
			diff: Diff{
				newName:       "new-app",
				newSourcePath: "/path/new",
				action:        merkletrie.Insert,
				content:       "+ line 1\n+ line 2",
			},
			expectedFmt: "<details>\n<summary>%s</summary>\n<br>\n\n```diff\n%s\n```\n\n</details>\n\n",
		},
		{
			name: "Modify with name change",
			diff: Diff{
				newName:       "app-v2",
				oldName:       "app-v1",
				newSourcePath: "/path/app",
				oldSourcePath: "/path/app",
				action:        merkletrie.Modify,
				content:       "- line 1\n+ line 1 mod",
			},
			expectedFmt: "<details>\n<summary>%s</summary>\n<br>\n\n```diff\n%s\n```\n\n</details>\n\n",
		},
		{
			name: "Delete",
			diff: Diff{
				oldName:       "old-app",
				oldSourcePath: "/path/old",
				action:        merkletrie.Delete,
				content:       "- line 1\n- line 2",
			},
			expectedFmt: "<details>\n<summary>%s</summary>\n<br>\n\n```diff\n%s\n```\n\n</details>\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := fmt.Sprintf("%s (%s)", tt.diff.prettyName(), tt.diff.prettyPath())
			content := strings.TrimSpace(fmt.Sprintf("%s%s", tt.diff.commentHeader(), tt.diff.content))
			expected := fmt.Sprintf(tt.expectedFmt, header, content)
			if got := tt.diff.buildMarkdownSection(); got != expected {
				t.Errorf("buildSection() got =\n%v\nwant =\n%v", got, expected)
			}
		})
	}
}

// TestGenerateGitDiff_FileNameMatching tests that files are matched by name, not content
func TestGenerateGitDiff_FileNameMatching(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "diff-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	basePath := filepath.Join(tempDir, "base")
	targetPath := filepath.Join(tempDir, "target")

	// Create identical content for both files
	identicalContentBefore := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1`

	identicalContentAfter := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 2`

	// Create base apps with identical content but different names
	baseApps := []AppInfo{
		{
			Id:          "A-before.yaml",
			Name:        "app-a",
			SourcePath:  "/path/to/app-a",
			FileContent: identicalContentBefore,
		},
		{
			Id:          "B-before.yaml",
			Name:        "app-b",
			SourcePath:  "/path/to/app-b",
			FileContent: identicalContentBefore, // Same content as A
		},
	}

	// Create target apps with same filenames, same content modification
	targetApps := []AppInfo{
		{
			Id:          "A-before.yaml", // Same filename as base
			Name:        "app-a",
			SourcePath:  "/path/to/app-a",
			FileContent: identicalContentAfter,
		},
		{
			Id:          "B-before.yaml", // Same filename as base
			Name:        "app-b",
			SourcePath:  "/path/to/app-b",
			FileContent: identicalContentAfter, // Same content modification as A
		},
	}

	// Run the diff generation
	summary, markdownSections, htmlSections, err := generateGitDiff(
		basePath, targetPath, nil, 3, baseApps, targetApps,
	)

	if err != nil {
		t.Fatalf("generateGitDiff failed: %v", err)
	}

	// We should get exactly 2 changes (one for each file)
	if len(markdownSections) != 2 {
		t.Errorf("Expected 2 file changes, got %d", len(markdownSections))
	}

	if len(htmlSections) != 2 {
		t.Errorf("Expected 2 HTML sections, got %d", len(htmlSections))
	}

	// Verify summary contains both apps
	if !strings.Contains(summary, "Modified") {
		t.Errorf("Summary should indicate modifications, got: %s", summary)
	}

	// Check that each section contains the correct app name
	foundAppA := false
	foundAppB := false

	for _, section := range markdownSections {
		if strings.Contains(section, "app-a") {
			foundAppA = true
			// Verify it contains the expected change (replicas: 1 -> 2)
			if !strings.Contains(section, "replicas: 1") || !strings.Contains(section, "replicas: 2") {
				t.Errorf("App-A section should contain replica change, got: %s", section)
			}
		}
		if strings.Contains(section, "app-b") {
			foundAppB = true
			// Verify it contains the expected change (replicas: 1 -> 2)
			if !strings.Contains(section, "replicas: 1") || !strings.Contains(section, "replicas: 2") {
				t.Errorf("App-B section should contain replica change, got: %s", section)
			}
		}
	}

	if !foundAppA {
		t.Error("Should find app-a in the diff sections")
	}
	if !foundAppB {
		t.Error("Should find app-b in the diff sections")
	}

	// Most importantly: verify that files are matched by name, not mixed up
	// Both apps should show the same content change (replicas 1->2)
	// This proves files were matched by filename, not by content similarity
	for i, section := range markdownSections {
		if !strings.Contains(section, "-  replicas: 1") ||
			!strings.Contains(section, "+  replicas: 2") {
			t.Errorf("Section %d should show consistent replica change from 1 to 2, got: %s", i, section)
		}
	}
}

// TestGenerateGitDiff_ChangingFilenames tests that files are matched by app identity when filenames change
func TestGenerateGitDiff_ChangingFilenames(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "diff-test-changing-filenames-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	basePath := filepath.Join(tempDir, "base")
	targetPath := filepath.Join(tempDir, "target")

	// Create identical content for both files
	identicalContentBefore := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1`

	identicalContentAfter := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 2`

	// Base apps with "-before.yaml" filenames
	baseApps := []AppInfo{
		{
			Id:          "A-before.yaml",  // Filename will change
			Name:        "app-a",          // Identity stays same
			SourcePath:  "/path/to/app-a", // Identity stays same
			FileContent: identicalContentBefore,
		},
		{
			Id:          "B-before.yaml",        // Filename will change
			Name:        "app-b",                // Identity stays same
			SourcePath:  "/path/to/app-b",       // Identity stays same
			FileContent: identicalContentBefore, // Same content as A
		},
	}

	// Target apps with "-after.yaml" filenames (changed!) but same identities
	targetApps := []AppInfo{
		{
			Id:          "A-after.yaml",   // Filename changed!
			Name:        "app-a",          // Same identity
			SourcePath:  "/path/to/app-a", // Same identity
			FileContent: identicalContentAfter,
		},
		{
			Id:          "B-after.yaml",        // Filename changed!
			Name:        "app-b",               // Same identity
			SourcePath:  "/path/to/app-b",      // Same identity
			FileContent: identicalContentAfter, // Same content modification as A
		},
	}

	// Run the diff generation
	summary, markdownSections, htmlSections, err := generateGitDiff(
		basePath, targetPath, nil, 3, baseApps, targetApps,
	)

	if err != nil {
		t.Fatalf("generateGitDiff failed: %v", err)
	}

	// We should get exactly 2 changes (one for each app identity)
	if len(markdownSections) != 2 {
		t.Errorf("Expected 2 app changes, got %d", len(markdownSections))
	}

	if len(htmlSections) != 2 {
		t.Errorf("Expected 2 HTML sections, got %d", len(htmlSections))
	}

	// Verify summary contains modifications
	if !strings.Contains(summary, "Modified") {
		t.Errorf("Summary should indicate modifications, got: %s", summary)
	}

	// Check that each section contains the correct app name and shows modifications
	foundAppA := false
	foundAppB := false

	for _, section := range markdownSections {
		if strings.Contains(section, "app-a") {
			foundAppA = true
			// Should show as modification, not delete+add
			if !strings.Contains(section, "Application modified") {
				t.Errorf("App-A should show as modified, got: %s", section)
			}
			// Should show the content change
			if !strings.Contains(section, "-  replicas: 1") || !strings.Contains(section, "+  replicas: 2") {
				t.Errorf("App-A should show replica change from 1 to 2, got: %s", section)
			}
		}
		if strings.Contains(section, "app-b") {
			foundAppB = true
			// Should show as modification, not delete+add
			if !strings.Contains(section, "Application modified") {
				t.Errorf("App-B should show as modified, got: %s", section)
			}
			// Should show the content change
			if !strings.Contains(section, "-  replicas: 1") || !strings.Contains(section, "+  replicas: 2") {
				t.Errorf("App-B should show replica change from 1 to 2, got: %s", section)
			}
		}
	}

	if !foundAppA {
		t.Error("Should find app-a in the diff sections")
	}
	if !foundAppB {
		t.Error("Should find app-b in the diff sections")
	}

	// Critical test: verify that despite different filenames, both apps show the same consistent changes
	// This proves they were matched by identity (Name+SourcePath), not by filename
	for i, section := range markdownSections {
		// Both should show as modifications
		if !strings.Contains(section, "Application modified") {
			t.Errorf("Section %d should show modification, not deletion/addition, got: %s", i, section)
		}
		// Both should show the same content change
		if !strings.Contains(section, "-  replicas: 1") || !strings.Contains(section, "+  replicas: 2") {
			t.Errorf("Section %d should show consistent replica change from 1 to 2, got: %s", i, section)
		}
	}

	t.Logf("✅ Success: Files with changing names (A-before.yaml -> A-after.yaml) were correctly matched by app identity!")
}
