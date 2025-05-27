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

	gitt "github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
)

type AppInfo struct {
	Id          string
	Name        string
	SourcePath  string
	FileContent string
}

// GenerateDiff generates a diff between base and target branches
func GenerateDiff(
	title string,
	outputFolder string,
	baseBranch *gitt.Branch,
	targetBranch *gitt.Branch,
	baseApps []AppInfo,
	targetApps []AppInfo,
	diffIgnoreRegex *string,
	lineCount uint,
	maxCharCount uint,
	timeInfo InfoBox,
	diffFormat string,
) error {

	maxDiffMessageCharCount := maxCharCount
	if maxDiffMessageCharCount == 0 {
		maxDiffMessageCharCount = 65536
	}

	log.Info().Msgf("🔮 Generating diff between %s and %s",
		baseBranch.Name, targetBranch.Name)

	// Set default context line count if not provided
	if lineCount <= 0 {
		lineCount = 3 // Default to 3 context lines if not specified
	}

	// Set default context line count if not provided
	if lineCount == 0 {
		lineCount = 10
	}

	// Generate diffs using go-git by creating temporary git repos
	basePath := fmt.Sprintf("%s/%s", outputFolder, baseBranch.Type())
	targetPath := fmt.Sprintf("%s/%s", outputFolder, targetBranch.Type())
	summary, fileSections, err := generateGitDiff(basePath, targetPath, diffIgnoreRegex, lineCount, baseApps, targetApps, diffFormat)
	if err != nil {
		return fmt.Errorf("failed to generate diff: %w", err)
	}

	infoBoxString := timeInfo.String()

	// Calculate the available space for the file sections
	remainingMaxChars := int(maxDiffMessageCharCount) - markdownTemplateLength() - len(summary) - len(infoBoxString) - len(title)

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

	var diffPath string
	switch diffFormat {
	case "html":
		// Generate HTML
		htmlDiff := printHTMLDiff(
			title,
			strings.TrimSpace(summary),
			strings.TrimSpace(combinedDiff.String()),
			infoBoxString,
		)
		diffPath = fmt.Sprintf("%s/diff.html", outputFolder)
		if err := utils.WriteFile(diffPath, htmlDiff); err != nil {
			return fmt.Errorf("failed to write html: %w", err)
		}
	default:
		// Generate and write markdown
		markdown := printMarkdownDiff(
			title,
			strings.TrimSpace(summary),
			strings.TrimSpace(combinedDiff.String()),
			infoBoxString,
		)
		diffPath = fmt.Sprintf("%s/diff.md", outputFolder)
		if err := utils.WriteFile(diffPath, markdown); err != nil {
			return fmt.Errorf("failed to write markdown: %w", err)
		}
	}

	log.Info().Msgf("🙏 Please check the %s file for differences", diffPath)
	return nil
}

func writeManifestsToDisk(apps []AppInfo, folder string) error {
	if err := utils.CreateFolder(folder, true); err != nil {
		return fmt.Errorf("failed to create folder: %s: %w", folder, err)
	}
	for _, app := range apps {
		if err := utils.WriteFile(fmt.Sprintf("%s/%s", folder, app.Id), app.FileContent); err != nil {
			return fmt.Errorf("failed to write manifest %s: %w", app.Id, err)
		}
	}
	return nil
}

// generateGitDiff creates temporary Git repositories and uses go-git to generate a diff
func generateGitDiff(
	basePath, targetPath string,
	diffIgnore *string,
	diffContextLines uint,
	baseApps []AppInfo,
	targetApps []AppInfo,
	diffFormat string,
) (string, []string, error) {

	// Write base manifests to disk
	if err := writeManifestsToDisk(baseApps, basePath); err != nil {
		return "", nil, fmt.Errorf("failed to write base manifests: %w", err)
	}

	// Write target manifests to disk
	if err := writeManifestsToDisk(targetApps, targetPath); err != nil {
		return "", nil, fmt.Errorf("failed to write target manifests: %w", err)
	}

	baseAppsMap := make(map[string]AppInfo)
	for _, app := range baseApps {
		baseAppsMap[app.Id] = app
	}

	targetAppsMap := make(map[string]AppInfo)
	for _, app := range targetApps {
		targetAppsMap[app.Id] = app
	}

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
		AllowEmptyCommits: true,
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
		AllowEmptyCommits: true,
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

	// Keep track of file paths by change type
	var changedFiles []Diff

	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return "", nil, fmt.Errorf("failed to get change action: %w", err)
		}

		from, to, err := change.Files()
		if err != nil {
			return "", nil, fmt.Errorf("failed to get files: %w", err)
		}

		diffContent := ""

		switch action {
		case merkletrie.Insert:

			if to != nil {
				blob, err := targetRepo.BlobObject(to.Hash)
				if err != nil {
					return "", nil, fmt.Errorf("failed to get target blob: %w", err)
				}

				content, err := getBlobContent(blob)
				if err != nil {
					return "", nil, fmt.Errorf("failed to read target blob: %w", err)
				}

				diffContent = formatNewFileDiff(content, diffContextLines, diffIgnore)
			}

		case merkletrie.Delete:

			if from != nil {
				blob, err := baseRepo.BlobObject(from.Hash)
				if err != nil {
					return "", nil, fmt.Errorf("failed to get base blob: %w", err)
				}

				content, err := getBlobContent(blob)
				if err != nil {
					return "", nil, fmt.Errorf("failed to read base blob: %w", err)
				}

				diffContent = formatDeletedFileDiff(content, diffContextLines, diffIgnore)
			}

		case merkletrie.Modify:

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
			diffContent = formatModifiedFileDiff(oldContent, newContent, diffContextLines, diffIgnore)
		}

		toName := ""
		fromName := ""
		if to != nil {
			toName = to.Name
		}
		if from != nil {
			fromName = from.Name
		}

		diff := Diff{
			newName:       targetAppsMap[toName].Name,
			oldName:       baseAppsMap[fromName].Name,
			newSourcePath: targetAppsMap[toName].SourcePath,
			oldSourcePath: baseAppsMap[fromName].SourcePath,
			action:        action,
			content:       diffContent,
		}

		// If the diff didn't change and the names are the same, skip it
		if diff.content == "" && diff.oldName == diff.newName && diff.oldSourcePath == diff.newSourcePath {
			continue
		}

		// print diff
		log.Debug().
			Str("newName", diff.newName).
			Str("oldName", diff.oldName).
			Str("newSourcePath", diff.newSourcePath).
			Str("oldSourcePath", diff.oldSourcePath).
			Str("action", diff.action.String()).
			Msg("Found diff")

		changedFiles = append(changedFiles, diff)
	}

	// Build summary
	summary := buildSummary(changedFiles)

	// Create array of formatted file sections
	fileSections := make([]string, 0, len(changedFiles))
	for _, diff := range changedFiles {

		// skips empty diffs
		if diff.content == "" {
			continue
		}

		// Get source path for this file, or use empty string if not found
		var fileSection string
		switch (diffFormat) {
		case "html":
			fileSection = diff.buildHTMLSection()
		default:
			fileSection = diff.buildSection()
		}
		fileSections = append(fileSections, fileSection)
	}

	if len(changedFiles) == 0 {
		return "No changes found", []string{"No changes found"}, nil
	}

	return summary, fileSections, nil
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
