package extract

import (
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
)

// ignoreDifferenceRule represents a subset of Argo CD's ignoreDifferences entry that we support.
// We currently implement jsonPointers. jqPathExpressions are not supported yet.
type ignoreDifferenceRule struct {
	Group        string
	Kind         string
	Name         string
	Namespace    string
	JSONPointers []string
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

	// Try Application path
	if list, found, err := unstructured.NestedSlice(app.Yaml.Object, "spec", "ignoreDifferences"); err == nil && found {
		for _, item := range list {
			if rule, ok := parseSingleIgnoreRule(item); ok {
				rules = append(rules, rule)
			}
		}
	}

	// Try ApplicationSet template path as fallback
	if len(rules) == 0 {
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

func parseSingleIgnoreRule(item interface{}) (ignoreDifferenceRule, bool) {
	m, ok := item.(map[string]interface{})
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
	if v, ok := m["jsonPointers"].([]interface{}); ok {
		for _, p := range v {
			if s, ok := p.(string); ok {
				rule.JSONPointers = append(rule.JSONPointers, s)
			}
		}
	}

	// We require at least Kind and one jsonPointer to be useful
	if rule.Kind == "" || len(rule.JSONPointers) == 0 {
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
func deleteOrMaskAtJSONPointer(obj map[string]interface{}, pointer string) {
	if pointer == "" {
		return
	}
	if pointer[0] != '/' {
		// Per RFC 6901, pointers must start with '/'
		return
	}

	// Split and unescape tokens
	tokens := strings.Split(pointer, "/")[1:]
	var parent interface{} = obj

	for i, rawTok := range tokens {
		tok := decodeJSONPointerToken(rawTok)
		last := i == len(tokens)-1

		switch cur := parent.(type) {
		case map[string]interface{}:
			if last {
				if _, exists := cur[tok]; exists {
					delete(cur, tok)
				}
				return
			}
			next, ok := cur[tok]
			if !ok {
				// Nothing to do
				return
			}
			parent = next

		case []interface{}:
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


