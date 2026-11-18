package argoapplication

import (
	"regexp"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/repository"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// templatePatchKeyLine matches a block-style YAML mapping entry, optionally
// introduced by a sequence indicator: `  key: value` or `  - key: value`.
var templatePatchKeyLine = regexp.MustCompile(`^( *)(- +)?([A-Za-z0-9_.-]+):(?: +(.*))?$`)

// blockScalarHeader matches the header of a literal or folded block scalar.
var blockScalarHeader = regexp.MustCompile(`^[|>][-+0-9]*$`)

type templatePatchLine struct {
	ignored   bool   // blank line, comment, template directive or block scalar content
	indent    int    // number of leading spaces
	keyIndent int    // column of the first key or value after an optional "- "
	isItem    bool   // line starts a sequence item
	key       string // mapping key, empty when the line is not a mapping entry
	rawValue  string // everything after "key: "
	valueAt   int    // byte offset of rawValue in the line
}

// redirectTemplatePatchSources redirects the targetRevision of sources defined
// in an ApplicationSet spec.templatePatch.
//
// spec.templatePatch is a Go template rendered by the ApplicationSet controller,
// so it is not valid YAML in general (for example, `{{- if ... }}` blocks).
// Sources defined there were never redirected, so base and target branches
// rendered identical ApplicationSets and the diff was silently empty.
//
// The patch is rewritten line by line: for every block-style
// `targetRevision:` entry, the sibling `repoURL:` (and `chart:`) entries of the
// same mapping are collected and the regular redirect rules from
// redirectSourceMap are applied. Template directives, comments, quoting and
// formatting are preserved. Sources without an explicit targetRevision and
// flow-style mappings are left untouched.
func (a *ArgoResource) redirectTemplatePatchSources(repoSelector *repository.Selector, branch string, redirectRevisions []string) error {
	if a.Kind != ApplicationSet || a.Yaml == nil {
		return nil
	}

	templatePatch, found, err := unstructured.NestedString(a.Yaml.Object, "spec", "templatePatch")
	if err != nil || !found || strings.TrimSpace(templatePatch) == "" {
		return nil
	}

	rawLines := strings.Split(templatePatch, "\n")
	lines := parseTemplatePatchLines(rawLines)
	changed := false

	for i, line := range lines {
		if line.ignored || line.key != "targetRevision" {
			continue
		}

		targetRevision, quote, suffix, ok := parseTemplatePatchScalar(line.rawValue)
		if !ok || targetRevision == "" {
			continue
		}

		source := map[string]any{"targetRevision": targetRevision}
		for _, sibling := range templatePatchMappingLines(lines, i) {
			switch sibling.key {
			case "repoURL":
				if repoURL, _, _, ok := parseTemplatePatchScalar(sibling.rawValue); ok {
					source["repoURL"] = repoURL
				}
			case "chart":
				source["chart"] = sibling.rawValue
			}
		}

		if err := a.redirectSourceMap(source, repoSelector, branch, redirectRevisions); err != nil {
			return err
		}

		newRevision, _ := source["targetRevision"].(string)
		if newRevision == targetRevision {
			continue
		}

		rawLines[i] = rawLines[i][:line.valueAt] + quoteTemplatePatchScalar(newRevision, quote) + suffix
		changed = true
	}

	if !changed {
		return nil
	}

	log.Debug().Str("patchType", "redirectSources").Str(a.Kind.ShortName(), a.GetLongName()).Msg("Redirected sources in 'spec.templatePatch'")
	return unstructured.SetNestedField(a.Yaml.Object, strings.Join(rawLines, "\n"), "spec", "templatePatch")
}

func parseTemplatePatchLines(rawLines []string) []templatePatchLine {
	lines := make([]templatePatchLine, len(rawLines))
	blockScalarIndent := -1

	for i, raw := range rawLines {
		trimmed := strings.TrimSpace(raw)
		indent := len(raw) - len(strings.TrimLeft(raw, " "))

		if blockScalarIndent >= 0 {
			if trimmed == "" || indent > blockScalarIndent {
				lines[i] = templatePatchLine{ignored: true}
				continue
			}
			blockScalarIndent = -1
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "{{") {
			lines[i] = templatePatchLine{ignored: true}
			continue
		}

		line := templatePatchLine{indent: indent, keyIndent: indent}
		if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
			line.isItem = true
			afterDash := raw[indent+1:]
			line.keyIndent = len(raw) - len(strings.TrimLeft(afterDash, " "))
		}

		if match := templatePatchKeyLine.FindStringSubmatchIndex(raw); match != nil {
			line.key = raw[match[6]:match[7]]
			line.keyIndent = match[6]
			if match[8] >= 0 {
				line.rawValue = raw[match[8]:match[9]]
				line.valueAt = match[8]
			} else {
				line.valueAt = len(raw)
			}

			if value, _, _, ok := parseTemplatePatchScalar(line.rawValue); ok && blockScalarHeader.MatchString(value) {
				blockScalarIndent = line.keyIndent
			}
		}

		lines[i] = line
	}

	return lines
}

// templatePatchMappingLines returns the entries of the mapping that contains
// lines[index], at the same key indentation.
func templatePatchMappingLines(lines []templatePatchLine, index int) []templatePatchLine {
	keyIndent := lines[index].keyIndent
	start := index

	if !lines[index].isItem {
		for j := index - 1; j >= 0; j-- {
			if lines[j].ignored {
				continue
			}
			if lines[j].isItem && lines[j].keyIndent == keyIndent {
				start = j
				break
			}
			if lines[j].indent < keyIndent {
				break
			}
			start = j
		}
	}

	end := index
	for j := index + 1; j < len(lines); j++ {
		if lines[j].ignored {
			continue
		}
		if lines[j].indent < keyIndent {
			break
		}
		end = j
	}

	var siblings []templatePatchLine
	for j := start; j <= end; j++ {
		if !lines[j].ignored && lines[j].key != "" && lines[j].keyIndent == keyIndent {
			siblings = append(siblings, lines[j])
		}
	}
	return siblings
}

// parseTemplatePatchScalar parses a single-line YAML scalar followed by an
// optional comment. It returns the value, the quote character used (0 for
// plain scalars) and the remainder of the line after the value.
func parseTemplatePatchScalar(raw string) (value string, quote byte, suffix string, ok bool) {
	if raw == "" {
		return "", 0, "", true
	}

	switch raw[0] {
	case '"', '\'':
		quote = raw[0]
		for i := 1; i < len(raw); i++ {
			switch {
			case quote == '"' && raw[i] == '\\':
				i++
			case quote == '\'' && raw[i] == '\'' && i+1 < len(raw) && raw[i+1] == '\'':
				i++
			case raw[i] == quote:
				return unquoteTemplatePatchScalar(raw[1:i], quote), quote, raw[i+1:], true
			}
		}
		return "", 0, "", false
	case '{', '[', '&', '*', '!':
		return "", 0, "", false
	}

	value, _, _ = strings.Cut(raw, " #")
	trimmed := strings.TrimRight(value, " ")
	return trimmed, 0, raw[len(trimmed):], true
}

func unquoteTemplatePatchScalar(value string, quote byte) string {
	if quote == '\'' {
		return strings.ReplaceAll(value, "''", "'")
	}
	return strings.NewReplacer(`\\`, `\`, `\"`, `"`).Replace(value)
}

func quoteTemplatePatchScalar(value string, quote byte) string {
	switch quote {
	case '\'':
		return "'" + strings.ReplaceAll(value, "'", "''") + "'"
	case '"':
		return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
	}
	if value == "" || strings.ContainsAny(value, ":#{}[],&*!|>'\"%@`") || strings.TrimSpace(value) != value {
		return quoteTemplatePatchScalar(value, '"')
	}
	return value
}
