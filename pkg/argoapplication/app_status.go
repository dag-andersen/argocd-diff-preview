package argoapplication

import (
	"errors"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/rs/zerolog/log"
)

func isErrorCondition(condType string) bool {
	return condType != "" && containsIgnoreCase(condType, "error")
}

// GetErrorStatusFromApplication returns the error status of an application
func GetErrorStatusFromApplication(argocd *argocd.ArgoCDInstallation, app ArgoResource) (reconciled bool, argoErrMessage error, err error) {

	application, err := argocd.K8sClient.GetArgoCDApplication(argocd.Namespace, app.Id)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get application %s: %w", app.GetLongName(), err)
	}

	if application.Status.ReconciledAt == nil { // not reconciled yet
		log.Debug().Str("App", app.GetLongName()).Msg("Application is not reconciled yet")
		return false, nil, nil
	}

	switch application.Status.Sync.Status {
	case v1alpha1.SyncStatusCodeOutOfSync, v1alpha1.SyncStatusCodeSynced:
		return true, nil, nil
	case v1alpha1.SyncStatusCodeUnknown:
		for _, condition := range application.Status.Conditions {
			if isErrorCondition(condition.Type) {
				return true, errors.New(condition.Message), nil
			}
		}
	}

	return true, nil, fmt.Errorf("application '%s' sync status is not set", app.GetLongName())
}
