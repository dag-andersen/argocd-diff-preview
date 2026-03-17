package reposerverextract

// Tests for the app-of-apps expansion logic in appofapps.go.
//
// RenderApplicationsFromBothBranchesWithAppOfApps requires a live Argo CD repo
// server and is covered by integration tests. This file focuses on the pure
// helper functions that can be exercised without any network or cluster:
//
//   - visitedKey – produces a unique deduplication key.
//
// Patching logic for discovered child Applications and ApplicationSets is
// delegated entirely to argoapplication.PatchApplication, which is tested in
// pkg/argoapplication.

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/stretchr/testify/assert"
)

func TestVisitedKey_Format(t *testing.T) {
	key := visitedKey("argocd", "my-app", git.Base)
	assert.Contains(t, key, "argocd", "key must contain the namespace")
	assert.Contains(t, key, "my-app", "key must contain the app name")
	assert.Contains(t, key, string(git.Base), "key must contain the branch type")
}

func TestVisitedKey_DifferentNames(t *testing.T) {
	key1 := visitedKey("argocd", "app-a", git.Base)
	key2 := visitedKey("argocd", "app-b", git.Base)
	assert.NotEqual(t, key1, key2,
		"different app names in the same namespace and branch must produce different keys")
}

func TestVisitedKey_DifferentNamespaces(t *testing.T) {
	key1 := visitedKey("argocd", "my-app", git.Base)
	key2 := visitedKey("other", "my-app", git.Base)
	assert.NotEqual(t, key1, key2,
		"same app name in different namespaces must produce different keys")
}

func TestVisitedKey_DifferentBranches(t *testing.T) {
	key1 := visitedKey("argocd", "my-app", git.Base)
	key2 := visitedKey("argocd", "my-app", git.Target)
	assert.NotEqual(t, key1, key2,
		"same app in different branches must produce different keys")
}

func TestVisitedKey_Deterministic(t *testing.T) {
	key1 := visitedKey("argocd", "my-app", git.Base)
	key2 := visitedKey("argocd", "my-app", git.Base)
	assert.Equal(t, key1, key2, "visitedKey must be deterministic")
}

// TestVisitedKey_NoPrefixCollision guards against naive concatenation bugs where
// two different (namespace, name) pairs produce the same key if no separator is used.
func TestVisitedKey_NoPrefixCollision(t *testing.T) {
	// namespace="argo-cd", name="app" must not equal namespace="argo", name="cd-app"
	key1 := visitedKey("argo-cd", "app", git.Base)
	key2 := visitedKey("argo", "cd-app", git.Base)
	assert.NotEqual(t, key1, key2,
		"visitedKey must use separators that prevent prefix collisions between namespace and name")
}

// TestVisitedKey_SameKubernetesIdentity verifies that two apps with identical
// namespace/name/branch produce the same key regardless of their deduplicated Id.
// This is the core property that prevents the triple-root duplicate bug.
func TestVisitedKey_SameKubernetesIdentity(t *testing.T) {
	key1 := visitedKey("argocd", "root", git.Target)
	key2 := visitedKey("argocd", "root", git.Target)
	assert.Equal(t, key1, key2,
		"apps with the same namespace/name/branch must share a visited key even if their Ids differ")
}
