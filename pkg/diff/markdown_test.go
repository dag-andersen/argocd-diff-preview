package diff

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMarkdownSectionHeader(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "Simple title",
			title:    "Test App",
			expected: "<details>\n<summary>Test App</summary>\n<br>\n\n```diff\n",
		},
		{
			name:     "Title with special characters",
			title:    "app-v2 (path/to/app)",
			expected: "<details>\n<summary>app-v2 (path/to/app)</summary>\n<br>\n\n```diff\n",
		},
		{
			name:     "Empty title",
			title:    "",
			expected: "<details>\n<summary></summary>\n<br>\n\n```diff\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdownSectionHeader(tt.title)
			if got != tt.expected {
				t.Errorf("markdownSectionHeader() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMarkdownSection_Build(t *testing.T) {
	tests := []struct {
		name              string
		section           MarkdownSection
		maxSize           int
		expectedContent   string
		expectedTruncated bool
	}{
		{
			name: "Full content fits",
			section: MarkdownSection{
				title:   "Test App",
				comment: "@@ Application added: Test App @@\n",
				content: "+ line 1\n+ line 2",
			},
			maxSize:           1000,
			expectedContent:   "<details>\n<summary>Test App</summary>\n<br>\n\n```diff\n@@ Application added: Test App @@\n+ line 1\n+ line 2\n```\n\n</details>\n\n",
			expectedTruncated: false,
		},
		{
			name: "Content needs truncation",
			section: MarkdownSection{
				title:   "App",
				comment: "@@ Test @@\n",
				content: strings.Repeat("Very long line that will be truncated\n", 10),
			},
			maxSize:           200, // Small max size to force truncation
			expectedTruncated: true,
		},
		{
			name: "Content too large, returns empty",
			section: MarkdownSection{
				title:   "Very Long Title That Takes Up Most Space",
				comment: "@@ Very long comment that takes up space @@\n",
				content: "Some content",
			},
			maxSize:           50, // Very small max size
			expectedContent:   "",
			expectedTruncated: true,
		},
		{
			name: "Content with trailing newlines",
			section: MarkdownSection{
				title:   "App",
				comment: "@@ Test @@\n",
				content: "+ line 1\n+ line 2\n\n\n",
			},
			maxSize:           1000,
			expectedContent:   "<details>\n<summary>App</summary>\n<br>\n\n```diff\n@@ Test @@\n+ line 1\n+ line 2\n```\n\n</details>\n\n",
			expectedTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotContent, gotTruncated := tt.section.build(tt.maxSize)

			if gotTruncated != tt.expectedTruncated {
				t.Errorf("MarkdownSection.build() truncated = %v, want %v", gotTruncated, tt.expectedTruncated)
			}

			if tt.expectedContent != "" && gotContent != tt.expectedContent {
				t.Errorf("MarkdownSection.build() content = %q, want %q", gotContent, tt.expectedContent)
			}

			// If truncated and not empty, should contain truncation warning
			if gotTruncated && gotContent != "" && !strings.Contains(gotContent, "🚨 Diff is too long") {
				t.Errorf("Truncated content should contain warning message, got: %q", gotContent)
			}
		})
	}
}

func TestMarkdownOutput_PrintDiff(t *testing.T) {
	tests := []struct {
		name                    string
		output                  MarkdownOutput
		maxSize                 int
		maxDiffMessageCharCount uint
		expectedContains        []string
		expectedNotContains     []string
	}{
		{
			name: "Basic output with sections",
			output: MarkdownOutput{
				title:   "Test Diff",
				summary: "Added: 1\nModified: 1",
				sections: []MarkdownSection{
					{
						title:   "App 1",
						comment: "@@ Application added: App 1 @@\n",
						content: "+ new content",
					},
					{
						title:   "App 2",
						comment: "@@ Application modified: App 2 @@\n",
						content: "- old content\n+ new content",
					},
				},
				infoBox: InfoBox{
					ApplicationCount: 2,
					FullDuration:     time.Second * 5,
				},
			},
			maxSize:                 10000,
			maxDiffMessageCharCount: 5000,
			expectedContains: []string{
				"## Test Diff",
				"Added: 1\nModified: 1",
				"<summary>App 1</summary>",
				"<summary>App 2</summary>",
				"@@ Application added: App 1 @@",
				"@@ Application modified: App 2 @@",
				"+ new content",
				"- old content",
				"Applications: 2",
			},
			expectedNotContains: []string{
				"⚠️⚠️⚠️ Diff exceeds max length",
				"No changes found",
			},
		},
		{
			name: "Empty sections shows no changes",
			output: MarkdownOutput{
				title:    "Empty Diff",
				summary:  "No changes",
				sections: []MarkdownSection{},
				infoBox: InfoBox{
					ApplicationCount: 0,
				},
			},
			maxSize:                 10000,
			maxDiffMessageCharCount: 5000,
			expectedContains: []string{
				"## Empty Diff",
				"No changes",
				"No changes found",
				"Applications: 0",
			},
			expectedNotContains: []string{
				"⚠️⚠️⚠️ Diff exceeds max length",
			},
		},
		{
			name: "Truncated output shows warning",
			output: MarkdownOutput{
				title:   "Large Diff",
				summary: "Large changes",
				sections: []MarkdownSection{
					{
						title:   "Large App",
						comment: "@@ Application modified: Large App @@\n",
						content: strings.Repeat("Very long diff content that will cause truncation\n", 100),
					},
				},
				infoBox: InfoBox{
					ApplicationCount: 1,
				},
			},
			maxSize:                 500, // Small size to force truncation
			maxDiffMessageCharCount: 500,
			expectedContains: []string{
				"## Large Diff",
				"⚠️⚠️⚠️ Diff exceeds max length of 500 characters",
			},
			expectedNotContains: []string{
				"No changes found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.output.printDiff(tt.maxSize, tt.maxDiffMessageCharCount)

			for _, expected := range tt.expectedContains {
				if !strings.Contains(got, expected) {
					t.Errorf("printDiff() should contain %q, but got:\n%s", expected, got)
				}
			}

			for _, notExpected := range tt.expectedNotContains {
				if strings.Contains(got, notExpected) {
					t.Errorf("printDiff() should not contain %q, but got:\n%s", notExpected, got)
				}
			}

			// Verify it's valid markdown structure
			if !strings.HasPrefix(strings.TrimSpace(got), "##") {
				t.Errorf("printDiff() should start with markdown header, got:\n%s", got)
			}

			if !strings.HasSuffix(got, "\n") {
				t.Errorf("printDiff() should end with newline")
			}
		})
	}
}

func TestMarkdownOutput_PrintDiff_EdgeCases(t *testing.T) {
	t.Run("Very small max size", func(t *testing.T) {
		output := MarkdownOutput{
			title:   "Test",
			summary: "Test summary",
			sections: []MarkdownSection{
				{
					title:   "App",
					comment: "@@ Test @@\n",
					content: "content",
				},
			},
			infoBox: InfoBox{ApplicationCount: 1},
		}

		// Max size smaller than template
		got := output.printDiff(10, 100)

		// Should still produce valid output
		if !strings.Contains(got, "## Test") {
			t.Errorf("Even with small max size, should contain title")
		}
	})

	t.Run("Zero max size", func(t *testing.T) {
		output := MarkdownOutput{
			title:   "Test",
			summary: "Test summary",
			sections: []MarkdownSection{
				{
					title:   "App",
					comment: "@@ Test @@\n",
					content: "content",
				},
			},
			infoBox: InfoBox{ApplicationCount: 1},
		}

		got := output.printDiff(0, 100)

		// Should handle gracefully
		if got == "" {
			t.Errorf("Should not return empty string even with zero max size")
		}
	})
}

func TestMarkdownSection_Build_TruncationBehavior(t *testing.T) {
	section := MarkdownSection{
		title:   "Test App",
		comment: "@@ Application modified: Test App @@\n",
		content: strings.Repeat("Line of content that will be truncated\n", 50),
	}

	// Test various max sizes
	sizes := []int{100, 500, 1000, 2000}

	for _, maxSize := range sizes {
		t.Run(fmt.Sprintf("MaxSize_%d", maxSize), func(t *testing.T) {
			content, truncated := section.build(maxSize)

			// If content is returned, it should not exceed max size
			if content != "" && len(content) > maxSize {
				t.Errorf("Content length %d exceeds maxSize %d", len(content), maxSize)
			}

			// If truncated, should either be empty or contain warning
			if truncated {
				if content != "" && !strings.Contains(content, "🚨 Diff is too long") {
					t.Errorf("Truncated non-empty content should contain warning")
				}
			}

			// If not truncated, should contain full content
			if !truncated {
				if !strings.Contains(content, section.comment) {
					t.Errorf("Non-truncated content should contain comment")
				}
				if !strings.Contains(content, strings.TrimRight(section.content, "\n")) {
					t.Errorf("Non-truncated content should contain original content")
				}
			}
		})
	}
}

func TestMarkdownOutput_TemplateReplacement(t *testing.T) {
	output := MarkdownOutput{
		title:   "Custom Title",
		summary: "Custom Summary\nWith Multiple Lines",
		sections: []MarkdownSection{
			{
				title:   "Test Section",
				comment: "@@ Test @@\n",
				content: "Test content",
			},
		},
		infoBox: InfoBox{
			ApplicationCount:           3,
			FullDuration:               time.Minute * 2,
			ExtractDuration:            time.Second * 30,
			ClusterCreationDuration:    time.Second * 45,
			ArgoCDInstallationDuration: time.Second * 15,
		},
	}

	got := output.printDiff(10000, 5000)

	// Verify all template placeholders are replaced
	templatePlaceholders := []string{"%title%", "%summary%", "%app_diffs%", "%info_box%"}
	for _, placeholder := range templatePlaceholders {
		if strings.Contains(got, placeholder) {
			t.Errorf("Template placeholder %s was not replaced in output:\n%s", placeholder, got)
		}
	}

	// Verify actual content is present
	expectedContent := []string{
		"## Custom Title",
		"Custom Summary\nWith Multiple Lines",
		"Test Section",
		"Applications: 3",
		"Full Run: 2m",
		"Rendering: 30s",
		"Cluster: 45s",
		"Argo CD: 15s",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(got, expected) {
			t.Errorf("Expected content %q not found in output:\n%s", expected, got)
		}
	}
}
