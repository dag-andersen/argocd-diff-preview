package diff

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/utils/merkletrie"
)

func buildSummary(changedFiles []Diff) string {
	var summaryBuilder strings.Builder

	addedCount := 0
	deletedCount := 0
	modifiedCount := 0

	for _, diff := range changedFiles {
		switch diff.action {
		case merkletrie.Insert:
			addedCount++
		case merkletrie.Delete:
			deletedCount++
		case merkletrie.Modify:
			modifiedCount++
		}
	}
	summaryBuilder.WriteString(fmt.Sprintf("Total: %d files changed\n", addedCount+deletedCount+modifiedCount))

	if addedCount > 0 {
		summaryBuilder.WriteString(fmt.Sprintf("\nAdded (%d):\n", addedCount))
		for _, diff := range changedFiles {
			if diff.action == merkletrie.Insert {
				summaryBuilder.WriteString(fmt.Sprintf("+ %s\n", diff.prettyName()))
			}
		}
	}

	if deletedCount > 0 {
		summaryBuilder.WriteString(fmt.Sprintf("\nDeleted (%d):\n", deletedCount))
		for _, diff := range changedFiles {
			if diff.action == merkletrie.Delete {
				summaryBuilder.WriteString(fmt.Sprintf("- %s\n", diff.prettyName()))
			}
		}
	}

	if modifiedCount > 0 {
		summaryBuilder.WriteString(fmt.Sprintf("\nModified (%d):\n", modifiedCount))
		for _, diff := range changedFiles {
			if diff.action == merkletrie.Modify {
				summaryBuilder.WriteString(fmt.Sprintf("± %s\n", diff.prettyName()))
			}
		}
	}

	return summaryBuilder.String()
}
