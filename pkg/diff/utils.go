package diff

import (
	"fmt"
	"strings"
)

const (
	metaCommentPrefix = "# META COMMENT -"
)

func addIdentifierLine(app AppInfo, content string) string {
	return fmt.Sprintf("%s App Name: %s\n%s Source Path: %s\n%s", metaCommentPrefix, app.Name, metaCommentPrefix, app.SourcePath, content)
}

// removeIdentifierLines removes lines from the beginning of content as long as they contain "# app-name"
func removeIdentifierLines(content string) string {
	if content == "" {
		return content
	}

	remainingContent := content

	for {
		// Find the next newline
		newlineIndex := strings.IndexByte(remainingContent, '\n')
		if newlineIndex == -1 {
			// No more newlines, check if the remaining content contains "# app-name"
			if strings.Contains(remainingContent, metaCommentPrefix) {
				return ""
			}
			return remainingContent
		}

		// Get the current line
		currentLine := remainingContent[:newlineIndex]

		// Check if the current line contains "# app-name"
		if !strings.Contains(currentLine, metaCommentPrefix) {
			// Found a line that doesn't contain "# app-name", return the remaining content
			return remainingContent
		}

		// Remove the current line and continue with the next line
		remainingContent = remainingContent[newlineIndex+1:]
	}
}
