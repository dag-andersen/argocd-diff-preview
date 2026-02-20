package diff

import (
	"fmt"

	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	gitt "github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/resource_filter"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
)

// DiffGeneratorFunc is the shared function signature for diff generators.
// Both GenerateDiff and GenerateMatchingDiff implement this signature.
type DiffGeneratorFunc func(
	title string,
	outputFolder string,
	baseBranch *gitt.Branch,
	targetBranch *gitt.Branch,
	baseManifests []extract.ExtractedApp,
	targetManifests []extract.ExtractedApp,
	diffIgnoreRegex *string,
	lineCount uint,
	maxCharCount uint,
	hideDeletedAppDiff bool,
	statsInfo StatsInfo,
	selectionInfo SelectionInfo,
	argocdUIURL string,
	ignoreResourceRules []resource_filter.IgnoreResourceRule,
) error

// writeOutputs writes the markdown and HTML diff outputs to disk.
// This is the shared output-writing tail used by both diff generators.
func writeOutputs(
	outputFolder string,
	title string,
	summary string,
	markdownSections []MarkdownSection,
	htmlSections []HTMLSection,
	statsInfo StatsInfo,
	selectionInfo SelectionInfo,
	maxDiffMessageCharCount uint,
) error {
	// Markdown
	log.Debug().Msg("Creating markdown output")
	markdownOutput := MarkdownOutput{
		title:         title,
		summary:       summary,
		sections:      markdownSections,
		statsInfo:     statsInfo,
		selectionInfo: selectionInfo,
	}
	markdown := markdownOutput.printDiff(maxDiffMessageCharCount)
	markdownPath := fmt.Sprintf("%s/diff.md", outputFolder)
	log.Debug().Msgf("Writing markdown output to %s", markdownPath)
	if err := utils.WriteFile(markdownPath, markdown); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}
	log.Debug().Msgf("Wrote markdown output to %s", markdownPath)

	// HTML
	log.Debug().Msg("Creating html output")
	htmlOutput := HTMLOutput{
		title:         title,
		summary:       summary,
		sections:      htmlSections,
		statsInfo:     statsInfo,
		selectionInfo: selectionInfo,
	}
	htmlDiff := htmlOutput.printDiff()
	htmlPath := fmt.Sprintf("%s/diff.html", outputFolder)
	log.Debug().Msgf("Writing html output to %s", htmlPath)
	if err := utils.WriteFile(htmlPath, htmlDiff); err != nil {
		return fmt.Errorf("failed to write html: %w", err)
	}
	log.Debug().Msgf("Wrote html output to %s", htmlPath)

	log.Info().Msgf("🙏 Please check the %s and %s files for differences", markdownPath, htmlPath)
	return nil
}
