package matching

import (
	"sort"

	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
)

// Pair represents a matched pair of ExtractedApps from base and target branches
type Pair struct {
	Base   *extract.ExtractedApp // nil if app was added in target
	Target *extract.ExtractedApp // nil if app was deleted in target
}

// MatchApps finds the best pairing between base and target ExtractedApps.
// It uses content similarity (with an identity bonus for matching Name + SourcePath)
// to find the best matches. This correctly handles cases where apps are renamed
// or content is moved between apps, while giving a tiebreaker advantage to apps
// that share the same identity.
//
// Returns a list of pairs where each pair contains matching base and target apps.
// If an app only exists in base, Target will be nil (deleted).
// If an app only exists in target, Base will be nil (added).
func MatchApps(baseApps, targetApps []extract.ExtractedApp) []Pair {
	if len(baseApps) == 0 && len(targetApps) == 0 {
		return nil
	}

	// Build indices for all apps
	baseIndices := make([]int, len(baseApps))
	for i := range baseApps {
		baseIndices[i] = i
	}
	targetIndices := make([]int, len(targetApps))
	for i := range targetApps {
		targetIndices[i] = i
	}

	// Match all apps by similarity (with identity bonus baked into the score)
	similarityPairs := matchAppsBySimilarity(baseApps, targetApps, baseIndices, targetIndices)

	// Build result pairs
	var pairs []Pair
	matchedBase := make(map[int]bool)
	matchedTarget := make(map[int]bool)

	for _, sp := range similarityPairs {
		pairs = append(pairs, Pair{
			Base:   &baseApps[sp.baseIdx],
			Target: &targetApps[sp.targetIdx],
		})
		matchedBase[sp.baseIdx] = true
		matchedTarget[sp.targetIdx] = true
	}

	// Add remaining unmatched as deletions
	for i := range baseApps {
		if !matchedBase[i] {
			pairs = append(pairs, Pair{
				Base:   &baseApps[i],
				Target: nil,
			})
		}
	}

	// Add remaining unmatched as additions
	for i := range targetApps {
		if !matchedTarget[i] {
			pairs = append(pairs, Pair{
				Base:   nil,
				Target: &targetApps[i],
			})
		}
	}

	return pairs
}

// matchAppsBySimilarity finds best matches for apps using content similarity
func matchAppsBySimilarity(
	baseApps, targetApps []extract.ExtractedApp,
	unmatchedBaseIndices, unmatchedTargetIndices []int,
) []scoredPair {
	// Compute all pairwise similarities for unmatched apps
	var candidates []scoredPair

	for _, baseIdx := range unmatchedBaseIndices {
		for _, targetIdx := range unmatchedTargetIndices {
			score := appSimilarity(&baseApps[baseIdx], &targetApps[targetIdx])
			if score > 0.3 { // Minimum threshold to consider a match
				candidates = append(candidates, scoredPair{
					baseIdx:   baseIdx,
					targetIdx: targetIdx,
					score:     score,
				})
			}
		}
	}

	// Sort by score descending (greedy matching), with deterministic tiebreakers
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].baseIdx != candidates[j].baseIdx {
			return candidates[i].baseIdx < candidates[j].baseIdx
		}
		return candidates[i].targetIdx < candidates[j].targetIdx
	})

	// Greedily pick best non-overlapping pairs
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

// appSimilarity computes similarity between two ExtractedApps
// Returns a score between 0 and 1, where 1 means identical.
//
// The score is primarily driven by resource content similarity (80%),
// with bonuses for matching name (10%) and matching source path (10%).
// The identity bonuses act as tiebreakers: when two candidate matches
// have similar content scores, the one with the same name+path wins.
// But when content clearly favors a different match, content wins.
func appSimilarity(a, b *extract.ExtractedApp) float64 {
	const (
		weightName       = 0.1
		weightSourcePath = 0.1
		weightResources  = 0.8
	)

	score := 0.0

	// Name similarity (exact match bonus)
	if a.Name == b.Name {
		score += weightName
	}

	// Source path similarity (exact match bonus)
	if a.SourcePath == b.SourcePath {
		score += weightSourcePath
	}

	// Resource-level similarity
	score += weightResources * resourceSetSimilarity(a.Manifests, b.Manifests)

	return score
}
