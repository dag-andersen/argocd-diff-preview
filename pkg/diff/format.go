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
	operation   diffmatchpatch.Operation
	text        string
	isChange    bool
	show        bool
	origLineNum int // line number in the new content, for resource lookup
}

// formatDiff formats diffmatchpatch.Diff into unified diff format
// resourceIndex is optional and used to insert resource headers at chunk boundaries
// Output format: each resource section has its header outside the code block, with the diff content inside
func formatDiff(diffs []diffmatchpatch.Diff, contextLines uint, ignorePattern *string, resourceIndex *ResourceIndex) changeInfo {
	var buffer bytes.Buffer

	// Process the diffs and format them in unified diff format
	// We'll keep track of context lines to include only the specified number
	// Also track the original line number in the new content for resource lookup
	var processedLines []processedLine

	// Track line number in the "new" content (DiffEqual and DiffInsert contribute to this)
	newLineNum := 0

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

			// Ignore specific hardcoded lines.
			if show && isChange {
				for _, pattern := range ignorePatterns {
					if strings.HasPrefix(strings.TrimLeft(line, " \t"), pattern) {
						show = false
						break
					}
				}
			}

			// For DiffEqual and DiffInsert, this line exists in the new content
			// For DiffDelete, it only exists in the old content
			lineNum := newLineNum
			if d.Type != diffmatchpatch.DiffDelete {
				newLineNum++
			}

			processedLines = append(processedLines, processedLine{
				operation:   d.Type,
				text:        line,
				isChange:    isChange,
				show:        show,
				origLineNum: lineNum,
			})
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
		return changeInfo{content: "", addedLines: 0, deletedLines: 0}
	}

	// Now create chunks of lines to include based on context
	type chunk struct {
		start int
		end   int
	}
	var chunks []chunk

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
			chunks = append(chunks, chunk{start: chunkStart, end: chunkEnd})

			chunkStart = max(0, currentLine-int(contextLines))
			chunkEnd = min(len(processedLines)-1, currentLine+int(contextLines))
		}
	}

	// Add the last chunk
	chunks = append(chunks, chunk{start: chunkStart, end: chunkEnd})

	// outputLine now includes a lineType to distinguish between diff content,
	// resource headers, and separators
	type lineType int
	const (
		lineTypeDiff lineType = iota
		lineTypeResourceHeader
		lineTypeSeparator
		lineTypeYAMLSeparator // the "---" line
	)

	type outputLine struct {
		operation diffmatchpatch.Operation
		text      string
		lType     lineType
	}
	var filteredLines []outputLine

	lastResourceHeader := ""

	for i, c := range chunks {
		// Insert resource header at the start of each chunk if resource changed
		if resourceIndex != nil && len(processedLines) > 0 {
			// Use the line number of the first line in the chunk
			firstLineNum := processedLines[c.start].origLineNum
			resource := resourceIndex.GetResourceForLine(firstLineNum)
			if resource != nil {
				header := resource.FormatHeader()
				if header != "" && header != lastResourceHeader {
					filteredLines = append(filteredLines, outputLine{
						operation: diffmatchpatch.DiffEqual,
						text:      header,
						lType:     lineTypeResourceHeader,
					})
					lastResourceHeader = header
				}
			}
		}

		// Add all lines in this chunk
		for j := c.start; j <= c.end; j++ {
			// Check if we're crossing a resource boundary within the chunk
			// Insert resource header when we see "---" and the next resource is different
			if resourceIndex != nil && strings.TrimSpace(processedLines[j].text) == "---" {
				// Add the --- line as a YAML separator
				filteredLines = append(filteredLines, outputLine{
					operation: processedLines[j].operation,
					text:      processedLines[j].text,
					lType:     lineTypeYAMLSeparator,
				})

				// Look up the resource for the next line (if there is one)
				if j+1 <= c.end {
					nextLineNum := processedLines[j+1].origLineNum
					resource := resourceIndex.GetResourceForLine(nextLineNum)
					if resource != nil {
						header := resource.FormatHeader()
						if header != "" && header != lastResourceHeader {
							filteredLines = append(filteredLines, outputLine{
								operation: diffmatchpatch.DiffEqual,
								text:      header,
								lType:     lineTypeResourceHeader,
							})
							lastResourceHeader = header
						}
					}
				}
				continue
			}

			filteredLines = append(filteredLines, outputLine{
				operation: processedLines[j].operation,
				text:      processedLines[j].text,
				lType:     lineTypeDiff,
			})
		}

		// Add separator if there's a next chunk and it's far enough away
		if i < len(chunks)-1 {
			nextChunk := chunks[i+1]
			skippedLines := nextChunk.start - c.end - 1

			if skippedLines > 0 {
				separator := fmt.Sprintf("@@ skipped %d lines (%d -> %d) @@", skippedLines, c.end+1, nextChunk.start-1)
				filteredLines = append(filteredLines, outputLine{
					operation: diffmatchpatch.DiffEqual,
					text:      separator,
					lType:     lineTypeSeparator,
				})
			}
		}
	}

	addedLines := 0
	deletedLines := 0

	// Write the filtered lines
	// When resourceIndex is provided, wrap each resource's diff content in code blocks
	// When resourceIndex is nil, output plain diff format (no code blocks)
	inCodeBlock := false
	useCodeBlocks := resourceIndex != nil

	for _, line := range filteredLines {
		switch line.lType {
		case lineTypeResourceHeader:
			// Close any open code block before resource header
			if inCodeBlock {
				buffer.WriteString("```\n")
				inCodeBlock = false
			}
			buffer.WriteString(line.text + "\n")
		case lineTypeYAMLSeparator:
			// Close any open code block before YAML separator
			if inCodeBlock {
				buffer.WriteString("```\n")
				inCodeBlock = false
			}
			buffer.WriteString("---\n")
		case lineTypeSeparator:
			// Skipped lines separator stays inside code block (or just as-is if no code blocks)
			buffer.WriteString(line.text + "\n")
		case lineTypeDiff:
			// Open code block if using code blocks and not already open
			if useCodeBlocks && !inCodeBlock {
				buffer.WriteString("```diff\n")
				inCodeBlock = true
			}
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
	}

	// Close any remaining open code block
	if inCodeBlock {
		buffer.WriteString("```\n")
	}

	return changeInfo{content: buffer.String(), addedLines: addedLines, deletedLines: deletedLines}
}

// formatNewFileDiff formats a diff for a new file using the go-git/utils/diff package
func formatNewFileDiff(content string, contextLines uint, ignorePattern *string, resourceIndex *ResourceIndex) changeInfo {
	// For new files, we diff from empty string to the content
	diffs := diff.Do("", content)
	return formatDiff(diffs, contextLines, ignorePattern, resourceIndex)
}

// formatDeletedFileDiff formats a diff for a deleted file using the go-git/utils/diff package
func formatDeletedFileDiff(content string, contextLines uint, ignorePattern *string, resourceIndex *ResourceIndex) changeInfo {
	// For deleted files, we diff from the content to empty string
	diffs := diff.Do(content, "")
	return formatDiff(diffs, contextLines, ignorePattern, resourceIndex)
}

// formatModifiedFileDiff formats a diff for a modified file using the go-git/utils/diff package
func formatModifiedFileDiff(oldContent, newContent string, contextLines uint, ignorePattern *string, resourceIndex *ResourceIndex) changeInfo {
	diffs := diff.Do(oldContent, newContent)
	return formatDiff(diffs, contextLines, ignorePattern, resourceIndex)
}
