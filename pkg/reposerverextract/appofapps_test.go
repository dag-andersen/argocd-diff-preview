package reposerverextract

// Tests for the app-of-apps expansion logic in appofapps.go.
//
// RenderApplicationsFromBothBranchesWithAppOfApps requires a live Argo CD repo
// server and is covered by integration tests.  This file focuses on the pure
// helper functions that can be exercised without any network or cluster:
//
//   - buildChildArgoResource  – constructs a patched ArgoResource from a child
//     Application manifest found in rendered output.
//   - visitedKey              – produces a unique deduplication key.

import (
	"fmt"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// makeParent builds a minimal parent ArgoResource on the given branch.
func makeParent(t *testing.T, name string, branch git.BranchType) argoapplication.ArgoResource {
	t.Helper()
	app := makeApp(t, fmt.Sprintf(`
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: %s
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/%s
  destination:
    namespace: argocd
`, name, name))
	app.Branch = branch
	return app
}

// makeChildManifest builds an unstructured Application manifest as it would
// appear in a parent app's rendered output.
func makeChildManifest(t *testing.T, rawYAML string) unstructured.Unstructured {
	t.Helper()
	var obj unstructured.Unstructured
	require.NoError(t, yaml.Unmarshal([]byte(rawYAML), &obj))
	return obj
}

// ─────────────────────────────────────────────────────────────────────────────
// buildChildArgoResource
// ─────────────────────────────────────────────────────────────────────────────

// TestBuildChildArgoResource_FileName verifies that the child's FileName is set
// to "parent: <parentName>" so users can trace the app-of-apps tree.
func TestBuildChildArgoResource_FileName(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    namespace: child-ns
`)

	result, err := buildChildArgoResource(child, parent, "argocd")
	require.NoError(t, err)
	assert.Equal(t, "parent: parent-app", result.FileName,
		"child FileName must be a breadcrumb pointing to the parent")
}

// TestBuildChildArgoResource_InheritsBranch verifies that the child inherits the
// parent's branch type (both Base and Target cases).
func TestBuildChildArgoResource_InheritsBranch(t *testing.T) {
	for _, branch := range []git.BranchType{git.Base, git.Target} {
		t.Run(string(branch), func(t *testing.T) {
			parent := makeParent(t, "parent-app", branch)

			child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    namespace: child-ns
`)

			result, err := buildChildArgoResource(child, parent, "argocd")
			require.NoError(t, err)
			assert.Equal(t, branch, result.Branch,
				"child must inherit parent's branch type (%s)", branch)
		})
	}
}

// TestBuildChildArgoResource_IdAndName verifies that Id and Name are set from
// the manifest's metadata.name.
func TestBuildChildArgoResource_IdAndName(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-child-app
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    namespace: default
`)

	result, err := buildChildArgoResource(child, parent, "argocd")
	require.NoError(t, err)
	assert.Equal(t, "my-child-app", result.Id)
	assert.Equal(t, "my-child-app", result.Name)
}

// TestBuildChildArgoResource_KindIsApplication verifies the Kind field.
func TestBuildChildArgoResource_KindIsApplication(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    namespace: default
`)

	result, err := buildChildArgoResource(child, parent, "argocd")
	require.NoError(t, err)
	assert.Equal(t, argoapplication.Application, result.Kind)
}

// TestBuildChildArgoResource_PatchesNamespace verifies that the child's
// namespace is overwritten with the ArgoCD namespace.
func TestBuildChildArgoResource_PatchesNamespace(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
  namespace: some-other-namespace
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    namespace: child-ns
`)

	result, err := buildChildArgoResource(child, parent, "my-argocd-ns")
	require.NoError(t, err)
	assert.Equal(t, "my-argocd-ns", result.Yaml.GetNamespace(),
		"child namespace must be overwritten with the ArgoCD namespace")
}

// TestBuildChildArgoResource_RemovesSyncPolicy verifies that automated sync is
// stripped so the child is never accidentally synced.
func TestBuildChildArgoResource_RemovesSyncPolicy(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
spec:
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    namespace: child-ns
`)

	result, err := buildChildArgoResource(child, parent, "argocd")
	require.NoError(t, err)

	_, found, _ := unstructured.NestedMap(result.Yaml.Object, "spec", "syncPolicy")
	assert.False(t, found, "syncPolicy must be removed from child Application")
}

// TestBuildChildArgoResource_SetsProjectToDefault verifies that any custom
// project is replaced with "default".
func TestBuildChildArgoResource_SetsProjectToDefault(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
spec:
  project: my-restricted-project
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    namespace: child-ns
`)

	result, err := buildChildArgoResource(child, parent, "argocd")
	require.NoError(t, err)

	project, _, _ := unstructured.NestedString(result.Yaml.Object, "spec", "project")
	assert.Equal(t, "default", project, "project must be reset to 'default'")
}

// TestBuildChildArgoResource_SetsDestinationServerToLocal verifies that the
// child's destination is always pointed at the local in-cluster server.
func TestBuildChildArgoResource_SetsDestinationServerToLocal(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    server: https://some-external-cluster.example.com
    namespace: child-ns
`)

	result, err := buildChildArgoResource(child, parent, "argocd")
	require.NoError(t, err)

	server, _, _ := unstructured.NestedString(result.Yaml.Object, "spec", "destination", "server")
	assert.Equal(t, "https://kubernetes.default.svc", server,
		"destination server must be set to the local cluster")
}

// TestBuildChildArgoResource_RemovesArgoCDFinalizers verifies that the
// resources-finalizer is stripped so the child can be deleted cleanly.
func TestBuildChildArgoResource_RemovesArgoCDFinalizers(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
  finalizers:
    - resources-finalizer.argocd.argoproj.io
    - some-other-finalizer
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    namespace: child-ns
`)

	result, err := buildChildArgoResource(child, parent, "argocd")
	require.NoError(t, err)

	finalizers := result.Yaml.GetFinalizers()
	assert.NotContains(t, finalizers, "resources-finalizer.argocd.argoproj.io",
		"ArgoCD finalizer must be removed")
	assert.Contains(t, finalizers, "some-other-finalizer",
		"non-ArgoCD finalizers must be preserved")
}

// TestBuildChildArgoResource_NoFinalizers verifies no panic when no finalizers.
func TestBuildChildArgoResource_NoFinalizers(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    namespace: child-ns
`)

	result, err := buildChildArgoResource(child, parent, "argocd")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestBuildChildArgoResource_EmptyName verifies that a manifest with no name
// returns an error rather than creating a broken ArgoResource.
func TestBuildChildArgoResource_EmptyName(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {}
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
`)

	_, err := buildChildArgoResource(child, parent, "argocd")
	assert.Error(t, err, "missing name must return an error")
}

// TestBuildChildArgoResource_DoesNotMutateOriginal verifies that the original
// manifest is not modified (we deep-copy before patching).
func TestBuildChildArgoResource_DoesNotMutateOriginal(t *testing.T) {
	parent := makeParent(t, "parent-app", git.Base)

	child := makeChildManifest(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: child-app
  namespace: original-ns
spec:
  project: custom-project
  syncPolicy:
    automated: {}
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/child
  destination:
    server: https://external.example.com
    namespace: child-ns
`)

	originalNS := child.GetNamespace()
	originalProject, _, _ := unstructured.NestedString(child.Object, "spec", "project")

	_, err := buildChildArgoResource(child, parent, "argocd")
	require.NoError(t, err)

	// The original manifest must be unchanged.
	assert.Equal(t, originalNS, child.GetNamespace(),
		"original manifest namespace must not be mutated")
	project, _, _ := unstructured.NestedString(child.Object, "spec", "project")
	assert.Equal(t, originalProject, project,
		"original manifest project must not be mutated")
	_, found, _ := unstructured.NestedMap(child.Object, "spec", "syncPolicy")
	assert.True(t, found, "original manifest syncPolicy must not be removed")
}

// ─────────────────────────────────────────────────────────────────────────────
// visitedKey
// ─────────────────────────────────────────────────────────────────────────────

func TestVisitedKey_Format(t *testing.T) {
	key := visitedKey("my-app", git.Base)
	assert.Contains(t, key, "my-app", "key must contain the app ID")
	assert.Contains(t, key, string(git.Base), "key must contain the branch type")
}

func TestVisitedKey_DifferentIDs(t *testing.T) {
	key1 := visitedKey("app-a", git.Base)
	key2 := visitedKey("app-b", git.Base)
	assert.NotEqual(t, key1, key2,
		"different app IDs on the same branch must produce different keys")
}

func TestVisitedKey_DifferentBranches(t *testing.T) {
	key1 := visitedKey("my-app", git.Base)
	key2 := visitedKey("my-app", git.Target)
	assert.NotEqual(t, key1, key2,
		"same app ID on different branches must produce different keys")
}

func TestVisitedKey_Deterministic(t *testing.T) {
	key1 := visitedKey("my-app", git.Base)
	key2 := visitedKey("my-app", git.Base)
	assert.Equal(t, key1, key2, "visitedKey must be deterministic")
}

// TestVisitedKey_NoPrefixCollision guards against naive concatenation bugs where
// ("ab", "c") == ("a", "bc") if the separator is omitted.
func TestVisitedKey_NoPrefixCollision(t *testing.T) {
	// Construct two app IDs that share a prefix/suffix with the branch string
	// to ensure the separator prevents false equality.
	key1 := visitedKey("app|base", git.Target)
	key2 := visitedKey("app", git.Base)
	// These are different (id, branch) pairs and must not collide.
	assert.NotEqual(t, key1, key2,
		"visitedKey must use a separator that prevents prefix collisions")
}
