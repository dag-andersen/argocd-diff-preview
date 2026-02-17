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
