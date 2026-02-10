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

// ResourceBlock represents a single resource's diff content without any formatting
type ResourceBlock struct {
	Header  string // e.g., "Deployment/my-deploy (default)" - NO formatting like #### or code fences
	Content string // Raw diff lines for this resource - NO code fences, just +/- prefixed lines
}

type changeInfo struct {
	blocks       []ResourceBlock // List of resource blocks with raw content
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
// Output: each ResourceBlock contains a header (e.g., "Deployment/my-app (default)") and raw diff content
func formatDiff(diffs []diffmatchpatch.Diff, contextLines uint, ignorePattern *string, resourceIndex *ResourceIndex) changeInfo {
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

	// No changes to show, so return empty changeInfo
	if len(changedLines) == 0 {
		return changeInfo{blocks: nil, addedLines: 0, deletedLines: 0}
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

	// Build ResourceBlocks directly from chunks
	// Each resource header starts a new block, and content is raw diff lines
	var blocks []ResourceBlock
	var currentBlock *ResourceBlock
	var contentBuffer bytes.Buffer
	addedLines := 0
	deletedLines := 0
	lastResourceHeader := ""

	// Helper to flush current block
	flushBlock := func() {
		if currentBlock != nil && contentBuffer.Len() > 0 {
			currentBlock.Content = strings.TrimRight(contentBuffer.String(), "\n")
			blocks = append(blocks, *currentBlock)
		} else if currentBlock == nil && contentBuffer.Len() > 0 {
			// Content before any resource header goes into a block with empty header
			blocks = append(blocks, ResourceBlock{
				Header:  "",
				Content: strings.TrimRight(contentBuffer.String(), "\n"),
			})
		}
		contentBuffer.Reset()
	}

	// Helper to write a diff line to the content buffer
	writeDiffLine := func(line processedLine) {
		switch line.operation {
		case diffmatchpatch.DiffInsert:
			addedLines++
			contentBuffer.WriteString("+" + line.text + "\n")
		case diffmatchpatch.DiffDelete:
			deletedLines++
			contentBuffer.WriteString("-" + line.text + "\n")
		default:
			contentBuffer.WriteString(" " + line.text + "\n")
		}
	}

	// Helper to start a new resource block if the header changed
	maybeStartNewBlock := func(header string) {
		if header != "" && header != lastResourceHeader {
			flushBlock()
			currentBlock = &ResourceBlock{Header: header}
			lastResourceHeader = header
		}
	}

	for i, c := range chunks {
		// Insert resource header at the start of each chunk if resource changed
		if resourceIndex != nil && len(processedLines) > 0 {
			// Use the line number of the first line in the chunk
			firstLineNum := processedLines[c.start].origLineNum
			if resource := resourceIndex.GetResourceForLine(firstLineNum); resource != nil {
				maybeStartNewBlock(resource.FormatHeader())
			}
		}

		// Add all lines in this chunk
		for j := c.start; j <= c.end; j++ {
			line := processedLines[j]

			// Check if we're crossing a resource boundary within the chunk
			// When we see "---", check if the next resource is different and start a new block
			// Skip the "---" line itself since we now split by resource into separate blocks
			if resourceIndex != nil && strings.TrimSpace(line.text) == "---" {
				// Look up the resource for the next line (if there is one)
				if j+1 <= c.end {
					nextLineNum := processedLines[j+1].origLineNum
					if resource := resourceIndex.GetResourceForLine(nextLineNum); resource != nil {
						maybeStartNewBlock(resource.FormatHeader())
					}
				}
				continue
			}

			// Handle "Skipped Resource:" lines - convert them to a header-only block
			// These are generated when resources match ignore rules
			if strings.HasPrefix(strings.TrimSpace(line.text), "Skipped Resource:") {
				// Flush current block and create a new one with just this as header
				flushBlock()
				skippedHeader := strings.TrimSpace(line.text)
				blocks = append(blocks, ResourceBlock{Header: skippedHeader, Content: ""})
				lastResourceHeader = skippedHeader
				currentBlock = nil
				continue
			}

			writeDiffLine(line)
		}

		// Add separator if there's a next chunk and it's far enough away
		if i < len(chunks)-1 {
			nextChunk := chunks[i+1]
			if skippedLines := nextChunk.start - c.end - 1; skippedLines > 0 {
				separator := fmt.Sprintf("@@ skipped %d lines (%d -> %d) @@", skippedLines, c.end+1, nextChunk.start-1)
				contentBuffer.WriteString(separator + "\n")
			}
		}
	}

	// Flush the last block
	flushBlock()

	return changeInfo{blocks: blocks, addedLines: addedLines, deletedLines: deletedLines}
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
