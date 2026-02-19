package matching

import (
	"maps"
	"strings"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/resource_filter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Helper to create an unstructured resource
func makeResource(apiVersion, kind, namespace, name string, extraData map[string]any) unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
	}
	maps.Copy(obj, extraData)
	return unstructured.Unstructured{Object: obj}
}

// Helper to create an ExtractedApp
func makeApp(id, name string, manifests []unstructured.Unstructured) extract.ExtractedApp {
	return extract.ExtractedApp{
		Id:         id,
		Name:       name,
		SourcePath: "/path/to/" + name,
		Manifests:  manifests,
		Branch:     git.Base,
	}
}

func TestMatchApps_ExactIdMatch(t *testing.T) {
	// Two apps with the same ID and same content should match
	deployment := makeResource("apps/v1", "Deployment", "default", "my-deploy", nil)

	baseApps := []extract.ExtractedApp{
		makeApp("app-1", "my-app", []unstructured.Unstructured{deployment}),
	}
	targetApps := []extract.ExtractedApp{
		makeApp("app-1", "my-app", []unstructured.Unstructured{deployment}),
	}

	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0].Base == nil || pairs[0].Target == nil {
		t.Fatal("expected both base and target to be non-nil")
	}
	// They should match because content is identical, not because of ID
	if pairs[0].Base.Id != "app-1" || pairs[0].Target.Id != "app-1" {
		t.Errorf("expected matched IDs to be app-1, got %s and %s", pairs[0].Base.Id, pairs[0].Target.Id)
	}
}

func TestMatchApps_DeletedApp(t *testing.T) {
	// App exists in base but not in target = deleted
	deployment := makeResource("apps/v1", "Deployment", "default", "my-deploy", nil)

	baseApps := []extract.ExtractedApp{
		makeApp("app-1", "my-app", []unstructured.Unstructured{deployment}),
	}
	targetApps := []extract.ExtractedApp{}

	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0].Base == nil {
		t.Fatal("expected base to be non-nil")
	}
	if pairs[0].Target != nil {
		t.Fatal("expected target to be nil (deleted app)")
	}
}

func TestMatchApps_AddedApp(t *testing.T) {
	// App exists in target but not in base = added
	deployment := makeResource("apps/v1", "Deployment", "default", "my-deploy", nil)

	baseApps := []extract.ExtractedApp{}
	targetApps := []extract.ExtractedApp{
		makeApp("app-1", "my-app", []unstructured.Unstructured{deployment}),
	}

	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0].Base != nil {
		t.Fatal("expected base to be nil (added app)")
	}
	if pairs[0].Target == nil {
		t.Fatal("expected target to be non-nil")
	}
}

func TestMatchApps_RenamedAppWithSimilarContent(t *testing.T) {
	// App is renamed but content is very similar - should match by similarity
	deployment := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app": "my-app",
				},
			},
		},
	})

	// Slightly modified deployment (same structure, small change)
	deploymentModified := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(5), // Changed from 3 to 5
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app": "my-app",
				},
			},
		},
	})

	baseApps := []extract.ExtractedApp{
		makeApp("old-app-id", "my-app", []unstructured.Unstructured{deployment}),
	}
	targetApps := []extract.ExtractedApp{
		makeApp("new-app-id", "my-app", []unstructured.Unstructured{deploymentModified}),
	}

	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0].Base == nil || pairs[0].Target == nil {
		t.Fatal("expected both base and target to be non-nil (matched by similarity)")
	}
	if pairs[0].Base.Id != "old-app-id" {
		t.Errorf("expected base ID to be old-app-id, got %s", pairs[0].Base.Id)
	}
	if pairs[0].Target.Id != "new-app-id" {
		t.Errorf("expected target ID to be new-app-id, got %s", pairs[0].Target.Id)
	}
}

func TestMatchApps_SwappedContent(t *testing.T) {
	// Test that matching is by CONTENT, not by ID
	// If app-1 and app-2 swap their content, we should match by content similarity
	// not by ID, resulting in cross-matching

	deploymentA := makeResource("apps/v1", "Deployment", "default", "deploy-a", map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app": "service-a",
				},
			},
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "container-a",
							"image": "image-a:v1",
						},
					},
				},
			},
		},
	})

	deploymentB := makeResource("apps/v1", "Deployment", "default", "deploy-b", map[string]any{
		"spec": map[string]any{
			"replicas": int64(5),
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app": "service-b",
				},
			},
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "container-b",
							"image": "image-b:v2",
						},
					},
				},
			},
		},
	})

	// Base: app-1 has deploymentA, app-2 has deploymentB
	baseApps := []extract.ExtractedApp{
		makeApp("app-1", "app-one", []unstructured.Unstructured{deploymentA}),
		makeApp("app-2", "app-two", []unstructured.Unstructured{deploymentB}),
	}

	// Target: SWAPPED - app-1 now has deploymentB, app-2 now has deploymentA
	targetApps := []extract.ExtractedApp{
		makeApp("app-1", "app-one", []unstructured.Unstructured{deploymentB}),
		makeApp("app-2", "app-two", []unstructured.Unstructured{deploymentA}),
	}

	pairs := MatchApps(baseApps, targetApps)

	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}

	// Check that matching was done by content, not by ID
	// base app-1 (deploymentA) should match target app-2 (deploymentA)
	// base app-2 (deploymentB) should match target app-1 (deploymentB)
	for _, p := range pairs {
		if p.Base == nil || p.Target == nil {
			t.Fatal("expected both base and target to be non-nil (content-based matching)")
		}

		// The key insight: if base has deploymentA, target should also have deploymentA
		baseHasDeployA := hasResourceNamed(p.Base.Manifests, "deploy-a")
		targetHasDeployA := hasResourceNamed(p.Target.Manifests, "deploy-a")
		baseHasDeployB := hasResourceNamed(p.Base.Manifests, "deploy-b")
		targetHasDeployB := hasResourceNamed(p.Target.Manifests, "deploy-b")

		if baseHasDeployA && !targetHasDeployA {
			t.Errorf("base has deploy-a but target doesn't - should have matched by content")
		}
		if baseHasDeployB && !targetHasDeployB {
			t.Errorf("base has deploy-b but target doesn't - should have matched by content")
		}
	}
}

// hasResourceNamed checks if any manifest has the given name
func hasResourceNamed(manifests []unstructured.Unstructured, name string) bool {
	for _, m := range manifests {
		if m.GetName() == name {
			return true
		}
	}
	return false
}

func TestMatchApps_CompletelyDifferentApps(t *testing.T) {
	// Two completely different apps should not match
	deployment := makeResource("apps/v1", "Deployment", "default", "deploy-a", map[string]any{
		"spec": map[string]any{
			"replicas": int64(1),
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "container-a",
							"image": "image-a:latest",
						},
					},
				},
			},
		},
	})

	configMap := makeResource("v1", "ConfigMap", "other-ns", "config-b", map[string]any{
		"data": map[string]any{
			"key1": "value1",
			"key2": "value2",
		},
	})

	baseApps := []extract.ExtractedApp{
		makeApp("app-a", "app-a", []unstructured.Unstructured{deployment}),
	}
	targetApps := []extract.ExtractedApp{
		makeApp("app-b", "app-b", []unstructured.Unstructured{configMap}),
	}

	pairs := MatchApps(baseApps, targetApps)

	// Should have 2 pairs: one deleted, one added
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs (1 deleted, 1 added), got %d", len(pairs))
	}

	hasDeleted := false
	hasAdded := false
	for _, p := range pairs {
		if p.Base != nil && p.Target == nil {
			hasDeleted = true
		}
		if p.Base == nil && p.Target != nil {
			hasAdded = true
		}
	}

	if !hasDeleted {
		t.Error("expected a deleted app pair")
	}
	if !hasAdded {
		t.Error("expected an added app pair")
	}
}

func TestMatchApps_MultipleApps(t *testing.T) {
	// Test with multiple apps: some match by ID, some by similarity, some added/deleted
	deploy1 := makeResource("apps/v1", "Deployment", "default", "deploy-1", nil)
	deploy2 := makeResource("apps/v1", "Deployment", "default", "deploy-2", nil)
	deploy3 := makeResource("apps/v1", "Deployment", "default", "deploy-3", nil)
	configMap := makeResource("v1", "ConfigMap", "default", "config", nil)

	baseApps := []extract.ExtractedApp{
		makeApp("app-1", "app-one", []unstructured.Unstructured{deploy1}),       // Will match by ID
		makeApp("app-2", "app-two", []unstructured.Unstructured{deploy2}),       // Will be deleted
		makeApp("app-3-old", "app-three", []unstructured.Unstructured{deploy3}), // Will match by similarity
	}
	targetApps := []extract.ExtractedApp{
		makeApp("app-1", "app-one", []unstructured.Unstructured{deploy1}),       // Matches by ID
		makeApp("app-3-new", "app-three", []unstructured.Unstructured{deploy3}), // Matches by similarity
		makeApp("app-4", "app-four", []unstructured.Unstructured{configMap}),    // New app
	}

	pairs := MatchApps(baseApps, targetApps)

	// Should have 4 pairs total
	if len(pairs) != 4 {
		t.Fatalf("expected 4 pairs, got %d", len(pairs))
	}

	// Count types
	matched := 0
	deleted := 0
	added := 0
	for _, p := range pairs {
		switch {
		case p.Base != nil && p.Target != nil:
			matched++
		case p.Base != nil && p.Target == nil:
			deleted++
		case p.Base == nil && p.Target != nil:
			added++
		}
	}

	if matched != 2 {
		t.Errorf("expected 2 matched pairs, got %d", matched)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted pair, got %d", deleted)
	}
	if added != 1 {
		t.Errorf("expected 1 added pair, got %d", added)
	}
}

func TestMatchApps_EmptyLists(t *testing.T) {
	pairs := MatchApps([]extract.ExtractedApp{}, []extract.ExtractedApp{})

	if len(pairs) != 0 {
		t.Errorf("expected 0 pairs for empty inputs, got %d", len(pairs))
	}
}

// Tests for ChangedResources

func TestChangedResources_IdenticalResources(t *testing.T) {
	// When resources are identical, ChangedResources should return empty
	deployment := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
		},
	})

	baseApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deployment})
	targetApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deployment})

	pair := Pair{Base: &baseApp, Target: &targetApp}
	changed := pair.ChangedResources()

	if len(changed) != 0 {
		t.Errorf("expected 0 changed resources for identical apps, got %d", len(changed))
	}
}

func TestChangedResources_ModifiedResource(t *testing.T) {
	// When a resource is modified, it should appear in ChangedResources
	deploymentBase := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
		},
	})
	deploymentTarget := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(5), // Changed
		},
	})

	baseApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deploymentBase})
	targetApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deploymentTarget})

	pair := Pair{Base: &baseApp, Target: &targetApp}
	changed := pair.ChangedResources()

	if len(changed) != 1 {
		t.Fatalf("expected 1 changed resource, got %d", len(changed))
	}
	if changed[0].Base == nil || changed[0].Target == nil {
		t.Error("expected both base and target to be non-nil for modified resource")
	}
}

func TestChangedResources_AddedResource(t *testing.T) {
	// When a resource is added in target
	deployment := makeResource("apps/v1", "Deployment", "default", "my-deploy", nil)
	configMap := makeResource("v1", "ConfigMap", "default", "my-config", nil)

	baseApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deployment})
	targetApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deployment, configMap})

	pair := Pair{Base: &baseApp, Target: &targetApp}
	changed := pair.ChangedResources()

	if len(changed) != 1 {
		t.Fatalf("expected 1 changed resource (the added ConfigMap), got %d", len(changed))
	}
	if changed[0].Base != nil {
		t.Error("expected base to be nil for added resource")
	}
	if changed[0].Target == nil {
		t.Error("expected target to be non-nil for added resource")
	}
	if changed[0].Target.GetKind() != "ConfigMap" {
		t.Errorf("expected added resource to be ConfigMap, got %s", changed[0].Target.GetKind())
	}
}

func TestChangedResources_DeletedResource(t *testing.T) {
	// When a resource is deleted in target
	deployment := makeResource("apps/v1", "Deployment", "default", "my-deploy", nil)
	configMap := makeResource("v1", "ConfigMap", "default", "my-config", nil)

	baseApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deployment, configMap})
	targetApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deployment})

	pair := Pair{Base: &baseApp, Target: &targetApp}
	changed := pair.ChangedResources()

	if len(changed) != 1 {
		t.Fatalf("expected 1 changed resource (the deleted ConfigMap), got %d", len(changed))
	}
	if changed[0].Base == nil {
		t.Error("expected base to be non-nil for deleted resource")
	}
	if changed[0].Target != nil {
		t.Error("expected target to be nil for deleted resource")
	}
	if changed[0].Base.GetKind() != "ConfigMap" {
		t.Errorf("expected deleted resource to be ConfigMap, got %s", changed[0].Base.GetKind())
	}
}

func TestChangedResources_RenamedResource(t *testing.T) {
	// When a resource is renamed but content is similar
	deploymentOld := makeResource("apps/v1", "Deployment", "default", "old-name", map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app": "my-app",
				},
			},
		},
	})
	deploymentNew := makeResource("apps/v1", "Deployment", "default", "new-name", map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app": "my-app",
				},
			},
		},
	})

	baseApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deploymentOld})
	targetApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deploymentNew})

	pair := Pair{Base: &baseApp, Target: &targetApp}
	changed := pair.ChangedResources()

	// Should match by similarity and show as modified (since names differ)
	if len(changed) != 1 {
		t.Fatalf("expected 1 changed resource pair (renamed), got %d", len(changed))
	}
	if changed[0].Base == nil || changed[0].Target == nil {
		t.Error("expected both base and target to be non-nil for renamed resource")
	}
}

func TestChangedResources_AppAdded(t *testing.T) {
	// When the entire app is new (no base)
	deployment := makeResource("apps/v1", "Deployment", "default", "my-deploy", nil)
	targetApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deployment})

	pair := Pair{Base: nil, Target: &targetApp}
	changed := pair.ChangedResources()

	if len(changed) != 1 {
		t.Fatalf("expected 1 resource (all added), got %d", len(changed))
	}
	if changed[0].Base != nil {
		t.Error("expected base to be nil for new app")
	}
	if changed[0].Target == nil {
		t.Error("expected target to be non-nil for new app")
	}
}

func TestChangedResources_AppDeleted(t *testing.T) {
	// When the entire app is deleted (no target)
	deployment := makeResource("apps/v1", "Deployment", "default", "my-deploy", nil)
	baseApp := makeApp("app-1", "my-app", []unstructured.Unstructured{deployment})

	pair := Pair{Base: &baseApp, Target: nil}
	changed := pair.ChangedResources()

	if len(changed) != 1 {
		t.Fatalf("expected 1 resource (all deleted), got %d", len(changed))
	}
	if changed[0].Base == nil {
		t.Error("expected base to be non-nil for deleted app")
	}
	if changed[0].Target != nil {
		t.Error("expected target to be nil for deleted app")
	}
}

func TestChangedResources_MixedChanges(t *testing.T) {
	// Complex scenario: one resource unchanged, one modified, one added, one deleted
	deployUnchanged := makeResource("apps/v1", "Deployment", "default", "unchanged", map[string]any{
		"spec": map[string]any{"replicas": int64(1)},
	})
	deployModifiedBase := makeResource("apps/v1", "Deployment", "default", "modified", map[string]any{
		"spec": map[string]any{"replicas": int64(2)},
	})
	deployModifiedTarget := makeResource("apps/v1", "Deployment", "default", "modified", map[string]any{
		"spec": map[string]any{"replicas": int64(5)},
	})
	configDeleted := makeResource("v1", "ConfigMap", "default", "deleted", nil)
	secretAdded := makeResource("v1", "Secret", "default", "added", nil)

	baseApp := makeApp("app-1", "my-app", []unstructured.Unstructured{
		deployUnchanged, deployModifiedBase, configDeleted,
	})
	targetApp := makeApp("app-1", "my-app", []unstructured.Unstructured{
		deployUnchanged, deployModifiedTarget, secretAdded,
	})

	pair := Pair{Base: &baseApp, Target: &targetApp}
	changed := pair.ChangedResources()

	// Should have 3 changes: modified, deleted, added (not the unchanged one)
	if len(changed) != 3 {
		t.Fatalf("expected 3 changed resources, got %d", len(changed))
	}

	hasModified := false
	hasDeleted := false
	hasAdded := false

	for _, rp := range changed {
		switch {
		case rp.Base != nil && rp.Target != nil:
			hasModified = true
			if rp.Base.GetName() != "modified" {
				t.Errorf("expected modified resource to be 'modified', got %s", rp.Base.GetName())
			}
		case rp.Base != nil && rp.Target == nil:
			hasDeleted = true
			if rp.Base.GetName() != "deleted" {
				t.Errorf("expected deleted resource to be 'deleted', got %s", rp.Base.GetName())
			}
		case rp.Base == nil && rp.Target != nil:
			hasAdded = true
			if rp.Target.GetName() != "added" {
				t.Errorf("expected added resource to be 'added', got %s", rp.Target.GetName())
			}
		}
	}

	if !hasModified {
		t.Error("expected to find a modified resource")
	}
	if !hasDeleted {
		t.Error("expected to find a deleted resource")
	}
	if !hasAdded {
		t.Error("expected to find an added resource")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		setA     map[string]bool
		setB     map[string]bool
		expected float64
	}{
		{
			name:     "identical sets",
			setA:     map[string]bool{"a": true, "b": true, "c": true},
			setB:     map[string]bool{"a": true, "b": true, "c": true},
			expected: 1.0,
		},
		{
			name:     "completely different",
			setA:     map[string]bool{"a": true, "b": true},
			setB:     map[string]bool{"c": true, "d": true},
			expected: 0.0,
		},
		{
			name:     "50% overlap",
			setA:     map[string]bool{"a": true, "b": true},
			setB:     map[string]bool{"b": true, "c": true},
			expected: 1.0 / 3.0, // intersection=1, union=3
		},
		{
			name:     "empty sets",
			setA:     map[string]bool{},
			setB:     map[string]bool{},
			expected: 1.0,
		},
		{
			name:     "one empty set",
			setA:     map[string]bool{"a": true},
			setB:     map[string]bool{},
			expected: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := jaccardSimilarity(tc.setA, tc.setB)
			if diff := result - tc.expected; diff > 0.001 || diff < -0.001 {
				t.Errorf("expected %f, got %f", tc.expected, result)
			}
		})
	}
}

func TestResourceSetSimilarity(t *testing.T) {
	deploy := makeResource("apps/v1", "Deployment", "default", "my-deploy", nil)
	service := makeResource("v1", "Service", "default", "my-service", nil)
	configMap := makeResource("v1", "ConfigMap", "default", "my-config", nil)

	tests := []struct {
		name       string
		manifestsA []unstructured.Unstructured
		manifestsB []unstructured.Unstructured
		minScore   float64 // Use minimum expected score since exact values depend on content
		maxScore   float64
	}{
		{
			name:       "identical resources",
			manifestsA: []unstructured.Unstructured{deploy, service},
			manifestsB: []unstructured.Unstructured{deploy, service},
			minScore:   0.9,
			maxScore:   1.0,
		},
		{
			name:       "completely different kinds",
			manifestsA: []unstructured.Unstructured{deploy},
			manifestsB: []unstructured.Unstructured{configMap},
			minScore:   0.0,
			maxScore:   0.2,
		},
		{
			name:       "empty vs non-empty",
			manifestsA: []unstructured.Unstructured{},
			manifestsB: []unstructured.Unstructured{deploy},
			minScore:   0.0,
			maxScore:   0.0,
		},
		{
			name:       "both empty",
			manifestsA: []unstructured.Unstructured{},
			manifestsB: []unstructured.Unstructured{},
			minScore:   1.0,
			maxScore:   1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := resourceSetSimilarity(tc.manifestsA, tc.manifestsB)
			if result < tc.minScore || result > tc.maxScore {
				t.Errorf("expected score between %f and %f, got %f", tc.minScore, tc.maxScore, result)
			}
		})
	}
}

func TestAppSimilarity(t *testing.T) {
	deploy := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
		},
	})

	deployModified := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(5),
		},
	})

	configMap := makeResource("v1", "ConfigMap", "default", "config", map[string]any{
		"data": map[string]any{
			"key": "value",
		},
	})

	tests := []struct {
		name     string
		appA     extract.ExtractedApp
		appB     extract.ExtractedApp
		minScore float64
		maxScore float64
	}{
		{
			name:     "identical apps",
			appA:     makeApp("app-1", "my-app", []unstructured.Unstructured{deploy}),
			appB:     makeApp("app-1", "my-app", []unstructured.Unstructured{deploy}),
			minScore: 0.9,
			maxScore: 1.0,
		},
		{
			name:     "same name, slightly different content",
			appA:     makeApp("app-1", "my-app", []unstructured.Unstructured{deploy}),
			appB:     makeApp("app-2", "my-app", []unstructured.Unstructured{deployModified}),
			minScore: 0.7,
			maxScore: 1.0,
		},
		{
			name:     "completely different",
			appA:     makeApp("app-1", "app-a", []unstructured.Unstructured{deploy}),
			appB:     makeApp("app-2", "app-b", []unstructured.Unstructured{configMap}),
			minScore: 0.0,
			maxScore: 0.3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := appSimilarity(&tc.appA, &tc.appB)
			if result < tc.minScore || result > tc.maxScore {
				t.Errorf("expected score between %f and %f, got %f", tc.minScore, tc.maxScore, result)
			}
		})
	}
}

// Tests for ResourcePair.Diff

func TestResourcePair_Diff_ModifiedResource(t *testing.T) {
	baseResource := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
		},
	})
	targetResource := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(5),
		},
	})

	rp := ResourcePair{Base: &baseResource, Target: &targetResource}
	result, err := rp.Diff(3)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected non-empty diff content for modified resource")
	}
	if result.AddedLines == 0 && result.DeletedLines == 0 {
		t.Error("expected some added or deleted lines")
	}
	// Should contain the change from 3 to 5
	if !strings.Contains(result.Content, "-") || !strings.Contains(result.Content, "+") {
		t.Error("expected diff to contain additions and deletions")
	}
}

func TestResourcePair_Diff_AddedResource(t *testing.T) {
	targetResource := makeResource("v1", "ConfigMap", "default", "my-config", map[string]any{
		"data": map[string]any{
			"key": "value",
		},
	})

	rp := ResourcePair{Base: nil, Target: &targetResource}
	result, err := rp.Diff(3)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected non-empty diff content for added resource")
	}
	if result.AddedLines == 0 {
		t.Error("expected added lines for new resource")
	}
	if result.DeletedLines != 0 {
		t.Error("expected no deleted lines for new resource")
	}
	// All lines should be additions
	for line := range strings.SplitSeq(result.Content, "\n") {
		if line != "" && !strings.HasPrefix(line, "+") {
			t.Errorf("expected all lines to be additions, got: %s", line)
		}
	}
}

func TestResourcePair_Diff_DeletedResource(t *testing.T) {
	baseResource := makeResource("v1", "ConfigMap", "default", "my-config", map[string]any{
		"data": map[string]any{
			"key": "value",
		},
	})

	rp := ResourcePair{Base: &baseResource, Target: nil}
	result, err := rp.Diff(3)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content == "" {
		t.Error("expected non-empty diff content for deleted resource")
	}
	if result.DeletedLines == 0 {
		t.Error("expected deleted lines for removed resource")
	}
	if result.AddedLines != 0 {
		t.Error("expected no added lines for removed resource")
	}
	// All lines should be deletions
	for line := range strings.SplitSeq(result.Content, "\n") {
		if line != "" && !strings.HasPrefix(line, "-") {
			t.Errorf("expected all lines to be deletions, got: %s", line)
		}
	}
}

func TestResourcePair_Diff_BothNil(t *testing.T) {
	rp := ResourcePair{Base: nil, Target: nil}
	result, err := rp.Diff(3)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "" {
		t.Errorf("expected empty diff for both nil, got: %s", result.Content)
	}
	if result.AddedLines != 0 || result.DeletedLines != 0 {
		t.Error("expected zero added/deleted lines for both nil")
	}
}

func TestResourcePair_Diff_IdenticalResources(t *testing.T) {
	resource := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
		},
	})

	rp := ResourcePair{Base: &resource, Target: &resource}
	result, err := rp.Diff(3)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "" {
		t.Errorf("expected empty diff for identical resources, got: %s", result.Content)
	}
	if result.AddedLines != 0 || result.DeletedLines != 0 {
		t.Error("expected zero added/deleted lines for identical resources")
	}
}

func TestResourcePair_Diff_ContextLines(t *testing.T) {
	// Create resources with many lines to test context handling
	baseResource := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(3),
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app":     "my-app",
					"version": "v1",
				},
			},
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "main",
							"image": "nginx:1.19",
							"ports": []any{
								map[string]any{
									"containerPort": int64(80),
								},
							},
						},
					},
				},
			},
		},
	})

	targetResource := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{
			"replicas": int64(5), // Changed
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app":     "my-app",
					"version": "v1",
				},
			},
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "main",
							"image": "nginx:1.20", // Changed
							"ports": []any{
								map[string]any{
									"containerPort": int64(80),
								},
							},
						},
					},
				},
			},
		},
	})

	// Test with different context line values
	rp := ResourcePair{Base: &baseResource, Target: &targetResource}

	// With 0 context lines, should only show changed lines
	result0, _ := rp.Diff(0)
	// With 10 context lines, should show more surrounding context
	result10, _ := rp.Diff(10)

	// Both should have the same number of actual changes
	if result0.AddedLines != result10.AddedLines {
		t.Errorf("added lines should be same regardless of context: %d vs %d", result0.AddedLines, result10.AddedLines)
	}
	if result0.DeletedLines != result10.DeletedLines {
		t.Errorf("deleted lines should be same regardless of context: %d vs %d", result0.DeletedLines, result10.DeletedLines)
	}

	// But more context should result in more total content
	if len(result10.Content) <= len(result0.Content) {
		t.Error("expected more content with more context lines")
	}
}

// Tests for resourceMatchesIgnoreRules

func TestResourceMatchesIgnoreRules_BaseMatches(t *testing.T) {
	base := makeResource("apps/v1", "Deployment", "default", "my-deploy", nil)
	rp := ResourcePair{Base: &base, Target: nil}

	rules := []resource_filter.IgnoreResourceRule{
		{Group: "apps", Kind: "Deployment", Name: "my-deploy"},
	}

	if !resourceMatchesIgnoreRules(&rp, rules) {
		t.Error("expected resource to match ignore rule via base")
	}
}

func TestResourceMatchesIgnoreRules_TargetMatches(t *testing.T) {
	target := makeResource("v1", "ConfigMap", "default", "my-config", nil)
	rp := ResourcePair{Base: nil, Target: &target}

	rules := []resource_filter.IgnoreResourceRule{
		{Group: "", Kind: "ConfigMap", Name: "my-config"},
	}

	if !resourceMatchesIgnoreRules(&rp, rules) {
		t.Error("expected resource to match ignore rule via target")
	}
}

func TestResourceMatchesIgnoreRules_NoMatch(t *testing.T) {
	base := makeResource("apps/v1", "Deployment", "default", "my-deploy", nil)
	target := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{"replicas": int64(5)},
	})
	rp := ResourcePair{Base: &base, Target: &target}

	rules := []resource_filter.IgnoreResourceRule{
		{Group: "", Kind: "ConfigMap", Name: "*"},
	}

	if resourceMatchesIgnoreRules(&rp, rules) {
		t.Error("expected resource NOT to match ignore rule")
	}
}

func TestResourceMatchesIgnoreRules_WildcardKind(t *testing.T) {
	base := makeResource("v1", "Secret", "default", "my-secret", nil)
	rp := ResourcePair{Base: &base, Target: nil}

	rules := []resource_filter.IgnoreResourceRule{
		{Group: "*", Kind: "Secret", Name: "*"},
	}

	if !resourceMatchesIgnoreRules(&rp, rules) {
		t.Error("expected wildcard rule to match")
	}
}

func TestResourceMatchesIgnoreRules_BothNil(t *testing.T) {
	rp := ResourcePair{Base: nil, Target: nil}

	rules := []resource_filter.IgnoreResourceRule{
		{Group: "*", Kind: "*", Name: "*"},
	}

	if resourceMatchesIgnoreRules(&rp, rules) {
		t.Error("expected no match when both base and target are nil")
	}
}

// Tests for buildCombinedResourceDiff with ignore rules

func TestBuildCombinedResourceDiff_SkippedResourceLine(t *testing.T) {
	// A modified resource that matches an ignore rule should produce a "Skipped Resource" line
	base := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{"replicas": int64(3)},
	})
	target := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{"replicas": int64(5)},
	})

	resources := []ResourcePair{{Base: &base, Target: &target}}
	rules := []resource_filter.IgnoreResourceRule{
		{Group: "apps", Kind: "Deployment", Name: "my-deploy"},
	}

	content, added, deleted, err := buildCombinedResourceDiff(resources, 3, nil, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(content, "Skipped Resource:") {
		t.Errorf("expected 'Skipped Resource:' in output, got: %s", content)
	}
	if !strings.Contains(content, "Kind: Deployment") {
		t.Errorf("expected 'Kind: Deployment' in output, got: %s", content)
	}
	if !strings.Contains(content, "Name: my-deploy") {
		t.Errorf("expected 'Name: my-deploy' in output, got: %s", content)
	}
	// Skipped resources should not count as added/deleted
	if added != 0 || deleted != 0 {
		t.Errorf("expected 0 added/deleted for skipped resource, got added=%d deleted=%d", added, deleted)
	}
}

func TestBuildCombinedResourceDiff_MixSkippedAndNormal(t *testing.T) {
	// One resource is ignored, another is not — should see both in output
	deployBase := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{"replicas": int64(3)},
	})
	deployTarget := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{"replicas": int64(5)},
	})
	configBase := makeResource("v1", "ConfigMap", "default", "my-config", map[string]any{
		"data": map[string]any{"key": "old-value"},
	})
	configTarget := makeResource("v1", "ConfigMap", "default", "my-config", map[string]any{
		"data": map[string]any{"key": "new-value"},
	})

	resources := []ResourcePair{
		{Base: &deployBase, Target: &deployTarget},
		{Base: &configBase, Target: &configTarget},
	}
	// Only ignore the Deployment, not the ConfigMap
	rules := []resource_filter.IgnoreResourceRule{
		{Group: "apps", Kind: "Deployment", Name: "*"},
	}

	content, added, deleted, err := buildCombinedResourceDiff(resources, 3, nil, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have the skipped line for Deployment
	if !strings.Contains(content, "Skipped Resource:") {
		t.Errorf("expected 'Skipped Resource:' for ignored Deployment, got: %s", content)
	}
	if !strings.Contains(content, "Kind: Deployment") {
		t.Errorf("expected 'Kind: Deployment' in skipped line, got: %s", content)
	}

	// Should have actual diff for ConfigMap (old-value -> new-value)
	if !strings.Contains(content, "old-value") || !strings.Contains(content, "new-value") {
		t.Errorf("expected ConfigMap diff with old-value/new-value in output, got: %s", content)
	}

	// Only the ConfigMap change should count
	if added == 0 || deleted == 0 {
		t.Errorf("expected nonzero added/deleted from ConfigMap diff, got added=%d deleted=%d", added, deleted)
	}
}

func TestBuildCombinedResourceDiff_NoIgnoreRules(t *testing.T) {
	// Without ignore rules, changed resources should produce normal diffs
	base := makeResource("v1", "ConfigMap", "default", "my-config", map[string]any{
		"data": map[string]any{"key": "old"},
	})
	target := makeResource("v1", "ConfigMap", "default", "my-config", map[string]any{
		"data": map[string]any{"key": "new"},
	})

	resources := []ResourcePair{{Base: &base, Target: &target}}

	content, added, deleted, err := buildCombinedResourceDiff(resources, 3, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(content, "Skipped Resource:") {
		t.Errorf("expected no 'Skipped Resource:' without ignore rules, got: %s", content)
	}
	if added == 0 || deleted == 0 {
		t.Errorf("expected nonzero added/deleted, got added=%d deleted=%d", added, deleted)
	}
}

func TestBuildCombinedResourceDiff_AllSkipped(t *testing.T) {
	// When all resources are ignored, output should only contain skipped lines
	deploy := makeResource("apps/v1", "Deployment", "default", "my-deploy", map[string]any{
		"spec": map[string]any{"replicas": int64(3)},
	})
	config := makeResource("v1", "ConfigMap", "default", "my-config", map[string]any{
		"data": map[string]any{"key": "value"},
	})

	// Added deployment + deleted configmap — both ignored
	resources := []ResourcePair{
		{Base: nil, Target: &deploy},
		{Base: &config, Target: nil},
	}
	rules := []resource_filter.IgnoreResourceRule{
		{Group: "*", Kind: "*", Name: "*"},
	}

	content, added, deleted, err := buildCombinedResourceDiff(resources, 3, nil, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have two skipped lines
	count := strings.Count(content, "Skipped Resource:")
	if count != 2 {
		t.Errorf("expected 2 'Skipped Resource:' lines, got %d in output: %s", count, content)
	}

	if added != 0 || deleted != 0 {
		t.Errorf("expected 0 added/deleted when all resources skipped, got added=%d deleted=%d", added, deleted)
	}
}

func TestBuildCombinedResourceDiff_SkippedAddedResource(t *testing.T) {
	// A newly added resource that matches an ignore rule should show as skipped
	target := makeResource("v1", "Secret", "default", "my-secret", map[string]any{
		"data": map[string]any{"password": "hunter2"},
	})

	resources := []ResourcePair{{Base: nil, Target: &target}}
	rules := []resource_filter.IgnoreResourceRule{
		{Group: "*", Kind: "Secret", Name: "*"},
	}

	content, added, deleted, err := buildCombinedResourceDiff(resources, 3, nil, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(content, "Skipped Resource:") {
		t.Errorf("expected 'Skipped Resource:' for added ignored resource, got: %s", content)
	}
	if !strings.Contains(content, "Kind: Secret") {
		t.Errorf("expected 'Kind: Secret' in skipped line, got: %s", content)
	}
	if added != 0 || deleted != 0 {
		t.Errorf("expected 0 added/deleted for skipped resource, got added=%d deleted=%d", added, deleted)
	}
}

func TestBuildCombinedResourceDiff_SkippedDeletedResource(t *testing.T) {
	// A deleted resource that matches an ignore rule should show as skipped
	base := makeResource("v1", "Secret", "default", "my-secret", map[string]any{
		"data": map[string]any{"password": "hunter2"},
	})

	resources := []ResourcePair{{Base: &base, Target: nil}}
	rules := []resource_filter.IgnoreResourceRule{
		{Group: "*", Kind: "Secret", Name: "*"},
	}

	content, added, deleted, err := buildCombinedResourceDiff(resources, 3, nil, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(content, "Skipped Resource:") {
		t.Errorf("expected 'Skipped Resource:' for deleted ignored resource, got: %s", content)
	}
	if added != 0 || deleted != 0 {
		t.Errorf("expected 0 added/deleted for skipped resource, got added=%d deleted=%d", added, deleted)
	}
}

func TestBuildCombinedResourceDiff_SkippedResourceFormat(t *testing.T) {
	// Verify the exact format of the skipped resource line
	base := makeResource("apiextensions.k8s.io/v1", "CustomResourceDefinition", "", "apps.example.com", nil)
	target := makeResource("apiextensions.k8s.io/v1", "CustomResourceDefinition", "", "apps.example.com", map[string]any{
		"spec": map[string]any{"group": "example.com"},
	})

	resources := []ResourcePair{{Base: &base, Target: &target}}
	rules := []resource_filter.IgnoreResourceRule{
		{Group: "*", Kind: "CustomResourceDefinition", Name: "*"},
	}

	content, _, _, err := buildCombinedResourceDiff(resources, 3, nil, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := " Skipped Resource: [ApiVersion: apiextensions.k8s.io/v1, Kind: CustomResourceDefinition, Name: apps.example.com]"
	if !strings.Contains(content, expected) {
		t.Errorf("expected exact format %q, got: %s", expected, content)
	}
}
