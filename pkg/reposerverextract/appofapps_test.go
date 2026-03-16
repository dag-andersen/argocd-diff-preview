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
	key1 := visitedKey("app|base", git.Target)
	key2 := visitedKey("app", git.Base)
	assert.NotEqual(t, key1, key2,
		"visitedKey must use a separator that prevents prefix collisions")
}
