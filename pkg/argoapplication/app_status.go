package argoapplication

import (
	"errors"
	"fmt"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"gopkg.in/yaml.v3"
)

func isErrorCondition(condType string) bool {
	return condType != "" && containsIgnoreCase(condType, "error")
}

// GetErrorStatusFromApplication returns the error status of an application
func GetErrorStatusFromApplication(argocd *argocd.ArgoCDInstallation, app ArgoResource) (argoErrMessage error, err error) {

	output, err := argocd.K8sClient.GetArgoCDApplication(argocd.Namespace, app.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to get application %s: %w", app.GetLongName(), err)
	}

	var appStatus struct {
		Status struct {
			Sync struct {
				Status string `yaml:"status"`
			} `yaml:"sync"`
			Conditions []struct {
				Type    string `yaml:"type"`
				Message string `yaml:"message"`
			} `yaml:"conditions"`
		} `yaml:"status"`
	}

	if err := yaml.Unmarshal([]byte(output), &appStatus); err != nil {
		return nil, fmt.Errorf("failed to parse application yaml for %s: %w", app.GetLongName(), err)
	}

	switch appStatus.Status.Sync.Status {
	case "OutOfSync", "Synced":
		return nil, nil
	case "Unknown":
		for _, condition := range appStatus.Status.Conditions {
			if isErrorCondition(condition.Type) {
				return errors.New(condition.Message), nil
			}
		}
	}

	return nil, fmt.Errorf("application '%s' sync status is not set", app.GetLongName())
}
