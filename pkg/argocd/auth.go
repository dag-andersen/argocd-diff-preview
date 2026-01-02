package argocd

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// getInitialPassword retrieves the initial admin password from Kubernetes secret
func (a *ArgoCDInstallation) getInitialPassword() (string, error) {
	var err error
	var err_fallback error
	secret, err := a.K8sClient.GetSecretValue(a.Namespace, "argocd-initial-admin-secret", "password")
	if err != nil {
		log.Debug().Msgf("Failed to get password in 'argocd-initial-admin-secret'. Trying to get fallback password in 'argocd-cluster' secret.")
		secret, err_fallback = a.K8sClient.GetSecretValue(a.Namespace, "argocd-cluster", "admin.password")
		if err_fallback != nil {
			log.Error().Err(err).Msgf("❌ Failed to get secret 'argocd-initial-admin-secret'")
			log.Error().Err(err_fallback).Msgf("❌ Failed to get fallback secret 'argocd-cluster'")
			return "", fmt.Errorf("failed to get secret: %w", err)
		}
	}

	return secret, nil
}

func (a *ArgoCDInstallation) login() error {
	log.Info().Msgf("🦑 Logging in to Argo CD through CLI...")

	// Get initial admin password
	password, err := a.getInitialPassword()
	if err != nil {
		return err
	}

	maxAttempts := 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Debug().Msgf("Login attempt %d/%d to Argo CD...", attempt, maxAttempts)

		// Build login command arguments
		args := []string{"login", "--insecure", "--username", "admin", "--password", password}
		if a.LoginOptions != "" {
			args = append(args, strings.Fields(a.LoginOptions)...)
		}

		out, err := a.runArgocdCommand(args...)
		if err == nil {
			log.Debug().Msgf("Login successful on attempt %d. Output: %s", attempt, out)
			break
		}

		if attempt >= maxAttempts {
			log.Error().Err(err).Msgf("❌ Failed to login to Argo CD after %d attempts", maxAttempts)
			return fmt.Errorf("failed to login after %d attempts", maxAttempts)
		}

		log.Debug().Msgf("Waiting 1s before next login attempt (%d/%d)...", attempt+1, maxAttempts)
		log.Warn().Err(err).Msgf("Argo CD login attempt %d/%d failed.", attempt, maxAttempts)
		time.Sleep(1 * time.Second)
	}

	log.Debug().Msg("Verifying login by listing applications...")
	if _, errList := a.runArgocdCommand("app", "list"); errList != nil {
		log.Error().Err(errList).Msg("❌ Failed to list applications after login (verification step).")
		return fmt.Errorf("login verification failed (unable to list applications): %w", errList)
	}

	return nil
}

// OnlyLogin performs only the login step without installing ArgoCD
func (a *ArgoCDInstallation) OnlyLogin() (time.Duration, error) {
	startTime := time.Now()

	// Login to ArgoCD
	if err := a.login(); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to login: %w", err)
	}

	log.Info().Msg("🦑 Logged in to Argo CD successfully")

	// Check Argo CD CLI version vs Argo CD Server version
	if err := a.CheckArgoCDCLIVersionVsServerVersion(); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to check argocd cli version vs server version: %w", err)
	}

	return time.Since(startTime), nil
}
