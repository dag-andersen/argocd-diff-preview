package diff

import (
	"fmt"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	gitt "github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/matching"
	"github.com/dag-andersen/argocd-diff-preview/pkg/resource_filter"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
)

// GenerateMatchingDiff generates a diff using similarity-based matching instead of ID-based matching.
// This correctly handles cases where apps or resources are renamed.
func GenerateMatchingDiff(
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
) error {
	maxDiffMessageCharCount := maxCharCount
	if maxDiffMessageCharCount <= 0 {
		maxDiffMessageCharCount = 65536
	}

	log.Info().Msgf("🔮 Generating diff between %s and %s (using similarity matching)",
		baseBranch.Name, targetBranch.Name)

	// Set default context line count if not provided
	if lineCount <= 0 {
		lineCount = 3
	}

	// Generate diffs using the matching package
	appDiffs, err := matching.GenerateAppDiffs(baseManifests, targetManifests, lineCount, diffIgnoreRegex, ignoreResourceRules)
	if err != nil {
		return fmt.Errorf("failed to generate matching diffs: %w", err)
	}

	// Handle hideDeletedAppDiff option
	if hideDeletedAppDiff {
		for i := range appDiffs {
			if appDiffs[i].Action == matching.ActionDeleted {
				appDiffs[i].Resources = nil
				appDiffs[i].AddedLines = 0
				appDiffs[i].DeletedLines = 0
			}
		}
	}

	// Build summary
	summary := buildMatchingSummary(appDiffs)

	// Convert to markdown/HTML sections
	markdownSections, htmlSections := buildMatchingSections(appDiffs, argocdUIURL)

	// Write YAML outputs for base and target branches
	if err := writeMatchingYAMLOutputs(outputFolder, baseBranch, targetBranch, baseManifests, targetManifests, ignoreResourceRules); err != nil {
		return err
	}

	// Markdown output
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

	// HTML output
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

// buildMatchingSummary builds a summary string from AppDiffs
func buildMatchingSummary(diffs []matching.AppDiff) string {
	if len(diffs) == 0 {
		return "No changes found"
	}

	var summaryBuilder strings.Builder

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

	fmt.Fprintf(&summaryBuilder, "Total: %d files changed\n", addedCount+deletedCount+modifiedCount)

	if addedCount > 0 {
		fmt.Fprintf(&summaryBuilder, "\nAdded (%d):\n", addedCount)
		for _, d := range diffs {
			if d.Action == matching.ActionAdded {
				fmt.Fprintf(&summaryBuilder, "+ %s%s\n", d.PrettyName(), d.ChangeStats())
			}
		}
	}

	if deletedCount > 0 {
		fmt.Fprintf(&summaryBuilder, "\nDeleted (%d):\n", deletedCount)
		for _, d := range diffs {
			if d.Action == matching.ActionDeleted {
				fmt.Fprintf(&summaryBuilder, "- %s%s\n", d.PrettyName(), d.ChangeStats())
			}
		}
	}

	if modifiedCount > 0 {
		fmt.Fprintf(&summaryBuilder, "\nModified (%d):\n", modifiedCount)
		for _, d := range diffs {
			if d.Action == matching.ActionModified {
				fmt.Fprintf(&summaryBuilder, "± %s%s\n", d.PrettyName(), d.ChangeStats())
			}
		}
	}

	return summaryBuilder.String()
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
			appName:   d.PrettyName(),
			filePath:  d.PrettyPath(),
			appURL:    appURL,
			resources: sections,
		})

		htmlSections = append(htmlSections, HTMLSection{
			appName:   d.PrettyName(),
			filePath:  d.PrettyPath(),
			appURL:    appURL,
			resources: sections,
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

// writeMatchingYAMLOutputs writes the combined YAML outputs for both branches
func writeMatchingYAMLOutputs(
	outputFolder string,
	baseBranch, targetBranch *gitt.Branch,
	baseManifests, targetManifests []extract.ExtractedApp,
	ignoreResourceRules []resource_filter.IgnoreResourceRule,
) error {
	// Collect YAML from base manifests
	var baseYAMLParts []string
	for _, app := range baseManifests {
		yaml, err := app.FlattenToString(ignoreResourceRules)
		if err != nil {
			return fmt.Errorf("failed to flatten base app %s: %w", app.Name, err)
		}
		if yaml != "" {
			baseYAMLParts = append(baseYAMLParts, yaml)
		}
	}

	// Collect YAML from target manifests
	var targetYAMLParts []string
	for _, app := range targetManifests {
		yaml, err := app.FlattenToString(ignoreResourceRules)
		if err != nil {
			return fmt.Errorf("failed to flatten target app %s: %w", app.Name, err)
		}
		if yaml != "" {
			targetYAMLParts = append(targetYAMLParts, yaml)
		}
	}

	// Write base YAML
	basePath := fmt.Sprintf("%s/%s.yaml", outputFolder, baseBranch.FolderName())
	if err := utils.WriteFile(basePath, strings.Join(baseYAMLParts, "\n---\n")); err != nil {
		log.Error().Msg("❌ Failed to write base manifests")
		return err
	}

	// Write target YAML
	targetPath := fmt.Sprintf("%s/%s.yaml", outputFolder, targetBranch.FolderName())
	if err := utils.WriteFile(targetPath, strings.Join(targetYAMLParts, "\n---\n")); err != nil {
		log.Error().Msg("❌ Failed to write target manifests")
		return err
	}

	return nil
}
