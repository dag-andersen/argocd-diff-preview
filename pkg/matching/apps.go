package matching

import (
	"math"
	"sort"

	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
)

// Pair represents a matched pair of ExtractedApps from base and target branches
type Pair struct {
	Base   *extract.ExtractedApp // nil if app was added in target
	Target *extract.ExtractedApp // nil if app was deleted in target
}

// appIdentityKey returns the identity key for an app: "name\x00sourcePath".
// This uniquely identifies an app in most cases. When multiple apps share the
// same name but have different source paths, the path disambiguates them.
func appIdentityKey(app *extract.ExtractedApp) string {
	return app.Name + "\x00" + app.SourcePath
}

// MatchApps finds the best pairing between base and target ExtractedApps.
// It uses a three-phase matching strategy:
//  1. Match by exact identity (name + source path) - strongest signal.
//  2. Match remaining by exact name only - catches apps whose source path
//     changed but name stayed the same. This also ensures deterministic results
//     for ApplicationSet-generated apps that produce nearly identical resources.
//  3. Match remaining by content similarity - handles renames, moves, etc.
//
// Returns a list of pairs where each pair contains matching base and target apps.
// If an app only exists in base, Target will be nil (deleted).
// If an app only exists in target, Base will be nil (added).
func MatchApps(baseApps, targetApps []extract.ExtractedApp) []Pair {
	if len(baseApps) == 0 && len(targetApps) == 0 {
		return nil
	}

	var pairs []Pair
	matchedBase := make(map[int]bool)
	matchedTarget := make(map[int]bool)

	// Phase 1: Match by exact identity (name + source path).
	targetByIdentity := make(map[string][]int)
	for i := range targetApps {
		key := appIdentityKey(&targetApps[i])
		targetByIdentity[key] = append(targetByIdentity[key], i)
	}

	baseByIdentity := make(map[string][]int)
	sortedIdentityKeys := make([]string, 0)
	for i := range baseApps {
		key := appIdentityKey(&baseApps[i])
		if _, exists := baseByIdentity[key]; !exists {
			sortedIdentityKeys = append(sortedIdentityKeys, key)
		}
		baseByIdentity[key] = append(baseByIdentity[key], i)
	}
	sort.Strings(sortedIdentityKeys)

	for _, key := range sortedIdentityKeys {
		baseIdxs := baseByIdentity[key]
		targetIdxs := targetByIdentity[key]

		matchLen := min(len(baseIdxs), len(targetIdxs))
		for i := range matchLen {
			bi := baseIdxs[i]
			ti := targetIdxs[i]
			pairs = append(pairs, Pair{
				Base:   &baseApps[bi],
				Target: &targetApps[ti],
			})
			matchedBase[bi] = true
			matchedTarget[ti] = true
		}
	}

	// Phase 2: Match remaining by exact name only.
	// This catches apps whose source path changed but name stayed the same,
	// and ensures deterministic results for ApplicationSet apps with nearly
	// identical resources.
	targetByName := make(map[string][]int)
	for i := range targetApps {
		if !matchedTarget[i] {
			targetByName[targetApps[i].Name] = append(targetByName[targetApps[i].Name], i)
		}
	}

	baseByName := make(map[string][]int)
	sortedBaseNames := make([]string, 0)
	for i := range baseApps {
		if !matchedBase[i] {
			name := baseApps[i].Name
			if _, exists := baseByName[name]; !exists {
				sortedBaseNames = append(sortedBaseNames, name)
			}
			baseByName[name] = append(baseByName[name], i)
		}
	}
	sort.Strings(sortedBaseNames)

	for _, name := range sortedBaseNames {
		baseIdxs := baseByName[name]
		targetIdxs := targetByName[name]

		matchLen := min(len(baseIdxs), len(targetIdxs))
		for i := range matchLen {
			bi := baseIdxs[i]
			ti := targetIdxs[i]
			pairs = append(pairs, Pair{
				Base:   &baseApps[bi],
				Target: &targetApps[ti],
			})
			matchedBase[bi] = true
			matchedTarget[ti] = true
		}
	}

	// Phase 3: Match remaining apps by content similarity.
	var unmatchedBaseIndices []int
	for i := range baseApps {
		if !matchedBase[i] {
			unmatchedBaseIndices = append(unmatchedBaseIndices, i)
		}
	}
	var unmatchedTargetIndices []int
	for i := range targetApps {
		if !matchedTarget[i] {
			unmatchedTargetIndices = append(unmatchedTargetIndices, i)
		}
	}

	if len(unmatchedBaseIndices) > 0 && len(unmatchedTargetIndices) > 0 {
		similarityPairs := matchAppsBySimilarity(baseApps, targetApps, unmatchedBaseIndices, unmatchedTargetIndices)
		for _, sp := range similarityPairs {
			pairs = append(pairs, Pair{
				Base:   &baseApps[sp.baseIdx],
				Target: &targetApps[sp.targetIdx],
			})
			matchedBase[sp.baseIdx] = true
			matchedTarget[sp.targetIdx] = true
		}
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
			if score >= 0.3 { // Minimum threshold to consider a match
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
		if math.Abs(candidates[i].score-candidates[j].score) > 1e-9 {
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
// The score is primarily driven by resource content similarity (70%),
// with bonuses for matching name (15%) and matching source path (15%).
// The identity bonuses ensure that apps with the same name+path are
// always matched, even when all resources change kind (e.g. Deployment
// → StatefulSet). When content clearly favors a different match,
// content still wins.
func appSimilarity(a, b *extract.ExtractedApp) float64 {
	const (
		weightName       = 0.15
		weightSourcePath = 0.15
		weightResources  = 0.7
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
