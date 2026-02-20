package diff

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
)

// Tests for ConvertArgoSelectionToSelectionInfo

func TestConvertArgoSelectionToSelectionInfo_EmptySelections(t *testing.T) {
	base := &argoapplication.ArgoSelection{}
	target := &argoapplication.ArgoSelection{}

	result := ConvertArgoSelectionToSelectionInfo(base, target)

	if result.Base.SkippedApplications != 0 {
		t.Errorf("expected 0 base skipped apps, got %d", result.Base.SkippedApplications)
	}
	if result.Base.SkippedApplicationSets != 0 {
		t.Errorf("expected 0 base skipped app sets, got %d", result.Base.SkippedApplicationSets)
	}
	if result.Target.SkippedApplications != 0 {
		t.Errorf("expected 0 target skipped apps, got %d", result.Target.SkippedApplications)
	}
	if result.Target.SkippedApplicationSets != 0 {
		t.Errorf("expected 0 target skipped app sets, got %d", result.Target.SkippedApplicationSets)
	}
}

func TestConvertArgoSelectionToSelectionInfo_MixedKinds(t *testing.T) {
	base := &argoapplication.ArgoSelection{
		SkippedApps: []argoapplication.ArgoResource{
			{Kind: argoapplication.Application},
			{Kind: argoapplication.Application},
			{Kind: argoapplication.ApplicationSet},
		},
	}
	target := &argoapplication.ArgoSelection{
		SkippedApps: []argoapplication.ArgoResource{
			{Kind: argoapplication.Application},
			{Kind: argoapplication.ApplicationSet},
			{Kind: argoapplication.ApplicationSet},
			{Kind: argoapplication.ApplicationSet},
		},
	}

	result := ConvertArgoSelectionToSelectionInfo(base, target)

	if result.Base.SkippedApplications != 2 {
		t.Errorf("expected 2 base skipped apps, got %d", result.Base.SkippedApplications)
	}
	if result.Base.SkippedApplicationSets != 1 {
		t.Errorf("expected 1 base skipped app set, got %d", result.Base.SkippedApplicationSets)
	}
	if result.Target.SkippedApplications != 1 {
		t.Errorf("expected 1 target skipped app, got %d", result.Target.SkippedApplications)
	}
	if result.Target.SkippedApplicationSets != 3 {
		t.Errorf("expected 3 target skipped app sets, got %d", result.Target.SkippedApplicationSets)
	}
}

func TestConvertArgoSelectionToSelectionInfo_OnlyApplications(t *testing.T) {
	base := &argoapplication.ArgoSelection{
		SkippedApps: []argoapplication.ArgoResource{
			{Kind: argoapplication.Application},
		},
	}
	target := &argoapplication.ArgoSelection{
		SkippedApps: []argoapplication.ArgoResource{
			{Kind: argoapplication.Application},
			{Kind: argoapplication.Application},
		},
	}

	result := ConvertArgoSelectionToSelectionInfo(base, target)

	if result.Base.SkippedApplications != 1 {
		t.Errorf("expected 1 base skipped app, got %d", result.Base.SkippedApplications)
	}
	if result.Base.SkippedApplicationSets != 0 {
		t.Errorf("expected 0 base skipped app sets, got %d", result.Base.SkippedApplicationSets)
	}
	if result.Target.SkippedApplications != 2 {
		t.Errorf("expected 2 target skipped apps, got %d", result.Target.SkippedApplications)
	}
	if result.Target.SkippedApplicationSets != 0 {
		t.Errorf("expected 0 target skipped app sets, got %d", result.Target.SkippedApplicationSets)
	}
}

func TestConvertArgoSelectionToSelectionInfo_OnlyApplicationSets(t *testing.T) {
	base := &argoapplication.ArgoSelection{
		SkippedApps: []argoapplication.ArgoResource{
			{Kind: argoapplication.ApplicationSet},
			{Kind: argoapplication.ApplicationSet},
		},
	}
	target := &argoapplication.ArgoSelection{}

	result := ConvertArgoSelectionToSelectionInfo(base, target)

	if result.Base.SkippedApplications != 0 {
		t.Errorf("expected 0 base skipped apps, got %d", result.Base.SkippedApplications)
	}
	if result.Base.SkippedApplicationSets != 2 {
		t.Errorf("expected 2 base skipped app sets, got %d", result.Base.SkippedApplicationSets)
	}
	if result.Target.SkippedApplications != 0 {
		t.Errorf("expected 0 target skipped apps, got %d", result.Target.SkippedApplications)
	}
	if result.Target.SkippedApplicationSets != 0 {
		t.Errorf("expected 0 target skipped app sets, got %d", result.Target.SkippedApplicationSets)
	}
}

// Tests for SelectionInfo.String()

func TestSelectionInfo_String_EqualCounts(t *testing.T) {
	info := SelectionInfo{
		Base:   AppSelectionInfo{SkippedApplications: 5, SkippedApplicationSets: 3},
		Target: AppSelectionInfo{SkippedApplications: 5, SkippedApplicationSets: 3},
	}

	result := info.String()
	if result != "" {
		t.Errorf("expected empty string when counts are equal, got %q", result)
	}
}

func TestSelectionInfo_String_DifferentApplicationCounts(t *testing.T) {
	info := SelectionInfo{
		Base:   AppSelectionInfo{SkippedApplications: 2, SkippedApplicationSets: 1},
		Target: AppSelectionInfo{SkippedApplications: 5, SkippedApplicationSets: 1},
	}

	result := info.String()
	if result == "" {
		t.Fatal("expected non-empty string when application counts differ")
	}
	if expected := "Applications: `2` (base) -> `5` (target)"; !contains(result, expected) {
		t.Errorf("expected %q in output, got %q", expected, result)
	}
}

func TestSelectionInfo_String_DifferentApplicationSetCounts(t *testing.T) {
	info := SelectionInfo{
		Base:   AppSelectionInfo{SkippedApplications: 1, SkippedApplicationSets: 0},
		Target: AppSelectionInfo{SkippedApplications: 1, SkippedApplicationSets: 2},
	}

	result := info.String()
	if result == "" {
		t.Fatal("expected non-empty string when app set counts differ")
	}
	if expected := "ApplicationSets: `0` (base) -> `2` (target)"; !contains(result, expected) {
		t.Errorf("expected %q in output, got %q", expected, result)
	}
}

func TestSelectionInfo_String_ZeroCounts(t *testing.T) {
	info := SelectionInfo{
		Base:   AppSelectionInfo{SkippedApplications: 0, SkippedApplicationSets: 0},
		Target: AppSelectionInfo{SkippedApplications: 0, SkippedApplicationSets: 0},
	}

	result := info.String()
	if result != "" {
		t.Errorf("expected empty string when all counts are zero, got %q", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
