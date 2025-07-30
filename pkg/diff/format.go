package diff

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/utils/diff"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// shouldIgnoreLine checks if a line should be ignored based on regex pattern
func shouldIgnoreLine(line, pattern string) bool {
	matched, err := regexp.MatchString(pattern, line)
	if err != nil {
		// If regex fails, fall back to simple string matching
		return strings.Contains(line, pattern)
	}
	return matched
}

// formatDiff formats diffmatchpatch.Diff into unified diff format
func formatDiff(diffs []diffmatchpatch.Diff, contextLines uint, ignorePattern *string) string {
	var buffer bytes.Buffer

	// Process the diffs and format them in unified diff format
	// We'll keep track of context lines to include only the specified number
	var processedLines []struct {
		prefix   string
		text     string
		isChange bool
		show     bool
	}

	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		// If the last element is empty (due to trailing newline), remove it
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		isChange := d.Type != diffmatchpatch.DiffEqual

		for _, line := range lines {
			// Determine if this line should be shown or filtered out
			show := true
			if isChange && ignorePattern != nil && *ignorePattern != "" {
				// Only apply regex filter to changed lines
				show = !shouldIgnoreLine(line, *ignorePattern)
			}

			prefix := " "
			switch d.Type {
			case diffmatchpatch.DiffDelete:
				prefix = "-"
			case diffmatchpatch.DiffInsert:
				prefix = "+"
			}

			processedLines = append(processedLines, struct {
				prefix   string
				text     string
				isChange bool
				show     bool
			}{prefix, line, isChange, show})
		}
	}

	// First find all changed lines that should be shown
	var changedLines []int
	for i, line := range processedLines {
		if line.isChange && line.show {
			changedLines = append(changedLines, i)
		}
	}

	// No changes to show, so return empty string
	if len(changedLines) == 0 {
		return ""
	}

	// Now create chunks of lines to include based on context
	var chunks []struct {
		start int
		end   int
	}

	// Start with the first changed line and its context
	chunkStart := max(0, changedLines[0]-int(contextLines))
	chunkEnd := min(len(processedLines)-1, changedLines[0]+int(contextLines))

	// Extend chunk to include other changed lines that are within 2*contextLines
	for i := 1; i < len(changedLines); i++ {
		currentLine := changedLines[i]
		// If this changed line is close to our current chunk, extend the chunk
		if currentLine-chunkEnd <= 2*int(contextLines) {
			chunkEnd = min(len(processedLines)-1, currentLine+int(contextLines))
		} else {
			// Otherwise, finish this chunk and start a new one
			chunks = append(chunks, struct {
				start int
				end   int
			}{chunkStart, chunkEnd})

			chunkStart = max(0, currentLine-int(contextLines))
			chunkEnd = min(len(processedLines)-1, currentLine+int(contextLines))
		}
	}

	// Add the last chunk
	chunks = append(chunks, struct {
		start int
		end   int
	}{chunkStart, chunkEnd})

	// Now build the output with separators between chunks
	var filteredLines []struct {
		prefix string
		text   string
	}

	for i, chunk := range chunks {
		// Add all lines in this chunk
		for j := chunk.start; j <= chunk.end; j++ {
			filteredLines = append(filteredLines, struct {
				prefix string
				text   string
			}{processedLines[j].prefix, processedLines[j].text})
		}

		// Add separator if there's a next chunk and it's far enough away
		if i < len(chunks)-1 {
			nextChunk := chunks[i+1]
			skippedLines := nextChunk.start - chunk.end - 1

			if skippedLines > 0 {
				separator := fmt.Sprintf("@@ skipped %d lines (%d -> %d) @@", skippedLines, chunk.end+1, nextChunk.start-1)
				filteredLines = append(filteredLines, struct {
					prefix string
					text   string
				}{"", separator})
			}
		}
	}

	// Write the filtered lines
	for _, line := range filteredLines {
		if strings.HasPrefix(line.text, "@@ skipped") {
			buffer.WriteString(line.text + "\n")
		} else {
			buffer.WriteString(line.prefix + line.text + "\n")
		}
	}

	return buffer.String()
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// formatNewFileDiff formats a diff for a new file using the go-git/utils/diff package
func formatNewFileDiff(content string, contextLines uint, ignorePattern *string) string {
	// For new files, we diff from empty string to the content
	diffs := diff.Do("", content)
	return formatDiff(diffs, contextLines, ignorePattern)
}

// formatDeletedFileDiff formats a diff for a deleted file using the go-git/utils/diff package
func formatDeletedFileDiff(content string, contextLines uint, ignorePattern *string) string {
	// For deleted files, we diff from the content to empty string
	diffs := diff.Do(content, "")
	return formatDiff(diffs, contextLines, ignorePattern)
}

// formatModifiedFileDiff formats a diff for a modified file using the go-git/utils/diff package
func formatModifiedFileDiff(oldContent, newContent string, contextLines uint, ignorePattern *string) string {
	diffs := diff.Do(oldContent, newContent)
	return formatDiff(diffs, contextLines, ignorePattern)
}
