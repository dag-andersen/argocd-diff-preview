package diff

import "strings"

const markdownTemplate = `
## Argo CD Diff Preview

Summary:
` + "```yaml" + `
%summary%
` + "```" + `

%app_diffs%
`

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
