package diff

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/diff"
	"github.com/go-git/go-git/v5/utils/merkletrie"
	"github.com/rs/zerolog/log"
	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/dag-andersen/argocd-diff-preview/pkg/types"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
)

const markdownTemplate = `
## Argo CD Diff Preview

Summary:
` + "```diff" + `
%summary%
` + "```" + `

<details>
<summary>Diff:</summary>
<br>

` + "```diff" + `
%diff%
` + "```" + `

</details>
`

// GenerateDiff generates a diff between base and target branches
func GenerateDiff(
	outputFolder string,
	baseBranch *types.Branch,
	targetBranch *types.Branch,
	baseManifests map[string]string,
	targetManifests map[string]string,
	diffIgnoreRegex *string,
	lineCount uint,
	maxCharCount uint,
) error {
	// Write base manifests to disk
	basePath := fmt.Sprintf("%s/%s", outputFolder, baseBranch.Type())
	if err := utils.CreateFolder(basePath, true); err != nil {
		return fmt.Errorf("failed to create base folder: %s: %w", basePath, err)
	}
	for name, manifest := range baseManifests {
		if err := utils.WriteFile(fmt.Sprintf("%s/%s", basePath, name), manifest); err != nil {
			return fmt.Errorf("failed to write base manifest %s: %w", name, err)
		}
	}

	// Write target manifests to disk
	targetPath := fmt.Sprintf("%s/%s", outputFolder, targetBranch.Type())
	if err := utils.CreateFolder(targetPath, true); err != nil {
		return fmt.Errorf("failed to create target folder: %s: %w", targetPath, err)
	}
	for name, manifest := range targetManifests {
		if err := utils.WriteFile(fmt.Sprintf("%s/%s", targetPath, name), manifest); err != nil {
			return fmt.Errorf("failed to write target manifest %s: %w", name, err)
		}
	}

	maxDiffMessageCharCount := maxCharCount
	if maxDiffMessageCharCount == 0 {
		maxDiffMessageCharCount = 65536
	}

	log.Info().Msgf("🔮 Generating diff between %s and %s",
		baseBranch.Name, targetBranch.Name)

	// verify that the output folders exist
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return fmt.Errorf("base path does not exist: %s", basePath)
	}
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return fmt.Errorf("target path does not exist: %s", targetPath)
	}

	// Set default context line count if not provided
	if lineCount <= 0 {
		lineCount = 3 // Default to 3 context lines if not specified
	}

	// Set default context line count if not provided
	if lineCount == 0 {
		lineCount = 10
	}

	// Generate diffs using go-git by creating temporary git repos
	summary, detailedDiff, err := generateGitDiff(basePath, targetPath, diffIgnoreRegex, lineCount)
	if err != nil {
		return fmt.Errorf("failed to generate diff: %w", err)
	}

	// Handle truncation if needed
	remainingMaxChars := int(maxDiffMessageCharCount) - markdownTemplateLength() - len(summary)
	warningMessage := fmt.Sprintf("\n\n ⚠️⚠️⚠️ Diff is too long. Truncated to %d characters. This can be adjusted with the `--max-diff-length` flag",
		maxDiffMessageCharCount)

	var diffTruncated string
	switch {
	case remainingMaxChars > len(detailedDiff):
		diffTruncated = detailedDiff // No need to truncate
	case remainingMaxChars > len(warningMessage):
		log.Warn().Msgf("🚨 Diff is too long. Truncating message to %d characters",
			maxDiffMessageCharCount)
		lastDiffChar := remainingMaxChars - len(warningMessage)
		diffTruncated = detailedDiff[:lastDiffChar] + warningMessage
	default:
		return fmt.Errorf("diff is too long and cannot be truncated. Increase the max length with `--max-diff-length`")
	}

	// Generate and write markdown
	markdown := printDiff(strings.TrimSpace(summary), strings.TrimSpace(diffTruncated))
	markdownPath := fmt.Sprintf("%s/diff.md", outputFolder)
	if err := utils.WriteFile(markdownPath, markdown); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	log.Info().Msgf("🙏 Please check the %s file for differences", markdownPath)
	return nil
}

// generateGitDiff creates temporary Git repositories and uses go-git to generate a diff
func generateGitDiff(basePath, targetPath string, diffIgnore *string, diffContextLines uint) (string, string, error) {
	// Create temporary directories for Git repositories
	baseRepoPath, err := os.MkdirTemp("", "base-repo-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir for base repo: %w", err)
	}
	defer os.RemoveAll(baseRepoPath)

	targetRepoPath, err := os.MkdirTemp("", "target-repo-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir for target repo: %w", err)
	}
	defer os.RemoveAll(targetRepoPath)

	// Initialize Git repositories
	baseRepo, err := git.PlainInit(baseRepoPath, false)
	if err != nil {
		return "", "", fmt.Errorf("failed to init base repo: %w", err)
	}

	targetRepo, err := git.PlainInit(targetRepoPath, false)
	if err != nil {
		return "", "", fmt.Errorf("failed to init target repo: %w", err)
	}

	// Copy files to Git repositories
	if err := copyFilesToRepo(basePath, baseRepoPath); err != nil {
		return "", "", fmt.Errorf("failed to copy base files: %w", err)
	}

	if err := copyFilesToRepo(targetPath, targetRepoPath); err != nil {
		return "", "", fmt.Errorf("failed to copy target files: %w", err)
	}

	// Add all files and commit in base repo
	baseWorktree, err := baseRepo.Worktree()
	if err != nil {
		return "", "", fmt.Errorf("failed to get base worktree: %w", err)
	}

	if err := baseWorktree.AddGlob("."); err != nil {
		return "", "", fmt.Errorf("failed to add files to base repo: %w", err)
	}

	baseCommitHash, err := baseWorktree.Commit("Base state", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "ArgoCD Diff Preview",
			Email: "noreply@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to commit to base repo: %w", err)
	}

	// Add all files and commit in target repo
	targetWorktree, err := targetRepo.Worktree()
	if err != nil {
		return "", "", fmt.Errorf("failed to get target worktree: %w", err)
	}

	if err := targetWorktree.AddGlob("."); err != nil {
		return "", "", fmt.Errorf("failed to add files to target repo: %w", err)
	}

	targetCommitHash, err := targetWorktree.Commit("Target state", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "ArgoCD Diff Preview",
			Email: "noreply@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to commit to target repo: %w", err)
	}

	// Retrieve commits
	baseCommit, err := baseRepo.CommitObject(baseCommitHash)
	if err != nil {
		return "", "", fmt.Errorf("failed to get base commit: %w", err)
	}

	targetCommit, err := targetRepo.CommitObject(targetCommitHash)
	if err != nil {
		return "", "", fmt.Errorf("failed to get target commit: %w", err)
	}

	// Get base and target trees
	baseTree, err := baseCommit.Tree()
	if err != nil {
		return "", "", fmt.Errorf("failed to get base tree: %w", err)
	}

	targetTree, err := targetCommit.Tree()
	if err != nil {
		return "", "", fmt.Errorf("failed to get target tree: %w", err)
	}

	// Compute diff between trees
	changes, err := baseTree.Diff(targetTree)
	if err != nil {
		return "", "", fmt.Errorf("failed to compute diff: %w", err)
	}

	// Generate diff output
	var summaryBuilder strings.Builder
	var diffBuilder strings.Builder
	var addedCount, modifiedCount, deletedCount int

	// Keep track of file paths by change type
	var addedFiles, deletedFiles, modifiedFiles []string

	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return "", "", fmt.Errorf("failed to get change action: %w", err)
		}

		from, to, err := change.Files()
		if err != nil {
			return "", "", fmt.Errorf("failed to get files: %w", err)
		}

		path := ""
		if from != nil {
			path = from.Name
		} else if to != nil {
			path = to.Name
		}

		switch action {
		case merkletrie.Insert:
			// File added
			addedCount++
			addedFiles = append(addedFiles, path)
			diffBuilder.WriteString(fmt.Sprintf("@ Application added: %s\n", path))

			if to != nil {
				blob, err := targetRepo.BlobObject(to.Hash)
				if err != nil {
					return "", "", fmt.Errorf("failed to get target blob: %w", err)
				}

				content, err := getBlobContent(blob)
				if err != nil {
					return "", "", fmt.Errorf("failed to read target blob: %w", err)
				}

				diffBuilder.WriteString(formatNewFileDiff(content, diffContextLines, diffIgnore))
			}

		case merkletrie.Delete:
			// File deleted
			deletedCount++
			deletedFiles = append(deletedFiles, path)

			diffBuilder.WriteString(fmt.Sprintf("@ Application deleted: %s\n", path))

			if from != nil {
				blob, err := baseRepo.BlobObject(from.Hash)
				if err != nil {
					return "", "", fmt.Errorf("failed to get base blob: %w", err)
				}

				content, err := getBlobContent(blob)
				if err != nil {
					return "", "", fmt.Errorf("failed to read base blob: %w", err)
				}

				diffBuilder.WriteString(formatDeletedFileDiff(content, diffContextLines, diffIgnore))
			}

		case merkletrie.Modify:
			// File modified
			modifiedCount++
			modifiedFiles = append(modifiedFiles, path)

			diffBuilder.WriteString(fmt.Sprintf("@ Application modified: %s\n", path))

			// Get content of both files and use the diff package
			var oldContent, newContent string

			if from != nil {
				blob, err := baseRepo.BlobObject(from.Hash)
				if err != nil {
					return "", "", fmt.Errorf("failed to get base blob: %w", err)
				}

				oldContent, err = getBlobContent(blob)
				if err != nil {
					return "", "", fmt.Errorf("failed to read base blob: %w", err)
				}
			}

			if to != nil {
				blob, err := targetRepo.BlobObject(to.Hash)
				if err != nil {
					return "", "", fmt.Errorf("failed to get target blob: %w", err)
				}

				newContent, err = getBlobContent(blob)
				if err != nil {
					return "", "", fmt.Errorf("failed to read target blob: %w", err)
				}
			}

			// Use diff.Do to generate the diff
			diffBuilder.WriteString(formatModifiedFileDiff(oldContent, newContent, diffContextLines, diffIgnore))
		}
	}

	// Build summary
	totalChanges := addedCount + deletedCount + modifiedCount
	summaryBuilder.WriteString(fmt.Sprintf("Total: %d files changed\n", totalChanges))

	if addedCount > 0 {
		summaryBuilder.WriteString(fmt.Sprintf("\nAdded (%d):\n", addedCount))
		for _, file := range addedFiles {
			summaryBuilder.WriteString(fmt.Sprintf("+ %s\n", file))
		}
	}

	if deletedCount > 0 {
		summaryBuilder.WriteString(fmt.Sprintf("\nDeleted (%d):\n", deletedCount))
		for _, file := range deletedFiles {
			summaryBuilder.WriteString(fmt.Sprintf("- %s\n", file))
		}
	}

	if modifiedCount > 0 {
		summaryBuilder.WriteString(fmt.Sprintf("\nModified (%d):\n", modifiedCount))
		for _, file := range modifiedFiles {
			summaryBuilder.WriteString(fmt.Sprintf("± %s\n", file))
		}
	}

	if totalChanges == 0 {
		return "No changes found", "No changes found", nil
	}

	return summaryBuilder.String(), diffBuilder.String(), nil
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
			if d.Type == diffmatchpatch.DiffDelete {
				prefix = "-"
			} else if d.Type == diffmatchpatch.DiffInsert {
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

// getBlobContent reads the content of a Git blob
func getBlobContent(blob *object.Blob) (string, error) {
	reader, err := blob.Reader()
	if err != nil {
		return "", err
	}
	defer reader.Close()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// copyFilesToRepo copies files from source directory to destination Git repository
func copyFilesToRepo(srcDir, destDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
}

func markdownTemplateLength() int {
	return len(strings.ReplaceAll(
		strings.ReplaceAll(markdownTemplate, "%summary%", ""),
		"%diff%", ""))
}

func printDiff(summary, diff string) string {
	return strings.TrimSpace(strings.ReplaceAll(
		strings.ReplaceAll(markdownTemplate, "%summary%", summary),
		"%diff%", diff)) + "\n"
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
