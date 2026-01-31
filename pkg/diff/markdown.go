package diff

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

type MarkdownSection struct {
	appName  string
	filePath string
	appURL   string
	comment  string
	content  string
}

func markdownSectionHeader(appName, filePath, appURL string) string {
	var header string
	if appURL != "" {
		header = fmt.Sprintf("### %s ([link](%s))\n\n", appName, appURL)
	} else {
		header = fmt.Sprintf("### %s\n\n", appName)
	}
	return header + fmt.Sprintf("File: %s\n\n<details>\n<summary>Details (Click me)</summary>\n<br>\n\n```diff\n", filePath)
}

func markdownSectionFooter() string {
	return "\n```\n\n</details>\n\n"
}

func (m *MarkdownSection) Size() int {
	return len(markdownSectionHeader(m.appName, m.filePath, m.appURL)) + len(m.comment) + len(m.content) + len(markdownSectionFooter())
}

var (
	minSizeForSectionContent = 100
	diffTooLongWarning       = "\n🚨 Diff is too long"
)

// build returns the section content and a boolean indicating if the section was truncated
func (m *MarkdownSection) build(maxSize int) (string, bool) {
	header := markdownSectionHeader(m.appName, m.filePath, m.appURL)
	footer := markdownSectionFooter()
	content := strings.TrimRight(m.content, "\n")

	spaceForContent := maxSize - len(header) - len(footer) - len(m.comment)

	if spaceForContent < 0 {
		log.Debug().Msgf("Markdown - Skipping section, because diff section does not fit in space: %d < 0", spaceForContent)
		return "", true
	}

	// if there is enough space for the content, return the full section
	if len(content) < spaceForContent {
		log.Debug().Msgf("Markdown - Diff section fits in space: %d < %d", spaceForContent, len(content))
		return header + m.comment + content + footer, false
	}

	log.Debug().Msgf("Markdown - diff section does not fit in space: %d < %d", spaceForContent, len(content))

	spaceBeforeDiffTooLongWarning := spaceForContent - len(diffTooLongWarning)

	if minSizeForSectionContent < spaceBeforeDiffTooLongWarning {
		truncatedContent := content[:spaceBeforeDiffTooLongWarning]
		truncatedContent = strings.TrimRight(truncatedContent, " \t\n\r")
		log.Debug().Msgf("Markdown - returning truncated content with warning")
		return header + m.comment + truncatedContent + diffTooLongWarning + footer, true
	}

	log.Debug().Msgf("Markdown - available space is below threashhold %d, returning empty string", minSizeForSectionContent)

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
	infoBoxBufferSize := 80

	warningMessage := fmt.Sprintf("⚠️⚠️⚠️ Diff exceeds max length of %d characters. Truncating to fit. This can be adjusted with the `--max-diff-length` flag",
		maxDiffMessageCharCount)

	availableSpaceForDetailedDiff := int(maxDiffMessageCharCount) - len(output) - len(warningMessage) - infoBoxBufferSize

	log.Debug().Msgf("availableSpaceForDetailedDiff: %d", availableSpaceForDetailedDiff)

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
