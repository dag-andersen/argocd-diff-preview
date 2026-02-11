package diff

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
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

// compileIgnorePattern compiles a regex pattern string into a *regexp.Regexp.
// Returns (nil, nil) if the pattern is empty (no pattern provided).
// Returns (nil, error) if the pattern is invalid.
func compileIgnorePattern(pattern *string) (*regexp.Regexp, error) {
	if pattern == nil || *pattern == "" {
		return nil, nil
	}
	compiled, err := regexp.Compile(*pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid ignore pattern regex %q: %w", *pattern, err)
	}
	return compiled, nil
}

// shouldIgnoreLine checks if a line should be ignored based on a pre-compiled regex pattern
func shouldIgnoreLine(line string, pattern *regexp.Regexp) bool {
	if pattern == nil {
		return false
	}
	return pattern.MatchString(line)
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

// chunk represents a contiguous range of lines to include in the diff output
type chunk struct {
	start int
	end   int
}

// formatDiff formats diffmatchpatch.Diff into unified diff format
// resourceIndex is optional and used to insert resource headers at chunk boundaries
// Output: each ResourceBlock contains a header (e.g., "Deployment/my-app (default)") and raw diff content
func formatDiff(diffs []diffmatchpatch.Diff, contextLines uint, ignorePattern *string, resourceIndex *ResourceIndex) (changeInfo, error) {
	// Phase 1: Process diffs into lines with metadata
	processedLines, err := processDiffLines(diffs, ignorePattern)
	if err != nil {
		return changeInfo{}, err
	}

	// Phase 2: Find indices of changed lines that should be shown
	changedLines := findChangedLines(processedLines)
	if len(changedLines) == 0 {
		return changeInfo{blocks: nil, addedLines: 0, deletedLines: 0}, nil
	}

	// Phase 3: Build chunks of lines to include based on context
	chunks := buildChunks(changedLines, len(processedLines), contextLines)

	// Phase 4: Build resource blocks from chunks
	return buildResourceBlocks(chunks, processedLines, resourceIndex)
}

// processDiffLines converts raw diffs into processedLine structs with metadata
func processDiffLines(diffs []diffmatchpatch.Diff, ignorePattern *string) ([]processedLine, error) {
	compiledIgnorePattern, err := compileIgnorePattern(ignorePattern)
	if err != nil {
		return nil, err
	}

	var processedLines []processedLine
	newLineNum := 0

	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		isChange := d.Type != diffmatchpatch.DiffEqual

		for _, line := range lines {
			show := shouldShowLine(line, isChange, compiledIgnorePattern)

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

	return processedLines, nil
}

// shouldShowLine determines if a line should be shown in the diff output
func shouldShowLine(line string, isChange bool, compiledIgnorePattern *regexp.Regexp) bool {
	if !isChange {
		return true
	}

	if compiledIgnorePattern != nil && shouldIgnoreLine(line, compiledIgnorePattern) {
		return false
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

// buildResourceBlocks converts chunks into ResourceBlocks with headers and diff content
func buildResourceBlocks(chunks []chunk, processedLines []processedLine, resourceIndex *ResourceIndex) (changeInfo, error) {
	var blocks []ResourceBlock
	var currentBlock *ResourceBlock
	var contentBuffer bytes.Buffer
	addedLines := 0
	deletedLines := 0
	lastResourceHeader := ""

	flushBlock := func() {
		if currentBlock != nil && contentBuffer.Len() > 0 {
			currentBlock.Content = strings.TrimRight(contentBuffer.String(), "\n")
			blocks = append(blocks, *currentBlock)
		} else if currentBlock == nil && contentBuffer.Len() > 0 {
			blocks = append(blocks, ResourceBlock{
				Header:  "",
				Content: strings.TrimRight(contentBuffer.String(), "\n"),
			})
		}
		contentBuffer.Reset()
	}

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

	maybeStartNewBlock := func(header string) {
		if header != "" && header != lastResourceHeader {
			flushBlock()
			currentBlock = &ResourceBlock{Header: header}
			lastResourceHeader = header
		}
	}

	for i, c := range chunks {
		if resourceIndex != nil && len(processedLines) > 0 {
			firstLineNum := processedLines[c.start].origLineNum
			if resource := resourceIndex.GetResourceForLine(firstLineNum); resource != nil {
				maybeStartNewBlock(resource.FormatHeader())
			}
		}

		for j := c.start; j <= c.end; j++ {
			line := processedLines[j]

			if resourceIndex != nil && strings.TrimSpace(line.text) == "---" {
				if j+1 <= c.end {
					nextLineNum := processedLines[j+1].origLineNum
					if resource := resourceIndex.GetResourceForLine(nextLineNum); resource != nil {
						maybeStartNewBlock(resource.FormatHeader())
					}
				}
				continue
			}

			if strings.HasSuffix(strings.TrimSpace(line.text), extract.HiddenResourceSuffix) {
				flushBlock()
				header := strings.TrimSpace(line.text)
				blocks = append(blocks, ResourceBlock{Header: header, Content: ""})
				lastResourceHeader = header
				currentBlock = nil
				continue
			}

			writeDiffLine(line)
		}

		if i < len(chunks)-1 {
			nextChunk := chunks[i+1]
			if skippedLines := nextChunk.start - c.end - 1; skippedLines > 0 {
				sameResource := true
				if resourceIndex != nil {
					nextLineNum := processedLines[nextChunk.start].origLineNum
					if nextResource := resourceIndex.GetResourceForLine(nextLineNum); nextResource != nil {
						sameResource = nextResource.FormatHeader() == lastResourceHeader
					}
				}
				if sameResource {
					separator := fmt.Sprintf("@@ skipped %d lines (%d -> %d) @@", skippedLines, c.end+1, nextChunk.start-1)
					contentBuffer.WriteString(separator + "\n")
				}
			}
		}
	}

	flushBlock()

	// Post-process blocks to detect kind changes and update headers
	for i := range blocks {
		blocks[i].Header = detectAndUpdateKindChange(blocks[i])
	}

	return changeInfo{blocks: blocks, addedLines: addedLines, deletedLines: deletedLines}, nil
}

// detectAndUpdateKindChange scans a block's content for kind changes (-kind: X, +kind: Y)
// and updates the header to show the transformation (e.g., "ConfigMap → Secret/name (ns)")
func detectAndUpdateKindChange(block ResourceBlock) string {
	if block.Header == "" || block.Content == "" {
		return block.Header
	}

	var oldKind, newKind string
	for line := range strings.SplitSeq(block.Content, "\n") {
		trimmed := strings.TrimSpace(line)
		if kind, found := strings.CutPrefix(trimmed, "-kind:"); found {
			oldKind = strings.TrimSpace(kind)
		} else if kind, found := strings.CutPrefix(trimmed, "+kind:"); found {
			newKind = strings.TrimSpace(kind)
		}
	}

	// If both old and new kind are found and different, update the header
	if oldKind != "" && newKind != "" && oldKind != newKind {
		// Header format is "Kind/Name (namespace)" or "Kind/Name"
		// We want to change it to "OldKind → NewKind/Name (namespace)"
		if idx := strings.Index(block.Header, "/"); idx != -1 {
			namePart := block.Header[idx:] // "/Name (namespace)" or "/Name"
			return oldKind + " → " + newKind + namePart
		}
	}

	return block.Header
}

// formatNewFileDiff formats a diff for a new file using the go-git/utils/diff package
func formatNewFileDiff(content string, contextLines uint, ignorePattern *string, resourceIndex *ResourceIndex) (changeInfo, error) {
	// For new files, we diff from empty string to the content
	diffs := diff.Do("", content)
	return formatDiff(diffs, contextLines, ignorePattern, resourceIndex)
}

// formatDeletedFileDiff formats a diff for a deleted file using the go-git/utils/diff package
func formatDeletedFileDiff(content string, contextLines uint, ignorePattern *string, resourceIndex *ResourceIndex) (changeInfo, error) {
	// For deleted files, we diff from the content to empty string
	diffs := diff.Do(content, "")
	return formatDiff(diffs, contextLines, ignorePattern, resourceIndex)
}

// formatModifiedFileDiff formats a diff for a modified file using the go-git/utils/diff package
func formatModifiedFileDiff(oldContent, newContent string, contextLines uint, ignorePattern *string, resourceIndex *ResourceIndex) (changeInfo, error) {
	diffs := diff.Do(oldContent, newContent)
	return formatDiff(diffs, contextLines, ignorePattern, resourceIndex)
}
