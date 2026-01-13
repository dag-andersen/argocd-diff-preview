package argoapplication

import (
	"errors"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
)

func isErrorCondition(condType string) bool {
	return condType != "" && containsIgnoreCase(condType, "error")
}

// GetApplicationStatus returns the error status of an application
func GetApplicationStatus(argocd *argocd.ArgoCDInstallation, app ArgoResource) (reconciled bool, isMarkedForRefresh bool, argoErrMessage error, internalError error, err error) {

	application, err := argocd.K8sClient.GetArgoCDApplication(argocd.Namespace, app.Id)
	if err != nil {
		return false, false, nil, nil, fmt.Errorf("failed to get application %s: %w", app.GetLongName(), err)
	}

	reconciled = application.Status.ReconciledAt != nil

	// Check if the refresh annotation exists
	annotations := application.GetAnnotations()
	if annotations != nil {
		_, exists := annotations["argocd.argoproj.io/refresh"]
		isMarkedForRefresh = exists
	}

	switch application.Status.Sync.Status {
	case v1alpha1.SyncStatusCodeOutOfSync, v1alpha1.SyncStatusCodeSynced:
		break
	case v1alpha1.SyncStatusCodeUnknown:
		for _, condition := range application.Status.Conditions {
			if isErrorCondition(condition.Type) {
				argoErrMessage = errors.New(condition.Message)
				break
			}
		}
	default:
		internalError = fmt.Errorf("application '%s' sync status is not set", app.GetLongName())
	}

	return reconciled, isMarkedForRefresh, argoErrMessage, internalError, nil
}
