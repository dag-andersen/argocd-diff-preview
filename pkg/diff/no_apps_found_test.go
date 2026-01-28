package diff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/app_selector"
)

func TestGetNoAppsFoundMessage(t *testing.T) {
	makeSelector := func(key, value string, op app_selector.Operator) app_selector.Selector {
		return app_selector.Selector{Key: key, Value: value, Operator: op}
	}

	tests := []struct {
		name         string
		selectors    []app_selector.Selector
		changedFiles []string
		expected     string
	}{
		{
			name:         "No selectors and no changed files",
			selectors:    nil,
			changedFiles: nil,
			expected:     "Found no Applications",
		},
		{
			name:         "Empty selectors and empty changed files",
			selectors:    []app_selector.Selector{},
			changedFiles: []string{},
			expected:     "Found no Applications",
		},
		{
			name: "Only selectors (single)",
			selectors: []app_selector.Selector{
				makeSelector("app", "myapp", app_selector.Eq),
			},
			changedFiles: nil,
			expected:     "Found no changed Applications that matched `app=myapp`",
		},
		{
			name: "Only selectors (multiple)",
			selectors: []app_selector.Selector{
				makeSelector("app", "myapp", app_selector.Eq),
				makeSelector("env", "prod", app_selector.Ne),
			},
			changedFiles: nil,
			expected:     "Found no changed Applications that matched `app=myapp, env!=prod`",
		},
		{
			name:         "Only changed files (single)",
			selectors:    nil,
			changedFiles: []string{"path/to/file.yaml"},
			expected:     "Found no changed Applications that watched these files: `path/to/file.yaml`",
		},
		{
			name:         "Only changed files (multiple)",
			selectors:    nil,
			changedFiles: []string{"path/to/file1.yaml", "path/to/file2.yaml", "another/file.yaml"},
			expected:     "Found no changed Applications that watched these files: `path/to/file1.yaml`, `path/to/file2.yaml`, `another/file.yaml`",
		},
		{
			name: "Both selectors and changed files",
			selectors: []app_selector.Selector{
				makeSelector("app", "myapp", app_selector.Eq),
			},
			changedFiles: []string{"path/to/file.yaml"},
			expected:     "Found no changed Applications that matched `app=myapp` and watched these files: `path/to/file.yaml`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNoAppsFoundMessage(tt.selectors, tt.changedFiles)
			if got != tt.expected {
				t.Errorf("getNoAppsFoundMessage() =\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}

func TestGetNoAppsFoundMessage_TruncatesLongLists(t *testing.T) {
	makeSelector := func(key, value string, op app_selector.Operator) app_selector.Selector {
		return app_selector.Selector{Key: key, Value: value, Operator: op}
	}

	t.Run("Truncates selectors at maxSelectorsToShow", func(t *testing.T) {
		selectors := make([]app_selector.Selector, maxSelectorsToShow+5)
		for i := range selectors {
			selectors[i] = makeSelector("app", "val"+string(rune('a'+i%26)), app_selector.Eq)
		}

		got := getNoAppsFoundMessage(selectors, nil)

		if !strings.Contains(got, "[5 more omitted]") {
			t.Errorf("Should show '[5 more omitted]' for selectors, got: %s", got)
		}
	})

	t.Run("Truncates files at maxChangedFilesToShow", func(t *testing.T) {
		files := make([]string, maxChangedFilesToShow+5)
		for i := range files {
			files[i] = "file" + string(rune('a'+i%26)) + ".yaml"
		}

		got := getNoAppsFoundMessage(nil, files)

		if !strings.Contains(got, "[5 more omitted]") {
			t.Errorf("Should show '[5 more omitted]' for files, got: %s", got)
		}
	})

	t.Run("No truncation when within limits", func(t *testing.T) {
		files := make([]string, maxChangedFilesToShow)
		for i := range files {
			files[i] = "file" + string(rune('a'+i%26)) + ".yaml"
		}

		got := getNoAppsFoundMessage(nil, files)

		if strings.Contains(got, "more omitted") {
			t.Errorf("Should not truncate at exactly maxChangedFilesToShow, got: %s", got)
		}
	})
}

func TestFormatSelectors(t *testing.T) {
	makeSelector := func(key, value string, op app_selector.Operator) app_selector.Selector {
		return app_selector.Selector{Key: key, Value: value, Operator: op}
	}

	t.Run("Empty selectors", func(t *testing.T) {
		got := formatSelectors(nil)
		if got != "" {
			t.Errorf("Expected empty string, got: %s", got)
		}
	})

	t.Run("Single selector", func(t *testing.T) {
		selectors := []app_selector.Selector{makeSelector("app", "test", app_selector.Eq)}
		got := formatSelectors(selectors)
		if got != "app=test" {
			t.Errorf("Expected 'app=test', got: %s", got)
		}
	})

	t.Run("Truncates at maxSelectorsToShow", func(t *testing.T) {
		selectors := make([]app_selector.Selector, maxSelectorsToShow+5)
		for i := range selectors {
			selectors[i] = makeSelector("k"+string(rune('a'+i%26)), "v", app_selector.Eq)
		}
		got := formatSelectors(selectors)
		if !strings.Contains(got, "[5 more omitted]") {
			t.Errorf("Expected '[5 more omitted]', got: %s", got)
		}
	})
}

func TestFormatChangedFiles(t *testing.T) {
	t.Run("Empty files", func(t *testing.T) {
		got := formatChangedFiles(nil)
		if got != "" {
			t.Errorf("Expected empty string, got: %s", got)
		}
	})

	t.Run("Single file", func(t *testing.T) {
		got := formatChangedFiles([]string{"test.yaml"})
		if got != "test.yaml" {
			t.Errorf("Expected 'test.yaml', got: %s", got)
		}
	})

	t.Run("Truncates at maxChangedFilesToShow", func(t *testing.T) {
		files := make([]string, maxChangedFilesToShow+5)
		for i := range files {
			files[i] = "file" + string(rune('a'+i%26)) + ".yaml"
		}
		got := formatChangedFiles(files)
		if !strings.Contains(got, "[5 more omitted]") {
			t.Errorf("Expected '[5 more omitted]', got: %s", got)
		}
	})
}

func TestGenerateNoAppsFoundMarkdown(t *testing.T) {
	tests := []struct {
		name            string
		title           string
		message         string
		expectedContain []string
	}{
		{
			name:    "Basic title and message",
			title:   "ArgoCD Diff Preview",
			message: "Found no Applications",
			expectedContain: []string{
				"## ArgoCD Diff Preview",
				"Found no Applications",
			},
		},
		{
			name:    "Title with special characters",
			title:   "Diff for PR #123",
			message: "Found no changed Applications that matched `app=myapp`",
			expectedContain: []string{
				"## Diff for PR #123",
				"Found no changed Applications that matched `app=myapp`",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateNoAppsFoundMarkdown(tt.title, tt.message)

			for _, contain := range tt.expectedContain {
				if !strings.Contains(got, contain) {
					t.Errorf("generateNoAppsFoundMarkdown() should contain %q, got:\n%s", contain, got)
				}
			}

			if got != strings.TrimSpace(got) {
				t.Errorf("generateNoAppsFoundMarkdown() should be trimmed, got:\n%q", got)
			}
		})
	}
}

func TestWriteNoAppsFoundMessage(t *testing.T) {
	makeSelector := func(key, value string, op app_selector.Operator) app_selector.Selector {
		return app_selector.Selector{Key: key, Value: value, Operator: op}
	}

	tests := []struct {
		name         string
		title        string
		selectors    []app_selector.Selector
		changedFiles []string
		wantContains []string
	}{
		{
			name:         "No apps found - no filters",
			title:        "ArgoCD Diff",
			selectors:    nil,
			changedFiles: nil,
			wantContains: []string{
				"## ArgoCD Diff",
				"Found no Applications",
			},
		},
		{
			name:  "No apps found - with selector",
			title: "PR Preview",
			selectors: []app_selector.Selector{
				makeSelector("app", "frontend", app_selector.Eq),
			},
			changedFiles: nil,
			wantContains: []string{
				"## PR Preview",
				"Found no changed Applications that matched `app=frontend`",
			},
		},
		{
			name:         "No apps found - with changed files",
			title:        "Diff Report",
			selectors:    nil,
			changedFiles: []string{"apps/myapp/values.yaml"},
			wantContains: []string{
				"## Diff Report",
				"Found no changed Applications that watched these files: `apps/myapp/values.yaml`",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "no-apps-found-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer func() { _ = os.RemoveAll(tempDir) }()

			err = WriteNoAppsFoundMessage(tt.title, tempDir, tt.selectors, tt.changedFiles)
			if err != nil {
				t.Fatalf("WriteNoAppsFoundMessage() error = %v", err)
			}

			markdownPath := filepath.Join(tempDir, "diff.md")
			markdownContent, err := os.ReadFile(markdownPath)
			if err != nil {
				t.Fatalf("Failed to read markdown file: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(string(markdownContent), want) {
					t.Errorf("Markdown file should contain %q, got:\n%s", want, string(markdownContent))
				}
			}

			// Verify HTML file has same content
			htmlPath := filepath.Join(tempDir, "diff.html")
			htmlContent, err := os.ReadFile(htmlPath)
			if err != nil {
				t.Fatalf("Failed to read HTML file: %v", err)
			}

			if string(markdownContent) != string(htmlContent) {
				t.Error("HTML and markdown files should have same content")
			}
		})
	}
}

func TestWriteNoAppsFoundMessage_InvalidOutputFolder(t *testing.T) {
	err := WriteNoAppsFoundMessage("Test", "/nonexistent/path/that/does/not/exist", nil, nil)
	if err == nil {
		t.Error("WriteNoAppsFoundMessage() should return error for invalid output folder")
	}
}
