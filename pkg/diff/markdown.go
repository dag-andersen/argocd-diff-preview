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

%info_box%
`

func markdownTemplateLength() int {
	template := strings.ReplaceAll(markdownTemplate, "%summary%", "")
	template = strings.ReplaceAll(template, "%app_diffs%", "")
	template = strings.ReplaceAll(template, "%title%", "")
	template = strings.ReplaceAll(template, "%info_box%", "")
	return len(template)
}

func printDiff(title, summary, diff string, infoBox string) string {
	markdown := strings.ReplaceAll(markdownTemplate, "%title%", title)
	markdown = strings.ReplaceAll(markdown, "%summary%", summary)
	markdown = strings.ReplaceAll(markdown, "%app_diffs%", diff)
	markdown = strings.ReplaceAll(markdown, "%info_box%", infoBox)
	return strings.TrimSpace(markdown) + "\n"
}
