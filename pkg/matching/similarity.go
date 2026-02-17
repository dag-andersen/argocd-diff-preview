package matching

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// scoredPair represents a potential match with its similarity score
type scoredPair struct {
	baseIdx   int
	targetIdx int
	score     float64
}

// resourceSetSimilarity computes Jaccard-like similarity between two sets of resources
// It matches resources by kind first, then compares content
func resourceSetSimilarity(manifestsA, manifestsB []unstructured.Unstructured) float64 {
	if len(manifestsA) == 0 && len(manifestsB) == 0 {
		return 1.0
	}
	if len(manifestsA) == 0 || len(manifestsB) == 0 {
		return 0.0
	}

	// Group by kind for more efficient matching
	byKindA := groupByKind(manifestsA)
	byKindB := groupByKind(manifestsB)

	// Collect all kinds
	allKinds := make(map[string]bool)
	for k := range byKindA {
		allKinds[k] = true
	}
	for k := range byKindB {
		allKinds[k] = true
	}

	totalScore := 0.0
	totalWeight := 0.0

	// Sort kinds for deterministic iteration
	sortedKinds := make([]string, 0, len(allKinds))
	for k := range allKinds {
		sortedKinds = append(sortedKinds, k)
	}
	sort.Strings(sortedKinds)

	for _, kind := range sortedKinds {
		resourcesA := byKindA[kind]
		resourcesB := byKindB[kind]

		// Weight by number of resources of this kind
		weight := float64(max(len(resourcesA), len(resourcesB)))
		totalWeight += weight

		if len(resourcesA) == 0 || len(resourcesB) == 0 {
			// No overlap for this kind
			continue
		}

		// Compute similarity within this kind
		kindScore := resourceListSimilarity(resourcesA, resourcesB)
		totalScore += weight * kindScore
	}

	if totalWeight == 0 {
		return 0.0
	}

	return totalScore / totalWeight
}

// groupByKind groups unstructured objects by their Kind
func groupByKind(manifests []unstructured.Unstructured) map[string][]unstructured.Unstructured {
	result := make(map[string][]unstructured.Unstructured)
	for _, m := range manifests {
		kind := m.GetKind()
		result[kind] = append(result[kind], m)
	}
	return result
}

// resourceListSimilarity computes similarity between two lists of resources of the same kind
func resourceListSimilarity(listA, listB []unstructured.Unstructured) float64 {
	if len(listA) == 0 && len(listB) == 0 {
		return 1.0
	}

	// Try to match by name first
	byNameA := make(map[string]*unstructured.Unstructured)
	for i := range listA {
		key := listA[i].GetNamespace() + "/" + listA[i].GetName()
		byNameA[key] = &listA[i]
	}

	matchedCount := 0
	totalSimilarity := 0.0

	usedA := make(map[string]bool)

	for i := range listB {
		key := listB[i].GetNamespace() + "/" + listB[i].GetName()
		if a, found := byNameA[key]; found {
			// Exact name match - compare content
			totalSimilarity += contentSimilarity(a, &listB[i])
			matchedCount++
			usedA[key] = true
		}
	}

	// For unmatched, find best content match (sort keys for deterministic order)
	unmatchedKeys := make([]string, 0)
	for key := range byNameA {
		if !usedA[key] {
			unmatchedKeys = append(unmatchedKeys, key)
		}
	}
	sort.Strings(unmatchedKeys)

	var unmatchedA []*unstructured.Unstructured
	for _, key := range unmatchedKeys {
		unmatchedA = append(unmatchedA, byNameA[key])
	}

	var unmatchedB []*unstructured.Unstructured
	for i := range listB {
		key := listB[i].GetNamespace() + "/" + listB[i].GetName()
		if _, found := byNameA[key]; !found {
			unmatchedB = append(unmatchedB, &listB[i])
		}
	}

	// Simple greedy matching for unmatched
	usedBIdx := make(map[int]bool)
	for _, a := range unmatchedA {
		bestScore := 0.0
		bestIdx := -1
		for j, b := range unmatchedB {
			if usedBIdx[j] {
				continue
			}
			score := contentSimilarity(a, b)
			if score > bestScore {
				bestScore = score
				bestIdx = j
			}
		}
		if bestIdx >= 0 && bestScore > 0.5 {
			totalSimilarity += bestScore
			matchedCount++
			usedBIdx[bestIdx] = true
		}
	}

	// Compute final score based on matched vs total
	totalResources := max(len(listA), len(listB))
	if totalResources == 0 {
		return 1.0
	}

	// Average similarity of matched pairs, penalized by unmatched count
	if matchedCount == 0 {
		return 0.0
	}

	avgSimilarity := totalSimilarity / float64(matchedCount)
	coverageRatio := float64(matchedCount) / float64(totalResources)

	return avgSimilarity * coverageRatio
}

// contentSimilarity computes line-based Jaccard similarity between two resources
func contentSimilarity(a, b *unstructured.Unstructured) float64 {
	// Convert to string representation for line comparison
	linesA := objectToLines(a.Object)
	linesB := objectToLines(b.Object)

	return jaccardSimilarity(linesA, linesB)
}

// objectToLines flattens an object to a set of "path=value" strings
func objectToLines(obj map[string]any) map[string]bool {
	lines := make(map[string]bool)
	flattenObject("", obj, lines)
	return lines
}

// flattenObject recursively flattens a nested map to "path=value" strings
func flattenObject(prefix string, obj any, result map[string]bool) {
	switch v := obj.(type) {
	case map[string]any:
		for key, value := range v {
			newPrefix := key
			if prefix != "" {
				newPrefix = prefix + "." + key
			}
			flattenObject(newPrefix, value, result)
		}
	case []any:
		for i, value := range v {
			newPrefix := prefix + "[" + string(rune('0'+i)) + "]"
			if i >= 10 {
				newPrefix = prefix + "[*]" // Simplify for large arrays
			}
			flattenObject(newPrefix, value, result)
		}
	default:
		// Leaf value
		line := prefix + "=" + toString(v)
		result[line] = true
	}
}

// toString converts a value to string
func toString(v any) string {
	if v == nil {
		return "<nil>"
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// jaccardSimilarity computes Jaccard similarity between two sets
func jaccardSimilarity(setA, setB map[string]bool) float64 {
	if len(setA) == 0 && len(setB) == 0 {
		return 1.0
	}
	if len(setA) == 0 || len(setB) == 0 {
		return 0.0
	}

	intersection := 0
	for key := range setA {
		if setB[key] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}
