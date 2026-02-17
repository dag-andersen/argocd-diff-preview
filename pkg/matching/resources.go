package matching

import (
	"reflect"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ResourcePair represents a matched pair of Kubernetes resources within an app pair
type ResourcePair struct {
	Base   *unstructured.Unstructured // nil if resource was added
	Target *unstructured.Unstructured // nil if resource was deleted
}

// ChangedResources returns only the resources that differ between base and target
// within this app pair. Identical resources are filtered out.
func (p *Pair) ChangedResources() []ResourcePair {
	if p.Base == nil && p.Target == nil {
		return nil
	}

	// If app was added (no base), all target resources are "added"
	if p.Base == nil {
		result := make([]ResourcePair, len(p.Target.Manifests))
		for i := range p.Target.Manifests {
			result[i] = ResourcePair{Base: nil, Target: &p.Target.Manifests[i]}
		}
		return result
	}

	// If app was deleted (no target), all base resources are "deleted"
	if p.Target == nil {
		result := make([]ResourcePair, len(p.Base.Manifests))
		for i := range p.Base.Manifests {
			result[i] = ResourcePair{Base: &p.Base.Manifests[i], Target: nil}
		}
		return result
	}

	// Match resources between base and target
	return matchResources(p.Base.Manifests, p.Target.Manifests)
}

// matchResources finds the best pairing between base and target resources,
// returning only pairs where the resources differ (or are added/deleted).
func matchResources(baseManifests, targetManifests []unstructured.Unstructured) []ResourcePair {
	// Group by kind for more efficient matching
	baseByKind := groupByKind(baseManifests)
	targetByKind := groupByKind(targetManifests)

	// Collect all kinds
	allKinds := make(map[string]bool)
	for k := range baseByKind {
		allKinds[k] = true
	}
	for k := range targetByKind {
		allKinds[k] = true
	}

	// Sort kinds for deterministic ordering
	sortedKinds := make([]string, 0, len(allKinds))
	for k := range allKinds {
		sortedKinds = append(sortedKinds, k)
	}
	sort.Strings(sortedKinds)

	var changedPairs []ResourcePair

	for _, kind := range sortedKinds {
		baseResources := baseByKind[kind]
		targetResources := targetByKind[kind]

		kindPairs := matchResourcesOfSameKind(baseResources, targetResources)
		changedPairs = append(changedPairs, kindPairs...)
	}

	return changedPairs
}

// matchResourcesOfSameKind matches resources of the same kind and returns only changed pairs
func matchResourcesOfSameKind(baseResources, targetResources []unstructured.Unstructured) []ResourcePair {
	if len(baseResources) == 0 && len(targetResources) == 0 {
		return nil
	}

	// If no base resources, all target resources are additions
	if len(baseResources) == 0 {
		result := make([]ResourcePair, len(targetResources))
		for i := range targetResources {
			result[i] = ResourcePair{Base: nil, Target: &targetResources[i]}
		}
		return result
	}

	// If no target resources, all base resources are deletions
	if len(targetResources) == 0 {
		result := make([]ResourcePair, len(baseResources))
		for i := range baseResources {
			result[i] = ResourcePair{Base: &baseResources[i], Target: nil}
		}
		return result
	}

	// Try to match by namespace/name first (identity matching)
	baseByKey := make(map[string]int) // key -> index
	for i := range baseResources {
		key := resourceKey(&baseResources[i])
		baseByKey[key] = i
	}

	matchedBaseIndices := make(map[int]bool)
	matchedTargetIndices := make(map[int]bool)
	var result []ResourcePair

	// Phase 1: Match by identity (namespace/name)
	for i := range targetResources {
		key := resourceKey(&targetResources[i])
		if baseIdx, found := baseByKey[key]; found {
			// Check if content is identical
			if !resourcesEqual(&baseResources[baseIdx], &targetResources[i]) {
				result = append(result, ResourcePair{
					Base:   &baseResources[baseIdx],
					Target: &targetResources[i],
				})
			}
			// Mark as matched (even if identical - we don't want to re-match)
			matchedBaseIndices[baseIdx] = true
			matchedTargetIndices[i] = true
		}
	}

	// Collect unmatched
	var unmatchedBase []int
	for i := range baseResources {
		if !matchedBaseIndices[i] {
			unmatchedBase = append(unmatchedBase, i)
		}
	}
	var unmatchedTarget []int
	for i := range targetResources {
		if !matchedTargetIndices[i] {
			unmatchedTarget = append(unmatchedTarget, i)
		}
	}

	// Phase 2: Match unmatched by content similarity
	if len(unmatchedBase) > 0 && len(unmatchedTarget) > 0 {
		similarityMatches := matchResourcesBySimilarity(baseResources, targetResources, unmatchedBase, unmatchedTarget)

		for _, sm := range similarityMatches {
			// Only include if not identical
			if !resourcesEqual(&baseResources[sm.baseIdx], &targetResources[sm.targetIdx]) {
				result = append(result, ResourcePair{
					Base:   &baseResources[sm.baseIdx],
					Target: &targetResources[sm.targetIdx],
				})
			}
			matchedBaseIndices[sm.baseIdx] = true
			matchedTargetIndices[sm.targetIdx] = true
		}

		// Update unmatched lists
		var stillUnmatchedBase []int
		for _, idx := range unmatchedBase {
			if !matchedBaseIndices[idx] {
				stillUnmatchedBase = append(stillUnmatchedBase, idx)
			}
		}
		unmatchedBase = stillUnmatchedBase

		var stillUnmatchedTarget []int
		for _, idx := range unmatchedTarget {
			if !matchedTargetIndices[idx] {
				stillUnmatchedTarget = append(stillUnmatchedTarget, idx)
			}
		}
		unmatchedTarget = stillUnmatchedTarget
	}

	// Remaining unmatched base resources are deletions
	for _, idx := range unmatchedBase {
		result = append(result, ResourcePair{
			Base:   &baseResources[idx],
			Target: nil,
		})
	}

	// Remaining unmatched target resources are additions
	for _, idx := range unmatchedTarget {
		result = append(result, ResourcePair{
			Base:   nil,
			Target: &targetResources[idx],
		})
	}

	return result
}

// resourceKey returns a string key for a resource (namespace/name)
func resourceKey(r *unstructured.Unstructured) string {
	return r.GetNamespace() + "/" + r.GetName()
}

// resourcesEqual checks if two resources are deeply equal
func resourcesEqual(a, b *unstructured.Unstructured) bool {
	return reflect.DeepEqual(a.Object, b.Object)
}

// matchResourcesBySimilarity finds best matches for unmatched resources using content similarity
func matchResourcesBySimilarity(
	baseResources, targetResources []unstructured.Unstructured,
	unmatchedBase, unmatchedTarget []int,
) []scoredPair {
	var candidates []scoredPair

	for _, baseIdx := range unmatchedBase {
		for _, targetIdx := range unmatchedTarget {
			score := contentSimilarity(&baseResources[baseIdx], &targetResources[targetIdx])
			if score > 0.5 { // Higher threshold for resource matching
				candidates = append(candidates, scoredPair{
					baseIdx:   baseIdx,
					targetIdx: targetIdx,
					score:     score,
				})
			}
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].baseIdx != candidates[j].baseIdx {
			return candidates[i].baseIdx < candidates[j].baseIdx
		}
		return candidates[i].targetIdx < candidates[j].targetIdx
	})

	// Greedy matching
	usedBase := make(map[int]bool)
	usedTarget := make(map[int]bool)
	var result []scoredPair

	for _, c := range candidates {
		if !usedBase[c.baseIdx] && !usedTarget[c.targetIdx] {
			result = append(result, c)
			usedBase[c.baseIdx] = true
			usedTarget[c.targetIdx] = true
		}
	}

	return result
}
