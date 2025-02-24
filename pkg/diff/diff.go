package diff

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/utils"
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
	diffIgnore *string,
	lineCount uint,
	maxCharCount uint,
) error {
	maxDiffMessageCharCount := maxCharCount
	if maxDiffMessageCharCount == 0 {
		maxDiffMessageCharCount = 65536
	}

	log.Printf("🔮 Generating diff between %s and %s",
		baseBranch.Name, targetBranch.Name)

	patternsToIgnore := ""
	if diffIgnore != nil && *diffIgnore != "" {
		patternsToIgnore = fmt.Sprintf("--ignore-matching-lines %s", *diffIgnore)
	}

	// Get summary diff
	summaryDiffCmd := fmt.Sprintf("git --no-pager diff --compact-summary --no-index %s %s/%s %s/%s",
		patternsToIgnore, outputFolder, baseBranch.Type(), outputFolder, targetBranch.Type())
	summaryOutput, err := gitDiffOutputCommand(summaryDiffCmd)
	if err != nil {
		return fmt.Errorf("failed to get summary diff: %w", err)
	}

	summaryAsString := parseDiffOutput(summaryOutput)

	// Get detailed diff
	if lineCount == 0 {
		lineCount = 10
	}

	diffCmd := fmt.Sprintf("git --no-pager diff --no-prefix -U%d --no-index %s %s/%s %s/%s",
		lineCount, patternsToIgnore, outputFolder, baseBranch.Type(), outputFolder, targetBranch.Type())
	diffOutput, err := gitDiffOutputCommand(diffCmd)
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
		log.Printf("🚨 Diff is too long. Truncating message to %d characters",
			maxDiffMessageCharCount)
		lastDiffChar := remainingMaxChars - len(warningMessage)
		diffTruncated = diffAsString[:lastDiffChar] + warningMessage
	default:
		return fmt.Errorf("diff is too long and cannot be truncated. Increase the max length with `--max-diff-length`")
	}

	// Generate and write markdown
	markdown := printDiff(summaryAsString, diffTruncated)
	markdownPath := fmt.Sprintf("%s/diff.md", outputFolder)
	if err := utils.WriteFile(markdownPath, markdown); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	log.Printf("🙏 Please check the %s file for differences", markdownPath)
	return nil
}

// Git diff command that gets the error output of a command
func gitDiffOutputCommand(cmd string) (string, error) {
	log.Printf("Getting summary diff with command: %s", cmd)
	command := exec.Command("sh", "-c", cmd)
	log.Printf("Getting summary diff with command: %s", command.String())
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	command.Stderr = &stderr
	command.Stdout = &stdout
	err := command.Run()
	if err != nil && strings.TrimSpace(stderr.String()) != "" {
		return "", fmt.Errorf("command failed: %s", stderr.String())
	}
	s := strings.TrimSpace(stdout.String())
	if s == "" {
		return "No changes found", nil
	}
	return s, nil
}

func markdownTemplateLength() int {
	return len(strings.ReplaceAll(
		strings.ReplaceAll(markdownTemplate, "%summary%", ""),
		"%diff%", ""))
}

func printDiff(summary, diff string) string {
	return strings.TrimSpace(strings.ReplaceAll(
		strings.ReplaceAll(markdownTemplate, "%summary%", summary),
		"%diff%", diff))
}

func parseDiffOutput(output string) string {
	if strings.TrimSpace(output) == "" {
		return "No changes found"
	}
	return strings.TrimRight(output, "\n")
}
