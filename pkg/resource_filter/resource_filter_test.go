package resource_filter

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFromString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []IgnoreResourceRule
		expectError bool
	}{
		{
			name:     "single rule",
			input:    "apps:Deployment:my-app",
			expected: []IgnoreResourceRule{{Group: "apps", Kind: "Deployment", Name: "my-app"}},
		},
		{
			name:  "multiple rules",
			input: "apps:Deployment:my-app,core:Secret:my-secret",
			expected: []IgnoreResourceRule{
				{Group: "apps", Kind: "Deployment", Name: "my-app"},
				{Group: "core", Kind: "Secret", Name: "my-secret"},
			},
		},
		{
			name:     "rule with wildcards",
			input:    "*:Secret:*",
			expected: []IgnoreResourceRule{{Group: "*", Kind: "Secret", Name: "*"}},
		},
		{
			name:     "all wildcards",
			input:    "*:*:*",
			expected: []IgnoreResourceRule{{Group: "*", Kind: "*", Name: "*"}},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: nil,
		},
		{
			name:     "rule with whitespace",
			input:    "  apps : Deployment : my-app  ",
			expected: []IgnoreResourceRule{{Group: "apps", Kind: "Deployment", Name: "my-app"}},
		},
		{
			name:  "multiple rules with whitespace",
			input: " apps:Deployment:app1 , core:ConfigMap:config ",
			expected: []IgnoreResourceRule{
				{Group: "apps", Kind: "Deployment", Name: "app1"},
				{Group: "core", Kind: "ConfigMap", Name: "config"},
			},
		},
		{
			name:        "invalid format - missing part",
			input:       "apps:Deployment",
			expectError: true,
		},
		{
			name:        "invalid format - too many parts",
			input:       "apps:Deployment:my-app:extra",
			expectError: true,
		},
		{
			name:        "invalid format - one valid one invalid",
			input:       "apps:Deployment:my-app,invalid",
			expectError: true,
		},
		{
			name:     "trailing comma",
			input:    "apps:Deployment:my-app,",
			expected: []IgnoreResourceRule{{Group: "apps", Kind: "Deployment", Name: "my-app"}},
		},
		{
			name:     "leading comma",
			input:    ",apps:Deployment:my-app",
			expected: []IgnoreResourceRule{{Group: "apps", Kind: "Deployment", Name: "my-app"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FromString(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("got %d rules, want %d", len(result), len(tt.expected))
				return
			}

			for i, rule := range result {
				if rule.Group != tt.expected[i].Group {
					t.Errorf("rule[%d].Group = %q, want %q", i, rule.Group, tt.expected[i].Group)
				}
				if rule.Kind != tt.expected[i].Kind {
					t.Errorf("rule[%d].Kind = %q, want %q", i, rule.Kind, tt.expected[i].Kind)
				}
				if rule.Name != tt.expected[i].Name {
					t.Errorf("rule[%d].Name = %q, want %q", i, rule.Name, tt.expected[i].Name)
				}
			}
		})
	}
}

func TestIgnoreResourceRule_Matches(t *testing.T) {
	tests := []struct {
		name     string
		rule     IgnoreResourceRule
		group    string
		kind     string
		resName  string
		expected bool
	}{
		{
			name:     "exact match",
			rule:     IgnoreResourceRule{Group: "apps", Kind: "Deployment", Name: "my-app"},
			group:    "apps",
			kind:     "Deployment",
			resName:  "my-app",
			expected: true,
		},
		{
			name:     "group mismatch",
			rule:     IgnoreResourceRule{Group: "apps", Kind: "Deployment", Name: "my-app"},
			group:    "core",
			kind:     "Deployment",
			resName:  "my-app",
			expected: false,
		},
		{
			name:     "kind mismatch",
			rule:     IgnoreResourceRule{Group: "apps", Kind: "Deployment", Name: "my-app"},
			group:    "apps",
			kind:     "StatefulSet",
			resName:  "my-app",
			expected: false,
		},
		{
			name:     "name mismatch",
			rule:     IgnoreResourceRule{Group: "apps", Kind: "Deployment", Name: "my-app"},
			group:    "apps",
			kind:     "Deployment",
			resName:  "other-app",
			expected: false,
		},
		{
			name:     "wildcard group",
			rule:     IgnoreResourceRule{Group: "*", Kind: "Deployment", Name: "my-app"},
			group:    "apps",
			kind:     "Deployment",
			resName:  "my-app",
			expected: true,
		},
		{
			name:     "wildcard kind",
			rule:     IgnoreResourceRule{Group: "apps", Kind: "*", Name: "my-app"},
			group:    "apps",
			kind:     "StatefulSet",
			resName:  "my-app",
			expected: true,
		},
		{
			name:     "wildcard name",
			rule:     IgnoreResourceRule{Group: "apps", Kind: "Deployment", Name: "*"},
			group:    "apps",
			kind:     "Deployment",
			resName:  "any-app",
			expected: true,
		},
		{
			name:     "all wildcards",
			rule:     IgnoreResourceRule{Group: "*", Kind: "*", Name: "*"},
			group:    "anything",
			kind:     "Whatever",
			resName:  "some-name",
			expected: true,
		},
		{
			name:     "wildcard group and name",
			rule:     IgnoreResourceRule{Group: "*", Kind: "Secret", Name: "*"},
			group:    "core",
			kind:     "Secret",
			resName:  "my-secret",
			expected: true,
		},
		{
			name:     "wildcard group and name - kind mismatch",
			rule:     IgnoreResourceRule{Group: "*", Kind: "Secret", Name: "*"},
			group:    "core",
			kind:     "ConfigMap",
			resName:  "my-config",
			expected: false,
		},
		{
			name:     "empty group matches empty",
			rule:     IgnoreResourceRule{Group: "", Kind: "Pod", Name: "my-pod"},
			group:    "",
			kind:     "Pod",
			resName:  "my-pod",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rule.matches(tt.group, tt.kind, tt.resName)
			if result != tt.expected {
				t.Errorf("Matches(%q, %q, %q) = %v, want %v",
					tt.group, tt.kind, tt.resName, result, tt.expected)
			}
		})
	}
}

func TestIgnoreResourceRule_String(t *testing.T) {
	rule := IgnoreResourceRule{Group: "apps", Kind: "Deployment", Name: "my-app"}
	expected := "[Group: apps, Kind: Deployment, Name: my-app]"
	result := rule.String()

	if result != expected {
		t.Errorf("String() = %q, want %q", result, expected)
	}
}

func TestGroupFromAPIVersion(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		expected   string
	}{
		{
			name:       "apps/v1",
			apiVersion: "apps/v1",
			expected:   "apps",
		},
		{
			name:       "networking.k8s.io/v1",
			apiVersion: "networking.k8s.io/v1",
			expected:   "networking.k8s.io",
		},
		{
			name:       "core v1 (no group)",
			apiVersion: "v1",
			expected:   "",
		},
		{
			name:       "custom group",
			apiVersion: "custom.example.com/v1beta1",
			expected:   "custom.example.com",
		},
		{
			name:       "empty string",
			apiVersion: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := groupFromAPIVersion(tt.apiVersion)
			if result != tt.expected {
				t.Errorf("groupFromAPIVersion(%q) = %q, want %q", tt.apiVersion, result, tt.expected)
			}
		})
	}
}

func TestMatchesAnySkipRule(t *testing.T) {
	tests := []struct {
		name     string
		manifest *unstructured.Unstructured
		rules    []IgnoreResourceRule
		expected bool
	}{
		{
			name: "empty rules returns false",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   map[string]any{"name": "my-app"},
				},
			},
			rules:    []IgnoreResourceRule{},
			expected: false,
		},
		{
			name: "nil rules returns false",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   map[string]any{"name": "my-app"},
				},
			},
			rules:    nil,
			expected: false,
		},
		{
			name: "single rule matches",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   map[string]any{"name": "my-app"},
				},
			},
			rules:    []IgnoreResourceRule{{Group: "apps", Kind: "Deployment", Name: "my-app"}},
			expected: true,
		},
		{
			name: "single rule does not match",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   map[string]any{"name": "my-app"},
				},
			},
			rules:    []IgnoreResourceRule{{Group: "apps", Kind: "StatefulSet", Name: "my-app"}},
			expected: false,
		},
		{
			name: "multiple rules - first matches",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   map[string]any{"name": "my-app"},
				},
			},
			rules: []IgnoreResourceRule{
				{Group: "apps", Kind: "Deployment", Name: "my-app"},
				{Group: "core", Kind: "Secret", Name: "my-secret"},
			},
			expected: true,
		},
		{
			name: "multiple rules - second matches",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata":   map[string]any{"name": "my-secret"},
				},
			},
			rules: []IgnoreResourceRule{
				{Group: "apps", Kind: "Deployment", Name: "my-app"},
				{Group: "", Kind: "Secret", Name: "my-secret"},
			},
			expected: true,
		},
		{
			name: "multiple rules - none match",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   map[string]any{"name": "my-app"},
				},
			},
			rules: []IgnoreResourceRule{
				{Group: "apps", Kind: "StatefulSet", Name: "my-app"},
				{Group: "core", Kind: "Secret", Name: "my-secret"},
			},
			expected: false,
		},
		{
			name: "wildcard group matches",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   map[string]any{"name": "my-app"},
				},
			},
			rules:    []IgnoreResourceRule{{Group: "*", Kind: "Deployment", Name: "my-app"}},
			expected: true,
		},
		{
			name: "wildcard kind matches",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   map[string]any{"name": "my-app"},
				},
			},
			rules:    []IgnoreResourceRule{{Group: "apps", Kind: "*", Name: "my-app"}},
			expected: true,
		},
		{
			name: "wildcard name matches",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   map[string]any{"name": "my-app"},
				},
			},
			rules:    []IgnoreResourceRule{{Group: "apps", Kind: "Deployment", Name: "*"}},
			expected: true,
		},
		{
			name: "all wildcards matches anything",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "random.io/v1",
					"kind":       "CustomResource",
					"metadata":   map[string]any{"name": "anything"},
				},
			},
			rules:    []IgnoreResourceRule{{Group: "*", Kind: "*", Name: "*"}},
			expected: true,
		},
		{
			name: "core resource (v1 apiVersion)",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata":   map[string]any{"name": "my-config"},
				},
			},
			rules:    []IgnoreResourceRule{{Group: "", Kind: "ConfigMap", Name: "my-config"}},
			expected: true,
		},
		{
			name: "core resource with wildcard group",
			manifest: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata":   map[string]any{"name": "my-secret"},
				},
			},
			rules:    []IgnoreResourceRule{{Group: "*", Kind: "Secret", Name: "*"}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesAnyIgnoreRule(tt.manifest, tt.rules)
			if result != tt.expected {
				t.Errorf("MatchesAnyIgnoreRule() = %v, want %v", result, tt.expected)
			}
		})
	}
}
