package diff

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/dag-andersen/argocd-diff-preview/pkg/types"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/sergi/go-diff/diffmatchpatch"
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
	debug bool,
) error {

	// Sort map keys for deterministic processing
	baseKeys := make([]string, 0, len(baseManifests))
	for k := range baseManifests {
		baseKeys = append(baseKeys, k)
	}
	sort.Strings(baseKeys)

	targetKeys := make([]string, 0, len(targetManifests))
	for k := range targetManifests {
		targetKeys = append(targetKeys, k)
	}
	sort.Strings(targetKeys)

	// convert map to string using sorted keys
	baseManifestString := ""
	for _, key := range baseKeys {
		baseManifestString += fmt.Sprintf("# %s\n", key)
		baseManifestString += baseManifests[key]
		baseManifestString += "\n\n"
	}

	targetManifestString := ""
	for _, key := range targetKeys {
		targetManifestString += fmt.Sprintf("# %s\n", key)
		targetManifestString += targetManifests[key]
		targetManifestString += "\n\n"
	}

	if debug {
		// Write base manifests to disk
		basePath := fmt.Sprintf("%s/%s", outputFolder, baseBranch.Type())
		if err := utils.WriteFile(basePath, baseManifestString); err != nil {
			return fmt.Errorf("failed to write base manifest: %w", err)
		}
		targetPath := fmt.Sprintf("%s/%s", outputFolder, targetBranch.Type())
		if err := utils.WriteFile(targetPath, targetManifestString); err != nil {
			return fmt.Errorf("failed to write target manifest: %w", err)
		}
	}

	dmp := diffmatchpatch.New()

	diff := dmp.DiffMain(baseManifestString, targetManifestString, false)
	diffText := dmp.DiffPrettyText(diff)

	log.Info().Msgf("🔮 Diff between %s and %s: %s",
		baseBranch.Name, targetBranch.Name, diffText)

	maxDiffMessageCharCount := maxCharCount
	if maxDiffMessageCharCount == 0 {
		maxDiffMessageCharCount = 65536
	}

	log.Info().Msgf("🔮 Generating diff between %s and %s",
		baseBranch.Name, targetBranch.Name)

	diffAsString := parseDiffOutput(diffText)

	// Handle truncation if needed
	remainingMaxChars := int(maxDiffMessageCharCount) - markdownTemplateLength()
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
	markdown := printDiff("summaryAsString", strings.TrimSpace(diffTruncated))
	markdownPath := fmt.Sprintf("%s/diff.md", outputFolder)
	if err := utils.WriteFile(markdownPath, markdown); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	log.Info().Msgf("🙏 Please check the %s file for differences", markdownPath)
	return nil
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
