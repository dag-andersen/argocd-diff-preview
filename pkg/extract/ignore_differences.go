package extract

import (
	"strconv"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
)

// ignoreDifferenceRule represents a subset of Argo CD's ignoreDifferences entry that we support.
// We support jsonPointers and jqPathExpressions.
type ignoreDifferenceRule struct {
	Group             string
	Kind              string
	Name              string
	Namespace         string
	JSONPointers      []string
	JQPathExpressions []string
}

const maskedValue = "<argocd-diff-preview:ignored>"

// parseIgnoreDifferencesFromApp extracts ignoreDifferences rules from an Application manifest.
// It supports both Application (spec.ignoreDifferences) and ApplicationSet templates
// (spec.template.spec.ignoreDifferences), though in our flow Applications are already generated.
func parseIgnoreDifferencesFromApp(app argoapplication.ArgoResource) []ignoreDifferenceRule {
	var rules []ignoreDifferenceRule

	if app.Yaml == nil {
		return rules
	}

	switch app.Kind {
	case argoapplication.Application:
		if list, found, err := unstructured.NestedSlice(app.Yaml.Object, "spec", "ignoreDifferences"); err == nil && found {
			for _, item := range list {
				if rule, ok := parseSingleIgnoreRule(item); ok {
					rules = append(rules, rule)
				}
			}
		}
	case argoapplication.ApplicationSet:
		if list, found, err := unstructured.NestedSlice(app.Yaml.Object, "spec", "template", "spec", "ignoreDifferences"); err == nil && found {
			for _, item := range list {
				if rule, ok := parseSingleIgnoreRule(item); ok {
					rules = append(rules, rule)
				}
			}
		}
	}

	return rules
}

func parseSingleIgnoreRule(item any) (ignoreDifferenceRule, bool) {
	m, ok := item.(map[string]any)
	if !ok {
		return ignoreDifferenceRule{}, false
	}

	rule := ignoreDifferenceRule{}
	if v, ok := m["group"].(string); ok {
		rule.Group = v
	}
	if v, ok := m["kind"].(string); ok {
		rule.Kind = v
	}
	if v, ok := m["name"].(string); ok {
		rule.Name = v
	}
	if v, ok := m["namespace"].(string); ok {
		rule.Namespace = v
	}
	if v, ok := m["jsonPointers"].([]any); ok {
		for _, p := range v {
			if s, ok := p.(string); ok {
				rule.JSONPointers = append(rule.JSONPointers, s)
			}
		}
	}
	if v, ok := m["jqPathExpressions"].([]any); ok {
		for _, p := range v {
			if s, ok := p.(string); ok {
				rule.JQPathExpressions = append(rule.JQPathExpressions, s)
			}
		}
	}

	// We require at least Kind and one jsonPointer or jqPathExpression to be useful
	if rule.Kind == "" || (len(rule.JSONPointers) == 0 && len(rule.JQPathExpressions) == 0) {
		return ignoreDifferenceRule{}, false
	}
	return rule, true
}

// applyIgnoreDifferencesToManifests mutates the manifests in-place to remove/mask fields
// specified by the ignoreDifferences rules. This ensures both branches produce identical
// content at those paths and therefore do not show up in the final diff.
func applyIgnoreDifferencesToManifests(manifests []unstructured.Unstructured, rules []ignoreDifferenceRule) {
	if len(rules) == 0 {
		return
	}

	for i := range manifests {
		m := &manifests[i]
		apiVersion := m.GetAPIVersion()
		kind := m.GetKind()
		name := m.GetName()
		namespace := m.GetNamespace()
		group := groupFromAPIVersion(apiVersion)

		for _, r := range rules {
			if !ruleMatches(r, group, kind, name, namespace) {
				continue
			}
			for _, ptr := range r.JSONPointers {
				deleteOrMaskAtJSONPointer(m.Object, ptr)
			}
			for _, expr := range r.JQPathExpressions {
				applyJQPathExpression(m.Object, expr)
			}
		}
	}
}

// applyJQPathExpression evaluates a jq expression and deletes/masks values at returned paths.
// The provided expression is wrapped with jq's path(<expr>) helper to obtain token arrays.
func applyJQPathExpression(obj map[string]any, expr string) {
	if expr == "" {
		return
	}
	q, err := gojq.Parse("path(" + expr + ")")
	if err != nil {
		log.Debug().Err(err).Msgf("ignoreDifferences: invalid jqPathExpression: %s", expr)
		return
	}
	code, err := gojq.Compile(q)
	if err != nil {
		log.Debug().Err(err).Msgf("ignoreDifferences: failed to compile jqPathExpression: %s", expr)
		return
	}
	iter := code.Run(obj)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			log.Debug().Err(err).Msg("ignoreDifferences: jq evaluation error")
			continue
		}

		// Expect a single path (array of tokens). If it's an array of arrays, handle each.
		if arr, ok := v.([]any); ok {
			applyTokens(obj, arr)
			continue
		}
		if arrs, ok := v.([][]any); ok {
			for _, tokens := range arrs {
				applyTokens(obj, tokens)
			}
		}
	}
}

// applyTokens traverses obj following jq path tokens and removes the final map key
// or masks the final array element.
func applyTokens(obj map[string]any, tokens []any) {
	var parent any = obj
	for i, tok := range tokens {
		last := i == len(tokens)-1
		switch cur := parent.(type) {
		case map[string]any:
			key, ok := tok.(string)
			if !ok {
				return
			}
			if last {
				delete(cur, key)
				return
			}
			next, ok := cur[key]
			if !ok {
				return
			}
			parent = next
		case []any:
			var idx int
			switch n := tok.(type) {
			case int:
				idx = n
			case int64:
				idx = int(n)
			case float64:
				idx = int(n)
			default:
				return
			}
			if idx < 0 || idx >= len(cur) {
				return
			}
			if last {
				cur[idx] = maskedValue
				return
			}
			parent = cur[idx]
		default:
			return
		}
	}
}

func ruleMatches(r ignoreDifferenceRule, group, kind, name, namespace string) bool {
	if r.Kind != "" && !strings.EqualFold(r.Kind, kind) {
		return false
	}
	// Group may be empty (core group). If set on rule, must match.
	if r.Group != "" && !strings.EqualFold(r.Group, group) {
		return false
	}
	if r.Name != "" && r.Name != name {
		return false
	}
	if r.Namespace != "" && r.Namespace != namespace {
		return false
	}
	return true
}

func groupFromAPIVersion(apiVersion string) string {
	// formats: "group/version" or "v1" (core)
	if strings.Contains(apiVersion, "/") {
		parts := strings.SplitN(apiVersion, "/", 2)
		return parts[0]
	}
	return ""
}

// deleteOrMaskAtJSONPointer applies a JSON Pointer (RFC 6901) to obj.
// If the target is a map key, it deletes the key. If it's an array index,
// it replaces the element with a masked value. Invalid pointers are ignored.
func deleteOrMaskAtJSONPointer(obj map[string]any, pointer string) {
	if pointer == "" {
		return
	}
	if pointer[0] != '/' {
		// Per RFC 6901, pointers must start with '/'
		return
	}

	// Split and unescape tokens
	tokens := strings.Split(pointer, "/")[1:]
	var parent any = obj

	for i, rawTok := range tokens {
		tok := decodeJSONPointerToken(rawTok)
		last := i == len(tokens)-1

		switch cur := parent.(type) {
		case map[string]any:
			if last {
				delete(cur, tok)
				return
			}
			next, ok := cur[tok]
			if !ok {
				// Nothing to do
				return
			}
			parent = next

		case []any:
			// Token must be an index
			idx, err := strconv.Atoi(tok)
			if err != nil || idx < 0 || idx >= len(cur) {
				return
			}
			if last {
				cur[idx] = maskedValue
				return
			}
			parent = cur[idx]

		default:
			// Can't traverse further
			log.Debug().Msgf("ignoreDifferences: cannot traverse token '%s' at pointer '%s'", tok, pointer)
			return
		}
	}
}

func decodeJSONPointerToken(s string) string {
	// RFC 6901: ~1 => '/', ~0 => '~'
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}
