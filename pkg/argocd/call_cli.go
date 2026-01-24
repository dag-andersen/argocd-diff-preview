package argocd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
)

// CLIOperations implements the Operations interface using the ArgoCD CLI.
type CLIOperations struct {
	k8sClient    *utils.K8sClient
	namespace    string
	loginOptions string
	authToken    string // When set, used as ARGOCD_AUTH_TOKEN env var for CLI commands
}

// runArgocdCommand executes an argocd CLI command with port forwarding and a 60-second timeout
func (c *CLIOperations) runArgocdCommand(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "argocd", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("ARGOCD_OPTS=--port-forward --port-forward-namespace=%s", c.namespace))

	// If an auth token is set, pass it as ARGOCD_AUTH_TOKEN environment variable
	if c.authToken != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("ARGOCD_AUTH_TOKEN=%s", c.authToken))
	}

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("argocd command timed out after 60 seconds: %w", err)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if errorMessage := strings.TrimSpace(string(exitErr.Stderr)); errorMessage != "" {
				return "", fmt.Errorf("argocd command failed with error: %s: %w", errorMessage, err)
			}
		}
		return "", fmt.Errorf("argocd command failed with output: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return string(output), nil
}

// Login performs login to ArgoCD using the CLI.
// If a token was provided during construction, this method will skip the normal authentication
// and use the provided token instead (passed as ARGOCD_AUTH_TOKEN env var to CLI commands).
func (c *CLIOperations) Login() error {
	// If a token is already set, skip the login process
	if c.authToken != "" {
		log.Info().Msg("🔑 Using provided auth token for Argo CD CLI authentication")
		log.Debug().Msg("Verifying token by listing applications...")
		if _, errList := c.runArgocdCommand("app", "list"); errList != nil {
			log.Error().Err(errList).Msg("❌ Failed to list applications with provided token (verification step).")
			return fmt.Errorf("token verification failed (unable to list applications): %w", errList)
		}
		return nil
	}

	log.Info().Msgf("🦑 Logging in to Argo CD through CLI...")

	// Get initial admin password
	password, err := getInitialPassword(c.k8sClient, c.namespace)
	if err != nil {
		return err
	}

	username := "admin"

	maxAttempts := 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Debug().Msgf("Login attempt %d/%d to Argo CD...", attempt, maxAttempts)

		// Build login command arguments
		args := []string{"login", "--insecure", "--username", username, "--password", password}
		if c.loginOptions != "" {
			args = append(args, strings.Fields(c.loginOptions)...)
		}

		out, err := c.runArgocdCommand(args...)
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
	if _, errList := c.runArgocdCommand("app", "list"); errList != nil {
		log.Error().Err(errList).Msg("❌ Failed to list applications after login (verification step).")
		return fmt.Errorf("login verification failed (unable to list applications): %w", errList)
	}

	return nil
}

// AppsetGenerate runs 'argocd appset generate' on a file and returns the output
func (c *CLIOperations) AppsetGenerate(appSetPath string) (string, error) {
	out, err := c.runArgocdCommand("appset", "generate", appSetPath, "-o", "yaml")
	if err != nil {
		return "", fmt.Errorf("failed to run argocd appset generate: %w", err)
	}

	return out, nil
}

// GetManifests returns the manifests for an application using the CLI
func (c *CLIOperations) GetManifests(appName string) (string, bool, error) {
	out, err := c.runArgocdCommand("app", "manifests", appName)
	if err != nil {
		if exists, err := c.k8sClient.CheckIfResourceExists(ApplicationGVR, c.namespace, appName); !exists && err != nil {
			log.Warn().Msgf("App '%s' does not exist", appName)
			return "", false, fmt.Errorf("app '%s' does not exist: %w", appName, err)
		}

		return "", true, fmt.Errorf("failed to get manifests for app: %w", err)
	}

	if strings.TrimSpace(out) == "" {
		log.Debug().Msgf("No manifests found with `argocd app manifests '%s'`", appName)
		return "", true, nil
	}

	return out, true, nil
}

// AddSourceNamespaceToDefaultAppProject adds "*" to the sourceNamespaces of the default AppProject
func (c *CLIOperations) AddSourceNamespaceToDefaultAppProject() error {
	if _, err := c.runArgocdCommand("proj", "add-source-namespace", "default", "*"); err != nil {
		return fmt.Errorf("failed to add extra permissions to the default AppProject: %w", err)
	}
	return nil
}

// CheckVersionCompatibility checks Argo CD CLI version vs Argo CD Server version
func (c *CLIOperations) CheckVersionCompatibility() error {
	var out string
	var err error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		out, err = c.runArgocdCommand("version", "-o", "json")
		if err == nil {
			break
		}
		if attempt < maxRetries {
			log.Debug().Msgf("argocd version command failed (attempt %d/%d), retrying in 1s: %v", attempt, maxRetries, err)
			time.Sleep(1 * time.Second)
		}
	}
	if err != nil {
		return fmt.Errorf("command 'argocd version -o json' failed: %w", err)
	}

	type versionInfo struct {
		Version string `json:"Version"`
	}

	type argocdVersionOutput struct {
		Client versionInfo `json:"client"`
		Server versionInfo `json:"server"`
	}

	var versionOutput argocdVersionOutput
	if err := json.Unmarshal([]byte(out), &versionOutput); err != nil {
		return fmt.Errorf("failed to parse argocd version output: %w", err)
	}

	log.Debug().Msgf("Argo CD Version: [CLI: '%s', Server: '%s']", versionOutput.Client.Version, versionOutput.Server.Version)

	clientMajor, clientMinor, err := extractMajorMinorVersion(versionOutput.Client.Version)
	if err != nil {
		return fmt.Errorf("failed to extract major minor version from cli version: %w", err)
	}
	serverMajor, serverMinor, err := extractMajorMinorVersion(versionOutput.Server.Version)
	if err != nil {
		return fmt.Errorf("failed to extract major minor version from server version: %w", err)
	}

	majorDrift, minorDrift := checkVersionDrift(clientMajor, clientMinor, serverMajor, serverMinor)
	if majorDrift {
		log.Warn().Msgf("⚠️ Argo CD CLI major version (%d.%d) differs from server major version (%d.%d). This may cause compatibility issues.", clientMajor, clientMinor, serverMajor, serverMinor)
	} else if minorDrift {
		log.Warn().Msgf("⚠️ Argo CD CLI minor version (%d.%d) differs significantly from server minor version (%d.%d). This may cause compatibility issues.", clientMajor, clientMinor, serverMajor, serverMinor)
	}

	return nil
}

// Cleanup is a no-op for CLI mode (no resources to clean up)
func (c *CLIOperations) Cleanup() {
	// No-op: CLI mode doesn't have resources that need cleanup
}

// IsExpectedError always returns false for CLI mode.
// Expected errors only occur in API mode when running with 'createClusterRoles: false'.
func (c *CLIOperations) IsExpectedError(errorMessage string) (bool, string) {
	return false, ""
}
