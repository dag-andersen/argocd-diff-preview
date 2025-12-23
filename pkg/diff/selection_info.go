package diff

import (
	"fmt"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
)

type AppSelectionInfo struct {
	SkippedApplications    int
	SkippedApplicationSets int
}

type SelectionInfo struct {
	Base   AppSelectionInfo
	Target AppSelectionInfo
}

func (t SelectionInfo) String() string {
	if t.Base.SkippedApplications != t.Target.SkippedApplications || t.Base.SkippedApplicationSets != t.Target.SkippedApplicationSets {
		return fmt.Sprintf("_Skipped resources_: \n- Applications: `%d` (base) -> `%d` (target)\n- ApplicationSets: `%d` (base) -> `%d` (target)\n", t.Base.SkippedApplications, t.Target.SkippedApplications, t.Base.SkippedApplicationSets, t.Target.SkippedApplicationSets)
	}
	return ""
}

func ConvertArgoSelectionToSelectionInfo(baseApps *argoapplication.ArgoSelection, targetApps *argoapplication.ArgoSelection) SelectionInfo {

	var baseSkippedApplications int
	var baseSkippedApplicationSets int
	var targetSkippedApplications int
	var targetSkippedApplicationSets int
	for _, app := range baseApps.SkippedApps {
		if app.Kind == argoapplication.Application {
			baseSkippedApplications++
		} else if app.Kind == argoapplication.ApplicationSet {
			baseSkippedApplicationSets++
		}
	}
	for _, app := range targetApps.SkippedApps {
		if app.Kind == argoapplication.Application {
			targetSkippedApplications++
		} else if app.Kind == argoapplication.ApplicationSet {
			targetSkippedApplicationSets++
		}
	}

	return SelectionInfo{
		Base: AppSelectionInfo{
			SkippedApplications:    baseSkippedApplications,
			SkippedApplicationSets: baseSkippedApplicationSets,
		},
		Target: AppSelectionInfo{
			SkippedApplications:    targetSkippedApplications,
			SkippedApplicationSets: targetSkippedApplicationSets,
		},
	}
}
