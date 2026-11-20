package arrayorder

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Rule is a parsed JSONPath expression. Every array matched by the path is sorted
// into a canonical order before diffing, so pure reorderings don't show up.
//
// Supported syntax (subset of Kubernetes/kubectl JSONPath):
//
//	.field         child field
//	..field        recursive descent: "field" at any depth
//	.*  / [*]      all map values / all array elements
//	[n]            array element n
//	['a.b']        child field whose name contains special characters
//	[?(@.kind == 'Deployment')] root string-field equality filter
//
// The path may optionally be prefixed with "$" or wrapped in "{...}".
type Rule struct {
	raw      string
	segments []segment
}

type segmentKind int

const (
	segField segmentKind = iota
	segWildcard
	segIndex
	segRecursive
	segRootFilter
)

type segment struct {
	kind  segmentKind
	name  string
	index int
	value string
}

var rootFilterPattern = regexp.MustCompile(`^\[\?\(\s*@\.([A-Za-z_][A-Za-z0-9_]*)\s*==\s*(?:'([^']*)'|"([^"]*)")\s*\)\]`)

func (r Rule) String() string {
	return r.raw
}

// FromString parses a comma or newline separated list of JSONPath expressions.
func FromString(s string) ([]Rule, error) {
	var rules []Rule
	for _, raw := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' }) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		rule, err := Parse(raw)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// Parse parses a single JSONPath expression.
func Parse(raw string) (Rule, error) {
	p := strings.TrimSpace(raw)
	if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
		p = strings.TrimSpace(p[1 : len(p)-1])
	}
	p = strings.TrimPrefix(p, "$")

	fail := func(msg string) (Rule, error) {
		return Rule{}, fmt.Errorf("invalid JSONPath %q: %s", raw, msg)
	}

	if p == "" {
		return fail("path is empty")
	}
	// Allow paths without leading dot, e.g. "spec.containers"
	if p[0] != '.' && p[0] != '[' {
		p = "." + p
	}

	var segments []segment
	for i := 0; i < len(p); {
		switch {
		case strings.HasPrefix(p[i:], ".."):
			i += 2
			name, next := readName(p, i)
			if name == "" || name == "*" {
				return fail("recursive descent '..' must be followed by a field name")
			}
			segments = append(segments, segment{kind: segRecursive, name: name})
			i = next
		case p[i] == '.':
			i++
			name, next := readName(p, i)
			if name == "" {
				return fail(fmt.Sprintf("missing field name at position %d", i))
			}
			if name == "*" {
				segments = append(segments, segment{kind: segWildcard})
			} else {
				segments = append(segments, segment{kind: segField, name: name})
			}
			i = next
		case p[i] == '[':
			if strings.HasPrefix(p[i:], "[?") {
				match := rootFilterPattern.FindStringSubmatch(p[i:])
				if len(segments) != 0 || match == nil {
					return fail("unsupported filter: use a root string equality filter such as $[?(@.kind == 'Deployment')]")
				}
				segments = append(segments, segment{kind: segRootFilter, name: match[1], value: match[2] + match[3]})
				i += len(match[0])
				continue
			}
			end := strings.IndexByte(p[i:], ']')
			if end < 0 {
				return fail("unclosed '['")
			}
			content := strings.TrimSpace(p[i+1 : i+end])
			i += end + 1
			switch {
			case content == "*":
				segments = append(segments, segment{kind: segWildcard})
			case len(content) >= 2 && (content[0] == '\'' || content[0] == '"') && content[len(content)-1] == content[0]:
				segments = append(segments, segment{kind: segField, name: content[1 : len(content)-1]})
			default:
				n, err := strconv.Atoi(content)
				if err != nil || n < 0 {
					return fail(fmt.Sprintf("unsupported bracket expression [%s] (use [*], [n] or ['field'])", content))
				}
				segments = append(segments, segment{kind: segIndex, index: n})
			}
		default:
			return fail(fmt.Sprintf("unexpected character %q at position %d", p[i], i))
		}
	}

	return Rule{raw: strings.TrimSpace(raw), segments: segments}, nil
}

func readName(p string, start int) (string, int) {
	end := start
	for end < len(p) && p[end] != '.' && p[end] != '[' {
		end++
	}
	return strings.TrimSpace(p[start:end]), end
}

// Apply sorts, in place, every array in obj matched by any of the rules.
func Apply(obj map[string]any, rules []Rule) {
	for _, rule := range rules {
		apply(obj, rule.segments)
	}
}

// Align places identical elements of matched arrays in the same order on both
// objects, leaving unmatched elements after them for a clearer resource diff.
func Align(base, target map[string]any, rules []Rule) {
	for _, rule := range rules {
		align(base, target, rule.segments)
	}
}

func align(base, target any, segments []segment) {
	if len(segments) == 0 {
		baseList, baseOK := base.([]any)
		targetList, targetOK := target.([]any)
		if baseOK && targetOK {
			alignLists(baseList, targetList)
		}
		return
	}

	seg, rest := segments[0], segments[1:]
	switch seg.kind {
	case segRootFilter:
		if matchesRootFilter(base, seg) && matchesRootFilter(target, seg) {
			align(base, target, rest)
		}
	case segField:
		baseMap, baseOK := base.(map[string]any)
		targetMap, targetOK := target.(map[string]any)
		if !baseOK || !targetOK {
			return
		}
		baseChild, baseOK := baseMap[seg.name]
		targetChild, targetOK := targetMap[seg.name]
		if baseOK && targetOK {
			align(baseChild, targetChild, rest)
		}
	case segIndex:
		baseList, baseOK := base.([]any)
		targetList, targetOK := target.([]any)
		if baseOK && targetOK && seg.index < len(baseList) && seg.index < len(targetList) {
			align(baseList[seg.index], targetList[seg.index], rest)
		}
	case segWildcard:
		switch baseValue := base.(type) {
		case map[string]any:
			targetValue, ok := target.(map[string]any)
			if !ok {
				return
			}
			for key, baseChild := range baseValue {
				if targetChild, exists := targetValue[key]; exists {
					align(baseChild, targetChild, rest)
				}
			}
		case []any:
			targetValue, ok := target.([]any)
			if !ok {
				return
			}
			for i := 0; i < len(baseValue) && i < len(targetValue); i++ {
				align(baseValue[i], targetValue[i], rest)
			}
		}
	case segRecursive:
		alignRecursive(base, target, seg.name, rest)
	}
}

func alignRecursive(base, target any, name string, rest []segment) {
	switch baseValue := base.(type) {
	case map[string]any:
		targetValue, ok := target.(map[string]any)
		if !ok {
			return
		}
		if baseChild, exists := baseValue[name]; exists {
			if targetChild, targetExists := targetValue[name]; targetExists {
				align(baseChild, targetChild, rest)
			}
		}
		for key, baseChild := range baseValue {
			if targetChild, exists := targetValue[key]; exists {
				alignRecursive(baseChild, targetChild, name, rest)
			}
		}
	case []any:
		targetValue, ok := target.([]any)
		if !ok {
			return
		}
		for i := 0; i < len(baseValue) && i < len(targetValue); i++ {
			alignRecursive(baseValue[i], targetValue[i], name, rest)
		}
	}
}

// alignLists pairs identical elements first, then elements sharing a "name", and puts
// each pair at the same position in both lists. Unpaired elements follow in their original order.
func alignLists(base, target []any) {
	baseKeys, ok := jsonKeys(base)
	if !ok {
		return
	}
	targetKeys, ok := jsonKeys(target)
	if !ok {
		return
	}

	pairOf := make([]int, len(base)) // base index -> target index, -1 if unpaired
	for i := range pairOf {
		pairOf[i] = -1
	}
	usedTarget := make([]bool, len(target))

	byJSON := make(map[string][]int, len(target))
	for j, key := range targetKeys {
		byJSON[key] = append(byJSON[key], j)
	}
	for i, key := range baseKeys {
		if queue := byJSON[key]; len(queue) > 0 {
			pairOf[i], usedTarget[queue[0]] = queue[0], true
			byJSON[key] = queue[1:]
		}
	}

	byName := make(map[string][]int)
	for j, item := range target {
		if name, ok := elementName(item); ok && !usedTarget[j] {
			byName[name] = append(byName[name], j)
		}
	}
	for i, item := range base {
		if pairOf[i] >= 0 {
			continue
		}
		if name, ok := elementName(item); ok {
			if queue := byName[name]; len(queue) > 0 {
				pairOf[i], usedTarget[queue[0]] = queue[0], true
				byName[name] = queue[1:]
			}
		}
	}

	baseOrder := make([]any, 0, len(base))
	targetOrder := make([]any, 0, len(target))
	for i, j := range pairOf {
		if j >= 0 {
			baseOrder = append(baseOrder, base[i])
			targetOrder = append(targetOrder, target[j])
		}
	}
	for i, j := range pairOf {
		if j < 0 {
			baseOrder = append(baseOrder, base[i])
		}
	}
	for j, item := range target {
		if !usedTarget[j] {
			targetOrder = append(targetOrder, item)
		}
	}
	copy(base, baseOrder)
	copy(target, targetOrder)
}

func jsonKeys(list []any) ([]string, bool) {
	keys := make([]string, len(list))
	for i, item := range list {
		encoded, err := json.Marshal(item)
		if err != nil {
			return nil, false
		}
		keys[i] = string(encoded)
	}
	return keys, true
}

func elementName(item any) (string, bool) {
	if m, ok := item.(map[string]any); ok {
		name, ok := m["name"].(string)
		return name, ok
	}
	return "", false
}

func apply(node any, segments []segment) {
	if len(segments) == 0 {
		if list, ok := node.([]any); ok {
			sortCanonical(list)
		}
		return
	}

	seg, rest := segments[0], segments[1:]
	switch seg.kind {
	case segRootFilter:
		if matchesRootFilter(node, seg) {
			apply(node, rest)
		}
	case segField:
		if m, ok := node.(map[string]any); ok {
			if child, ok := m[seg.name]; ok {
				apply(child, rest)
			}
		}
	case segIndex:
		if list, ok := node.([]any); ok && seg.index < len(list) {
			apply(list[seg.index], rest)
		}
	case segWildcard:
		switch v := node.(type) {
		case map[string]any:
			for _, child := range v {
				apply(child, rest)
			}
		case []any:
			for _, child := range v {
				apply(child, rest)
			}
		}
	case segRecursive:
		applyRecursive(node, seg.name, rest)
	}
}

func matchesRootFilter(node any, filter segment) bool {
	obj, ok := node.(map[string]any)
	if !ok {
		return false
	}
	value, ok := obj[filter.name].(string)
	return ok && value == filter.value
}

func applyRecursive(node any, name string, rest []segment) {
	switch v := node.(type) {
	case map[string]any:
		if child, ok := v[name]; ok {
			apply(child, rest)
		}
		for _, child := range v {
			applyRecursive(child, name, rest)
		}
	case []any:
		for _, child := range v {
			applyRecursive(child, name, rest)
		}
	}
}

// sortCanonical sorts by the "name" field when every element has one (e.g. env vars,
// ports, volumes), otherwise by the element's JSON representation. Elements sharing a
// name are ordered by their JSON, so e.g. one volume mounted at two paths sorts deterministically.
func sortCanonical(list []any) {
	if len(list) < 2 {
		return
	}
	keys := make([]string, len(list))
	names := make([]string, len(list))
	allNamed := true
	for i, e := range list {
		b, err := json.Marshal(e)
		if err != nil {
			return
		}
		keys[i] = string(b)
		name, ok := elementName(e)
		allNamed = allNamed && ok
		names[i] = name
	}
	if allNamed {
		for i := range keys {
			keys[i] = names[i] + "\x00" + keys[i]
		}
	}
	sort.Stable(byKey{list: list, keys: keys})
}

type byKey struct {
	list []any
	keys []string
}

func (b byKey) Len() int           { return len(b.list) }
func (b byKey) Less(i, j int) bool { return b.keys[i] < b.keys[j] }
func (b byKey) Swap(i, j int) {
	b.list[i], b.list[j] = b.list[j], b.list[i]
	b.keys[i], b.keys[j] = b.keys[j], b.keys[i]
}
