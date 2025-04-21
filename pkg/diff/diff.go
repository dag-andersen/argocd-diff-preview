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
	diffContent   string
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

func (d *Diff) buildSection() string {
	header := fmt.Sprintf("%s (%s)", d.prettyName(), d.prettyPath())

	content := strings.TrimSpace(fmt.Sprintf("%s%s", d.commentHeader(), d.diffContent))

	return fmt.Sprintf("<details>\n<summary>%s</summary>\n<br>\n\n```diff\n%s\n```\n\n</details>\n\n", header, content)
}
