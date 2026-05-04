package duplicates

import (
	"fmt"
	"testing"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// Helper function to create test YAML
func createTestYAML(kind, name, namespace string) *unstructured.Unstructured {
	yamlStr := `apiVersion: argoproj.io/v1alpha1
kind: ` + kind + `
metadata:
  name: ` + name + `
  namespace: ` + namespace + `
spec:
  destination:
    namespace: ` + namespace

	var obj unstructured.Unstructured
	err := yaml.Unmarshal([]byte(yamlStr), &obj)
	if err != nil {
		panic(err)
	}
	return &obj
}

func createTestYAMLWithAnnotations(kind, name, namespace string, annotations map[string]string) *unstructured.Unstructured {
	annotationLines := ""
	if len(annotations) > 0 {
		annotationLines = "\n  annotations:"
		for key, value := range annotations {
			annotationLines += "\n    " + key + ": \"" + value + "\""
		}
	}

	yamlStr := `apiVersion: argoproj.io/v1alpha1
kind: ` + kind + `
metadata:
  name: ` + name + `
  namespace: ` + namespace + annotationLines + `
spec:
  destination:
    namespace: ` + namespace

	var obj unstructured.Unstructured
	err := yaml.Unmarshal([]byte(yamlStr), &obj)
	if err != nil {
		panic(err)
	}
	return &obj
}

// Helper function to create test ArgoResource
func createTestArgoResource(kind, name, fileName string, branch git.BranchType) argoapplication.ArgoResource {
	yamlObj := createTestYAML(kind, name, "default")
	var appKind argoapplication.ApplicationKind
	if kind == "Application" {
		appKind = argoapplication.Application
	} else {
		appKind = argoapplication.ApplicationSet
	}

	return *argoapplication.NewArgoResource(yamlObj, appKind, name, name, fileName, branch)
}

func createTestArgoResourceWithAnnotations(kind, name, fileName string, branch git.BranchType, annotations map[string]string) argoapplication.ArgoResource {
	yamlObj := createTestYAMLWithAnnotations(kind, name, "default", annotations)
	var appKind argoapplication.ApplicationKind
	if kind == "Application" {
		appKind = argoapplication.Application
	} else {
		appKind = argoapplication.ApplicationSet
	}

	return *argoapplication.NewArgoResource(yamlObj, appKind, name, name, fileName, branch)
}

// Helper function to create ArgoSelection from apps
func createArgoSelection(apps []argoapplication.ArgoResource) *argoapplication.ArgoSelection {
	return &argoapplication.ArgoSelection{
		SelectedApps: apps,
		SkippedApps:  []argoapplication.ArgoResource{},
	}
}

func TestFilterDuplicates_BasicFunctionality(t *testing.T) {
	// Create test apps
	app1 := createTestArgoResource("Application", "app1", "app1.yaml", git.Base)
	app2 := createTestArgoResource("Application", "app2", "app2.yaml", git.Base)
	app3 := createTestArgoResource("Application", "app3", "app3.yaml", git.Base)

	apps := createArgoSelection([]argoapplication.ArgoResource{app1, app2, app3})

	// Create duplicates (app1 and app3 are duplicates)
	duplicates := []*unstructured.Unstructured{
		app1.Yaml,
		app3.Yaml,
	}

	// Filter duplicates
	result := filterDuplicates(apps, duplicates)

	// Should only return app2
	assert.Len(t, result.SelectedApps, 1)
	assert.Equal(t, "app2", result.SelectedApps[0].Name)
	// Duplicates should be moved to SkippedApps
	assert.Len(t, result.SkippedApps, 2)
}

func TestFilterDuplicates_NoDuplicates(t *testing.T) {
	// Create test apps
	app1 := createTestArgoResource("Application", "app1", "app1.yaml", git.Base)
	app2 := createTestArgoResource("Application", "app2", "app2.yaml", git.Base)

	apps := createArgoSelection([]argoapplication.ArgoResource{app1, app2})

	// Create duplicates that don't match any apps
	app3 := createTestArgoResource("Application", "app3", "app3.yaml", git.Base)
	duplicates := []*unstructured.Unstructured{app3.Yaml}

	// Filter duplicates
	result := filterDuplicates(apps, duplicates)

	// Should return all apps
	assert.Len(t, result.SelectedApps, 2)
	assert.Equal(t, "app1", result.SelectedApps[0].Name)
	assert.Equal(t, "app2", result.SelectedApps[1].Name)
	assert.Len(t, result.SkippedApps, 0)
}

func TestFilterDuplicates_AllDuplicates(t *testing.T) {
	// Create test apps
	app1 := createTestArgoResource("Application", "app1", "app1.yaml", git.Base)
	app2 := createTestArgoResource("Application", "app2", "app2.yaml", git.Base)

	apps := createArgoSelection([]argoapplication.ArgoResource{app1, app2})

	// All apps are duplicates
	duplicates := []*unstructured.Unstructured{
		app1.Yaml,
		app2.Yaml,
	}

	// Filter duplicates
	result := filterDuplicates(apps, duplicates)

	// Should return empty list
	assert.Len(t, result.SelectedApps, 0)
	assert.Len(t, result.SkippedApps, 2)
}

func TestFilterDuplicates_EmptyInputs(t *testing.T) {
	t.Run("empty apps", func(t *testing.T) {
		apps := createArgoSelection([]argoapplication.ArgoResource{})
		app1 := createTestArgoResource("Application", "app1", "app1.yaml", git.Base)
		duplicates := []*unstructured.Unstructured{app1.Yaml}

		result := filterDuplicates(apps, duplicates)
		assert.Len(t, result.SelectedApps, 0)
		assert.Len(t, result.SkippedApps, 0)
	})

	t.Run("empty duplicates", func(t *testing.T) {
		app1 := createTestArgoResource("Application", "app1", "app1.yaml", git.Base)
		apps := createArgoSelection([]argoapplication.ArgoResource{app1})
		duplicates := []*unstructured.Unstructured{}

		result := filterDuplicates(apps, duplicates)
		assert.Len(t, result.SelectedApps, 1)
		assert.Equal(t, "app1", result.SelectedApps[0].Name)
		assert.Len(t, result.SkippedApps, 0)
	})

	t.Run("both empty", func(t *testing.T) {
		apps := createArgoSelection([]argoapplication.ArgoResource{})
		duplicates := []*unstructured.Unstructured{}

		result := filterDuplicates(apps, duplicates)
		assert.Len(t, result.SelectedApps, 0)
		assert.Len(t, result.SkippedApps, 0)
	})
}

func TestFilterDuplicates_MixedKinds(t *testing.T) {
	// Create mixed Application and ApplicationSet resources
	app1 := createTestArgoResource("Application", "app1", "app1.yaml", git.Base)
	appSet1 := createTestArgoResource("ApplicationSet", "appset1", "appset1.yaml", git.Base)
	app2 := createTestArgoResource("Application", "app2", "app2.yaml", git.Base)

	apps := createArgoSelection([]argoapplication.ArgoResource{app1, appSet1, app2})

	// Only app1 is a duplicate
	duplicates := []*unstructured.Unstructured{app1.Yaml}

	result := filterDuplicates(apps, duplicates)

	// Should return appSet1 and app2
	assert.Len(t, result.SelectedApps, 2)
	names := []string{result.SelectedApps[0].Name, result.SelectedApps[1].Name}
	assert.Contains(t, names, "appset1")
	assert.Contains(t, names, "app2")
	assert.Len(t, result.SkippedApps, 1)
}

func TestFilterDuplicates_Performance(t *testing.T) {
	// Test with a reasonable number of apps to verify O(n+m) complexity
	const numApps = 100
	const numDuplicates = 50

	// Create many apps
	var appsList []argoapplication.ArgoResource
	for i := range numApps {
		app := createTestArgoResource("Application",
			fmt.Sprintf("app%d", i),
			fmt.Sprintf("app%d.yaml", i),
			git.Base)
		appsList = append(appsList, app)
	}

	apps := createArgoSelection(appsList)

	// Create duplicates (first half of apps)
	var duplicates []*unstructured.Unstructured
	for i := range numDuplicates {
		duplicates = append(duplicates, appsList[i].Yaml)
	}

	// Measure time
	start := time.Now()
	result := filterDuplicates(apps, duplicates)
	duration := time.Since(start)

	// Verify correctness
	assert.Len(t, result.SelectedApps, numApps-numDuplicates)
	assert.Len(t, result.SkippedApps, numDuplicates)

	// Performance should be reasonable (less than 100ms for 100 apps)
	assert.Less(t, duration, 100*time.Millisecond,
		"filterDuplicates took too long: %v", duration)

	t.Logf("Filtered %d apps with %d duplicates in %v", numApps, numDuplicates, duration)
}

func TestFilterDuplicates_IdenticalContent(t *testing.T) {
	// Create two apps with identical YAML content but different instances
	app1 := createTestArgoResource("Application", "same-app", "app1.yaml", git.Base)
	app2 := createTestArgoResource("Application", "same-app", "app2.yaml", git.Base)

	apps := createArgoSelection([]argoapplication.ArgoResource{app1, app2})

	// Use one as duplicate
	duplicates := []*unstructured.Unstructured{app1.Yaml}

	result := filterDuplicates(apps, duplicates)

	// Both should be filtered since they have identical YAML content
	// (app1 matches directly, app2 has same content so also matches)
	assert.Len(t, result.SelectedApps, 0)
	assert.Len(t, result.SkippedApps, 2)
}

func TestFilterDuplicates_PreservesExistingSkippedApps(t *testing.T) {
	// Create test apps
	app1 := createTestArgoResource("Application", "app1", "app1.yaml", git.Base)
	app2 := createTestArgoResource("Application", "app2", "app2.yaml", git.Base)
	skippedApp := createTestArgoResource("Application", "skipped", "skipped.yaml", git.Base)

	apps := &argoapplication.ArgoSelection{
		SelectedApps: []argoapplication.ArgoResource{app1, app2},
		SkippedApps:  []argoapplication.ArgoResource{skippedApp},
	}

	// app1 is a duplicate
	duplicates := []*unstructured.Unstructured{app1.Yaml}

	result := filterDuplicates(apps, duplicates)

	// Should preserve existing skipped apps and add new ones
	assert.Len(t, result.SelectedApps, 1)
	assert.Equal(t, "app2", result.SelectedApps[0].Name)
	assert.Len(t, result.SkippedApps, 2) // skippedApp + app1
}

func TestRemoveIdenticalCopiesBetweenBranches_ApplicationSetWithRenderAlwaysIsNotRemoved(t *testing.T) {
	baseAppSet := createTestArgoResourceWithAnnotations(
		"ApplicationSet",
		"appset-1",
		"appset.yaml",
		git.Base,
		map[string]string{"argocd-diff-preview/render": "always"},
	)
	targetAppSet := createTestArgoResourceWithAnnotations(
		"ApplicationSet",
		"appset-1",
		"appset.yaml",
		git.Target,
		map[string]string{"argocd-diff-preview/render": "always"},
	)

	base := createArgoSelection([]argoapplication.ArgoResource{baseAppSet})
	target := createArgoSelection([]argoapplication.ArgoResource{targetAppSet})

	baseResult, targetResult := RemoveIdenticalCopiesBetweenBranches(base, target)

	assert.Len(t, baseResult.SelectedApps, 1)
	assert.Len(t, targetResult.SelectedApps, 1)
	assert.Equal(t, "appset-1", baseResult.SelectedApps[0].Name)
	assert.Equal(t, "appset-1", targetResult.SelectedApps[0].Name)
	assert.Len(t, baseResult.SkippedApps, 0)
	assert.Len(t, targetResult.SkippedApps, 0)
}

func TestRemoveIdenticalCopiesBetweenBranches_ApplicationSetWithoutRenderAlwaysIsRemoved(t *testing.T) {
	baseAppSet := createTestArgoResource("ApplicationSet", "appset-2", "appset.yaml", git.Base)
	targetAppSet := createTestArgoResource("ApplicationSet", "appset-2", "appset.yaml", git.Target)

	base := createArgoSelection([]argoapplication.ArgoResource{baseAppSet})
	target := createArgoSelection([]argoapplication.ArgoResource{targetAppSet})

	baseResult, targetResult := RemoveIdenticalCopiesBetweenBranches(base, target)

	assert.Len(t, baseResult.SelectedApps, 0)
	assert.Len(t, targetResult.SelectedApps, 0)
	assert.Len(t, baseResult.SkippedApps, 1)
	assert.Len(t, targetResult.SkippedApps, 1)
}

func TestRemoveIdenticalCopiesBetweenBranches_ApplicationSetWithRenderAlwaysOnOneSideIsNotRemoved(t *testing.T) {
	baseAppSet := createTestArgoResourceWithAnnotations(
		"ApplicationSet",
		"appset-3",
		"appset.yaml",
		git.Base,
		map[string]string{"argocd-diff-preview/render": "always"},
	)
	targetAppSet := createTestArgoResource("ApplicationSet", "appset-3", "appset.yaml", git.Target)

	base := createArgoSelection([]argoapplication.ArgoResource{baseAppSet})
	target := createArgoSelection([]argoapplication.ArgoResource{targetAppSet})

	baseResult, targetResult := RemoveIdenticalCopiesBetweenBranches(base, target)

	assert.Len(t, baseResult.SelectedApps, 1)
	assert.Len(t, targetResult.SelectedApps, 1)
	assert.Len(t, baseResult.SkippedApps, 0)
	assert.Len(t, targetResult.SkippedApps, 0)
}

func TestRemoveIdenticalCopiesBetweenBranches_ApplicationSetWithRenderChangedIsRemoved(t *testing.T) {
	baseAppSet := createTestArgoResourceWithAnnotations(
		"ApplicationSet",
		"appset-4",
		"appset.yaml",
		git.Base,
		map[string]string{"argocd-diff-preview/render": "changed"},
	)
	targetAppSet := createTestArgoResourceWithAnnotations(
		"ApplicationSet",
		"appset-4",
		"appset.yaml",
		git.Target,
		map[string]string{"argocd-diff-preview/render": "changed"},
	)

	base := createArgoSelection([]argoapplication.ArgoResource{baseAppSet})
	target := createArgoSelection([]argoapplication.ArgoResource{targetAppSet})

	baseResult, targetResult := RemoveIdenticalCopiesBetweenBranches(base, target)

	assert.Len(t, baseResult.SelectedApps, 0)
	assert.Len(t, targetResult.SelectedApps, 0)
	assert.Len(t, baseResult.SkippedApps, 1)
	assert.Len(t, targetResult.SkippedApps, 1)
}
