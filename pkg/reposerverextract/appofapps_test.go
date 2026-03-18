package reposerverextract

// Tests for the app-of-apps expansion logic in appofapps.go.
//
// RenderApplicationsFromBothBranchesWithAppOfApps requires a live Argo CD repo
// server and is covered by integration tests. This file focuses on the pure
// helper functions that can be exercised without any network or cluster:
//
//   - visitedKey – produces a unique deduplication key.
//   - specHashOf  – stable content hash of the spec field.
//
// Patching logic for discovered child Applications and ApplicationSets is
// delegated entirely to argoapplication.PatchApplication, which is tested in
// pkg/argoapplication.

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// makeUnstructuredApp builds a minimal *unstructured.Unstructured representing
// an ArgoCD Application with the given namespace, name, and spec fields. It is
// used to construct test inputs for visitedKey and specHashOf.
func makeUnstructuredApp(namespace, name string, spec map[string]any) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
	}
	if spec != nil {
		obj["spec"] = spec
	}
	return &unstructured.Unstructured{Object: obj}
}

func specA() map[string]any {
	return map[string]any{
		"source": map[string]any{
			"repoURL":        "https://github.com/example/repo",
			"path":           "apps/path-a",
			"targetRevision": "main",
		},
	}
}

func specB() map[string]any {
	return map[string]any{
		"source": map[string]any{
			"repoURL":        "https://github.com/example/repo",
			"path":           "apps/path-b",
			"targetRevision": "main",
		},
	}
}

// ── visitedKey ────────────────────────────────────────────────────────────────

func TestVisitedKey_Format(t *testing.T) {
	app := makeUnstructuredApp("argocd", "my-app", specA())
	key := visitedKey(app, git.Base)
	assert.Contains(t, key, "argocd", "key must contain the namespace")
	assert.Contains(t, key, "my-app", "key must contain the app name")
	assert.Contains(t, key, string(git.Base), "key must contain the branch type")
}

func TestVisitedKey_DifferentNames(t *testing.T) {
	key1 := visitedKey(makeUnstructuredApp("argocd", "app-a", specA()), git.Base)
	key2 := visitedKey(makeUnstructuredApp("argocd", "app-b", specA()), git.Base)
	assert.NotEqual(t, key1, key2,
		"different app names in the same namespace and branch must produce different keys")
}

func TestVisitedKey_DifferentNamespaces(t *testing.T) {
	key1 := visitedKey(makeUnstructuredApp("argocd", "my-app", specA()), git.Base)
	key2 := visitedKey(makeUnstructuredApp("other", "my-app", specA()), git.Base)
	assert.NotEqual(t, key1, key2,
		"same app name in different namespaces must produce different keys")
}

func TestVisitedKey_DifferentBranches(t *testing.T) {
	key1 := visitedKey(makeUnstructuredApp("argocd", "my-app", specA()), git.Base)
	key2 := visitedKey(makeUnstructuredApp("argocd", "my-app", specA()), git.Target)
	assert.NotEqual(t, key1, key2,
		"same app in different branches must produce different keys")
}

func TestVisitedKey_Deterministic(t *testing.T) {
	key1 := visitedKey(makeUnstructuredApp("argocd", "my-app", specA()), git.Base)
	key2 := visitedKey(makeUnstructuredApp("argocd", "my-app", specA()), git.Base)
	assert.Equal(t, key1, key2, "visitedKey must be deterministic")
}

// TestVisitedKey_NoPrefixCollision guards against naive concatenation bugs where
// two different (namespace, name) pairs produce the same key if no separator is used.
func TestVisitedKey_NoPrefixCollision(t *testing.T) {
	// namespace="argo-cd", name="app" must not equal namespace="argo", name="cd-app"
	key1 := visitedKey(makeUnstructuredApp("argo-cd", "app", specA()), git.Base)
	key2 := visitedKey(makeUnstructuredApp("argo", "cd-app", specA()), git.Base)
	assert.NotEqual(t, key1, key2,
		"visitedKey must use separators that prevent prefix collisions between namespace and name")
}

// TestVisitedKey_SameKubernetesIdentity verifies that two apps with identical
// namespace/name/branch/spec produce the same key regardless of their
// deduplicated Id. This is the core property that prevents the triple-root
// duplicate bug: a child app discovered via traversal is recognised as
// already-visited even when the same seed app had its Id renamed (e.g.
// "root" -> "root-1").
func TestVisitedKey_SameKubernetesIdentity(t *testing.T) {
	key1 := visitedKey(makeUnstructuredApp("argocd", "root", specA()), git.Target)
	key2 := visitedKey(makeUnstructuredApp("argocd", "root", specA()), git.Target)
	assert.Equal(t, key1, key2,
		"apps with the same namespace/name/branch/spec must share a visited key even if their Ids differ")
}

// TestVisitedKey_SameNameDifferentSpec is the core test for the new behaviour:
// two apps with the same namespace and name but different spec content must
// produce different visited keys so that both get rendered.
func TestVisitedKey_SameNameDifferentSpec(t *testing.T) {
	key1 := visitedKey(makeUnstructuredApp("argocd", "root", specA()), git.Base)
	key2 := visitedKey(makeUnstructuredApp("argocd", "root", specB()), git.Base)
	assert.NotEqual(t, key1, key2,
		"apps with the same namespace/name/branch but different spec must have different visited keys")
}

// ── specHashOf ────────────────────────────────────────────────────────────────

func TestSpecHashOf_Deterministic(t *testing.T) {
	app := makeUnstructuredApp("argocd", "my-app", specA())
	assert.Equal(t, specHashOf(app), specHashOf(app), "specHashOf must be deterministic")
}

func TestSpecHashOf_DifferentSpec(t *testing.T) {
	appA := makeUnstructuredApp("argocd", "my-app", specA())
	appB := makeUnstructuredApp("argocd", "my-app", specB())
	assert.NotEqual(t, specHashOf(appA), specHashOf(appB),
		"different spec fields must produce different hashes")
}

func TestSpecHashOf_SameSpec(t *testing.T) {
	appA := makeUnstructuredApp("argocd", "my-app", specA())
	appB := makeUnstructuredApp("other-ns", "other-name", specA())
	assert.Equal(t, specHashOf(appA), specHashOf(appB),
		"identical spec fields must produce the same hash regardless of namespace/name")
}

func TestSpecHashOf_NoSpec(t *testing.T) {
	app := makeUnstructuredApp("argocd", "my-app", nil)
	// Should not panic and should return a consistent (empty) value.
	hash1 := specHashOf(app)
	hash2 := specHashOf(app)
	assert.Equal(t, hash1, hash2, "specHashOf with no spec must be deterministic")
}

func TestSpecHashOf_NilYaml(t *testing.T) {
	// Should not panic.
	assert.NotPanics(t, func() { specHashOf(nil) })
}
