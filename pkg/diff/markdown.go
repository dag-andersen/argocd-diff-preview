package diff

import "strings"

const markdownTemplate = `
## %title%

Summary:
` + "```yaml" + `
%summary%
` + "```" + `

%app_diffs%
`

func markdownTemplateLength() int {
	return len(strings.ReplaceAll(
		strings.ReplaceAll(
			strings.ReplaceAll(markdownTemplate, "%summary%", ""),
			"%app_diffs%", ""),
		"%title%", ""))
}

func printDiff(title, summary, diff string) string {
	markdown := strings.ReplaceAll(markdownTemplate, "%title%", title)
	markdown = strings.ReplaceAll(markdown, "%summary%", summary)
	markdown = strings.ReplaceAll(markdown, "%app_diffs%", diff)
	return strings.TrimSpace(markdown) + "\n"
}
