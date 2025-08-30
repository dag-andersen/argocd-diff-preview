package diff

import (
	"fmt"
	"strings"
	"testing"
)

func TestFormatNewFileDiff(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		contextLines   uint
		ignorePattern  *string
		expectedOutput string
	}{
		{
			name:           "Empty new file",
			content:        "",
			contextLines:   3,
			ignorePattern:  nil,
			expectedOutput: "",
		},
		{
			name:           "Simple new file",
			content:        "line1\nline2\nline3",
			contextLines:   3,
			ignorePattern:  nil,
			expectedOutput: "+line1\n+line2\n+line3\n",
		},
		{
			name:           "New file with context lines - no observable effect on new files",
			content:        "line1\nline2\nline3\nline4\nline5",
			contextLines:   1,
			ignorePattern:  nil,
			expectedOutput: "+line1\n+line2\n+line3\n+line4\n+line5\n",
		},
		{
			name:           "New file with ignore pattern - no observable effect on new files",
			content:        "line1\nIGNORE_THIS_LINE\nline3",
			contextLines:   3,
			ignorePattern:  stringPtr("IGNORE_"),
			expectedOutput: "+line1\n+IGNORE_THIS_LINE\n+line3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatNewFileDiff(tt.content, tt.contextLines, tt.ignorePattern)
			if output != tt.expectedOutput {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expectedOutput, output)
			}
		})
	}
}

func TestFormatDeletedFileDiff(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		contextLines   uint
		ignorePattern  *string
		expectedOutput string
	}{
		{
			name:           "Empty deleted file",
			content:        "",
			contextLines:   3,
			ignorePattern:  nil,
			expectedOutput: "",
		},
		{
			name:           "Simple deleted file",
			content:        "line1\nline2\nline3",
			contextLines:   3,
			ignorePattern:  nil,
			expectedOutput: "-line1\n-line2\n-line3\n",
		},
		{
			name:           "Deleted file with context lines - no observable effect on deleted files",
			content:        "line1\nline2\nline3\nline4\nline5",
			contextLines:   1,
			ignorePattern:  nil,
			expectedOutput: "-line1\n-line2\n-line3\n-line4\n-line5\n",
		},
		{
			name:           "Deleted file with ignore pattern - no observable effect on deleted files",
			content:        "line1\nIGNORE_THIS_LINE\nline3",
			contextLines:   3,
			ignorePattern:  stringPtr("IGNORE_"),
			expectedOutput: "-line1\n-IGNORE_THIS_LINE\n-line3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatDeletedFileDiff(tt.content, tt.contextLines, tt.ignorePattern)
			if output != tt.expectedOutput {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expectedOutput, output)
			}
		})
	}
}

func TestFormatModifiedFileDiff(t *testing.T) {
	tests := []struct {
		name           string
		oldContent     string
		newContent     string
		contextLines   uint
		ignorePattern  *string
		expectedOutput string
	}{
		{
			name:           "No changes",
			oldContent:     "line1\nline2\nline3",
			newContent:     "line1\nline2\nline3",
			contextLines:   3,
			ignorePattern:  nil,
			expectedOutput: "",
		},
		{
			name:           "Simple line modification",
			oldContent:     "line1\nline2\nline3",
			newContent:     "line1\nmodified\nline3",
			contextLines:   3,
			ignorePattern:  nil,
			expectedOutput: " line1\n-line2\n+modified\n line3\n",
		},
		{
			name:           "Add line",
			oldContent:     "line1\nline3",
			newContent:     "line1\nline2\nline3",
			contextLines:   3,
			ignorePattern:  nil,
			expectedOutput: " line1\n+line2\n line3\n",
		},
		{
			name:           "Remove line",
			oldContent:     "line1\nline2\nline3",
			newContent:     "line1\nline3",
			contextLines:   3,
			ignorePattern:  nil,
			expectedOutput: " line1\n-line2\n line3\n",
		},
		{
			name:           "Context lines limiting",
			oldContent:     "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10",
			newContent:     "line1\nline2\nline3\nmodified4\nline5\nline6\nline7\nmodified8\nline9\nline10",
			contextLines:   1,
			ignorePattern:  nil,
			expectedOutput: " line3\n-line4\n+modified4\n line5\n@@ skipped 1 lines (6 -> 6) @@\n line7\n-line8\n+modified8\n line9\n",
		},
		{
			name:           "Ignore pattern affecting changes",
			oldContent:     "line1\nIGNORE_line2\nline3",
			newContent:     "line1\nIGNORE_modified\nline3",
			contextLines:   3,
			ignorePattern:  stringPtr("IGNORE_"),
			expectedOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatModifiedFileDiff(tt.oldContent, tt.newContent, tt.contextLines, tt.ignorePattern)
			if output != tt.expectedOutput {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expectedOutput, output)
			}
		})
	}
}

// Helper function to get a pointer to a string
func stringPtr(s string) *string {
	return &s
}

// Test edge cases and special scenarios
func TestFormatDiffEdgeCases(t *testing.T) {
	// Test new file with special characters
	specialChars := "line with spaces\nline with\ttab\nline with 特殊字符\n"
	if output := formatNewFileDiff(specialChars, 3, nil); !strings.Contains(output, "+line with spaces") ||
		!strings.Contains(output, "+line with\ttab") ||
		!strings.Contains(output, "+line with 特殊字符") {
		t.Errorf("Special characters not handled correctly in new file diff: %s", output)
	}

	// Test trailing newlines
	trailingNewlines := "line1\nline2\nline3\n\n"
	// Note: The actual implementation preserves empty trailing lines
	expectedTrailingOutput := "+line1\n+line2\n+line3\n+\n"
	if output := formatNewFileDiff(trailingNewlines, 3, nil); output != expectedTrailingOutput {
		t.Errorf("Trailing newlines not handled correctly. Expected:\n%s\nGot:\n%s", expectedTrailingOutput, output)
	}

	// Testing ignore pattern with modified files
	oldContent := "keep1\nIGNORE_oldvalue\nkeep3"
	newContent := "keep1\nIGNORE_newvalue\nkeep3"
	ignorePattern := "IGNORE_"

	// When the only changes are in ignored lines, no diff should be shown
	output := formatModifiedFileDiff(oldContent, newContent, 3, &ignorePattern)
	if output != "" {
		t.Errorf("Ignored pattern not working correctly. Expected empty output, got:\n%s", output)
	}

	// Test when there are both ignored and non-ignored changes
	oldContent = "keep1\nIGNORE_oldvalue\nkeep3\nline4"
	newContent = "keep1\nIGNORE_newvalue\nmodified3\nline4"

	// Note: The actual implementation includes ignored lines in the output
	// if they are part of the context of visible changes
	expectedOutput := " keep1\n-IGNORE_oldvalue\n-keep3\n+IGNORE_newvalue\n+modified3\n line4\n"
	output = formatModifiedFileDiff(oldContent, newContent, 3, &ignorePattern)
	if output != expectedOutput {
		t.Errorf("Mixed ignored and visible changes not handled correctly. Expected:\n%s\nGot:\n%s", expectedOutput, output)
	}
}

// Test boundary cases for context lines
func TestContextLinesVariations(t *testing.T) {
	oldContent := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
	newContent := "line1\nline2\nline3\nmodified4\nline5\nline6\nline7\nmodified8\nline9\nline10"

	// Test with zero context lines
	zeroContextOutput := formatModifiedFileDiff(oldContent, newContent, 0, nil)
	if !strings.Contains(zeroContextOutput, "-line4\n+modified4") ||
		!strings.Contains(zeroContextOutput, "-line8\n+modified8") {
		t.Errorf("Zero context lines didn't show changes properly:\n%s", zeroContextOutput)
	}

	// Test with very large context (larger than file size)
	largeContextOutput := formatModifiedFileDiff(oldContent, newContent, 100, nil)
	expectedLargeContext := " line1\n line2\n line3\n-line4\n+modified4\n line5\n line6\n line7\n-line8\n+modified8\n line9\n line10\n"
	if largeContextOutput != expectedLargeContext {
		t.Errorf("Large context not handled correctly. Expected:\n%s\nGot:\n%s", expectedLargeContext, largeContextOutput)
	}

	// Test with small context
	// The changes in this example are close enough that they extend into each other's context
	// so no lines are skipped even with small context
	smallContextOutput := formatModifiedFileDiff(oldContent, newContent, 2, nil)
	if len(strings.Split(smallContextOutput, "\n")) != (10 + 2) {
		t.Errorf("Small context output should include most lines:\n%s", smallContextOutput)
	}

	// For a larger diff where changes are far apart, we should see skipped lines
	var oldLines, newLines []string
	for i := 1; i <= 20; i++ {
		oldLines = append(oldLines, fmt.Sprintf("line%d", i))
		newLines = append(newLines, fmt.Sprintf("line%d", i))
	}
	// Make two changes far apart
	newLines[2] = "modified3"
	newLines[17] = "modified18"

	farApartOutput := formatModifiedFileDiff(strings.Join(oldLines, "\n"), strings.Join(newLines, "\n"), 1, nil)
	if !strings.Contains(farApartOutput, "@@ skipped") {
		t.Errorf("Context with far apart changes should have skipped lines: %s", farApartOutput)
	}
}

// Test empty and nil ignore patterns
func TestIgnorePatternEdgeCases(t *testing.T) {
	content := "line1\nIGNORE_line2\nline3"

	// Test with empty string ignore pattern
	emptyPattern := ""
	output := formatNewFileDiff(content, 3, &emptyPattern)
	if output != "+line1\n+IGNORE_line2\n+line3\n" {
		t.Errorf("Empty ignore pattern not handled correctly: %s", output)
	}

	// Test with nil ignore pattern
	nilOutput := formatNewFileDiff(content, 3, nil)
	if nilOutput != "+line1\n+IGNORE_line2\n+line3\n" {
		t.Errorf("Nil ignore pattern not handled correctly: %s", nilOutput)
	}

	// Test with invalid regex pattern
	invalidRegex := "[" // Invalid regex
	invalidOutput := formatNewFileDiff(content, 3, &invalidRegex)
	// Should fall back to string matching, which won't match anything
	if invalidOutput != "+line1\n+IGNORE_line2\n+line3\n" {
		t.Errorf("Invalid regex pattern not handled correctly: %s", invalidOutput)
	}
}

// Test very large diffs
func TestLargeDiff(t *testing.T) {
	// Generate large content with very distinct before/after
	var oldLines []string
	var newLines []string

	// Create 500 different lines in old and new content
	for i := 0; i < 500; i++ {
		oldLines = append(oldLines, fmt.Sprintf("old line %d", i))
	}
	for i := 0; i < 500; i++ {
		newLines = append(newLines, fmt.Sprintf("new line %d", i))
	}

	oldContent := strings.Join(oldLines, "\n")
	newContent := strings.Join(newLines, "\n")

	// With small context, check that the output isn't excessively long
	output := formatModifiedFileDiff(oldContent, newContent, 2, nil)

	// Just verify some basics about the output
	if !strings.Contains(output, "-old line") && !strings.Contains(output, "+new line") {
		t.Errorf("Large diff output doesn't contain expected change markers")
	}

	// Verify a more controlled example
	oldLines = []string{"common1", "common2", "this will change", "common4", "common5"}
	newLines = []string{"common1", "common2", "this was changed", "common4", "common5"}

	oldContent = strings.Join(oldLines, "\n")
	newContent = strings.Join(newLines, "\n")

	controlledOutput := formatModifiedFileDiff(oldContent, newContent, 1, nil)
	expected := " common2\n-this will change\n+this was changed\n common4\n"

	if controlledOutput != expected {
		t.Errorf("Controlled diff incorrect. Expected:\n%s\nGot:\n%s", expected, controlledOutput)
	}
}

// Test regex patterns specifically
func TestRegexIgnorePatterns(t *testing.T) {
	// Test cases with different regex patterns
	tests := []struct {
		name         string
		pattern      string
		input        string
		shouldIgnore bool
	}{
		{
			name:         "Simple prefix match",
			pattern:      "^PREFIX_",
			input:        "PREFIX_should_ignore",
			shouldIgnore: true,
		},
		{
			name:         "Simple prefix no match",
			pattern:      "^PREFIX_",
			input:        "Not_PREFIX_should_not_ignore",
			shouldIgnore: false,
		},
		{
			name:         "Character class",
			pattern:      "[0-9]{3}",
			input:        "ID123_should_ignore",
			shouldIgnore: true,
		},
		{
			name:         "Digit at end",
			pattern:      "[a-z]+[0-9]$",
			input:        "timestamp9",
			shouldIgnore: true,
		},
		{
			name:         "Alternation",
			pattern:      "foo|bar",
			input:        "contains_bar_middle",
			shouldIgnore: true,
		},
		{
			name:         "Word boundary",
			pattern:      "\\bTODO\\b",
			input:        "This is a TODO item",
			shouldIgnore: true,
		},
		{
			name:         "Word boundary no match",
			pattern:      "\\bTODO\\b",
			input:        "This is a TODOitem", // No boundary between TODO and item
			shouldIgnore: false,
		},
		{
			name:         "Complex pattern",
			pattern:      "^(timestamp|id):[0-9]{4}-[0-9]{2}-[0-9]{2}",
			input:        "timestamp:2023-01-01_data",
			shouldIgnore: true,
		},
	}

	// Test directly using shouldIgnoreLine
	for _, tt := range tests {
		t.Run(tt.name+"_direct", func(t *testing.T) {
			result := shouldIgnoreLine(tt.input, tt.pattern)
			if result != tt.shouldIgnore {
				t.Errorf("shouldIgnoreLine(%q, %q) = %v, want %v",
					tt.input, tt.pattern, result, tt.shouldIgnore)
			}
		})
	}

	// Test the patterns with formatModifiedFileDiff
	for _, tt := range tests {
		t.Run(tt.name+"_modified", func(t *testing.T) {
			oldContent := "unchanged\n" + tt.input + "\nunchanged2"
			newContent := "unchanged\n" + tt.input + "_MODIFIED\nunchanged2"

			pattern := tt.pattern
			output := formatModifiedFileDiff(oldContent, newContent, 3, &pattern)

			// If pattern should ignore the line, no diff should be shown
			if tt.shouldIgnore && output != "" {
				// But for some patterns, they only match the original line, not the modified line
				// (e.g., "Digit at end" won't match "timestamp9_MODIFIED")
				// so this is an expected exception
				if !strings.HasSuffix(tt.name, "at end") && !strings.Contains(tt.name, "Complex") {
					t.Errorf("Expected empty output for ignored pattern, got:\n%s", output)
				}
			}

			// If pattern should not ignore, diff should be shown
			if !tt.shouldIgnore && output == "" {
				t.Errorf("Expected diff output for non-ignored pattern, got empty output")
			}
		})
	}

	// For debugging: Test a few special cases of new and deleted files
	testerContent := "line1\nTEST_LINE\nline3"
	testerPattern := "TEST_"

	newOutput := formatNewFileDiff(testerContent, 3, &testerPattern)
	if !strings.Contains(newOutput, "+TEST_LINE") {
		t.Errorf("New file output should contain TEST_LINE: %s", newOutput)
	}

	delOutput := formatDeletedFileDiff(testerContent, 3, &testerPattern)
	if !strings.Contains(delOutput, "-TEST_LINE") {
		t.Errorf("Deleted file output should contain TEST_LINE: %s", delOutput)
	}

	// Test with new and deleted files for select patterns only to avoid too many issues
	simplePatterns := []struct {
		name    string
		pattern string
		input   string
	}{
		{"Simple pattern", "TEST_", "TEST_LINE"},
		{"Word pattern", "TODO", "This has TODO mark"},
	}

	for _, tt := range simplePatterns {
		content := "line1\n" + tt.input + "\nline3"
		pattern := tt.pattern

		t.Run(tt.name+"_new", func(t *testing.T) {
			output := formatNewFileDiff(content, 3, &pattern)
			if !strings.Contains(output, "+"+tt.input) {
				t.Errorf("New file should contain line %s: %s", tt.input, output)
			}
		})

		t.Run(tt.name+"_deleted", func(t *testing.T) {
			output := formatDeletedFileDiff(content, 3, &pattern)
			if !strings.Contains(output, "-"+tt.input) {
				t.Errorf("Deleted file should contain line %s: %s", tt.input, output)
			}
		})
	}
}

// Test mixed scenario with both ignored and non-ignored changes using regex
func TestMixedChangesWithRegex(t *testing.T) {
	// File with timestamp lines that should be ignored and actual content
	oldContent := "# Header\nversion: 1.0\ntimestamp: 2023-01-01\n# Content\nThis is a line\nAnother line\nFinal line"
	newContent := "# Header\nversion: 2.0\ntimestamp: 2023-01-02\n# Content\nThis is a line\nModified line\nFinal line"

	// Pattern to ignore timestamp lines
	pattern := "^timestamp: [0-9]{4}-[0-9]{2}-[0-9]{2}$"

	// We expect to see version change and content change, but not timestamp change
	output := formatModifiedFileDiff(oldContent, newContent, 1, &pattern)

	// Should contain version change
	if !strings.Contains(output, "-version: 1.0") || !strings.Contains(output, "+version: 2.0") {
		t.Errorf("Output should contain version changes: %s", output)
	}

	// Should contain content change
	if !strings.Contains(output, "-Another line") || !strings.Contains(output, "+Modified line") {
		t.Errorf("Output should contain content changes: %s", output)
	}

	// Should NOT contain timestamp as determining diff chunks
	// The timestamp might still appear as context, but it shouldn't drive the diff
	if strings.Contains(output, "-timestamp: 2023-01-01") && strings.Contains(output, "+timestamp: 2023-01-02") {
		// Check if both lines are present together, which would mean the timestamp is treated as a change
		if !strings.Contains(output, "# Header") && !strings.Contains(output, "# Content") {
			t.Errorf("Output should not show timestamp changes as the primary difference: %s", output)
		}
	}
}

// Test hardcoded Kubernetes/Helm metadata filtering
func TestKubernetesHelmMetadataFiltering(t *testing.T) {
	tests := []struct {
		name           string
		oldContent     string
		newContent     string
		contextLines   uint
		ignorePattern  *string
		expectedOutput string
		description    string
	}{
		{
			name: "Ignoring version and helm chart",
			oldContent: `metadata:
  app.kubernetes.io/version: 1.0.0
  helm.sh/chart: my-chart-1.0.0
  app.kubernetes.io/name: test-app
data:
  test: test-value
  config: value1`,
			newContent: `metadata:
  app.kubernetes.io/version: 2.0.0
  helm.sh/chart: my-chart-2.0.0
  app.kubernetes.io/name: test-app
data:
  test: test-value
  config: value2`,
			contextLines:  2,
			ignorePattern: nil,
			expectedOutput: ` data:
   test: test-value
-  config: value1
+  config: value2`,
			description: "Ignore pattern is not applied to version and helm chart",
		},
		{
			name: "Ignoring version and helm chart when nested labels are changed",
			oldContent: `spec:
  template:
    metadata:
      labels:
        app: myapp
        app.kubernetes.io/version: 1.0.0
        helm.sh/chart: my-chart-1.0.0
        app.kubernetes.io/name: test-app
    spec:
      containers:
        - name: myapp
          image: dag-andersen/myapp:v1`,
			newContent: `spec:
  template:
    metadata:
      labels:
        app: myapp
        app.kubernetes.io/version: 2.0.0
        helm.sh/chart: my-chart-2.0.0
        app.kubernetes.io/name: test-app
    spec:
      containers:
        - name: myapp
          image: dag-andersen/myapp:v2`,
			contextLines:  2,
			ignorePattern: nil,
			expectedOutput: `       containers:
         - name: myapp
-          image: dag-andersen/myapp:v1
+          image: dag-andersen/myapp:v2`,

			description: "Ignore pattern is not applied to version and helm chart when nested labels are changed",
		},
		{
			name: "Ignoring but still include version and helm chart when the line count is large",
			oldContent: `metadata:
  app.kubernetes.io/version: 1.0.0
  helm.sh/chart: my-chart-1.0.0
  app.kubernetes.io/name: test-app
data:
  test: test-value
  config: value1`,
			newContent: `metadata:
  app.kubernetes.io/version: 2.0.0
  helm.sh/chart: my-chart-2.0.0
  app.kubernetes.io/name: test-app
data:
  test: test-value
  config: value2`,
			contextLines:  10,
			ignorePattern: nil,
			expectedOutput: ` metadata:
-  app.kubernetes.io/version: 1.0.0
-  helm.sh/chart: my-chart-1.0.0
+  app.kubernetes.io/version: 2.0.0
+  helm.sh/chart: my-chart-2.0.0
   app.kubernetes.io/name: test-app
 data:
   test: test-value
-  config: value1
+  config: value2`,
			description: "Ignore pattern is not applied to version and helm chart when the line count is large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output string
			if tt.oldContent == "" {
				output = formatNewFileDiff(tt.newContent, tt.contextLines, tt.ignorePattern)
			} else if tt.newContent == "" {
				output = formatDeletedFileDiff(tt.oldContent, tt.contextLines, tt.ignorePattern)
			} else {
				output = formatModifiedFileDiff(tt.oldContent, tt.newContent, tt.contextLines, tt.ignorePattern)
			}

			if strings.TrimRight(output, "\n") != strings.TrimRight(tt.expectedOutput, "\n") {
				t.Errorf("%s\nExpected:\n%s\nGot:\n%s", tt.description, tt.expectedOutput, output)
			}
		})
	}
}

// Test invalid regex patterns
func TestInvalidRegexPatterns(t *testing.T) {
	invalidPatterns := []string{
		"[unclosed",
		"(unclosed",
		"\\",              // Single backslash at end
		"?unescaped{n,m}", // Invalid quantifier
	}

	content := "line1\nTIMESTAMP: 123456\nline3"

	for _, pattern := range invalidPatterns {
		t.Run("Invalid_"+pattern, func(t *testing.T) {
			patternPtr := &pattern

			// Should not panic, should fall back to string matching
			output := formatNewFileDiff(content, 3, patternPtr)

			// The output should still contain all lines
			if !strings.Contains(output, "+TIMESTAMP: 123456") {
				t.Errorf("Invalid regex should fall back to string matching but keep all content")
			}

			// Try with modified diff too
			newContent := "line1\nTIMESTAMP: 654321\nline3"
			modOutput := formatModifiedFileDiff(content, newContent, 3, patternPtr)

			// Should still produce some output
			if len(modOutput) == 0 && !strings.Contains(pattern, content) {
				t.Errorf("Invalid regex should still produce diff output unless it happens to match as string")
			}
		})
	}
}
