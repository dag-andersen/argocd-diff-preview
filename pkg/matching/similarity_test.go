package matching

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestResourceListSimilarity_DuplicateNamesInB_NotDropped(t *testing.T) {
	// Regression test: When listB has more resources with the same namespace/name
	// than listA, the extra B resources should still appear in the unmatched pool
	// and be considered in the similarity calculation.
	//
	// Bug scenario before fix:
	// - listA has 1 resource named "default/foo"
	// - listB has 2 resources named "default/foo"
	// - B[0] matches A[0] by name → consumed
	// - B[1] has no A match left, but its key still exists in byNameA (as empty slice)
	//   so it was excluded from unmatchedB with `_, found := byNameA[key]; !found`
	// - B[1] was silently dropped: neither matched nor unmatched

	a := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(1)},
	})

	b1 := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(1)},
	})
	b2 := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(99)},
	})

	listA := []unstructured.Unstructured{a}
	listB := []unstructured.Unstructured{b1, b2}

	score := resourceListSimilarity(listA, listB)

	// With 1 resource in A and 2 in B:
	// - 1 matched pair (a ↔ b1, identical content → similarity 1.0)
	// - 1 unmatched B resource (b2)
	// totalResources = max(1, 2) = 2
	// matchedCount = 1, avgSimilarity = 1.0, coverageRatio = 1/2 = 0.5
	// score = 1.0 * 0.5 = 0.5
	//
	// Before the fix, b2 was silently dropped, giving:
	// matchedCount = 1, avgSimilarity = 1.0, coverageRatio = 1/2 = 0.5
	// This happened to produce the same result in this case, but the unmatched
	// pool was incorrect. We verify the score is correct and also test the
	// asymmetric case below.
	if score < 0.4 || score > 0.6 {
		t.Errorf("expected score ~0.5 (one match out of 2 resources), got %f", score)
	}
}

func TestResourceListSimilarity_DuplicateNamesInB_ExtraUnmatchedGetsGreedyMatch(t *testing.T) {
	// The real impact of the bug: when A has an unmatched resource that SHOULD
	// greedily match with B's extra duplicate, but the duplicate was dropped.
	//
	// listA: "default/foo" (replicas=1), "default/bar" (replicas=99)
	// listB: "default/foo" (replicas=1), "default/foo" (replicas=99)
	//
	// Expected matching:
	// 1. Name match: A's "foo" ↔ B's first "foo" (identical → 1.0)
	// 2. Unmatched: A has "bar"(replicas=99), B has second "foo"(replicas=99)
	//    These should greedily match via content similarity (high similarity)
	//
	// Before fix: B's second "foo" was dropped from unmatchedB, so A's "bar"
	// had nothing to match with, reducing the overall score.

	aFoo := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(1), "image": "nginx"},
	})
	aBar := makeResource("apps/v1", "Deployment", "default", "bar", map[string]any{
		"spec": map[string]any{"replicas": int64(99), "image": "nginx"},
	})

	bFoo1 := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(1), "image": "nginx"},
	})
	bFoo2 := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(99), "image": "nginx"},
	})

	listA := []unstructured.Unstructured{aFoo, aBar}
	listB := []unstructured.Unstructured{bFoo1, bFoo2}

	score := resourceListSimilarity(listA, listB)

	// Both A resources should match (one by name, one by greedy content).
	// With the fix: matchedCount=2, totalResources=2, coverageRatio=1.0
	// Score should be high (close to 1.0) since content is very similar.
	//
	// Before the fix: matchedCount=1 (only the name match), coverageRatio=0.5
	// Score would be ~0.5 instead of ~1.0
	if score < 0.7 {
		t.Errorf("expected high score (>0.7) when extra B duplicates can greedily match unmatched A, got %f", score)
	}
}

func TestResourceListSimilarity_SymmetricDuplicates(t *testing.T) {
	// When both lists have the same number of duplicates, all should match by name.
	a1 := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(1)},
	})
	a2 := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(2)},
	})
	b1 := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(1)},
	})
	b2 := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(2)},
	})

	listA := []unstructured.Unstructured{a1, a2}
	listB := []unstructured.Unstructured{b1, b2}

	score := resourceListSimilarity(listA, listB)

	// Both match by name. Content is the same ordering so a1↔b1 and a2↔b2.
	// Both pairs are identical → similarity 1.0 each.
	// matchedCount=2, totalResources=2, coverage=1.0, score=1.0
	if score < 0.9 {
		t.Errorf("expected score ~1.0 for symmetric duplicates, got %f", score)
	}
}

func TestResourceListSimilarity_DuplicateNamesInA_NotDropped(t *testing.T) {
	// Mirror case: A has more duplicates than B.
	// Extra A resources should end up in unmatchedA.
	a1 := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(1), "image": "nginx"},
	})
	a2 := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(99), "image": "nginx"},
	})

	bFoo := makeResource("apps/v1", "Deployment", "default", "foo", map[string]any{
		"spec": map[string]any{"replicas": int64(1), "image": "nginx"},
	})
	bBar := makeResource("apps/v1", "Deployment", "default", "bar", map[string]any{
		"spec": map[string]any{"replicas": int64(99), "image": "nginx"},
	})

	listA := []unstructured.Unstructured{a1, a2}
	listB := []unstructured.Unstructured{bFoo, bBar}

	score := resourceListSimilarity(listA, listB)

	// a1 matches bFoo by name. a2 is unmatched in A, bBar is unmatched in B.
	// a2 and bBar have very similar content → greedy match.
	// Score should be high.
	if score < 0.7 {
		t.Errorf("expected high score (>0.7) when extra A duplicates can greedily match, got %f", score)
	}
}
