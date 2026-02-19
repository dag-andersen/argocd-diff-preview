package matching

import (
	"strings"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// Helper to parse YAML string into unstructured.Unstructured
func parseYAML(t *testing.T, yamlStr string) unstructured.Unstructured {
	t.Helper()
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(yamlStr), &obj); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}
	return unstructured.Unstructured{Object: obj}
}

// Helper to create an ExtractedApp from YAML strings
func makeAppFromYAML(t *testing.T, id, name string, yamls ...string) extract.ExtractedApp {
	t.Helper()
	manifests := make([]unstructured.Unstructured, len(yamls))
	for i, y := range yamls {
		manifests[i] = parseYAML(t, y)
	}
	return extract.ExtractedApp{
		Id:         id,
		Name:       name,
		SourcePath: "/path/to/" + name,
		Manifests:  manifests,
		Branch:     git.Base,
	}
}

// TestFullFlow_NewApp tests adding a completely new application
func TestFullFlow_NewApp(t *testing.T) {
	deploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 1`

	baseApps := []extract.ExtractedApp{}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", deploymentYAML),
	}

	// Step 1: Match apps
	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0].Base != nil {
		t.Error("expected Base to be nil for new app")
	}
	if pairs[0].Target == nil {
		t.Fatal("expected Target to be non-nil for new app")
	}

	// Step 2: Get changed resources
	changedResources := pairs[0].ChangedResources()
	if len(changedResources) != 1 {
		t.Fatalf("expected 1 changed resource, got %d", len(changedResources))
	}

	// Step 3: Generate diff
	rp := changedResources[0]
	if rp.Base != nil {
		t.Error("expected Base to be nil for added resource")
	}
	if rp.Target == nil {
		t.Error("expected Target to be non-nil for added resource")
	}

	diff, err := rp.Diff(3)
	if err != nil {
		t.Fatalf("failed to generate diff: %v", err)
	}

	// Exact expected output - all lines prefixed with "+" for addition
	expectedContent := `+apiVersion: apps/v1
+kind: Deployment
+metadata:
+  name: my-app
+  namespace: default
+spec:
+  replicas: 1
`

	if diff.Content != expectedContent {
		t.Errorf("diff content mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedContent, diff.Content)
	}
	if diff.AddedLines != 7 {
		t.Errorf("expected 7 added lines, got %d", diff.AddedLines)
	}
	if diff.DeletedLines != 0 {
		t.Errorf("expected 0 deleted lines, got %d", diff.DeletedLines)
	}
}

// TestFullFlow_DeletedApp tests removing an application
func TestFullFlow_DeletedApp(t *testing.T) {
	deploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 1`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", deploymentYAML),
	}
	targetApps := []extract.ExtractedApp{}

	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0].Base == nil {
		t.Fatal("expected Base to be non-nil for deleted app")
	}
	if pairs[0].Target != nil {
		t.Error("expected Target to be nil for deleted app")
	}

	changedResources := pairs[0].ChangedResources()
	if len(changedResources) != 1 {
		t.Fatalf("expected 1 changed resource, got %d", len(changedResources))
	}

	diff, err := changedResources[0].Diff(3)
	if err != nil {
		t.Fatalf("failed to generate diff: %v", err)
	}

	// Exact expected output - all lines prefixed with "-" for deletion
	expectedContent := `-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  name: my-app
-  namespace: default
-spec:
-  replicas: 1
`

	if diff.Content != expectedContent {
		t.Errorf("diff content mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedContent, diff.Content)
	}
	if diff.AddedLines != 0 {
		t.Errorf("expected 0 added lines, got %d", diff.AddedLines)
	}
	if diff.DeletedLines != 7 {
		t.Errorf("expected 7 deleted lines, got %d", diff.DeletedLines)
	}
}

// TestFullFlow_ModifiedResource tests modifying a resource within an app
func TestFullFlow_ModifiedResource(t *testing.T) {
	baseDeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 1`

	targetDeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", baseDeploymentYAML),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", targetDeploymentYAML),
	}

	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}

	changedResources := pairs[0].ChangedResources()
	if len(changedResources) != 1 {
		t.Fatalf("expected 1 changed resource, got %d", len(changedResources))
	}

	diff, err := changedResources[0].Diff(3)
	if err != nil {
		t.Fatalf("failed to generate diff: %v", err)
	}

	// Exact expected output - context lines with space prefix, changes with +/-
	expectedContent := `   name: my-app
   namespace: default
 spec:
-  replicas: 1
+  replicas: 3
`

	if diff.Content != expectedContent {
		t.Errorf("diff content mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedContent, diff.Content)
	}
	if diff.AddedLines != 1 {
		t.Errorf("expected 1 added line, got %d", diff.AddedLines)
	}
	if diff.DeletedLines != 1 {
		t.Errorf("expected 1 deleted line, got %d", diff.DeletedLines)
	}
}

// containsLine checks if a multi-line string contains a specific line
func containsLine(content, line string) bool {
	for l := range strings.SplitSeq(content, "\n") {
		if l == line {
			return true
		}
	}
	return false
}

// TestFullFlow_RenamedResource tests that renamed resources are matched by content
func TestFullFlow_RenamedResource(t *testing.T) {
	baseDeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: old-name
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app`

	targetDeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: new-name
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", baseDeploymentYAML),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", targetDeploymentYAML),
	}

	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}

	changedResources := pairs[0].ChangedResources()

	// Should be 1 modified resource (matched by similarity), not 1 deleted + 1 added
	if len(changedResources) != 1 {
		t.Fatalf("expected 1 changed resource (rename matched by similarity), got %d", len(changedResources))
	}

	rp := changedResources[0]
	if rp.Base == nil || rp.Target == nil {
		t.Fatal("expected both Base and Target to be non-nil for renamed resource")
	}
	if rp.Base.GetName() != "old-name" {
		t.Errorf("expected base name 'old-name', got %q", rp.Base.GetName())
	}
	if rp.Target.GetName() != "new-name" {
		t.Errorf("expected target name 'new-name', got %q", rp.Target.GetName())
	}

	diff, err := rp.Diff(3)
	if err != nil {
		t.Fatalf("failed to generate diff: %v", err)
	}

	// The diff should only show the name change, not the entire resource as added/deleted
	// Verify it contains the name change lines
	if !containsLine(diff.Content, "-  name: old-name") {
		t.Errorf("expected diff to contain '-  name: old-name', got:\n%s", diff.Content)
	}
	if !containsLine(diff.Content, "+  name: new-name") {
		t.Errorf("expected diff to contain '+  name: new-name', got:\n%s", diff.Content)
	}

	// Should have 1 added and 1 deleted line (the name change)
	if diff.AddedLines != 1 {
		t.Errorf("expected 1 added line, got %d", diff.AddedLines)
	}
	if diff.DeletedLines != 1 {
		t.Errorf("expected 1 deleted line, got %d", diff.DeletedLines)
	}
}

// TestFullFlow_UnchangedResourcesFiltered tests that identical resources don't appear in diff
func TestFullFlow_UnchangedResourcesFiltered(t *testing.T) {
	deploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 1`

	serviceYAML := `apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: default
spec:
  type: ClusterIP`

	// Both base and target have the same resources
	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", deploymentYAML, serviceYAML),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", deploymentYAML, serviceYAML),
	}

	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}

	changedResources := pairs[0].ChangedResources()

	// No changes, so should be empty
	if len(changedResources) != 0 {
		t.Errorf("expected 0 changed resources for identical apps, got %d", len(changedResources))
	}
}

// TestFullFlow_MixedChanges tests a complex scenario with multiple types of changes
func TestFullFlow_MixedChanges(t *testing.T) {
	// Unchanged resource
	serviceYAML := `apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: default
spec:
  type: ClusterIP`

	// Modified resource
	baseDeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 1`

	targetDeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 5`

	// Deleted resource
	configMapYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  key: value`

	// Added resource
	secretYAML := `apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: default
stringData:
  password: secret123`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app",
			serviceYAML,        // unchanged
			baseDeploymentYAML, // modified
			configMapYAML,      // deleted
		),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app",
			serviceYAML,          // unchanged
			targetDeploymentYAML, // modified
			secretYAML,           // added
		),
	}

	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}

	changedResources := pairs[0].ChangedResources()

	// Should have 3 changes: modified deployment, deleted configmap, added secret
	// The unchanged service should NOT appear
	if len(changedResources) != 3 {
		t.Fatalf("expected 3 changed resources, got %d", len(changedResources))
	}

	// Expected exact outputs for each change type
	expectedModifiedDiff := `   name: my-app
   namespace: default
 spec:
-  replicas: 1
+  replicas: 5
`

	expectedDeletedDiff := `-apiVersion: v1
-data:
-  key: value
-kind: ConfigMap
-metadata:
-  name: my-config
-  namespace: default
`

	expectedAddedDiff := `+apiVersion: v1
+kind: Secret
+metadata:
+  name: my-secret
+  namespace: default
+stringData:
+  password: secret123
`

	// Verify each resource change with exact output
	for _, rp := range changedResources {
		diff, err := rp.Diff(3)
		if err != nil {
			t.Fatalf("failed to generate diff: %v", err)
		}

		switch {
		case rp.Base != nil && rp.Target != nil:
			// Modified Deployment
			if rp.Base.GetKind() != "Deployment" {
				t.Errorf("expected modified resource to be Deployment, got %s", rp.Base.GetKind())
			}
			if diff.Content != expectedModifiedDiff {
				t.Errorf("modified diff mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedModifiedDiff, diff.Content)
			}
			if diff.AddedLines != 1 || diff.DeletedLines != 1 {
				t.Errorf("expected 1 added, 1 deleted for modified; got %d added, %d deleted", diff.AddedLines, diff.DeletedLines)
			}

		case rp.Base == nil && rp.Target != nil:
			// Added Secret
			if rp.Target.GetKind() != "Secret" {
				t.Errorf("expected added resource to be Secret, got %s", rp.Target.GetKind())
			}
			if diff.Content != expectedAddedDiff {
				t.Errorf("added diff mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedAddedDiff, diff.Content)
			}
			if diff.AddedLines != 7 || diff.DeletedLines != 0 {
				t.Errorf("expected 7 added, 0 deleted for added; got %d added, %d deleted", diff.AddedLines, diff.DeletedLines)
			}

		case rp.Base != nil && rp.Target == nil:
			// Deleted ConfigMap
			if rp.Base.GetKind() != "ConfigMap" {
				t.Errorf("expected deleted resource to be ConfigMap, got %s", rp.Base.GetKind())
			}
			if diff.Content != expectedDeletedDiff {
				t.Errorf("deleted diff mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedDeletedDiff, diff.Content)
			}
			if diff.AddedLines != 0 || diff.DeletedLines != 7 {
				t.Errorf("expected 0 added, 7 deleted for deleted; got %d added, %d deleted", diff.AddedLines, diff.DeletedLines)
			}
		}
	}
}

// TestFullFlow_MultipleApps tests matching across multiple applications
func TestFullFlow_MultipleApps(t *testing.T) {
	app1DeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app1-deploy
  namespace: default
spec:
  replicas: 1`

	app2DeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app2-deploy
  namespace: default
spec:
  replicas: 2`

	app3DeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app3-deploy
  namespace: default
spec:
  replicas: 3`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "app1", app1DeploymentYAML), // unchanged
		makeAppFromYAML(t, "app-2", "app2", app2DeploymentYAML), // will be deleted
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "app1", app1DeploymentYAML), // unchanged
		makeAppFromYAML(t, "app-3", "app3", app3DeploymentYAML), // new app
	}

	pairs := MatchApps(baseApps, targetApps)

	// Should have 3 pairs: 1 matched (unchanged), 1 deleted, 1 added
	if len(pairs) != 3 {
		t.Fatalf("expected 3 pairs, got %d", len(pairs))
	}

	matchedApps := 0
	deletedApps := 0
	addedApps := 0
	for _, p := range pairs {
		switch {
		case p.Base != nil && p.Target != nil:
			matchedApps++
		case p.Base != nil && p.Target == nil:
			deletedApps++
		case p.Base == nil && p.Target != nil:
			addedApps++
		}
	}

	if matchedApps != 1 {
		t.Errorf("expected 1 matched app, got %d", matchedApps)
	}
	if deletedApps != 1 {
		t.Errorf("expected 1 deleted app, got %d", deletedApps)
	}
	if addedApps != 1 {
		t.Errorf("expected 1 added app, got %d", addedApps)
	}
}

// TestFullFlow_ContextLines tests that context lines work correctly
func TestFullFlow_ContextLines(t *testing.T) {
	baseYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
  labels:
    app: my-app
    version: v1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: main
        image: nginx:1.19`

	targetYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
  labels:
    app: my-app
    version: v1
spec:
  replicas: 5
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: main
        image: nginx:1.20`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", baseYAML),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", targetYAML),
	}

	pairs := MatchApps(baseApps, targetApps)
	changedResources := pairs[0].ChangedResources()

	if len(changedResources) != 1 {
		t.Fatalf("expected 1 changed resource, got %d", len(changedResources))
	}

	// Test with 2 context lines
	diff, err := changedResources[0].Diff(2)
	if err != nil {
		t.Fatalf("failed to generate diff: %v", err)
	}

	// With 2 context lines, we should see limited context around each change
	// The two changes (replicas and image) are far apart, so they should be in separate chunks
	// Verify key changes are present
	if !containsLine(diff.Content, "-  replicas: 1") {
		t.Errorf("expected diff to contain '-  replicas: 1', got:\n%s", diff.Content)
	}
	if !containsLine(diff.Content, "+  replicas: 5") {
		t.Errorf("expected diff to contain '+  replicas: 5', got:\n%s", diff.Content)
	}

	// Check for the image change (YAML may serialize slightly differently)
	if !strings.Contains(diff.Content, "nginx:1.19") || !strings.Contains(diff.Content, "nginx:1.20") {
		t.Errorf("expected diff to contain image change from nginx:1.19 to nginx:1.20, got:\n%s", diff.Content)
	}

	// Should have 2 changes: replicas and image
	if diff.AddedLines != 2 {
		t.Errorf("expected 2 added lines, got %d", diff.AddedLines)
	}
	if diff.DeletedLines != 2 {
		t.Errorf("expected 2 deleted lines, got %d", diff.DeletedLines)
	}

	// Should have a "skipped lines" indicator since the changes are far apart
	if !strings.Contains(diff.Content, "@@ skipped") {
		t.Errorf("expected diff to contain '@@ skipped' indicator for separated chunks, got:\n%s", diff.Content)
	}
}

// TestFullFlow_TwoBaseResourcesOneTarget documents the current behavior when base has 2 resources
// and target has 1 resource whose content resembles the deleted resource's data.
// Because resources are matched individually (by name/kind/content similarity), the output shows
// my-config as a modified resource and other-config as a separately deleted resource, joined by
// a `---` separator. This differs from a plain `git diff` which would show them as a single
// merged text blob.
func TestFullFlow_TwoBaseResourcesOneTarget(t *testing.T) {
	baseConfigYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  key: value`

	baseOtherConfigYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: other-config
  namespace: default
data:
  keyOne: "1"
  keyTwo: "2"
  keyThree: "3"`

	targetConfigYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  keyOne: "1"
  keyTwo: "2"
  keyThree: "3"`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", baseConfigYAML, baseOtherConfigYAML),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", targetConfigYAML),
	}

	diffs, err := GenerateAppDiffs(baseApps, targetApps, 10, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}

	d := diffs[0]
	if d.Action != ActionModified {
		t.Errorf("expected action=modified, got %s", d.Action)
	}
	if d.PrettyName() != "my-app" {
		t.Errorf("expected name=my-app, got %s", d.PrettyName())
	}

	// Because resources are diffed individually, the output is now per-resource:
	// 1. my-config as modified (data changed from key:value to keyOne/keyThree/keyTwo)
	// 2. other-config as fully deleted (all lines prefixed with `-`)
	// Note: a plain `git diff` would instead show these as a single merged text blob.
	if len(d.Resources) != 2 {
		t.Fatalf("expected 2 resource diffs, got %d", len(d.Resources))
	}

	// First resource: my-config (modified)
	expectedMyConfig := ` apiVersion: v1
 data:
-  key: value
+  keyOne: "1"
+  keyThree: "3"
+  keyTwo: "2"
 kind: ConfigMap
 metadata:
   name: my-config
   namespace: default
`
	if d.Resources[0].Kind != "ConfigMap" || d.Resources[0].Name != "my-config" {
		t.Errorf("expected first resource to be ConfigMap/my-config, got %s/%s", d.Resources[0].Kind, d.Resources[0].Name)
	}
	if d.Resources[0].Content != expectedMyConfig {
		t.Errorf("my-config diff mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedMyConfig, d.Resources[0].Content)
	}

	// Second resource: other-config (deleted)
	expectedOtherConfig := `-apiVersion: v1
-data:
-  keyOne: "1"
-  keyThree: "3"
-  keyTwo: "2"
-kind: ConfigMap
-metadata:
-  name: other-config
-  namespace: default
`
	if d.Resources[1].Kind != "ConfigMap" || d.Resources[1].Name != "other-config" {
		t.Errorf("expected second resource to be ConfigMap/other-config, got %s/%s", d.Resources[1].Kind, d.Resources[1].Name)
	}
	if d.Resources[1].Content != expectedOtherConfig {
		t.Errorf("other-config diff mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedOtherConfig, d.Resources[1].Content)
	}
	if d.AddedLines != 3 {
		t.Errorf("expected 3 added lines, got %d", d.AddedLines)
	}
	if d.DeletedLines != 10 {
		t.Errorf("expected 10 deleted lines, got %d", d.DeletedLines)
	}
}

// TestFullFlow_KindChangeWithUnchangedResources tests a kind change alongside unchanged resources.
// The Deployment→StatefulSet change should appear as modified, while the unchanged Service
// should be filtered out entirely (no diff output for identical resources).
func TestFullFlow_KindChangeWithUnchangedResources(t *testing.T) {
	serviceYAML := `apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: default
spec:
  ports:
  - port: 80
  selector:
    app: my-app`

	baseDeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app`

	targetStatefulSetYAML := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", baseDeploymentYAML, serviceYAML),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", targetStatefulSetYAML, serviceYAML),
	}

	diffs, err := GenerateAppDiffs(baseApps, targetApps, 10, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff (modified), got %d", len(diffs))
	}

	d := diffs[0]
	if d.Action != ActionModified {
		t.Errorf("expected action=modified, got %s", d.Action)
	}

	// Only the kind-changed resource should appear; the identical Service is filtered out
	if len(d.Resources) != 1 {
		t.Fatalf("expected 1 resource diff (only the kind change), got %d", len(d.Resources))
	}

	resource := d.Resources[0]
	if resource.Kind != "StatefulSet" {
		t.Errorf("expected resource kind=StatefulSet, got %s", resource.Kind)
	}
	if resource.Name != "my-app" {
		t.Errorf("expected resource name=my-app, got %s", resource.Name)
	}
	if !strings.Contains(resource.Content, "-kind: Deployment") {
		t.Error("expected diff to contain '-kind: Deployment'")
	}
	if !strings.Contains(resource.Content, "+kind: StatefulSet") {
		t.Error("expected diff to contain '+kind: StatefulSet'")
	}
}

// TestFullFlow_KindChangeWithContentChanges tests a kind change combined with spec changes.
// Both the kind change and content changes should appear in a single modified resource diff.
func TestFullFlow_KindChangeWithContentChanges(t *testing.T) {
	baseYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    spec:
      containers:
      - name: app
        image: nginx:1.19`

	targetYAML := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 5
  selector:
    matchLabels:
      app: my-app
  template:
    spec:
      containers:
      - name: app
        image: nginx:1.21`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", baseYAML),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", targetYAML),
	}

	diffs, err := GenerateAppDiffs(baseApps, targetApps, 10, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff (modified), got %d", len(diffs))
	}

	d := diffs[0]
	if d.Action != ActionModified {
		t.Errorf("expected action=modified, got %s", d.Action)
	}
	if len(d.Resources) != 1 {
		t.Fatalf("expected 1 resource diff, got %d", len(d.Resources))
	}

	resource := d.Resources[0]
	if resource.Kind != "StatefulSet" {
		t.Errorf("expected resource kind=StatefulSet, got %s", resource.Kind)
	}

	// Should show kind change, replicas change, and image change
	if !strings.Contains(resource.Content, "-kind: Deployment") {
		t.Error("expected diff to show kind change from Deployment")
	}
	if !strings.Contains(resource.Content, "+kind: StatefulSet") {
		t.Error("expected diff to show kind change to StatefulSet")
	}
	if !strings.Contains(resource.Content, "-  replicas: 3") {
		t.Error("expected diff to show replicas change from 3")
	}
	if !strings.Contains(resource.Content, "+  replicas: 5") {
		t.Error("expected diff to show replicas change to 5")
	}
	if !strings.Contains(resource.Content, "image: nginx:1.19") {
		t.Errorf("expected diff to show image change from 1.19, got:\n%s", resource.Content)
	}
	if !strings.Contains(resource.Content, "image: nginx:1.21") {
		t.Errorf("expected diff to show image change to 1.21, got:\n%s", resource.Content)
	}
}

// TestFullFlow_CompleteResourceReplacement tests that when a resource is completely replaced
// (different name, different kind, different content), it shows as one deleted and one added
// resource within the same app diff.
func TestFullFlow_CompleteResourceReplacement(t *testing.T) {
	baseYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: old-app
  namespace: default
spec:
  replicas: 3`

	targetYAML := `apiVersion: batch/v1
kind: CronJob
metadata:
  name: new-job
  namespace: default
spec:
  schedule: "*/5 * * * *"`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", baseYAML),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", targetYAML),
	}

	diffs, err := GenerateAppDiffs(baseApps, targetApps, 10, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("expected 1 app diff (modified), got %d", len(diffs))
	}

	d := diffs[0]
	if d.Action != ActionModified {
		t.Errorf("expected action=modified, got %s", d.Action)
	}

	// Two resource diffs: one deleted (Deployment/old-app) and one added (CronJob/new-job)
	if len(d.Resources) != 2 {
		t.Fatalf("expected 2 resource diffs, got %d", len(d.Resources))
	}

	// Resources are sorted by apiVersion, then kind, then name
	// CronJob (batch/v1) comes after Deployment (apps/v1) alphabetically by apiVersion
	deploymentRes := d.Resources[0]
	cronJobRes := d.Resources[1]

	if deploymentRes.Kind != "Deployment" || deploymentRes.Name != "old-app" {
		t.Errorf("expected first resource to be Deployment/old-app, got %s/%s", deploymentRes.Kind, deploymentRes.Name)
	}
	// All lines should be deletions (prefixed with -)
	for line := range strings.SplitSeq(strings.TrimSuffix(deploymentRes.Content, "\n"), "\n") {
		if !strings.HasPrefix(line, "-") {
			t.Errorf("expected all lines in deleted resource to start with '-', got: %q", line)
			break
		}
	}

	if cronJobRes.Kind != "CronJob" || cronJobRes.Name != "new-job" {
		t.Errorf("expected second resource to be CronJob/new-job, got %s/%s", cronJobRes.Kind, cronJobRes.Name)
	}
	// All lines should be additions (prefixed with +)
	for line := range strings.SplitSeq(strings.TrimSuffix(cronJobRes.Content, "\n"), "\n") {
		if !strings.HasPrefix(line, "+") {
			t.Errorf("expected all lines in added resource to start with '+', got: %q", line)
			break
		}
	}
}

// TestFullFlow_MultipleKindChanges tests that multiple resources can change kind simultaneously.
// Both a Deployment→StatefulSet and a ConfigMap→Secret change should produce a single
// modified app diff with two modified resource entries.
func TestFullFlow_MultipleKindChanges(t *testing.T) {
	baseDeployYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3`

	baseConfigYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: default
data:
  key: value`

	targetStatefulSetYAML := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3`

	targetSecretYAML := `apiVersion: v1
kind: Secret
metadata:
  name: my-config
  namespace: default
data:
  key: dmFsdWU=`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", baseDeployYAML, baseConfigYAML),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", targetStatefulSetYAML, targetSecretYAML),
	}

	diffs, err := GenerateAppDiffs(baseApps, targetApps, 10, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("expected 1 app diff (modified), got %d", len(diffs))
	}

	d := diffs[0]
	if d.Action != ActionModified {
		t.Errorf("expected action=modified, got %s", d.Action)
	}

	// Both resources changed kind, so we expect 2 resource diffs
	if len(d.Resources) != 2 {
		t.Fatalf("expected 2 resource diffs, got %d", len(d.Resources))
	}

	// Resources are sorted by apiVersion, kind, namespace, name.
	// apps/v1 StatefulSet comes before v1 Secret (by apiVersion)
	statefulSetRes := d.Resources[0]
	secretRes := d.Resources[1]

	if statefulSetRes.Kind != "StatefulSet" || statefulSetRes.Name != "my-app" {
		t.Errorf("expected first resource to be StatefulSet/my-app, got %s/%s", statefulSetRes.Kind, statefulSetRes.Name)
	}
	if !strings.Contains(statefulSetRes.Content, "-kind: Deployment") {
		t.Error("expected StatefulSet diff to show kind change from Deployment")
	}
	if !strings.Contains(statefulSetRes.Content, "+kind: StatefulSet") {
		t.Error("expected StatefulSet diff to show kind change to StatefulSet")
	}

	if secretRes.Kind != "Secret" || secretRes.Name != "my-config" {
		t.Errorf("expected second resource to be Secret/my-config, got %s/%s", secretRes.Kind, secretRes.Name)
	}
	if !strings.Contains(secretRes.Content, "-kind: ConfigMap") {
		t.Error("expected Secret diff to show kind change from ConfigMap")
	}
	if !strings.Contains(secretRes.Content, "+kind: Secret") {
		t.Error("expected Secret diff to show kind change to Secret")
	}
}

// TestFullFlow_KindChange tests that changing a resource's kind (e.g. Deployment → StatefulSet)
// with the same name+namespace produces a single modified diff, not separate deleted+added diffs.
// Resources are matched by name+namespace first (regardless of kind), so a kind change is treated
// as a modification of the existing resource.
func TestFullFlow_KindChange(t *testing.T) {
	baseDeploymentYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app`

	targetStatefulSetYAML := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app`

	baseApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", baseDeploymentYAML),
	}
	targetApps := []extract.ExtractedApp{
		makeAppFromYAML(t, "app-1", "my-app", targetStatefulSetYAML),
	}

	diffs, err := GenerateAppDiffs(baseApps, targetApps, 10, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resources are matched by name+namespace across kinds, so the Deployment→StatefulSet
	// change produces a single modified diff showing the kind change.
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff (modified), got %d", len(diffs))
	}

	modifiedDiff := diffs[0]

	if modifiedDiff.Action != ActionModified {
		t.Errorf("expected diff action=modified, got %s", modifiedDiff.Action)
	}
	if modifiedDiff.PrettyName() != "my-app" {
		t.Errorf("expected app name=my-app, got %s", modifiedDiff.PrettyName())
	}
	if len(modifiedDiff.Resources) != 1 {
		t.Fatalf("expected 1 resource in modified diff, got %d", len(modifiedDiff.Resources))
	}

	// The resource header uses the target kind (StatefulSet)
	resource := modifiedDiff.Resources[0]
	if resource.Kind != "StatefulSet" {
		t.Errorf("expected resource kind=StatefulSet, got %s", resource.Kind)
	}
	if resource.Name != "my-app" {
		t.Errorf("expected resource name=my-app, got %s", resource.Name)
	}
	if resource.Header() != "StatefulSet/my-app (default)" {
		t.Errorf("expected header 'StatefulSet/my-app (default)', got %q", resource.Header())
	}

	// The diff content should show the kind change
	expectedDiff := ` apiVersion: apps/v1
-kind: Deployment
+kind: StatefulSet
 metadata:
   name: my-app
   namespace: default
 spec:
   replicas: 3
   selector:
     matchLabels:
       app: my-app
`
	if resource.Content != expectedDiff {
		t.Errorf("Kind change diff mismatch.\n\nExpected:\n%s\n\nActual:\n%s", expectedDiff, resource.Content)
	}
}
