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
		appName  string
		filePath string
		appURL   string
		expected string
	}{
		{
			name:     "Simple app and file without URL",
			appName:  "Test App",
			filePath: "path/to/app",
			appURL:   "",
			expected: "<details>\n<summary>Test App (path/to/app)</summary>\n<br>\n\n",
		},
		{
			name:     "App with ArgoCD URL",
			appName:  "app-v2",
			filePath: "path/to/app",
			appURL:   "https://argocd.example.com/applications/app-v2",
			expected: "<details>\n<summary>app-v2 [<a href=\"https://argocd.example.com/applications/app-v2\">link</a>] (path/to/app)</summary>\n<br>\n\n",
		},
		{
			name:     "Empty app name without URL",
			appName:  "",
			filePath: "path/to/app",
			appURL:   "",
			expected: "<details>\n<summary> (path/to/app)</summary>\n<br>\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markdownSectionHeader(tt.appName, tt.filePath, tt.appURL)
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
				appName:  "Test App",
				filePath: "path/to/app.yaml",
				appURL:   "",
				comment:  "@@ Application added: Test App @@\n",
				blocks:   []ResourceBlock{{Content: "+ line 1\n+ line 2"}},
			},
			maxSize:           1000,
			expectedContent:   "<details>\n<summary>Test App (path/to/app.yaml)</summary>\n<br>\n\n@@ Application added: Test App @@\n```diff\n+ line 1\n+ line 2\n```\n</details>\n\n",
			expectedTruncated: false,
		},
		{
			name: "Content needs truncation",
			section: MarkdownSection{
				appName:  "App",
				filePath: "path.yaml",
				appURL:   "",
				comment:  "@@ Test @@\n",
				blocks:   []ResourceBlock{{Content: strings.Repeat("Very long line that will be truncated\n", 10)}},
			},
			maxSize:           200, // Small max size to force truncation
			expectedTruncated: true,
		},
		{
			name: "Content too large, returns empty",
			section: MarkdownSection{
				appName:  "Very Long Title That Takes Up Most Space",
				filePath: "path/to/app.yaml",
				appURL:   "",
				comment:  "@@ Very long comment that takes up space @@\n",
				blocks:   []ResourceBlock{{Content: "Some content"}},
			},
			maxSize:           50, // Very small max size
			expectedContent:   "",
			expectedTruncated: true,
		},
		{
			name: "Content with trailing newlines",
			section: MarkdownSection{
				appName:  "App",
				filePath: "path.yaml",
				appURL:   "",
				comment:  "@@ Test @@\n",
				blocks:   []ResourceBlock{{Content: "+ line 1\n+ line 2\n\n\n"}},
			},
			maxSize:           1000,
			expectedContent:   "<details>\n<summary>App (path.yaml)</summary>\n<br>\n\n@@ Test @@\n```diff\n+ line 1\n+ line 2\n\n\n```\n</details>\n\n",
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
			if gotTruncated && gotContent != "" && !strings.Contains(gotContent, diffTooLongWarning) {
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
						appName:  "App 1",
						filePath: "path/to/app1.yaml",
						appURL:   "",
						comment:  "@@ Application added: App 1 @@\n",
						blocks:   []ResourceBlock{{Content: "+ new content"}},
					},
					{
						appName:  "App 2",
						filePath: "path/to/app2.yaml",
						appURL:   "",
						comment:  "@@ Application modified: App 2 @@\n",
						blocks:   []ResourceBlock{{Content: "- old content\n+ new content"}},
					},
				},
				statsInfo: StatsInfo{
					ApplicationCount: 2,
					FullDuration:     time.Second * 5,
				},
			},
			maxSize:                 10000,
			maxDiffMessageCharCount: 5000,
			expectedContains: []string{
				"## Test Diff",
				"Added: 1\nModified: 1",
				"<summary>App 1 (path/to/app1.yaml)</summary>",
				"<summary>App 2 (path/to/app2.yaml)</summary>",
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
				statsInfo: StatsInfo{
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
						appName:  "Large App",
						filePath: "path/to/large.yaml",
						appURL:   "",
						comment:  "@@ Application modified: Large App @@\n",
						blocks:   []ResourceBlock{{Content: strings.Repeat("Very long diff content that will cause truncation\n", 100)}},
					},
				},
				statsInfo: StatsInfo{
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
			got := tt.output.printDiff(tt.maxDiffMessageCharCount)

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
					appName:  "App",
					filePath: "path.yaml",
					appURL:   "",
					comment:  "@@ Test @@\n",
					blocks:   []ResourceBlock{{Content: "content"}},
				},
			},
			statsInfo: StatsInfo{ApplicationCount: 1},
		}

		// Max size smaller than template
		got := output.printDiff(30)

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
					appName:  "App",
					filePath: "path.yaml",
					appURL:   "",
					comment:  "@@ Test @@\n",
					blocks:   []ResourceBlock{{Content: "content"}},
				},
			},
			statsInfo: StatsInfo{ApplicationCount: 1},
		}

		got := output.printDiff(0)

		// Should handle gracefully
		if got == "" {
			t.Errorf("Should not return empty string even with zero max size")
		}
	})
}

func TestMarkdownSection_Build_EdgeCases(t *testing.T) {
	header := markdownSectionHeader("App", "path.yaml", "")
	footer := markdownSectionFooter()
	headerFooterLen := len(header) + len(footer)

	t.Run("Empty content", func(t *testing.T) {
		section := MarkdownSection{
			appName:  "App",
			filePath: "path.yaml",
			appURL:   "",
			comment:  "",
			blocks:   []ResourceBlock{{Content: strings.Repeat("x", 500)}},
		}
		content, truncated := section.build(1000)
		if truncated {
			t.Errorf("Empty content should not be truncated")
		}
		if !strings.Contains(content, "<summary>App (path.yaml)</summary>") {
			t.Errorf("Should contain the section summary")
		}
	})

	t.Run("Content is only trailing newlines", func(t *testing.T) {
		section := MarkdownSection{
			appName:  "App",
			filePath: "path.yaml",
			appURL:   "",
			comment:  "",
			blocks:   []ResourceBlock{{Content: "actual content\n\n\n"}},
		}
		content, truncated := section.build(1000)
		if truncated {
			t.Errorf("Content with trailing newlines should not be truncated")
		}
		// Content is preserved inside code fences
		if !strings.Contains(content, "actual content") {
			t.Errorf("Should preserve the actual content")
		}
		// Should have code fences
		if !strings.Contains(content, "```diff") {
			t.Errorf("Should have code fence markers")
		}
	})

	t.Run("spaceForContent is exactly 0", func(t *testing.T) {
		section := MarkdownSection{
			appName:  "App",
			filePath: "path.yaml",
			appURL:   "",
			comment:  "",
			blocks:   []ResourceBlock{{Content: "x"}},
		}
		// maxSize = headerFooterLen + 0 = exactly enough for header/footer, no content
		content, truncated := section.build(headerFooterLen)
		if !truncated {
			t.Errorf("Should be truncated when spaceForContent is 0")
		}
		if content != "" {
			t.Errorf("Should return empty string when spaceForContent is 0")
		}
	})

	t.Run("Content length equals spaceForContent exactly", func(t *testing.T) {
		// The condition is len(content) < spaceForContent, so equal should trigger truncation path
		section := MarkdownSection{
			appName:  "App",
			filePath: "path.yaml",
			appURL:   "",
			comment:  "",
			blocks:   []ResourceBlock{{Content: strings.Repeat("x", 200)}},
		}
		maxSize := headerFooterLen + 200
		content, truncated := section.build(maxSize)
		// With exactly equal, it should go to truncation path but have enough space
		if !truncated {
			t.Errorf("Should be truncated when content equals space exactly")
		}
		if !strings.Contains(content, diffTooLongWarning) {
			t.Errorf("Should contain truncation warning")
		}
	})

	t.Run("Just enough space for minSizeForSectionContent threshold", func(t *testing.T) {
		// We need spaceBeforeDiffTooLongWarning > minSizeForSectionContent
		// spaceBeforeDiffTooLongWarning = spaceForContent - len(diffTooLongWarning)
		// So spaceForContent needs to be > minSizeForSectionContent + len(diffTooLongWarning)
		section := MarkdownSection{
			appName:  "App",
			filePath: "path.yaml",
			appURL:   "",
			comment:  "",
			blocks:   []ResourceBlock{{Content: strings.Repeat("x", 500)}},
		}
		// Set maxSize so spaceForContent is exactly minSizeForSectionContent + 1 + warning length (just above threshold)
		spaceForContent := minSizeForSectionContent + 1 + len(diffTooLongWarning)
		maxSize := headerFooterLen + spaceForContent
		content, truncated := section.build(maxSize)
		if !truncated {
			t.Errorf("Should be truncated")
		}
		if content == "" {
			t.Errorf("Should return content when above minSizeForSectionContent threshold")
		}
		if !strings.Contains(content, diffTooLongWarning) {
			t.Errorf("Should contain truncation warning")
		}
	})

	t.Run("Just below minSizeForSectionContent threshold", func(t *testing.T) {
		section := MarkdownSection{
			appName:  "App",
			filePath: "path.yaml",
			appURL:   "",
			comment:  "",
			blocks:   []ResourceBlock{{Content: strings.Repeat("x", 500)}},
		}
		// Set maxSize so spaceForContent is exactly minSizeForSectionContent + warning length (at threshold, not above)
		spaceForContent := minSizeForSectionContent + len(diffTooLongWarning)
		maxSize := headerFooterLen + spaceForContent
		content, truncated := section.build(maxSize)
		if !truncated {
			t.Errorf("Should be truncated")
		}
		if content != "" {
			t.Errorf("Should return empty when at/below minSizeForSectionContent threshold, got: %q", content)
		}
	})

	t.Run("Truncation preserves valid content without trailing whitespace", func(t *testing.T) {
		section := MarkdownSection{
			appName:  "App",
			filePath: "path.yaml",
			appURL:   "",
			comment:  "",
			blocks:   []ResourceBlock{{Content: "line1\nline2   \t\nline3"}},
		}
		// Force truncation that cuts off at whitespace area
		maxSize := headerFooterLen + 150
		content, truncated := section.build(maxSize)
		if truncated && content != "" {
			// Verify no trailing whitespace before the warning
			if strings.Contains(content, "   \t\n🚨") || strings.Contains(content, " \n🚨") {
				t.Errorf("Truncated content should not have trailing whitespace before warning")
			}
		}
	})
}

func TestMarkdownSection_Build_TruncationBehavior(t *testing.T) {
	sectionContent := strings.Repeat("Line of content that will be truncated\n", 50)
	section := MarkdownSection{
		appName:  "Test App",
		filePath: "path/to/app.yaml",
		appURL:   "",
		comment:  "@@ Application modified: Test App @@\n",
		blocks:   []ResourceBlock{{Content: sectionContent}},
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
				if content != "" && !strings.Contains(content, diffTooLongWarning) {
					t.Errorf("Truncated non-empty content should contain warning")
				}
			}

			// If not truncated, should contain full content
			if !truncated {
				if !strings.Contains(content, section.comment) {
					t.Errorf("Non-truncated content should contain comment")
				}
				if !strings.Contains(content, strings.TrimRight(sectionContent, "\n")) {
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
				appName:  "Test Section",
				filePath: "path/to/section.yaml",
				appURL:   "",
				comment:  "@@ Test @@\n",
				blocks:   []ResourceBlock{{Content: "Test content"}},
			},
		},
		statsInfo: StatsInfo{
			ApplicationCount:           3,
			FullDuration:               time.Minute * 2,
			ExtractDuration:            time.Second * 30,
			ClusterCreationDuration:    time.Second * 45,
			ArgoCDInstallationDuration: time.Second * 15,
		},
	}

	got := output.printDiff(5000)

	// Verify all template placeholders are replaced
	templatePlaceholders := []string{"%title%", "%summary%", "%app_diffs%", "%selection_changes%", "%info_box%"}
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

func TestMarkdownOutput_SelectionChanges(t *testing.T) {
	t.Run("No selection changes when counts are equal", func(t *testing.T) {
		output := MarkdownOutput{
			title:   "Test Diff",
			summary: "Summary",
			sections: []MarkdownSection{
				{
					appName:  "App",
					filePath: "path.yaml",
					appURL:   "",
					comment:  "@@ Test @@\n",
					blocks:   []ResourceBlock{{Content: "content"}},
				},
			},
			selectionInfo: SelectionInfo{
				Base:   AppSelectionInfo{SkippedApplications: 2, SkippedApplicationSets: 1},
				Target: AppSelectionInfo{SkippedApplications: 2, SkippedApplicationSets: 1},
			},
			statsInfo: StatsInfo{ApplicationCount: 1},
		}

		got := output.printDiff(5000)

		// Should NOT contain skipped resources info when counts are equal
		if strings.Contains(got, "_Skipped resources_") {
			t.Errorf("Should not contain skipped resources info when counts are equal, got:\n%s", got)
		}
	})

	t.Run("Shows selection changes when Application counts differ", func(t *testing.T) {
		output := MarkdownOutput{
			title:   "Test Diff",
			summary: "Summary",
			sections: []MarkdownSection{
				{
					appName:  "App",
					filePath: "path.yaml",
					appURL:   "",
					comment:  "@@ Test @@\n",
					blocks:   []ResourceBlock{{Content: "content"}},
				},
			},
			selectionInfo: SelectionInfo{
				Base:   AppSelectionInfo{SkippedApplications: 2, SkippedApplicationSets: 1},
				Target: AppSelectionInfo{SkippedApplications: 5, SkippedApplicationSets: 1},
			},
			statsInfo: StatsInfo{ApplicationCount: 1},
		}

		got := output.printDiff(5000)

		expectedContent := []string{
			"_Skipped resources_",
			"Applications: `2` (base) -> `5` (target)",
			"ApplicationSets: `1` (base) -> `1` (target)",
		}

		for _, expected := range expectedContent {
			if !strings.Contains(got, expected) {
				t.Errorf("Expected content %q not found in output:\n%s", expected, got)
			}
		}
	})

	t.Run("Shows selection changes when ApplicationSet counts differ", func(t *testing.T) {
		output := MarkdownOutput{
			title:   "Test Diff",
			summary: "Summary",
			sections: []MarkdownSection{
				{
					appName:  "App",
					filePath: "path.yaml",
					appURL:   "",
					comment:  "@@ Test @@\n",
					blocks:   []ResourceBlock{{Content: "content"}},
				},
			},
			selectionInfo: SelectionInfo{
				Base:   AppSelectionInfo{SkippedApplications: 3, SkippedApplicationSets: 0},
				Target: AppSelectionInfo{SkippedApplications: 3, SkippedApplicationSets: 2},
			},
			statsInfo: StatsInfo{ApplicationCount: 1},
		}

		got := output.printDiff(5000)

		expectedContent := []string{
			"_Skipped resources_",
			"Applications: `3` (base) -> `3` (target)",
			"ApplicationSets: `0` (base) -> `2` (target)",
		}

		for _, expected := range expectedContent {
			if !strings.Contains(got, expected) {
				t.Errorf("Expected content %q not found in output:\n%s", expected, got)
			}
		}
	})
}
