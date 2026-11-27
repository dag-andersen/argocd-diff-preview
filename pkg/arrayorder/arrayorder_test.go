package arrayorder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func parseYAML(t *testing.T, y string) map[string]any {
	t.Helper()
	obj := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(y), &obj))
	return obj
}

func toYAML(t *testing.T, obj map[string]any) string {
	t.Helper()
	b, err := yaml.Marshal(obj)
	require.NoError(t, err)
	return string(b)
}

func mustRules(t *testing.T, s string) []Rule {
	t.Helper()
	rules, err := FromString(s)
	require.NoError(t, err)
	return rules
}

const deployment = `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      initContainers:
      - name: init
        env:
        - name: VAR_B
        - name: VAR_A
      containers:
      - name: app
        args: ["--z", "--a"]
        env:
        - name: VAR_B
          value: "2"
        - name: VAR_A
          value: "1"
        ports:
        - containerPort: 9090
        - containerPort: 8080
`

func TestFromString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr string
	}{
		{name: "empty", input: "  ", want: nil},
		{name: "single", input: ".spec.containers[*].env", want: []string{".spec.containers[*].env"}},
		{name: "comma and newline separated", input: ".a.b,\n ..env \n,$.c[0].d", want: []string{".a.b", "..env", "$.c[0].d"}},
		{name: "kubectl braces", input: "{.spec.containers[*].env}", want: []string{"{.spec.containers[*].env}"}},
		{name: "no leading dot", input: "spec.volumes", want: []string{"spec.volumes"}},
		{name: "quoted field", input: ".metadata.annotations['a.b/c']", want: []string{".metadata.annotations['a.b/c']"}},
		{name: "unclosed bracket", input: ".spec.containers[*", wantErr: "unclosed"},
		{name: "nested filter not supported", input: ".spec.containers[?(@.name=='a')]", wantErr: "unsupported filter"},
		{name: "unsupported filter operator", input: "$[?(@.kind != 'Deployment')].spec", wantErr: "unsupported filter"},
		{name: "unsupported compound filter", input: "$[?(@.kind == 'Deployment' || @.kind == 'Pod')].spec", wantErr: "unsupported filter"},
		{name: "empty field", input: ".spec..", wantErr: "recursive descent"},
		{name: "missing field name", input: ".spec.[*]", wantErr: "missing field name"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rules, err := FromString(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			var got []string
			for _, r := range rules {
				got = append(got, r.String())
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestApply_ExactPathOnlySortsMatchedArray(t *testing.T) {
	obj := parseYAML(t, deployment)
	Apply(obj, mustRules(t, ".spec.template.spec.containers[*].env"))

	assert.Equal(t, parseYAML(t, `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      initContainers:
      - name: init
        env:
        - name: VAR_B
        - name: VAR_A
      containers:
      - name: app
        args: ["--z", "--a"]
        env:
        - name: VAR_A
          value: "1"
        - name: VAR_B
          value: "2"
        ports:
        - containerPort: 9090
        - containerPort: 8080
`), obj)
}

func TestApply_RecursiveDescentAndUnnamedElements(t *testing.T) {
	obj := parseYAML(t, deployment)
	Apply(obj, mustRules(t, "..env, ..containers[0].ports"))

	assert.Equal(t, parseYAML(t, `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      initContainers:
      - name: init
        env:
        - name: VAR_A
        - name: VAR_B
      containers:
      - name: app
        args: ["--z", "--a"]
        env:
        - name: VAR_A
          value: "1"
        - name: VAR_B
          value: "2"
        ports:
        - containerPort: 8080
        - containerPort: 9090
`), obj)
}

func TestApply_ScalarArrayWildcardAndMissingPath(t *testing.T) {
	obj := parseYAML(t, `
spec:
  groups:
    a: [c, b, a]
    b: [3, 1, 2]
`)
	Apply(obj, mustRules(t, ".spec.groups.*, .does.not.exist[*], .spec.groups.a[5]"))
	assert.Equal(t, "spec:\n  groups:\n    a:\n    - a\n    - b\n    - c\n    b:\n    - 1\n    - 2\n    - 3\n", toYAML(t, obj))
}

func TestApply_IsDeterministicForPermutations(t *testing.T) {
	a := parseYAML(t, `
list:
- {name: x, value: "1"}
- {name: y, valueFrom: {secretKeyRef: {name: s, key: k}}}
- {name: z, value: "3"}
`)
	b := parseYAML(t, `
list:
- {name: z, value: "3"}
- {name: x, value: "1"}
- {name: y, valueFrom: {secretKeyRef: {name: s, key: k}}}
`)
	rules := mustRules(t, ".list")
	Apply(a, rules)
	Apply(b, rules)
	assert.Equal(t, toYAML(t, a), toYAML(t, b))
}

func TestApply_RootKindFilter(t *testing.T) {
	for _, expression := range []string{
		`$[?(@.kind == 'Deployment')].spec.template.spec.containers[*].env`,
		`$[?(@.kind=="Deployment")].spec.template.spec.containers[*].env`,
	} {
		t.Run(expression, func(t *testing.T) {
			rules := mustRules(t, expression)
			for _, kind := range []string{"Deployment", "StatefulSet", ""} {
				t.Run(kind, func(t *testing.T) {
					obj := parseYAML(t, deployment)
					if kind == "" {
						delete(obj, "kind")
					} else {
						obj["kind"] = kind
					}
					expected := parseYAML(t, toYAML(t, obj))
					if kind == "Deployment" {
						Apply(expected, mustRules(t, ".spec.template.spec.containers[*].env"))
					}
					Apply(obj, rules)
					assert.Equal(t, expected, obj)
				})
			}
		})
	}
}

func TestAlign_RootKindFilter(t *testing.T) {
	rules := mustRules(t, `$[?(@.kind == 'Deployment')].list`)
	for _, kind := range []string{"Deployment", "StatefulSet"} {
		t.Run(kind, func(t *testing.T) {
			base := map[string]any{"kind": kind, "list": []any{"a", "b"}}
			target := map[string]any{"kind": kind, "list": []any{"b", "c"}}
			Align(base, target, rules)
			if kind == "Deployment" {
				assert.Equal(t, []any{"b", "a"}, base["list"])
			} else {
				assert.Equal(t, []any{"a", "b"}, base["list"])
			}
			assert.Equal(t, []any{"b", "c"}, target["list"])
		})
	}
}

func TestAlign_PairsChangedNamedElementsAtSortedPosition(t *testing.T) {
	rules := mustRules(t, ".list")
	base := parseYAML(t, `
list:
- {name: c, value: "1"}
- {name: a, value: "1"}
- {name: b, value: "1"}
`)
	target := parseYAML(t, `
list:
- {name: b, value: "1"}
- {name: c, value: "2"}
- {name: a, value: "1"}
`)
	Apply(base, rules)
	Apply(target, rules)
	Align(base, target, rules)

	assert.Equal(t, []any{"a", "b", "c"}, namesOf(base["list"]))
	assert.Equal(t, []any{"a", "b", "c"}, namesOf(target["list"]))
}

func TestAlign_UnpairedElementsFollowPairedOnes(t *testing.T) {
	rules := mustRules(t, ".list")
	base := parseYAML(t, "list: [{name: old}, {name: keep}]")
	target := parseYAML(t, "list: [{name: keep}, {name: new}]")
	Align(base, target, rules)

	assert.Equal(t, []any{"keep", "old"}, namesOf(base["list"]))
	assert.Equal(t, []any{"keep", "new"}, namesOf(target["list"]))
}

func TestAlign_PreservesMultiplicityOfDuplicates(t *testing.T) {
	rules := mustRules(t, ".list")
	base := parseYAML(t, "list: [p, q, p]")
	target := parseYAML(t, "list: [p, q]")
	Align(base, target, rules)

	assert.ElementsMatch(t, []any{"p", "q", "p"}, base["list"])
	assert.ElementsMatch(t, []any{"p", "q"}, target["list"])
	assert.Equal(t, []any{"p", "q"}, base["list"].([]any)[:2])
	assert.Equal(t, []any{"p", "q"}, target["list"])
}

func TestAlign_RecursiveDescentAndWildcard(t *testing.T) {
	base := parseYAML(t, deployment)
	target := parseYAML(t, deployment)
	target["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)["env"] = []any{
		map[string]any{"name": "VAR_A", "value": "1"},
		map[string]any{"name": "VAR_B", "value": "changed"},
	}
	Align(base, target, mustRules(t, "..containers[*].env"))

	baseEnv := base["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)["env"]
	assert.Equal(t, []any{"VAR_B", "VAR_A"}, namesOf(baseEnv))
	targetEnv := target["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)["env"]
	assert.Equal(t, []any{"VAR_B", "VAR_A"}, namesOf(targetEnv))
}

func TestAlign_IgnoresMismatchedShapes(t *testing.T) {
	rules := mustRules(t, ".list")
	base := map[string]any{"list": []any{"b", "a"}}
	target := map[string]any{"list": "not-a-list"}
	Align(base, target, rules)
	assert.Equal(t, []any{"b", "a"}, base["list"])

	Align(map[string]any{}, map[string]any{"list": []any{"b"}}, rules)
}

func TestParse_RootFilterOnlyAllowedAsFirstSegment(t *testing.T) {
	_, err := Parse(`.spec[?(@.kind == 'Deployment')].template`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported filter")
}

func namesOf(v any) []any {
	var names []any
	for _, e := range v.([]any) {
		if m, ok := e.(map[string]any); ok {
			names = append(names, m["name"])
		} else {
			names = append(names, e)
		}
	}
	return names
}

func TestApply_SameNameDifferentContentIsDeterministic(t *testing.T) {
	rules := mustRules(t, ".list")
	a := parseYAML(t, "list: [{name: data, mountPath: /a}, {name: other}, {name: data, mountPath: /b}]")
	b := parseYAML(t, "list: [{name: data, mountPath: /b}, {name: data, mountPath: /a}, {name: other}]")
	Apply(a, rules)
	Apply(b, rules)

	assert.Equal(t, toYAML(t, a), toYAML(t, b))
	assert.Equal(t, []any{"data", "data", "other"}, namesOf(a["list"]))
}
