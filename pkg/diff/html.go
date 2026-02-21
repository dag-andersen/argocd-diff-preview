package diff

import (
	"fmt"
	"html"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/matching"
)

type HTMLOutput struct {
	title         string
	summary       string
	sections      []HTMLSection
	statsInfo     StatsInfo
	selectionInfo SelectionInfo
}

const htmlTemplate = `
<html>
<head>
<style>
body {
	font-family: arial;
}
.container {
	margin: auto;
	width: 910px;
}
.diffs {
	margin: 20px 0 20px 0;
}
.diff_container {
	width: 910px;
	overflow-x: scroll;
	border-radius: 8px;
	background:rgb(239, 239, 239);
	scrollbar-width: none;
	margin: 10px 0 10px 0;
}
table {
	font-family: monospace;
	border-spacing: 0px;
	width: 100%;
}
tr.normal_line {
	background:rgb(239, 239, 239);
}
tr.added_line {
	background:rgb(169, 216, 184);
}
tr.removed_line {
	background:rgb(247, 149, 173);
}
tr.comment_line {
	background:rgb(197, 194, 194);
}
.resource_header {
	font-family: monospace;
	font-size: 14px;
	color: rgb(80, 80, 80);
	margin: 15px 0 5px 0;
	padding: 0;
}
.resource_header:first-of-type {
	margin-top: 10px;
}
pre {
	margin: 0;
	padding-left: 15px;
	padding-right: 15px;
}
</style>
</head>
<body>
<div class="container">
<h1>%title%</h1>

<p>Summary:</p>
<pre>%summary%</pre>

<div class="diffs">
%app_diffs%
</div>
%selection_changes%
<pre>%info_box%</pre>
</div>
</body>
</html>
`

type HTMLSection struct {
	appName     string
	filePath    string
	appURL      string
	resources   []ResourceSection
	emptyReason matching.EmptyReason
}

// emptyReasonHTML returns the HTML-formatted message for an EmptyReason
func emptyReasonHTML(reason matching.EmptyReason) string {
	switch reason {
	case matching.EmptyReasonNoResources:
		return "<p><em>Application rendered no resources</em></p>"
	case matching.EmptyReasonHiddenDiff:
		return "<p><em>Diff hidden because <code>--hide-deleted-app-diff</code> is enabled</em></p>"
	default:
		return ""
	}
}

const htmlSectionTemplate = `
<details>
<summary>
%summary%
</summary>
%body%
</details>
`

const htmlLine = `
	<tr class="%s"><td><pre>%s</pre></td></tr>`

func (h *HTMLSection) printHTMLSection() string {
	s := htmlSectionTemplate

	// Build summary with optional link
	var summary string
	if h.appURL != "" {
		summary = fmt.Sprintf(`%s [<a href="%s">link</a>] (%s)`,
			html.EscapeString(h.appName),
			html.EscapeString(h.appURL),
			html.EscapeString(h.filePath))
	} else {
		summary = fmt.Sprintf(`%s (%s)`,
			html.EscapeString(h.appName),
			html.EscapeString(h.filePath))
	}
	s = strings.ReplaceAll(s, "%summary%", summary)

	var body strings.Builder

	if len(h.resources) == 0 {
		fmt.Fprintf(&body, "\n%s\n", emptyReasonHTML(h.emptyReason))
	} else {
		for _, r := range h.resources {
			fmt.Fprintf(&body, "\n<h4 class=\"resource_header\">%s</h4>\n", html.EscapeString(r.Header))
			if r.IsSkipped {
				body.WriteString("<p><em>Skipped</em></p>\n")
			} else {
				body.WriteString("<div class=\"diff_container\">\n<table>\n")
				for line := range strings.Lines(r.Content) {
					line = strings.TrimRight(line, " \t\r\n")
					if len(line) == 0 {
						continue
					}
					switch line[0] {
					case '@':
						fmt.Fprintf(&body, htmlLine, "comment_line", html.EscapeString(line))
					case '-':
						fmt.Fprintf(&body, htmlLine, "removed_line", html.EscapeString(line))
					case '+':
						fmt.Fprintf(&body, htmlLine, "added_line", html.EscapeString(line))
					default:
						fmt.Fprintf(&body, htmlLine, "normal_line", html.EscapeString(line))
					}
				}
				body.WriteString("\n</table>\n</div>\n")
			}
		}
	}

	s = strings.ReplaceAll(s, "%body%", body.String())

	return s
}

func (h *HTMLOutput) printDiff() string {
	var sectionsDiff strings.Builder

	for _, section := range h.sections {
		sectionsDiff.WriteString(section.printHTMLSection())
	}

	if sectionsDiff.Len() == 0 {
		sectionsDiff.WriteString("No changes found")
	}

	output := strings.ReplaceAll(htmlTemplate, "%title%", h.title)
	output = strings.ReplaceAll(output, "%summary%", strings.TrimSpace(h.summary))
	output = strings.ReplaceAll(output, "%app_diffs%", strings.TrimSpace(sectionsDiff.String()))
	selection_changes := ""
	if s := h.selectionInfo.String(); s != "" {
		selection_changes = fmt.Sprintf("\n<pre>%s</pre>\n<br>\n", s)
	}
	output = strings.ReplaceAll(output, "%selection_changes%", selection_changes)
	output = strings.ReplaceAll(output, "%info_box%", h.statsInfo.String())
	return strings.TrimSpace(output) + "\n"
}
