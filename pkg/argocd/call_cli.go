package argocd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// runArgocdCommand executes an argocd CLI command with port forwarding
func (a *ArgoCDInstallation) runArgocdCommand(args ...string) (string, error) {
	cmd := exec.Command("argocd", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("ARGOCD_OPTS=--port-forward --port-forward-namespace=%s", a.Namespace))
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if errorMessage := strings.TrimSpace(string(exitErr.Stderr)); errorMessage != "" {
				return "", fmt.Errorf("argocd command failed with error: %s: %w", errorMessage, err)
			}
		}
		return "", fmt.Errorf("argocd command failed with output: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}

// loginViaCLI performs login to ArgoCD using the CLI
func (a *ArgoCDInstallation) loginViaCLI() error {
	log.Info().Msgf("🦑 Logging in to Argo CD through CLI...")

	// Get initial admin password
	password, err := a.getInitialPassword()
	if err != nil {
		return err
	}

	username := "admin"

	maxAttempts := 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Debug().Msgf("Login attempt %d/%d to Argo CD...", attempt, maxAttempts)

		// Build login command arguments
		args := []string{"login", "--insecure", "--username", username, "--password", password}
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

// appsetGenerateCLI runs 'argocd appset generate' on a file and returns the output
func (a *ArgoCDInstallation) appsetGenerateCLI(appSetPath string) (string, error) {
	out, err := a.runArgocdCommand("appset", "generate", appSetPath, "-o", "yaml")
	if err != nil {
		return "", fmt.Errorf("failed to run argocd appset generate: %w", err)
	}

	return out, nil
}

// getManifestsCLI returns the manifests for an application using the CLI
func (a *ArgoCDInstallation) getManifestsCLI(appName string) (string, bool, error) {
	out, err := a.runArgocdCommand("app", "manifests", appName)
	if err != nil {
		exists, _ := a.K8sClient.CheckIfResourceExists(ApplicationGVR, a.Namespace, appName)
		if !exists {
			log.Warn().Msgf("App '%s' does not exist", appName)
		}

		return "", exists, fmt.Errorf("failed to get manifests for app: %w", err)
	}

	if strings.TrimSpace(out) == "" {
		log.Debug().Msgf("No manifests found with `argocd app manifests %s`", appName)
		return "", true, nil
	}

	return out, true, nil
}

func (a *ArgoCDInstallation) addSourceNamespaceToDefaultAppProjectCLI() error {
	if _, err := a.runArgocdCommand("proj", "add-source-namespace", "default", "*"); err != nil {
		return fmt.Errorf("failed to add extra permissions to the default AppProject: %w", err)
	}
	return nil
}
