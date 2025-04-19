package diff

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/go-git/go-git/v5/utils/merkletrie"
	"github.com/rs/zerolog/log"

	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	gitt "github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
)

const markdownTemplate = `
## Argo CD Diff Preview

Summary:
` + "```yaml" + `
%summary%
` + "```" + `

%app_diffs%
`

// GenerateDiff generates a diff between base and target branches
func GenerateDiff(
	outputFolder string,
	baseBranch *gitt.Branch,
	targetBranch *gitt.Branch,
	baseApps []extract.ExtractedApp,
	targetApps []extract.ExtractedApp,
	diffIgnoreRegex *string,
	lineCount uint,
	maxCharCount uint,
) error {
	// Write base manifests to disk
	basePath := fmt.Sprintf("%s/%s", outputFolder, baseBranch.Type())
	if err := utils.CreateFolder(basePath, true); err != nil {
		return fmt.Errorf("failed to create base folder: %s: %w", basePath, err)
	}

	// Create a map to store source paths for each app
	baseSourcePaths := make(map[string]string)
	for _, app := range baseApps {
		baseSourcePaths[app.Name] = app.SourcePath
		if err := utils.WriteFile(fmt.Sprintf("%s/%s", basePath, app.Name), app.Manifest); err != nil {
			return fmt.Errorf("failed to write base manifest %s: %w", app.Name, err)
		}
	}

	// Write target manifests to disk
	targetPath := fmt.Sprintf("%s/%s", outputFolder, targetBranch.Type())
	if err := utils.CreateFolder(targetPath, true); err != nil {
		return fmt.Errorf("failed to create target folder: %s: %w", targetPath, err)
	}

	// Create a map to store source paths for each app
	targetSourcePaths := make(map[string]string)
	for _, app := range targetApps {
		targetSourcePaths[app.Name] = app.SourcePath
		if err := utils.WriteFile(fmt.Sprintf("%s/%s", targetPath, app.Name), app.Manifest); err != nil {
			return fmt.Errorf("failed to write target manifest %s: %w", app.Name, err)
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
	summary, fileSections, err := generateGitDiff(basePath, targetPath, diffIgnoreRegex, lineCount, baseSourcePaths, targetSourcePaths)
	if err != nil {
		return fmt.Errorf("failed to generate diff: %w", err)
	}

	// Calculate the available space for the file sections
	remainingMaxChars := int(maxDiffMessageCharCount) - markdownTemplateLength() - len(summary)

	// Warning message to be added if we need to truncate
	warningMessage := fmt.Sprintf("\n\n ⚠️⚠️⚠️ Diff is too long. Truncated to %d characters. This can be adjusted with the `--max-diff-length` flag",
		maxDiffMessageCharCount)

	// Concatenate file sections up to the max character limit
	var combinedDiff strings.Builder
	var includedSections int

	// Calculate total size of all sections
	totalSize := 0
	for _, section := range fileSections {
		totalSize += len(section)
	}

	// Check if truncation is needed
	if totalSize <= remainingMaxChars {
		// No truncation needed, include all sections
		for _, section := range fileSections {
			combinedDiff.WriteString(section)
			includedSections++
		}
	} else {
		// Truncation needed
		log.Warn().Msgf("🚨 Diff is too long. Truncating message to %d characters", maxDiffMessageCharCount)

		currentSize := 0
		for i, section := range fileSections {
			// Check if adding this section would exceed the limit (accounting for warning message)
			if currentSize+len(section) > remainingMaxChars-len(warningMessage) {
				// We can't add this full section
				// If this is the first section and empty builder, add a partial section
				if i == 0 && combinedDiff.Len() == 0 {
					// Add as much of the section as possible
					availableSpace := remainingMaxChars - len(warningMessage) - currentSize
					if availableSpace > 0 {
						combinedDiff.WriteString(section[:availableSpace])
					}
				}

				// Add warning and break
				combinedDiff.WriteString(warningMessage)
				break
			}

			// This section fits, add it
			combinedDiff.WriteString(section)
			includedSections++
			currentSize += len(section)
		}
	}

	// Generate and write markdown
	markdown := printDiff(strings.TrimSpace(summary), strings.TrimSpace(combinedDiff.String()))
	markdownPath := fmt.Sprintf("%s/diff.md", outputFolder)
	if err := utils.WriteFile(markdownPath, markdown); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	log.Info().Msgf("🙏 Please check the %s file for differences", markdownPath)
	return nil
}

// generateGitDiff creates temporary Git repositories and uses go-git to generate a diff
func generateGitDiff(
	basePath, targetPath string,
	diffIgnore *string,
	diffContextLines uint,
	baseSourcePaths map[string]string,
	targetSourcePaths map[string]string,
) (string, []string, error) {
	// Create temporary directories for Git repositories
	baseRepoPath, err := os.MkdirTemp("", "base-repo-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir for base repo: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(baseRepoPath); err != nil {
			log.Warn().Err(err).Msg("⚠️ Failed to remove temporary base repo path")
		}
	}()

	targetRepoPath, err := os.MkdirTemp("", "target-repo-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir for target repo: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(targetRepoPath); err != nil {
			log.Warn().Err(err).Msg("⚠️ Failed to remove temporary target repo path")
		}
	}()

	// Initialize Git repositories
	baseRepo, err := git.PlainInit(baseRepoPath, false)
	if err != nil {
		return "", nil, fmt.Errorf("failed to init base repo: %w", err)
	}

	targetRepo, err := git.PlainInit(targetRepoPath, false)
	if err != nil {
		return "", nil, fmt.Errorf("failed to init target repo: %w", err)
	}

	// Copy files to Git repositories
	if err := copyFilesToRepo(basePath, baseRepoPath); err != nil {
		return "", nil, fmt.Errorf("failed to copy base files: %w", err)
	}

	if err := copyFilesToRepo(targetPath, targetRepoPath); err != nil {
		return "", nil, fmt.Errorf("failed to copy target files: %w", err)
	}

	// Add all files and commit in base repo
	baseWorktree, err := baseRepo.Worktree()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get base worktree: %w", err)
	}

	if err := baseWorktree.AddGlob("."); err != nil {
		return "", nil, fmt.Errorf("failed to add files to base repo: %w", err)
	}

	baseCommitHash, err := baseWorktree.Commit("Base state", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "ArgoCD Diff Preview",
			Email: "noreply@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to commit to base repo: %w", err)
	}

	// Add all files and commit in target repo
	targetWorktree, err := targetRepo.Worktree()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get target worktree: %w", err)
	}

	if err := targetWorktree.AddGlob("."); err != nil {
		return "", nil, fmt.Errorf("failed to add files to target repo: %w", err)
	}

	targetCommitHash, err := targetWorktree.Commit("Target state", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "ArgoCD Diff Preview",
			Email: "noreply@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to commit to target repo: %w", err)
	}

	// Retrieve commits
	baseCommit, err := baseRepo.CommitObject(baseCommitHash)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get base commit: %w", err)
	}

	targetCommit, err := targetRepo.CommitObject(targetCommitHash)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get target commit: %w", err)
	}

	// Get base and target trees
	baseTree, err := baseCommit.Tree()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get base tree: %w", err)
	}

	targetTree, err := targetCommit.Tree()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get target tree: %w", err)
	}

	// Compute diff between trees
	changes, err := baseTree.Diff(targetTree)
	if err != nil {
		return "", nil, fmt.Errorf("failed to compute diff: %w", err)
	}

	// Generate diff output
	var summaryBuilder strings.Builder
	var addedCount, modifiedCount, deletedCount int

	// Keep track of file paths by change type
	var addedFiles, deletedFiles, modifiedFiles []string

	// Keep track of old file name (if any)
	oldFileName := make(map[string]string)

	// Map to store individual app diffs
	appDiffs := make(map[string]string)

	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return "", nil, fmt.Errorf("failed to get change action: %w", err)
		}

		from, to, err := change.Files()
		if err != nil {
			return "", nil, fmt.Errorf("failed to get files: %w", err)
		}

		path := ""
		if to != nil {
			path = to.Name
		} else if from != nil {
			path = from.Name
		}

		var appDiffBuilder strings.Builder

		switch action {
		case merkletrie.Insert:
			// File added
			addedCount++
			addedFiles = append(addedFiles, path)
			appDiffBuilder.WriteString(fmt.Sprintf("@@ Application added: %s @@\n", path))

			if to != nil {
				blob, err := targetRepo.BlobObject(to.Hash)
				if err != nil {
					return "", nil, fmt.Errorf("failed to get target blob: %w", err)
				}

				content, err := getBlobContent(blob)
				if err != nil {
					return "", nil, fmt.Errorf("failed to read target blob: %w", err)
				}

				appDiffBuilder.WriteString(formatNewFileDiff(content, diffContextLines, diffIgnore))
			}

			appDiffs[path] = appDiffBuilder.String()

		case merkletrie.Delete:
			// File deleted
			deletedCount++
			deletedFiles = append(deletedFiles, path)
			appDiffBuilder.WriteString(fmt.Sprintf("@@ Application deleted: %s @@\n", path))

			if from != nil {
				blob, err := baseRepo.BlobObject(from.Hash)
				if err != nil {
					return "", nil, fmt.Errorf("failed to get base blob: %w", err)
				}

				content, err := getBlobContent(blob)
				if err != nil {
					return "", nil, fmt.Errorf("failed to read base blob: %w", err)
				}

				appDiffBuilder.WriteString(formatDeletedFileDiff(content, diffContextLines, diffIgnore))
			}

			appDiffs[path] = appDiffBuilder.String()

		case merkletrie.Modify:
			// File modified
			modifiedCount++
			modifiedFiles = append(modifiedFiles, path)

			appDiffBuilder.WriteString(fmt.Sprintf("@@ Application modified: %s @@\n", path))

			// Store old file name for modified file
			if from != nil && from.Name != to.Name {
				log.Debug().Str("from", from.Name).Str("to", to.Name).Msg("Storing old file name")
				oldFileName[path] = from.Name
			}

			// Get content of both files and use the diff package
			var oldContent, newContent string

			if from != nil {
				blob, err := baseRepo.BlobObject(from.Hash)
				if err != nil {
					return "", nil, fmt.Errorf("failed to get base blob: %w", err)
				}

				oldContent, err = getBlobContent(blob)
				if err != nil {
					return "", nil, fmt.Errorf("failed to read base blob: %w", err)
				}
			}

			if to != nil {
				blob, err := targetRepo.BlobObject(to.Hash)
				if err != nil {
					return "", nil, fmt.Errorf("failed to get target blob: %w", err)
				}

				newContent, err = getBlobContent(blob)
				if err != nil {
					return "", nil, fmt.Errorf("failed to read target blob: %w", err)
				}
			}

			// Use diff.Do to generate the diff
			appDiffBuilder.WriteString(formatModifiedFileDiff(oldContent, newContent, diffContextLines, diffIgnore))

			appDiffs[path] = appDiffBuilder.String()
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

	allChanges := append(addedFiles, append(deletedFiles, modifiedFiles...)...)
	log.Debug().Str("allChanges", fmt.Sprintf("%v", allChanges)).Msg("All changes")

	// print base source paths
	log.Debug().Str("baseSourcePaths", fmt.Sprintf("%v", baseSourcePaths)).Msg("Base source paths")

	// print target source paths
	log.Debug().Str("targetSourcePaths", fmt.Sprintf("%v", targetSourcePaths)).Msg("Target source paths")

	// Create array of formatted file sections
	fileSections := make([]string, 0, len(changes))
	for _, file := range allChanges {
		if diff, ok := appDiffs[file]; ok {
			// Get source path for this file, or use empty string if not found
			baseSourcePath := baseSourcePaths[file]
			targetSourcePath := targetSourcePaths[file]
			var filePathPart string

			switch {
			case baseSourcePath != "" && targetSourcePath != "" && baseSourcePath != targetSourcePath:
				filePathPart = fmt.Sprintf("%s -> %s", baseSourcePath, targetSourcePath)
			case baseSourcePath != "":
				filePathPart = baseSourcePath
			case targetSourcePath != "":
				filePathPart = targetSourcePath
			default:
				filePathPart = file
			}

			// Get old file name if it exists
			oldFileName := oldFileName[file]
			var fileNamePart string
			if oldFileName != "" {
				fileNamePart = fmt.Sprintf("%s -> %s", oldFileName, file)
			} else {
				fileNamePart = file
			}

			header := fmt.Sprintf("%s (%s)", fileNamePart, filePathPart)

			diffContent := strings.TrimSpace(diff)

			fileSection := fmt.Sprintf("<details>\n<summary>%s</summary>\n<br>\n\n```diff\n%s\n```\n\n</details>\n\n", header, diffContent)
			fileSections = append(fileSections, fileSection)
		}
	}

	if totalChanges == 0 {
		return "No changes found", []string{"No changes found"}, nil
	}

	return summaryBuilder.String(), fileSections, nil
}

// getBlobContent reads the content of a Git blob
func getBlobContent(blob *object.Blob) (string, error) {
	reader, err := blob.Reader()
	if err != nil {
		return "", err
	}
	defer func() {
		if err := reader.Close(); err != nil {
			log.Warn().Err(err).Msg("⚠️ Failed to close blob reader")
		}
	}()

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
		"%app_diffs%", ""))
}

func printDiff(summary, diff string) string {
	return strings.TrimSpace(strings.ReplaceAll(
		strings.ReplaceAll(markdownTemplate, "%summary%", summary),
		"%app_diffs%", diff)) + "\n"
}
