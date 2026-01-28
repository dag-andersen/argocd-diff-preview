package diff

import (
	"fmt"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/app_selector"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
)

// Constants for markdown template
const noAppsFoundTemplate = `
## %title%

%message%
`

// Limits for list truncation
const (
	maxSelectorsToShow    = 20
	maxChangedFilesToShow = 30
)

// WriteNoAppsFoundMessage writes a message to the output folder when no applications are found
func WriteNoAppsFoundMessage(
	title string,
	outputFolder string,
	selectors []app_selector.Selector,
	changedFiles []string,
) error {
	message := getNoAppsFoundMessage(selectors, changedFiles)
	markdown := generateNoAppsFoundMarkdown(title, message)
	markdownPath := fmt.Sprintf("%s/diff.md", outputFolder)
	htmlPath := fmt.Sprintf("%s/diff.html", outputFolder)

	if err := utils.WriteFile(markdownPath, markdown); err != nil {
		return fmt.Errorf("failed to write no apps found message to markdown: %w", err)
	}

	// Write the same content to HTML file for consistency
	if err := utils.WriteFile(htmlPath, markdown); err != nil {
		return fmt.Errorf("failed to write no apps found message to html: %w", err)
	}

	return nil
}

// generateNoAppsFoundMarkdown generates markdown from the message
func generateNoAppsFoundMarkdown(title, message string) string {
	markdown := strings.ReplaceAll(noAppsFoundTemplate, "%title%", title)
	markdown = strings.ReplaceAll(markdown, "%message%", message)
	return strings.TrimSpace(markdown)
}

// getNoAppsFoundMessage generates an appropriate message based on selectors and changed files
func getNoAppsFoundMessage(selectors []app_selector.Selector, changedFiles []string) string {
	switch {
	case len(selectors) > 0 && len(changedFiles) > 0:
		return fmt.Sprintf(
			"Found no changed Applications that matched `%s` and watched these files: `%s`",
			formatSelectors(selectors), formatChangedFiles(changedFiles),
		)
	case len(selectors) > 0:
		return fmt.Sprintf("Found no changed Applications that matched `%s`", formatSelectors(selectors))
	case len(changedFiles) > 0:
		return fmt.Sprintf("Found no changed Applications that watched these files: `%s`", formatChangedFiles(changedFiles))
	default:
		return "Found no Applications"
	}
}

// formatSelectors formats selectors with truncation at maxSelectorsToShow
func formatSelectors(selectors []app_selector.Selector) string {
	if len(selectors) == 0 {
		return ""
	}
	limit := min(len(selectors), maxSelectorsToShow)
	strs := make([]string, limit)
	for i := range limit {
		strs[i] = selectors[i].String()
	}
	result := strings.Join(strs, ", ")
	if len(selectors) > maxSelectorsToShow {
		result += fmt.Sprintf(" [%d more omitted]", len(selectors)-maxSelectorsToShow)
	}
	return result
}

// formatChangedFiles formats changed files with truncation at maxChangedFilesToShow
func formatChangedFiles(files []string) string {
	if len(files) == 0 {
		return ""
	}
	limit := min(len(files), maxChangedFilesToShow)
	result := strings.Join(files[:limit], "`, `")
	if len(files) > maxChangedFilesToShow {
		result += fmt.Sprintf("` [%d more omitted]", len(files)-maxChangedFilesToShow)
	}
	return result
}
