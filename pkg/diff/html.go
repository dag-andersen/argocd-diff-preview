package diff

import (
	"fmt"
	"html"
	"strings"
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
	appName       string
	filePath      string
	appURL        string
	commentHeader string
	content       string
}

const htmlSection = `
<details>
<summary>%summary%</summary>
<div class="diff_container">
<table>
	%rows%
</table>
</div>
</details>
`

const htmlLine = `
	<tr class="%s"><td><pre>%s</pre></td></tr>`

func (h *HTMLSection) printHTMLSection() string {
	s := htmlSection

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

	var rows strings.Builder
	// Pre-allocate capacity based on content length to avoid reallocations
	// Each line gets ~50 chars of HTML wrapper + up to ~40 chars for HTML escaping expansion
	estimatedLines := strings.Count(h.content, "\n") + 1
	htmlOverhead := estimatedLines * 90
	rows.Grow(len(h.content) + len(h.commentHeader) + htmlOverhead)

	// Add comment header
	fmt.Fprintf(&rows, htmlLine, "comment_line", html.EscapeString(strings.TrimRight(h.commentHeader, " \t\r\n")))

	// Process content lines
	for line := range strings.Lines(h.content) {
		line = strings.TrimRight(line, " \t\r\n")
		if len(line) == 0 {
			continue // Skip empty lines
		}
		switch line[0] {
		case '@':
			fmt.Fprintf(&rows, htmlLine, "comment_line", html.EscapeString(line))
		case '-':
			fmt.Fprintf(&rows, htmlLine, "removed_line", html.EscapeString(line))
		case '+':
			fmt.Fprintf(&rows, htmlLine, "added_line", html.EscapeString(line))
		default:
			fmt.Fprintf(&rows, htmlLine, "normal_line", html.EscapeString(line))
		}
	}
	s = strings.ReplaceAll(s, "%rows%", rows.String())

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
