package diff

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/utils/merkletrie"
)

type Diff struct {
	newName       string
	oldName       string
	newSourcePath string
	oldSourcePath string
	action        merkletrie.Action
	changeInfo    changeInfo
}

func (d *Diff) prettyName() string {
	switch {
	case d.newName != "" && d.oldName != "" && d.newName != d.oldName:
		return fmt.Sprintf("%s -> %s", d.oldName, d.newName)
	case d.newName != "":
		return d.newName
	case d.oldName != "":
		return d.oldName
	default:
		return "Unknown"
	}
}

func (d *Diff) changeStats() string {
	switch {
	case d.changeInfo.addedLines > 0 && d.changeInfo.deletedLines > 0:
		return fmt.Sprintf(" (+%d|-%d)", d.changeInfo.addedLines, d.changeInfo.deletedLines)
	case d.changeInfo.addedLines > 0:
		return fmt.Sprintf(" (+%d)", d.changeInfo.addedLines)
	case d.changeInfo.deletedLines > 0:
		return fmt.Sprintf(" (-%d)", d.changeInfo.deletedLines)
	default:
		return ""
	}
}

func (d *Diff) prettyPath() string {
	switch {
	case d.newSourcePath != "" && d.oldSourcePath != "" && d.newSourcePath != d.oldSourcePath:
		return fmt.Sprintf("%s -> %s", d.oldSourcePath, d.newSourcePath)
	case d.newSourcePath != "":
		return d.newSourcePath
	case d.oldSourcePath != "":
		return d.oldSourcePath
	default:
		return "Unknown"
	}
}

func (d *Diff) commentHeader() string {
	switch d.action {
	case merkletrie.Insert:
		return fmt.Sprintf("@@ Application added: %s (%s) @@\n", d.prettyName(), d.prettyPath())
	case merkletrie.Delete:
		return fmt.Sprintf("@@ Application deleted: %s (%s) @@\n", d.prettyName(), d.prettyPath())
	case merkletrie.Modify:
		return fmt.Sprintf("@@ Application modified: %s (%s) @@\n", d.prettyName(), d.prettyPath())
	default:
		return ""
	}
}

func (d *Diff) buildAppURL(argocdUIURL string) string {
	if argocdUIURL == "" {
		return ""
	}

	appName := d.oldName
	if appName == "" {
		appName = d.newName
	}

	if appName == "" {
		return ""
	}

	baseURL := strings.TrimRight(argocdUIURL, "/")

	return fmt.Sprintf("%s/applications/%s", baseURL, appName)
}

func (d *Diff) buildMarkdownSection(argocdUIURL string) MarkdownSection {
	return MarkdownSection{
		appName:  d.prettyName(),
		filePath: d.prettyPath(),
		appURL:   d.buildAppURL(argocdUIURL),
		comment:  d.commentHeader(),
		blocks:   d.changeInfo.blocks,
	}
}

func (d *Diff) buildHTMLSection(argocdUIURL string) HTMLSection {
	return HTMLSection{
		appName:       d.prettyName(),
		filePath:      d.prettyPath(),
		appURL:        d.buildAppURL(argocdUIURL),
		commentHeader: d.commentHeader(),
		blocks:        d.changeInfo.blocks,
	}
}
