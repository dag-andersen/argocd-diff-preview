package diff

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
) error {

	maxDiffMessageCharCount := maxCharCount
	if maxDiffMessageCharCount <= 0 {
		maxDiffMessageCharCount = 65536
	}

	log.Info().Msgf("🔮 Generating diff between %s and %s",
		baseBranch.Name, targetBranch.Name)

	// Set default context line count if not provided
	if lineCount <= 0 {
		lineCount = 3 // Default to 3 context lines if not specified
	}

	// Generate diffs using go-git by creating temporary git repos
	basePath := fmt.Sprintf("%s/%s", outputFolder, baseBranch.Type())
	targetPath := fmt.Sprintf("%s/%s", outputFolder, targetBranch.Type())
	summary, markdownFileSections, htmlFileSections, err := generateGitDiff(basePath, targetPath, diffIgnoreRegex, lineCount, baseApps, targetApps)
	if err != nil {
		return fmt.Errorf("failed to generate diff: %w", err)
	}

	infoBoxString := timeInfo.String()

	// Calculate the available space for the file sections
	availableSpaceAfterOverhead := int(maxDiffMessageCharCount) - markdownTemplateLength() - len(summary) - len(infoBoxString) - len(title)
	if availableSpaceAfterOverhead < 0 {
		availableSpaceAfterOverhead = 0
	}

	// Markdown
	MarkdownOutput := MarkdownOutput{
		title:    title,
		summary:  summary,
		sections: markdownFileSections,
		infoBox:  timeInfo,
	}
	markdown := MarkdownOutput.printDiff(availableSpaceAfterOverhead, maxDiffMessageCharCount)
	markdownPath := fmt.Sprintf("%s/diff.md", outputFolder)
	if err := utils.WriteFile(markdownPath, markdown); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)

	}

	// HTML
	HTMLOutput := HTMLOutput{
		title:    title,
		summary:  summary,
		sections: htmlFileSections,
		infoBox:  timeInfo,
	}
	htmlDiff := HTMLOutput.printDiff()
	htmlPath := fmt.Sprintf("%s/diff.html", outputFolder)
	if err := utils.WriteFile(htmlPath, htmlDiff); err != nil {
		return fmt.Errorf("failed to write html: %w", err)
	}

	log.Info().Msgf("🙏 Please check the %s and %s files for differences", markdownPath, htmlPath)
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
) (string, []MarkdownSection, []HTMLSection, error) {

	// Write base manifests to disk
	if err := writeManifestsToDisk(baseApps, basePath); err != nil {
		return "", nil, nil, fmt.Errorf("failed to write base manifests: %w", err)
	}

	// Write target manifests to disk
	if err := writeManifestsToDisk(targetApps, targetPath); err != nil {
		return "", nil, nil, fmt.Errorf("failed to write target manifests: %w", err)
	}

	baseAppsMap := make(map[string]AppInfo)
	for _, app := range baseApps {
		baseAppsMap[app.Id] = app
	}

	targetAppsMap := make(map[string]AppInfo)
	for _, app := range targetApps {
		targetAppsMap[app.Id] = app
	}

	// Create temporary directory for single Git repository
	repoPath, err := os.MkdirTemp("", "diff-repo-*")
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to create temp dir for repo: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(repoPath); err != nil {
			log.Warn().Err(err).Msg("⚠️ Failed to remove temporary repo path")
		}
	}()

	// Initialize single Git repository
	repo, err := git.PlainInit(repoPath, false)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to init repo: %w", err)
	}

	// Get worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	// Copy base files to repository and commit
	if err := copyFilesToRepo(basePath, repoPath); err != nil {
		return "", nil, nil, fmt.Errorf("failed to copy base files: %w", err)
	}

	if err := worktree.AddGlob("."); err != nil {
		return "", nil, nil, fmt.Errorf("failed to add base files to repo: %w", err)
	}

	author := &object.Signature{
		Name:  "ArgoCD Diff Preview",
		Email: "noreply@example.com",
		When:  time.Now(),
	}

	baseCommitHash, err := worktree.Commit("Base state", &git.CommitOptions{
		Author:            author,
		AllowEmptyCommits: true,
	})
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to commit base state: %w", err)
	}

	// Clear the working directory and copy target files
	if err := clearWorkingDirectory(repoPath); err != nil {
		return "", nil, nil, fmt.Errorf("failed to clear working directory: %w", err)
	}

	if err := copyFilesToRepo(targetPath, repoPath); err != nil {
		return "", nil, nil, fmt.Errorf("failed to copy target files: %w", err)
	}

	if err := worktree.AddGlob("."); err != nil {
		return "", nil, nil, fmt.Errorf("failed to add target files to repo: %w", err)
	}

	targetCommitHash, err := worktree.Commit("Target state", &git.CommitOptions{
		Author:            author,
		AllowEmptyCommits: true,
	})
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to commit target state: %w", err)
	}

	// Retrieve commits
	baseCommit, err := repo.CommitObject(baseCommitHash)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to get base commit: %w", err)
	}

	targetCommit, err := repo.CommitObject(targetCommitHash)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to get target commit: %w", err)
	}

	// Get base and target trees
	baseTree, err := baseCommit.Tree()
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to get base tree: %w", err)
	}

	targetTree, err := targetCommit.Tree()
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to get target tree: %w", err)
	}

	// Compute diff between trees
	changes, err := baseTree.Diff(targetTree)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to compute diff: %w", err)
	}

	// Keep track of file paths by change type
	var changedFiles []Diff

	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to get change action: %w", err)
		}

		from, to, err := change.Files()
		if err != nil {
			return "", nil, nil, fmt.Errorf("failed to get files: %w", err)
		}

		diffContent := ""

		switch action {
		case merkletrie.Insert:

			if to != nil {
				blob, err := repo.BlobObject(to.Hash)
				if err != nil {
					return "", nil, nil, fmt.Errorf("failed to get target blob: %w", err)
				}

				content, err := getBlobContent(blob)
				if err != nil {
					return "", nil, nil, fmt.Errorf("failed to read target blob: %w", err)
				}

				diffContent = formatNewFileDiff(content, diffContextLines, diffIgnore)
			}

		case merkletrie.Delete:

			if from != nil {
				blob, err := repo.BlobObject(from.Hash)
				if err != nil {
					return "", nil, nil, fmt.Errorf("failed to get base blob: %w", err)
				}

				content, err := getBlobContent(blob)
				if err != nil {
					return "", nil, nil, fmt.Errorf("failed to read base blob: %w", err)
				}

				diffContent = formatDeletedFileDiff(content, diffContextLines, diffIgnore)
			}

		case merkletrie.Modify:

			// Get content of both files and use the diff package
			var oldContent, newContent string

			if from != nil {
				blob, err := repo.BlobObject(from.Hash)
				if err != nil {
					return "", nil, nil, fmt.Errorf("failed to get base blob: %w", err)
				}

				oldContent, err = getBlobContent(blob)
				if err != nil {
					return "", nil, nil, fmt.Errorf("failed to read base blob: %w", err)
				}
			}

			if to != nil {
				blob, err := repo.BlobObject(to.Hash)
				if err != nil {
					return "", nil, nil, fmt.Errorf("failed to get target blob: %w", err)
				}

				newContent, err = getBlobContent(blob)
				if err != nil {
					return "", nil, nil, fmt.Errorf("failed to read target blob: %w", err)
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

	if len(changedFiles) == 0 {
		return "No changes found", nil, nil, nil
	}

	// Build summary
	summary := buildSummary(changedFiles)

	// Create arrays of formatted file sections
	markdownFileSections := make([]MarkdownSection, 0, len(changedFiles))
	htmlFileSections := make([]HTMLSection, 0, len(changedFiles))
	for _, diff := range changedFiles {

		// skips empty diffs
		if diff.content == "" {
			continue
		}

		// Get source path for this file, or use empty string if not found
		markdownFileSections = append(markdownFileSections, diff.buildMarkdownSection())
		htmlFileSections = append(htmlFileSections, diff.buildHTMLSection())
	}

	return summary, markdownFileSections, htmlFileSections, nil
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

// clearWorkingDirectory removes all files and directories from the given path, but keeps the directory itself
func clearWorkingDirectory(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Skip .git directory to preserve Git repository structure
		if entry.Name() == ".git" {
			continue
		}

		entryPath := filepath.Join(path, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return err
		}
	}

	return nil
}
