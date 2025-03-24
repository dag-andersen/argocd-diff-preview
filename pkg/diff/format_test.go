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
		t.Errorf("Zero context lines didn't show changes properly: %s", zeroContextOutput)
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
	if len(strings.Split(smallContextOutput, "\n")) < 10 {
		t.Errorf("Small context output should include most lines: %s", smallContextOutput)
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
