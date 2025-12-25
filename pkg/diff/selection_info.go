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
		return fmt.Sprintf("_Skipped resources_: \n- Applications: `%d` (base) -> `%d` (target)\n- ApplicationSets: `%d` (base) -> `%d` (target)", t.Base.SkippedApplications, t.Target.SkippedApplications, t.Base.SkippedApplicationSets, t.Target.SkippedApplicationSets)
	}
	return ""
}

func ConvertArgoSelectionToSelectionInfo(baseApps *argoapplication.ArgoSelection, targetApps *argoapplication.ArgoSelection) SelectionInfo {

	var baseSkippedApplications int
	var baseSkippedApplicationSets int
	var targetSkippedApplications int
	var targetSkippedApplicationSets int
	for _, app := range baseApps.SkippedApps {
		switch app.Kind {
		case argoapplication.Application:
			baseSkippedApplications++
		case argoapplication.ApplicationSet:
			baseSkippedApplicationSets++
		}
	}
	for _, app := range targetApps.SkippedApps {
		switch app.Kind {
		case argoapplication.Application:
			targetSkippedApplications++
		case argoapplication.ApplicationSet:
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
