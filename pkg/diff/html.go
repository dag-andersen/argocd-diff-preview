package diff

import (
	"fmt"
	"html"
	"strings"
)

type HTMLOutput struct {
	title    string
	summary  string
	sections []HTMLSection
	infoBox  InfoBox
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

<pre>%info_box%</pre>
</div>
</body>
</html>
`

type HTMLSection struct {
	header        string
	commentHeader string
	content       string
}

const htmlSection = `
<details>
<summary>
%header%
</summary>
<div class="diff_container">
<table>
	%rows%
</table>
</div>
</details>
`

const htmlLine = `<tr class="%s"><td><pre>%s</pre></td></tr>`

func (h *HTMLSection) printHTMLSection() string {
	s := strings.ReplaceAll(htmlSection, "%header%", html.EscapeString(h.header))

	rows := fmt.Sprintf(htmlLine, "comment_line", html.EscapeString(h.commentHeader))
	for _, line := range strings.Split(h.content, "\n") {
		if len(line) == 0 {
			rows += fmt.Sprintf(htmlLine, "normal_line", html.EscapeString(line))
			continue
		}
		switch line[0] {
		case '@':
			rows += fmt.Sprintf(htmlLine, "comment_line", html.EscapeString(line))
		case '-':
			rows += fmt.Sprintf(htmlLine, "removed_line", html.EscapeString(line))
		case '+':
			rows += fmt.Sprintf(htmlLine, "added_line", html.EscapeString(line))
		default:
			rows += fmt.Sprintf(htmlLine, "normal_line", html.EscapeString(line))
		}
	}
	s = strings.ReplaceAll(s, "%rows%", rows)

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
	output = strings.ReplaceAll(output, "%info_box%", h.infoBox.String())
	return strings.TrimSpace(output) + "\n"
}
