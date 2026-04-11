// generator.go orchestrates diff generation and delegates to format-specific
// output files (markdown.go, html.go). Helper functions in this file should
// remain format-agnostic - avoid embedding HTML tags or Markdown syntax here.
package diff

import (
	"fmt"
	"strings"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	gitt "github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/matching"
	"github.com/dag-andersen/argocd-diff-preview/pkg/resource_filter"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
)

// GeneratePreview generates a diff using similarity-based matching instead of ID-based matching.
// This correctly handles cases where apps or resources are renamed.
func GeneratePreview(
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
	summaryThreshold uint,
	ignoreResourceRules []resource_filter.IgnoreResourceRule,
) (time.Duration, error) {
	startTime := time.Now()
	maxDiffMessageCharCount := maxCharCount
	if maxDiffMessageCharCount <= 0 {
		maxDiffMessageCharCount = 65536
	}

	log.Info().Msgf("🔮 Generating diff between %s and %s",
		baseBranch.Name, targetBranch.Name)

	// Set default context line count if not provided
	if lineCount <= 0 {
		lineCount = 3
	}

	// Generate diffs using the matching package
	appDiffs, err := matching.GenerateAppDiffs(baseManifests, targetManifests, lineCount, diffIgnoreRegex, ignoreResourceRules)
	if err != nil {
		return time.Since(startTime), fmt.Errorf("failed to generate matching diffs: %w", err)
	}

	// Handle hideDeletedAppDiff option
	if hideDeletedAppDiff {
		for i := range appDiffs {
			if appDiffs[i].Action == matching.ActionDeleted {
				appDiffs[i].Resources = nil
				appDiffs[i].AddedLines = 0
				appDiffs[i].DeletedLines = 0
				appDiffs[i].EmptyReason = matching.EmptyReasonHiddenDiff
			}
		}
	}

	// Build summary
	fullSummary, compactSummary := buildSummary(appDiffs, int(summaryThreshold))

	// Convert to markdown/HTML sections
	markdownSections, htmlSections := buildMatchingSections(appDiffs, argocdUIURL)

	// Markdown
	log.Debug().Msg("Creating markdown output")
	markdownOutput := MarkdownOutput{
		title:          title,
		fullSummary:    fullSummary,
		compactSummary: compactSummary,
		sections:       markdownSections,
		statsInfo:      statsInfo,
		selectionInfo:  selectionInfo,
	}
	markdown := markdownOutput.printDiff(maxDiffMessageCharCount)
	markdownPath := fmt.Sprintf("%s/diff.md", outputFolder)
	log.Debug().Msgf("Writing markdown output to %s", markdownPath)
	if err := utils.WriteFile(markdownPath, markdown); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to write markdown: %w", err)
	}
	log.Debug().Msgf("Wrote markdown output to %s", markdownPath)

	// HTML
	log.Debug().Msg("Creating html output")
	htmlOutput := HTMLOutput{
		title:          title,
		fullSummary:    fullSummary,
		compactSummary: compactSummary,
		sections:       htmlSections,
		statsInfo:      statsInfo,
		selectionInfo:  selectionInfo,
	}
	htmlDiff := htmlOutput.printDiff()
	htmlPath := fmt.Sprintf("%s/diff.html", outputFolder)
	log.Debug().Msgf("Writing html output to %s", htmlPath)
	if err := utils.WriteFile(htmlPath, htmlDiff); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to write html: %w", err)
	}
	log.Debug().Msgf("Wrote html output to %s", htmlPath)

	log.Info().Msgf("🙏 Please check the %s and %s files for differences", markdownPath, htmlPath)

	return time.Since(startTime), nil
}

// buildSummary returns (fullSummary, compactSummary).
// fullSummary is always the full application list with per-app details (plain text).
// compactSummary is a short counts-only summary returned when total > threshold;
// it is empty when the threshold is not exceeded.
// Pass threshold=0 to never produce a compact summary.
func buildSummary(diffs []matching.AppDiff, threshold int) (string, string) {
	if len(diffs) == 0 {
		return "No changes found", ""
	}

	addedCount := 0
	deletedCount := 0
	modifiedCount := 0

	for _, d := range diffs {
		switch d.Action {
		case matching.ActionAdded:
			addedCount++
		case matching.ActionDeleted:
			deletedCount++
		case matching.ActionModified:
			modifiedCount++
		}
	}

	total := addedCount + deletedCount + modifiedCount

	var listBuilder strings.Builder

	if addedCount > 0 {
		fmt.Fprintf(&listBuilder, "Added (%d):\n", addedCount)
		for _, d := range diffs {
			if d.Action == matching.ActionAdded {
				fmt.Fprintf(&listBuilder, "+ %s%s\n", d.PrettyName(), d.ChangeStats())
			}
		}
	}

	if deletedCount > 0 {
		if listBuilder.Len() > 0 {
			fmt.Fprintln(&listBuilder)
		}
		fmt.Fprintf(&listBuilder, "Deleted (%d):\n", deletedCount)
		for _, d := range diffs {
			if d.Action == matching.ActionDeleted {
				fmt.Fprintf(&listBuilder, "- %s%s\n", d.PrettyName(), d.ChangeStats())
			}
		}
	}

	if modifiedCount > 0 {
		if listBuilder.Len() > 0 {
			fmt.Fprintln(&listBuilder)
		}
		fmt.Fprintf(&listBuilder, "Modified (%d):\n", modifiedCount)
		for _, d := range diffs {
			if d.Action == matching.ActionModified {
				fmt.Fprintf(&listBuilder, "± %s%s\n", d.PrettyName(), d.ChangeStats())
			}
		}
	}

	if threshold > 0 && total > threshold {
		var compact strings.Builder
		fmt.Fprintf(&compact, "Total: %d applications changed\n", total)
		if addedCount > 0 {
			fmt.Fprintf(&compact, "Added: %d\n", addedCount)
		}
		if deletedCount > 0 {
			fmt.Fprintf(&compact, "Deleted: %d\n", deletedCount)
		}
		if modifiedCount > 0 {
			fmt.Fprintf(&compact, "Modified: %d\n", modifiedCount)
		}

		return listBuilder.String(), compact.String()
	}

	return listBuilder.String(), ""
}

// buildMatchingSections converts AppDiffs to markdown and HTML sections
func buildMatchingSections(diffs []matching.AppDiff, argocdUIURL string) ([]MarkdownSection, []HTMLSection) {
	markdownSections := make([]MarkdownSection, 0, len(diffs))
	htmlSections := make([]HTMLSection, 0, len(diffs))

	for _, d := range diffs {
		appURL := buildAppURLFromDiff(d, argocdUIURL)

		// Convert matching.ResourceDiff → diff.ResourceSection
		sections := make([]ResourceSection, len(d.Resources))
		for i, r := range d.Resources {
			sections[i] = ResourceSection{
				Header:    r.Header(),
				Content:   r.Content,
				IsSkipped: r.IsSkipped,
			}
		}

		markdownSections = append(markdownSections, MarkdownSection{
			appName:     d.PrettyName(),
			filePath:    d.PrettyPath(),
			appURL:      appURL,
			resources:   sections,
			emptyReason: d.EmptyReason,
		})

		htmlSections = append(htmlSections, HTMLSection{
			appName:     d.PrettyName(),
			filePath:    d.PrettyPath(),
			appURL:      appURL,
			resources:   sections,
			emptyReason: d.EmptyReason,
		})
	}

	return markdownSections, htmlSections
}

// buildAppURLFromDiff builds the ArgoCD UI URL for an app diff
func buildAppURLFromDiff(d matching.AppDiff, argocdUIURL string) string {
	if argocdUIURL == "" {
		return ""
	}

	appName := d.OldName
	if appName == "" {
		appName = d.NewName
	}

	if appName == "" {
		return ""
	}

	baseURL := strings.TrimRight(argocdUIURL, "/")
	return fmt.Sprintf("%s/applications/%s", baseURL, appName)
}
