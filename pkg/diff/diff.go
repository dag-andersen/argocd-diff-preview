package diff

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/dag-andersen/argocd-diff-preview/pkg/types"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
)

const markdownTemplate = `
## Argo CD Diff Preview

Summary:
` + "```bash" + `
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
	diffIgnore *string,
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

	patternsToIgnore := ""
	if diffIgnore != nil && *diffIgnore != "" {
		patternsToIgnore = fmt.Sprintf("--ignore-matching-lines \"%s\"", *diffIgnore)
	}

	// verify that the output folder exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return fmt.Errorf("base path does not exist: %s", basePath)
	}
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return fmt.Errorf("target path does not exist: %s", targetPath)
	}

	// Get summary diff
	summaryDiffCmd := fmt.Sprintf("git --no-pager diff --compact-summary --no-index %s %s %s",
		patternsToIgnore, baseBranch.Type(), targetBranch.Type())
	summaryOutput, err := gitDiffOutputCommand(outputFolder, summaryDiffCmd)
	if err != nil {
		return fmt.Errorf("failed to get summary diff: %w", err)
	}

	summaryAsString := parseDiffOutput(summaryOutput)

	// Get detailed diff
	if lineCount == 0 {
		lineCount = 10
	}

	diffCmd := fmt.Sprintf("git --no-pager diff --no-prefix -U%d --no-index %s %s %s",
		lineCount, patternsToIgnore, baseBranch.Type(), targetBranch.Type())
	diffOutput, err := gitDiffOutputCommand(outputFolder, diffCmd)
	if err != nil {
		return fmt.Errorf("failed to get detailed diff: %w", err)
	}

	diffAsString := parseDiffOutput(diffOutput)

	// Handle truncation if needed
	remainingMaxChars := int(maxDiffMessageCharCount) - markdownTemplateLength() - len(summaryAsString)
	warningMessage := fmt.Sprintf("\n\n ⚠️⚠️⚠️ Diff is too long. Truncated to %d characters. This can be adjusted with the `--max-diff-length` flag",
		maxDiffMessageCharCount)

	var diffTruncated string
	switch {
	case remainingMaxChars > len(diffAsString):
		diffTruncated = diffAsString // No need to truncate
	case remainingMaxChars > len(warningMessage):
		log.Warn().Msgf("🚨 Diff is too long. Truncating message to %d characters",
			maxDiffMessageCharCount)
		lastDiffChar := remainingMaxChars - len(warningMessage)
		diffTruncated = diffAsString[:lastDiffChar] + warningMessage
	default:
		return fmt.Errorf("diff is too long and cannot be truncated. Increase the max length with `--max-diff-length`")
	}

	// Generate and write markdown
	markdown := printDiff(summaryAsString, strings.TrimSpace(diffTruncated))
	markdownPath := fmt.Sprintf("%s/diff.md", outputFolder)
	if err := utils.WriteFile(markdownPath, markdown); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	log.Info().Msgf("🙏 Please check the %s file for differences", markdownPath)
	return nil
}

// Git diff command that gets the error output of a command
func gitDiffOutputCommand(fromFolder string, cmd string) (string, error) {
	log.Debug().Msgf("Getting diff with command: %s", cmd)
	command := exec.Command("sh", "-c", cmd)
	command.Dir = fromFolder
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	command.Stderr = &stderr
	command.Stdout = &stdout
	err := command.Run()
	if err != nil && strings.TrimSpace(stderr.String()) != "" {
		return "", fmt.Errorf("command failed: %s", stderr.String())
	}
	stdoutString := stdout.String()
	if strings.TrimSpace(stdoutString) == "" {
		return "No changes found", nil
	}
	return strings.TrimRight(stdoutString, "\n"), nil
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

func parseDiffOutput(output string) string {
	if strings.TrimSpace(output) == "" {
		return "No changes found"
	}
	return strings.TrimRight(output, "\n")
}
