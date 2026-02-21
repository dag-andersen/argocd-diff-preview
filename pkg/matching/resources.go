package matching

import (
	"math"
	"reflect"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	// similarityThresholdSameKind is the Jaccard similarity threshold (0-1) required
	// to match two unmatched resources of the SAME kind based on content similarity.
	// This helps catch resource renames (e.g., Deployment/old-name -> Deployment/new-name).
	similarityThresholdSameKind = 0.5

	// similarityThresholdCrossKind is the Jaccard similarity threshold (0-1) required
	// to match two unmatched resources of DIFFERENT kinds based on content similarity.
	// This helps catch both kind AND name changes (e.g., Deployment/old -> StatefulSet/new).
	// It is slightly lower than same-kind because cross-kind resources naturally have
	// less overlap (e.g., Deployment spec vs StatefulSet spec).
	similarityThresholdCrossKind = 0.35
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
//
// It uses a four-phase matching strategy:
//  1. Match by kind+name+namespace (exact identity) — strongest signal
//  2. Match remaining by name+namespace across kinds — catches kind changes
//     (e.g. Deployment → StatefulSet with the same name)
//  3. Match remaining within the same kind by content similarity — catches name changes
//     (e.g. Deployment/old-name → Deployment/new-name)
//  4. Final fallback across ANY kind by content similarity — catches both kind and name changes
//     (e.g. Deployment/old-name → StatefulSet/new-name)
func matchResources(baseManifests, targetManifests []unstructured.Unstructured) []ResourcePair {
	matchedBaseIndices := make(map[int]bool)
	matchedTargetIndices := make(map[int]bool)
	var changedPairs []ResourcePair

	// Phase 1: Match by kind+name+namespace (exact identity)
	baseByFullKey := make(map[string][]int) // kind/namespace/name -> indices
	for i := range baseManifests {
		key := fullResourceKey(&baseManifests[i])
		baseByFullKey[key] = append(baseByFullKey[key], i)
	}
	targetByFullKey := make(map[string][]int)
	for i := range targetManifests {
		key := fullResourceKey(&targetManifests[i])
		targetByFullKey[key] = append(targetByFullKey[key], i)
	}

	// Sort keys for deterministic ordering
	sortedFullKeys := sortedMapKeys(baseByFullKey)

	for _, key := range sortedFullKeys {
		baseIdxs := baseByFullKey[key]
		targetIdxs := targetByFullKey[key]

		matchLen := min(len(baseIdxs), len(targetIdxs))
		for i := range matchLen {
			bi := baseIdxs[i]
			ti := targetIdxs[i]
			matchedBaseIndices[bi] = true
			matchedTargetIndices[ti] = true
			if !resourcesEqual(&baseManifests[bi], &targetManifests[ti]) {
				changedPairs = append(changedPairs, ResourcePair{
					Base:   &baseManifests[bi],
					Target: &targetManifests[ti],
				})
			}
		}
	}

	// Phase 2: Match remaining by name+namespace across kinds (for kind changes)
	unmatchedBase := unmatchedIndices(len(baseManifests), matchedBaseIndices)
	unmatchedTarget := unmatchedIndices(len(targetManifests), matchedTargetIndices)

	if len(unmatchedBase) > 0 && len(unmatchedTarget) > 0 {
		baseByKey := make(map[string][]int) // namespace/name -> indices
		for _, i := range unmatchedBase {
			key := resourceKey(&baseManifests[i])
			baseByKey[key] = append(baseByKey[key], i)
		}
		targetByKey := make(map[string][]int)
		for _, i := range unmatchedTarget {
			key := resourceKey(&targetManifests[i])
			targetByKey[key] = append(targetByKey[key], i)
		}

		sortedKeys := sortedMapKeys(baseByKey)
		for _, key := range sortedKeys {
			baseIdxs := baseByKey[key]
			targetIdxs := targetByKey[key]

			matchLen := min(len(baseIdxs), len(targetIdxs))
			for i := range matchLen {
				bi := baseIdxs[i]
				ti := targetIdxs[i]
				if matchedBaseIndices[bi] || matchedTargetIndices[ti] {
					continue
				}
				matchedBaseIndices[bi] = true
				matchedTargetIndices[ti] = true
				if !resourcesEqual(&baseManifests[bi], &targetManifests[ti]) {
					changedPairs = append(changedPairs, ResourcePair{
						Base:   &baseManifests[bi],
						Target: &targetManifests[ti],
					})
				}
			}
		}
	}

	// Phase 3: Match remaining within the same kind by content similarity
	unmatchedBase = unmatchedIndices(len(baseManifests), matchedBaseIndices)
	unmatchedTarget = unmatchedIndices(len(targetManifests), matchedTargetIndices)

	if len(unmatchedBase) > 0 && len(unmatchedTarget) > 0 {
		kindPairs := matchUnmatchedByKind(baseManifests, targetManifests, unmatchedBase, unmatchedTarget)

		for _, sm := range kindPairs {
			if !resourcesEqual(&baseManifests[sm.baseIdx], &targetManifests[sm.targetIdx]) {
				changedPairs = append(changedPairs, ResourcePair{
					Base:   &baseManifests[sm.baseIdx],
					Target: &targetManifests[sm.targetIdx],
				})
			}
			matchedBaseIndices[sm.baseIdx] = true
			matchedTargetIndices[sm.targetIdx] = true
		}
	}

	// Phase 4: Final fallback - match across ANY kind by similarity, with lower threshold
	unmatchedBase = unmatchedIndices(len(baseManifests), matchedBaseIndices)
	unmatchedTarget = unmatchedIndices(len(targetManifests), matchedTargetIndices)

	if len(unmatchedBase) > 0 && len(unmatchedTarget) > 0 {
		similarityMatches := matchResourcesBySimilarity(baseManifests, targetManifests, unmatchedBase, unmatchedTarget, similarityThresholdCrossKind)

		for _, sm := range similarityMatches {
			if !resourcesEqual(&baseManifests[sm.baseIdx], &targetManifests[sm.targetIdx]) {
				changedPairs = append(changedPairs, ResourcePair{
					Base:   &baseManifests[sm.baseIdx],
					Target: &targetManifests[sm.targetIdx],
				})
			}
			matchedBaseIndices[sm.baseIdx] = true
			matchedTargetIndices[sm.targetIdx] = true
		}
	}

	// Remaining unmatched base resources are deletions
	unmatchedBase = unmatchedIndices(len(baseManifests), matchedBaseIndices)

	// Remaining unmatched target resources are additions
	unmatchedTarget = unmatchedIndices(len(targetManifests), matchedTargetIndices)

	// Now add the final additions and deletions that couldn't be matched at all
	for _, idx := range unmatchedBase {
		changedPairs = append(changedPairs, ResourcePair{
			Base:   &baseManifests[idx],
			Target: nil,
		})
	}
	for _, idx := range unmatchedTarget {
		changedPairs = append(changedPairs, ResourcePair{
			Base:   nil,
			Target: &targetManifests[idx],
		})
	}

	return changedPairs
}

// unmatchedIndices returns the indices from 0..n-1 that are NOT in the matched map.
func unmatchedIndices(n int, matched map[int]bool) []int {
	var result []int
	for i := range n {
		if !matched[i] {
			result = append(result, i)
		}
	}
	return result
}

// fullResourceKey returns a string key for a resource (kind/namespace/name)
func fullResourceKey(r *unstructured.Unstructured) string {
	return r.GetKind() + "/" + r.GetNamespace() + "/" + r.GetName()
}

// sortedMapKeys returns the sorted keys of a map[string][]int
func sortedMapKeys(m map[string][]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// matchUnmatchedByKind groups unmatched resources by kind and matches within each kind.
// Returns scoredPair with indices into the original baseManifests/targetManifests slices.
func matchUnmatchedByKind(baseManifests, targetManifests []unstructured.Unstructured, unmatchedBase, unmatchedTarget []int) []scoredPair {
	// Group unmatched by kind
	baseByKind := make(map[string][]int)
	for _, i := range unmatchedBase {
		kind := baseManifests[i].GetKind()
		baseByKind[kind] = append(baseByKind[kind], i)
	}
	targetByKind := make(map[string][]int)
	for _, i := range unmatchedTarget {
		kind := targetManifests[i].GetKind()
		targetByKind[kind] = append(targetByKind[kind], i)
	}

	allKinds := make(map[string]bool)
	for k := range baseByKind {
		allKinds[k] = true
	}
	for k := range targetByKind {
		allKinds[k] = true
	}
	sortedKinds := make([]string, 0, len(allKinds))
	for k := range allKinds {
		sortedKinds = append(sortedKinds, k)
	}
	sort.Strings(sortedKinds)

	var matchedPairs []scoredPair

	for _, kind := range sortedKinds {
		baseIdxs := baseByKind[kind]
		targetIdxs := targetByKind[kind]

		// Build slices of the actual resources for matching
		baseRes := make([]unstructured.Unstructured, len(baseIdxs))
		for i, idx := range baseIdxs {
			baseRes[i] = baseManifests[idx]
		}
		targetRes := make([]unstructured.Unstructured, len(targetIdxs))
		for i, idx := range targetIdxs {
			targetRes[i] = targetManifests[idx]
		}

		kindPairs := matchResourcesOfSameKind(baseRes, targetRes)

		// Only include modified pairs (not deleted/added - those wait for Phase 4).
		// Translate local indices back to original indices.
		for _, p := range kindPairs {
			if p.baseIdx >= 0 && p.targetIdx >= 0 {
				matchedPairs = append(matchedPairs, scoredPair{
					baseIdx:   baseIdxs[p.baseIdx],
					targetIdx: targetIdxs[p.targetIdx],
					score:     p.score,
				})
			}
		}
	}

	return matchedPairs
}

// matchResourcesOfSameKind matches resources of the same kind and returns index pairs.
// The returned scoredPair indices are local to the provided baseResources/targetResources slices.
// A baseIdx of -1 means the resource was added (no base), a targetIdx of -1 means deleted (no target).
func matchResourcesOfSameKind(baseResources, targetResources []unstructured.Unstructured) []scoredPair {
	if len(baseResources) == 0 && len(targetResources) == 0 {
		return nil
	}

	// If no base resources, all target resources are additions
	if len(baseResources) == 0 {
		result := make([]scoredPair, len(targetResources))
		for i := range targetResources {
			result[i] = scoredPair{baseIdx: -1, targetIdx: i}
		}
		return result
	}

	// If no target resources, all base resources are deletions
	if len(targetResources) == 0 {
		result := make([]scoredPair, len(baseResources))
		for i := range baseResources {
			result[i] = scoredPair{baseIdx: i, targetIdx: -1}
		}
		return result
	}

	// Try to match by namespace/name first (identity matching)
	baseByKey := make(map[string][]int) // key -> indices
	for i := range baseResources {
		key := resourceKey(&baseResources[i])
		baseByKey[key] = append(baseByKey[key], i)
	}

	matchedBaseIndices := make(map[int]bool)
	matchedTargetIndices := make(map[int]bool)
	var result []scoredPair

	// Phase 1: Match by identity (namespace/name)
	for i := range targetResources {
		if matchedTargetIndices[i] {
			continue
		}
		key := resourceKey(&targetResources[i])
		if baseIdxs, found := baseByKey[key]; found {
			// Find the first unmatched base with this key
			for _, baseIdx := range baseIdxs {
				if matchedBaseIndices[baseIdx] {
					continue
				}
				// Check if content is identical
				if !resourcesEqual(&baseResources[baseIdx], &targetResources[i]) {
					result = append(result, scoredPair{
						baseIdx:   baseIdx,
						targetIdx: i,
						score:     1.0, // exact identity match
					})
				}
				// Mark as matched (even if identical - we don't want to re-match)
				matchedBaseIndices[baseIdx] = true
				matchedTargetIndices[i] = true
				break
			}
		}
	}

	// Collect unmatched
	unmatchedBase := unmatchedIndices(len(baseResources), matchedBaseIndices)
	unmatchedTarget := unmatchedIndices(len(targetResources), matchedTargetIndices)

	// Phase 2: Match unmatched by content similarity
	if len(unmatchedBase) > 0 && len(unmatchedTarget) > 0 {
		similarityMatches := matchResourcesBySimilarity(baseResources, targetResources, unmatchedBase, unmatchedTarget, similarityThresholdSameKind)

		for _, sm := range similarityMatches {
			// Only include if not identical
			if !resourcesEqual(&baseResources[sm.baseIdx], &targetResources[sm.targetIdx]) {
				result = append(result, sm)
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
		result = append(result, scoredPair{
			baseIdx:   idx,
			targetIdx: -1,
		})
	}

	// Remaining unmatched target resources are additions
	for _, idx := range unmatchedTarget {
		result = append(result, scoredPair{
			baseIdx:   -1,
			targetIdx: idx,
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
	threshold float64,
) []scoredPair {
	var candidates []scoredPair

	for _, baseIdx := range unmatchedBase {
		for _, targetIdx := range unmatchedTarget {
			score := contentSimilarity(&baseResources[baseIdx], &targetResources[targetIdx])
			if score > threshold {
				candidates = append(candidates, scoredPair{
					baseIdx:   baseIdx,
					targetIdx: targetIdx,
					score:     score,
				})
			}
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if math.Abs(candidates[i].score-candidates[j].score) > 1e-9 {
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
