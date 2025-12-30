package extract

import (
	"fmt"
	"strings"
)

// SkipResourceRule represents a rule to skip a difference in the diff
type SkipResourceRule struct {
	Group string
	Kind  string
	Name  string
}

// String returns the string representation of the DiffSkipRule
func (s *SkipResourceRule) String() string {
	return fmt.Sprintf("Group: %s, Kind: %s, Name: %s", s.Group, s.Kind, s.Name)
}

// format is --diff-skip-rule="group:kind:name,group:kind:name"
// * means any value

// FromString creates a new DiffSkipRule from a string representation
func FromString(s string) ([]SkipResourceRule, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}

	rules := strings.Split(s, ",")

	var diffSkipRules []SkipResourceRule

	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		parts := strings.Split(rule, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid diff skip rule format: %s (expected group:kind:name)", rule)
		}

		diffSkipRules = append(diffSkipRules, SkipResourceRule{
			Group: strings.TrimSpace(parts[0]),
			Kind:  strings.TrimSpace(parts[1]),
			Name:  strings.TrimSpace(parts[2]),
		})
	}

	return diffSkipRules, nil
}

// Matches checks if the DiffSkipRule matches the given group, kind, and name
// A "*" in any field matches any value
func (s *SkipResourceRule) Matches(group, kind, name string) bool {
	return (s.Group == "*" || s.Group == group) &&
		(s.Kind == "*" || s.Kind == kind) &&
		(s.Name == "*" || s.Name == name)
}
