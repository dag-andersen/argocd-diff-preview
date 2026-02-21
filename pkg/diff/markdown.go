package diff

import (
	"fmt"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/matching"
	"github.com/rs/zerolog/log"
)

type MarkdownSection struct {
	appName     string
	filePath    string
	appURL      string
	resources   []ResourceSection
	emptyReason matching.EmptyReason
}

// emptyReasonMarkdown returns the markdown-formatted message for an EmptyReason
func emptyReasonMarkdown(reason matching.EmptyReason) string {
	switch reason {
	case matching.EmptyReasonNoResources:
		return "_Application rendered no resources_"
	case matching.EmptyReasonHiddenDiff:
		return "_Diff hidden because `--hide-deleted-app-diff` is enabled_"
	default:
		return ""
	}
}

func markdownSectionHeader(appName, filePath, appURL string) string {
	var summary string
	if appURL != "" {
		summary = fmt.Sprintf("%s [<a href=\"%s\">link</a>] (%s)", appName, appURL, filePath)
	} else {
		summary = fmt.Sprintf("%s (%s)", appName, filePath)
	}
	return fmt.Sprintf("<details>\n<summary>%s</summary>\n<br>\n\n", summary)
}

func markdownSectionFooter() string {
	return "</details>\n\n"
}

var (
	minSizeForSectionContent = 100
	diffTooLongWarning       = "\n🚨 Diff is too long"
)

// build returns the section content and a boolean indicating if the section was truncated
func (m *MarkdownSection) build(maxSize int) (string, bool) {
	header := markdownSectionHeader(m.appName, m.filePath, m.appURL)
	footer := markdownSectionFooter()

	var body strings.Builder

	if len(m.resources) == 0 {
		body.WriteString(emptyReasonMarkdown(m.emptyReason))
		body.WriteString("\n\n")
	} else {
		for _, r := range m.resources {
			if r.IsSkipped {
				fmt.Fprintf(&body, "#### %s\n\n_Skipped_\n\n", r.Header)
			} else {
				content := strings.TrimRight(r.Content, "\n")
				fmt.Fprintf(&body, "#### %s\n```diff\n%s\n```\n", r.Header, content)
			}
		}
	}

	totalSize := len(header) + body.Len() + len(footer)

	if totalSize <= maxSize {
		log.Debug().Msgf("Markdown - Diff section fits in space: %d <= %d", totalSize, maxSize)
		return header + body.String() + footer, false
	}

	// Truncation: include full resources until budget is exhausted
	log.Debug().Msgf("Markdown - diff section does not fit in space: %d > %d", totalSize, maxSize)

	spaceForBody := maxSize - len(header) - len(footer) - len(diffTooLongWarning) - 1 // -1 for the "\n" after warning

	if spaceForBody < minSizeForSectionContent {
		log.Debug().Msgf("Markdown - available space is below threshold %d, returning empty string", minSizeForSectionContent)
		return "", true
	}

	var truncatedBody strings.Builder
	for _, r := range m.resources {
		var part string
		if r.IsSkipped {
			part = fmt.Sprintf("#### %s\n\n_Skipped_\n\n", r.Header)
		} else {
			content := strings.TrimRight(r.Content, "\n")
			part = fmt.Sprintf("#### %s\n```diff\n%s\n```\n", r.Header, content)
		}

		// If the full resource fits, include it and continue
		if truncatedBody.Len()+len(part) <= spaceForBody {
			truncatedBody.WriteString(part)
			continue
		}

		// Resource doesn't fit — try to include a truncated version.
		// Skipped sections are small and not worth partially including.
		if r.IsSkipped {
			break
		}

		// Split into header/content/footer so we can truncate only the content
		// while structurally guaranteeing the code fence closes
		resHeader := fmt.Sprintf("#### %s\n```diff\n", r.Header)
		resFooter := "\n```\n"
		remaining := spaceForBody - truncatedBody.Len() - len(resHeader) - len(resFooter)
		if remaining > minSizeForSectionContent {
			content := strings.TrimRight(r.Content, "\n")
			truncatedContent := content[:min(remaining, len(content))]
			truncatedContent = strings.TrimRight(truncatedContent, " \t\n\r")
			truncatedBody.WriteString(resHeader)
			truncatedBody.WriteString(truncatedContent)
			truncatedBody.WriteString(resFooter)
		}
		break
	}

	log.Debug().Msgf("Markdown - returning truncated content with warning")
	return header + truncatedBody.String() + diffTooLongWarning + "\n" + footer, true
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
