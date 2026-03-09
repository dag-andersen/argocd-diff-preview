package diff

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/utils/merkletrie"
)

// buildSummary returns (inlineContent, detailsBlock).
// inlineContent goes in the yaml block; when total > threshold it only contains counts.
// detailsBlock is a collapsible HTML details section with the full file list (empty when below threshold).
// Pass threshold=0 to always return the full list inline.
func buildSummary(changedFiles []Diff, threshold int) (string, string) {
	addedFilesCount := 0
	deletedFilesCount := 0
	modifiedFilesCount := 0

	for _, diff := range changedFiles {
		switch diff.action {
		case merkletrie.Insert:
			addedFilesCount++
		case merkletrie.Delete:
			deletedFilesCount++
		case merkletrie.Modify:
			modifiedFilesCount++
		}
	}

	total := addedFilesCount + deletedFilesCount + modifiedFilesCount

	var listBuilder strings.Builder
	if 0 < addedFilesCount {
		fmt.Fprintf(&listBuilder, "\nAdded (%d):\n", addedFilesCount)
		for _, diff := range changedFiles {
			if diff.action == merkletrie.Insert {
				fmt.Fprintf(&listBuilder, "+ %s%s\n", diff.prettyName(), diff.changeStats())
			}
		}
	}
	if 0 < deletedFilesCount {
		fmt.Fprintf(&listBuilder, "\nDeleted (%d):\n", deletedFilesCount)
		for _, diff := range changedFiles {
			if diff.action == merkletrie.Delete {
				fmt.Fprintf(&listBuilder, "- %s%s\n", diff.prettyName(), diff.changeStats())
			}
		}
	}
	if 0 < modifiedFilesCount {
		fmt.Fprintf(&listBuilder, "\nModified (%d):\n", modifiedFilesCount)
		for _, diff := range changedFiles {
			if diff.action == merkletrie.Modify {
				fmt.Fprintf(&listBuilder, "± %s%s\n", diff.prettyName(), diff.changeStats())
			}
		}
	}

	header := fmt.Sprintf("Total: %d files changed\n", total)

	if threshold > 0 && total > threshold {
		var compact strings.Builder
		fmt.Fprint(&compact, header)
		if 0 < addedFilesCount {
			fmt.Fprintf(&compact, "\nAdded: %d\n", addedFilesCount)
		}
		if 0 < deletedFilesCount {
			fmt.Fprintf(&compact, "Deleted: %d\n", deletedFilesCount)
		}
		if 0 < modifiedFilesCount {
			fmt.Fprintf(&compact, "Modified: %d\n", modifiedFilesCount)
		}
		details := fmt.Sprintf("<details>\n<summary>Changed files (%d)</summary>\n\n```yaml\n%s```\n\n</details>\n", total, listBuilder.String())
		return compact.String(), details
	}

	return header + listBuilder.String(), ""
}
