package resource_filter

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// IgnoreResourceRule represents a rule to skip a difference in the diff
type IgnoreResourceRule struct {
	Group string
	Kind  string
	Name  string
}

// String returns the string representation of the IgnoreResourceRule
func (s *IgnoreResourceRule) String() string {
	return fmt.Sprintf("[Group: %s, Kind: %s, Name: %s]", s.Group, s.Kind, s.Name)
}

// format is --ignore-resources="group:kind:name,group:kind:name"
// * means any value

// FromString creates a new IgnoreResourceRule from a string representation
func FromString(s string) ([]IgnoreResourceRule, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}

	rules := strings.Split(s, ",")

	var ignoreResourceRules []IgnoreResourceRule

	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		parts := strings.Split(rule, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid ignore resource rule format: %s (expected group:kind:name)", rule)
		}

		ignoreResourceRules = append(ignoreResourceRules, IgnoreResourceRule{
			Group: strings.TrimSpace(parts[0]),
			Kind:  strings.TrimSpace(parts[1]),
			Name:  strings.TrimSpace(parts[2]),
		})
	}

	return ignoreResourceRules, nil
}

// matches checks if the IgnoreResourceRule matches the given group, kind, and name
// A "*" in any field matches any value
func (s *IgnoreResourceRule) matches(group, kind, name string) bool {
	return (s.Group == "*" || s.Group == group) &&
		(s.Kind == "*" || s.Kind == kind) &&
		(s.Name == "*" || s.Name == name)
}

func MatchesAnyIgnoreRule(manifest *unstructured.Unstructured, ignoreResourceRules []IgnoreResourceRule) bool {
	group := groupFromAPIVersion(manifest.GetAPIVersion())
	for _, rule := range ignoreResourceRules {
		if rule.matches(group, manifest.GetKind(), manifest.GetName()) {
			return true
		}
	}
	return false
}

func groupFromAPIVersion(apiVersion string) string {
	// formats: "group/version" or "v1" (core)
	if strings.Contains(apiVersion, "/") {
		parts := strings.SplitN(apiVersion, "/", 2)
		return parts[0]
	}
	return ""
}
