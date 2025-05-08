package diff

import (
	"strings"
)

const markdownTemplate = `
## %title%

Summary:
` + "```yaml" + `
%summary%
` + "```" + `

%app_diffs%

Ran in %execution_time%
`

func markdownTemplateLength() int {
	return len(strings.ReplaceAll(
		strings.ReplaceAll(
			strings.ReplaceAll(
				strings.ReplaceAll(markdownTemplate, "%summary%", ""),
				"%app_diffs%", ""),
			"%title%", ""),
		"%execution_time%", ""))
}

func printDiff(title, summary, diff string, executionTime string) string {
	markdown := strings.ReplaceAll(markdownTemplate, "%title%", title)
	markdown = strings.ReplaceAll(markdown, "%summary%", summary)
	markdown = strings.ReplaceAll(markdown, "%app_diffs%", diff)
	markdown = strings.ReplaceAll(markdown, "%execution_time%", executionTime)
	return strings.TrimSpace(markdown) + "\n"
}
