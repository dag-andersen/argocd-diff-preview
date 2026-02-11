package diff

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/utils/diff"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// Patterns that should always be ignored
var ignorePatterns = []string{
	"app.kubernetes.io/version: ",
	"helm.sh/chart: ",
	"checksum/config: ",
	"checksum/rules: ",
	"checksum/certs: ",
	"checksum/cmd-params: ",
	"checksum/cm: ",
	"checksum/config-maps: ",
	"checksum/secrets: ",
	"caBundle: ",
}

// shouldIgnoreLine checks if a line should be ignored based on regex pattern
func shouldIgnoreLine(line, pattern string) bool {
	matched, err := regexp.MatchString(pattern, line)
	if err != nil {
		// If regex fails, fall back to simple string matching
		return strings.Contains(line, pattern)
	}
	return matched
}

type changeInfo struct {
	content      string
	addedLines   int
	deletedLines int
}

// processedLine represents a line in the diff with metadata for processing
type processedLine struct {
	operation diffmatchpatch.Operation
	text      string
	isChange  bool
	show      bool
}

// chunk represents a contiguous range of lines to include in the diff output
type chunk struct {
	start int
	end   int
}

// formatDiff formats diffmatchpatch.Diff into unified diff format
func formatDiff(diffs []diffmatchpatch.Diff, contextLines uint, ignorePattern *string) changeInfo {
	// Phase 1: Process diffs into lines with metadata
	processedLines := processDiffLines(diffs, ignorePattern)

	// Phase 2: Find indices of changed lines that should be shown
	changedLines := findChangedLines(processedLines)
	if len(changedLines) == 0 {
		return changeInfo{content: "", addedLines: 0, deletedLines: 0}
	}

	// Phase 3: Build chunks of lines to include based on context
	chunks := buildChunks(changedLines, len(processedLines), contextLines)

	// Phase 4: Build output from chunks
	return buildOutput(chunks, processedLines)
}

// processDiffLines converts raw diffs into processedLine structs with metadata
func processDiffLines(diffs []diffmatchpatch.Diff, ignorePattern *string) []processedLine {
	var processedLines []processedLine

	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		isChange := d.Type != diffmatchpatch.DiffEqual

		for _, line := range lines {
			show := shouldShowLine(line, isChange, ignorePattern)
			processedLines = append(processedLines, processedLine{
				operation: d.Type,
				text:      line,
				isChange:  isChange,
				show:      show,
			})
		}
	}

	return processedLines
}

// shouldShowLine determines if a line should be shown in the diff output
func shouldShowLine(line string, isChange bool, ignorePattern *string) bool {
	if !isChange {
		return true
	}

	if ignorePattern != nil && *ignorePattern != "" {
		if shouldIgnoreLine(line, *ignorePattern) {
			return false
		}
	}

	for _, pattern := range ignorePatterns {
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), pattern) {
			return false
		}
	}

	return true
}

// findChangedLines returns indices of lines that have visible changes
func findChangedLines(processedLines []processedLine) []int {
	var changedLines []int
	for i, line := range processedLines {
		if line.isChange && line.show {
			changedLines = append(changedLines, i)
		}
	}
	return changedLines
}

// buildChunks groups changed lines into chunks with surrounding context
func buildChunks(changedLines []int, totalLines int, contextLines uint) []chunk {
	var chunks []chunk

	chunkStart := max(0, changedLines[0]-int(contextLines))
	chunkEnd := min(totalLines-1, changedLines[0]+int(contextLines))

	for i := 1; i < len(changedLines); i++ {
		currentLine := changedLines[i]
		if currentLine-chunkEnd <= 2*int(contextLines) {
			chunkEnd = min(totalLines-1, currentLine+int(contextLines))
		} else {
			chunks = append(chunks, chunk{start: chunkStart, end: chunkEnd})
			chunkStart = max(0, currentLine-int(contextLines))
			chunkEnd = min(totalLines-1, currentLine+int(contextLines))
		}
	}

	chunks = append(chunks, chunk{start: chunkStart, end: chunkEnd})
	return chunks
}

// buildOutput converts chunks into the final diff output string
func buildOutput(chunks []chunk, processedLines []processedLine) changeInfo {
	var buffer bytes.Buffer
	addedLines := 0
	deletedLines := 0

	for i, c := range chunks {
		// Add all lines in this chunk
		for j := c.start; j <= c.end; j++ {
			line := processedLines[j]
			switch line.operation {
			case diffmatchpatch.DiffInsert:
				addedLines++
				buffer.WriteString("+" + line.text + "\n")
			case diffmatchpatch.DiffDelete:
				deletedLines++
				buffer.WriteString("-" + line.text + "\n")
			default:
				buffer.WriteString(" " + line.text + "\n")
			}
		}

		// Add separator if there's a next chunk and it's far enough away
		if i < len(chunks)-1 {
			nextChunk := chunks[i+1]
			if skippedLines := nextChunk.start - c.end - 1; skippedLines > 0 {
				separator := fmt.Sprintf("@@ skipped %d lines (%d -> %d) @@", skippedLines, c.end+1, nextChunk.start-1)
				buffer.WriteString(separator + "\n")
			}
		}
	}

	return changeInfo{content: buffer.String(), addedLines: addedLines, deletedLines: deletedLines}
}

// formatNewFileDiff formats a diff for a new file using the go-git/utils/diff package
func formatNewFileDiff(content string, contextLines uint, ignorePattern *string) changeInfo {
	// For new files, we diff from empty string to the content
	diffs := diff.Do("", content)
	return formatDiff(diffs, contextLines, ignorePattern)
}

// formatDeletedFileDiff formats a diff for a deleted file using the go-git/utils/diff package
func formatDeletedFileDiff(content string, contextLines uint, ignorePattern *string) changeInfo {
	// For deleted files, we diff from the content to empty string
	diffs := diff.Do(content, "")
	return formatDiff(diffs, contextLines, ignorePattern)
}

// formatModifiedFileDiff formats a diff for a modified file using the go-git/utils/diff package
func formatModifiedFileDiff(oldContent, newContent string, contextLines uint, ignorePattern *string) changeInfo {
	diffs := diff.Do(oldContent, newContent)
	return formatDiff(diffs, contextLines, ignorePattern)
}
