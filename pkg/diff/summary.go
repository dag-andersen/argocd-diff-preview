package diff

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/utils/merkletrie"
)

func buildSummary(changedFiles []Diff) string {
	var summaryBuilder strings.Builder

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

	fmt.Fprintf(&summaryBuilder, "Total: %d files changed\n", addedFilesCount+deletedFilesCount+modifiedFilesCount)

	if 0 < addedFilesCount {
		fmt.Fprintf(&summaryBuilder, "\nAdded (%d):\n", addedFilesCount)
		for _, diff := range changedFiles {
			if diff.action == merkletrie.Insert {
				fmt.Fprintf(&summaryBuilder, "+ %s%s\n", diff.prettyName(), diff.changeStats())
			}
		}
	}

	if 0 < deletedFilesCount {
		fmt.Fprintf(&summaryBuilder, "\nDeleted (%d):\n", deletedFilesCount)
		for _, diff := range changedFiles {
			if diff.action == merkletrie.Delete {
				fmt.Fprintf(&summaryBuilder, "- %s%s\n", diff.prettyName(), diff.changeStats())
			}
		}
	}

	if 0 < modifiedFilesCount {
		fmt.Fprintf(&summaryBuilder, "\nModified (%d):\n", modifiedFilesCount)
		for _, diff := range changedFiles {
			if diff.action == merkletrie.Modify {
				fmt.Fprintf(&summaryBuilder, "± %s%s\n", diff.prettyName(), diff.changeStats())
			}
		}
	}

	return summaryBuilder.String()
}
