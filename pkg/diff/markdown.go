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
	title         string
	summary       string
	sections      []MarkdownSection
	statsInfo     StatsInfo
	selectionInfo SelectionInfo
}

const markdownTemplate = `
## %title%

Summary:
` + "```yaml" + `
%summary%
` + "```" + `

%app_diffs%
%selection_changes%
%info_box%
`

func (m *MarkdownOutput) printDiff(maxDiffMessageCharCount uint) string {

	output := strings.ReplaceAll(markdownTemplate, "%title%", m.title)
	output = strings.ReplaceAll(output, "%summary%", strings.TrimSpace(m.summary))
	selection_changes := ""
	if s := m.selectionInfo.String(); s != "" {
		selection_changes = fmt.Sprintf("\n%s\n", s)
	}
	output = strings.ReplaceAll(output, "%selection_changes%", selection_changes)

	// the InfoBox has a dynamic size. This is a problem for the integration tests, because the output is not deterministic.
	// By adding a buffer, we ensure availableSpaceForDetailedDiff has a fixed size
	infoBoxBufferSize := 100

	warningMessage := fmt.Sprintf("⚠️⚠️⚠️ Diff exceeds max length of %d characters. Truncating to fit. This can be adjusted with the `--max-diff-length` flag",
		maxDiffMessageCharCount)

	availableSpaceForDetailedDiff := int(maxDiffMessageCharCount) - len(output) - len(warningMessage) - infoBoxBufferSize

	var sectionsDiff strings.Builder

	spaceRemaining := availableSpaceForDetailedDiff
	AddWarning := false

	for _, section := range m.sections {
		if spaceRemaining <= 0 {
			break
		}
		sectionContent, truncated := section.build(spaceRemaining)
		sectionsDiff.WriteString(sectionContent)
		if truncated {
			AddWarning = true
		}
		spaceRemaining -= len(sectionContent)
	}

	if AddWarning {
		sectionsDiff.WriteString(warningMessage)
	}

	if sectionsDiff.Len() == 0 {
		sectionsDiff.WriteString("No changes found")
	}

	output = strings.ReplaceAll(output, "%info_box%", m.statsInfo.String())
	output = strings.ReplaceAll(output, "%app_diffs%", strings.TrimSpace(sectionsDiff.String()))

	output = strings.TrimSpace(output) + "\n"

	if AddWarning {
		// log warning
		log.Warn().Msgf("🚨 Markdown diff is too long, which exceeds --max-diff-length (%d). Truncating to %d characters. This can be adjusted with the `--max-diff-length` flag", maxDiffMessageCharCount, len(output))
		log.Warn().Msgf("🚨 HTML diff is not affected by this truncation")
	}

	return output
}
