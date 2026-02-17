package matching

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/utils/diff"
	"github.com/sergi/go-diff/diffmatchpatch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// DiffResult contains the diff output and statistics for a resource pair
type DiffResult struct {
	Content      string // The formatted diff content
	AddedLines   int    // Number of lines added
	DeletedLines int    // Number of lines deleted
}

// Diff generates a unified diff between the base and target resources.
// Returns a DiffResult with the formatted diff and line statistics.
//
// For added resources (Base is nil), shows all lines as additions.
// For deleted resources (Target is nil), shows all lines as deletions.
// For modified resources, shows a unified diff with context.
func (rp *ResourcePair) Diff(contextLines uint) (DiffResult, error) {
	baseYAML, err := resourceToYAML(rp.Base)
	if err != nil {
		return DiffResult{}, fmt.Errorf("failed to marshal base resource: %w", err)
	}

	targetYAML, err := resourceToYAML(rp.Target)
	if err != nil {
		return DiffResult{}, fmt.Errorf("failed to marshal target resource: %w", err)
	}

	// Generate diff
	diffs := diff.Do(baseYAML, targetYAML)
	result := formatDiff(diffs, contextLines)

	return result, nil
}

// resourceToYAML converts an unstructured resource to YAML string.
// Returns empty string for nil resources.
func resourceToYAML(r *unstructured.Unstructured) (string, error) {
	if r == nil {
		return "", nil
	}

	yamlBytes, err := yaml.Marshal(r.Object)
	if err != nil {
		return "", err
	}

	return string(yamlBytes), nil
}

// formatDiff formats diffmatchpatch.Diff into unified diff format with context
func formatDiff(diffs []diffmatchpatch.Diff, contextLines uint) DiffResult {
	// Process diffs into lines with metadata
	processedLines := processDiffLines(diffs)

	// Find indices of changed lines
	changedLines := findChangedLines(processedLines)
	if len(changedLines) == 0 {
		return DiffResult{Content: "", AddedLines: 0, DeletedLines: 0}
	}

	// Build chunks of lines to include based on context
	chunks := buildChunks(changedLines, len(processedLines), contextLines)

	// Build output from chunks
	return buildOutput(chunks, processedLines)
}

// processedLine represents a line in the diff with metadata
type processedLine struct {
	operation diffmatchpatch.Operation
	text      string
}

// chunk represents a contiguous range of lines to include in output
type chunk struct {
	start int
	end   int
}

// processDiffLines converts raw diffs into processedLine structs
func processDiffLines(diffs []diffmatchpatch.Diff) []processedLine {
	var processedLines []processedLine

	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		for _, line := range lines {
			processedLines = append(processedLines, processedLine{
				operation: d.Type,
				text:      line,
			})
		}
	}

	return processedLines
}

// findChangedLines returns indices of lines that have changes (insert or delete)
func findChangedLines(processedLines []processedLine) []int {
	var changedLines []int
	for i, line := range processedLines {
		if line.operation != diffmatchpatch.DiffEqual {
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
			// Extend current chunk
			chunkEnd = min(totalLines-1, currentLine+int(contextLines))
		} else {
			// Start new chunk
			chunks = append(chunks, chunk{start: chunkStart, end: chunkEnd})
			chunkStart = max(0, currentLine-int(contextLines))
			chunkEnd = min(totalLines-1, currentLine+int(contextLines))
		}
	}

	chunks = append(chunks, chunk{start: chunkStart, end: chunkEnd})
	return chunks
}

// buildOutput converts chunks into the final diff output string
func buildOutput(chunks []chunk, processedLines []processedLine) DiffResult {
	var buffer bytes.Buffer
	addedLines := 0
	deletedLines := 0

	for i, c := range chunks {
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

		// Add separator if there's a next chunk
		if i < len(chunks)-1 {
			nextChunk := chunks[i+1]
			if skippedLines := nextChunk.start - c.end - 1; skippedLines > 0 {
				separator := fmt.Sprintf("@@ skipped %d lines (%d -> %d) @@", skippedLines, c.end+1, nextChunk.start-1)
				buffer.WriteString(separator + "\n")
			}
		}
	}

	return DiffResult{Content: buffer.String(), AddedLines: addedLines, DeletedLines: deletedLines}
}
