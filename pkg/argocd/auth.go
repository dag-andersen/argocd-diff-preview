package argocd

import (
	"fmt"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
)

// getInitialPassword retrieves the initial admin password from Kubernetes secret
func getInitialPassword(k8sClient *utils.K8sClient, namespace string) (string, error) {
	var err error
	var err_fallback error
	secret, err := k8sClient.GetSecretValue(namespace, "argocd-initial-admin-secret", "password")
	if err != nil {
		log.Debug().Msgf("Failed to get password in 'argocd-initial-admin-secret'. Trying to get fallback password in 'argocd-cluster' secret.")
		secret, err_fallback = k8sClient.GetSecretValue(namespace, "argocd-cluster", "admin.password")
		if err_fallback != nil {
			log.Error().Err(err).Msgf("❌ Failed to get secret 'argocd-initial-admin-secret'")
			log.Error().Err(err_fallback).Msgf("❌ Failed to get fallback secret 'argocd-cluster'")
			return "", fmt.Errorf("failed to get secret: %w", err)
		}
	}

	return secret, nil
}
