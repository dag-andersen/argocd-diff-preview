package diff

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

type MarkdownSection struct {
	title   string
	comment string
	content string
}

func markdownSectionHeader(title string) string {
	return fmt.Sprintf("<details>\n<summary>%s</summary>\n<br>\n\n```diff\n", title)
}

func markdownSectionFooter() string {
	return "\n```\n\n</details>\n\n"
}

func (m *MarkdownSection) Size() int {
	return len(markdownSectionHeader(m.title)) + len(m.comment) + len(m.content) + len(markdownSectionFooter())
}

// build returns the section content and a boolean indicating if the section was truncated
func (m *MarkdownSection) build(maxSize int) (string, bool) {
	header := markdownSectionHeader(m.title)
	footer := markdownSectionFooter()
	content := strings.TrimRight(m.content, "\n")

	spaceForContent := maxSize - len(header) - len(footer) - len(m.comment)

	// if there is enough space for the content, return the full section
	if len(content) < spaceForContent {
		return header + m.comment + content + footer, false
	}

	diffTooLongWarning := "\n🚨 Diff is too long"

	spaceBeforeDiffTooLongWarning := spaceForContent - len(diffTooLongWarning)

	minNumberOfCharacters := 100
	if minNumberOfCharacters < spaceBeforeDiffTooLongWarning {
		truncatedContent := content[:spaceBeforeDiffTooLongWarning]
		truncatedContent = strings.TrimRight(truncatedContent, " \t\n\r")
		return header + m.comment + truncatedContent + diffTooLongWarning + footer, true
	}

	// if there is not enough space for the content, return an empty string
	return "", true
}

type MarkdownOutput struct {
	title    string
	summary  string
	sections []MarkdownSection
	infoBox  InfoBox
}

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

func (m *MarkdownOutput) printDiff(maxSize int, maxDiffMessageCharCount uint) string {

	warningMessage := fmt.Sprintf("⚠️⚠️⚠️ Diff exceeds max length of %d characters. Truncating to fit. This can be adjusted with the `--max-diff-length` flag",
		maxDiffMessageCharCount)

	var sectionsDiff strings.Builder

	sizeLeft := maxSize - len(warningMessage)
	AddWarning := false

	for _, section := range m.sections {
		sectionContent, truncated := section.build(sizeLeft)
		sectionsDiff.WriteString(sectionContent)
		if truncated {
			AddWarning = true
		}
		sizeLeft -= len(sectionContent)
	}

	if AddWarning {
		sectionsDiff.WriteString(warningMessage)
	}

	if sectionsDiff.Len() == 0 {
		sectionsDiff.WriteString("No changes found")
	}

	output := strings.ReplaceAll(markdownTemplate, "%title%", m.title)
	output = strings.ReplaceAll(output, "%summary%", strings.TrimSpace(m.summary))
	output = strings.ReplaceAll(output, "%app_diffs%", strings.TrimSpace(sectionsDiff.String()))
	output = strings.ReplaceAll(output, "%info_box%", m.infoBox.String())
	output = strings.TrimSpace(output) + "\n"

	if AddWarning {
		// log warning
		log.Warn().Msgf("🚨 Markdown diff is too long, which exceeds --max-diff-length (%d). Truncating to %d characters. This can be adjusted with the `--max-diff-length` flag", maxDiffMessageCharCount, len(output))
		log.Warn().Msgf("🚨 HTML diff is not affected by this truncation")
	}

	return output
}
