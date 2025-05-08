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

Rendered %app_count% Applications in %execution_time%
`

func markdownTemplateLength() int {
	template := strings.ReplaceAll(markdownTemplate, "%summary%", "")
	template = strings.ReplaceAll(template, "%app_diffs%", "")
	template = strings.ReplaceAll(template, "%title%", "")
	template = strings.ReplaceAll(template, "%execution_time%", "")
	template = strings.ReplaceAll(template, "%app_count%", "")
	return len(template)
}

func printDiff(title, summary, diff string, executionTime string, appCount string) string {
	markdown := strings.ReplaceAll(markdownTemplate, "%title%", title)
	markdown = strings.ReplaceAll(markdown, "%summary%", summary)
	markdown = strings.ReplaceAll(markdown, "%app_diffs%", diff)
	markdown = strings.ReplaceAll(markdown, "%execution_time%", executionTime)
	markdown = strings.ReplaceAll(markdown, "%app_count%", appCount)
	return strings.TrimSpace(markdown) + "\n"
}
