package matching

import (
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
	return generateResourceDiff(*rp, contextLines, nil)
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

// chunk represents a contiguous range of lines to include in output
type chunk struct {
	start int
	end   int
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
