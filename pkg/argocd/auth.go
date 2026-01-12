package argocd

import (
	"fmt"

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

// // getTokenFromConfig reads the auth token from the ArgoCD CLI config file
// func (a *ArgoCDInstallation) getTokenFromConfig() (string, error) {
// 	// Get the home directory
// 	homeDir, err := os.UserHomeDir()
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get home directory: %w", err)
// 	}

// 	// Construct the path to the ArgoCD config file
// 	configPath := filepath.Join(homeDir, ".config", "argocd", "config")

// 	// Read the config file
// 	configData, err := os.ReadFile(configPath)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to read ArgoCD config file: %w", err)
// 	}

// 	// Define the structure to parse the config
// 	type Context struct {
// 		Name   string `yaml:"name"`
// 		Server string `yaml:"server"`
// 		User   string `yaml:"user"`
// 	}

// 	type Server struct {
// 		Server          string `yaml:"server"`
// 		GRPCWebRootPath string `yaml:"grpc-web-root-path"`
// 		Insecure        bool   `yaml:"insecure,omitempty"`
// 		Core            bool   `yaml:"core,omitempty"`
// 	}

// 	type User struct {
// 		Name         string `yaml:"name"`
// 		AuthToken    string `yaml:"auth-token,omitempty"`
// 		RefreshToken string `yaml:"refresh-token,omitempty"`
// 	}

// 	type Config struct {
// 		Contexts       []Context `yaml:"contexts"`
// 		CurrentContext string    `yaml:"current-context"`
// 		PromptsEnabled bool      `yaml:"prompts-enabled"`
// 		Servers        []Server  `yaml:"servers"`
// 		Users          []User    `yaml:"users"`
// 	}

// 	// Parse the YAML config
// 	var config Config
// 	if err := yaml.Unmarshal(configData, &config); err != nil {
// 		return "", fmt.Errorf("failed to parse ArgoCD config: %w", err)
// 	}

// 	// Find the current context
// 	var currentContextUser string
// 	for _, ctx := range config.Contexts {
// 		if ctx.Name == config.CurrentContext {
// 			currentContextUser = ctx.User
// 			log.Debug().Msgf("Found current context '%s' with user '%s' in ArgoCD config at path: '%s'", config.CurrentContext, currentContextUser, configPath)
// 			break
// 		}
// 	}

// 	if currentContextUser == "" {
// 		return "", fmt.Errorf("current context '%s' not found in contexts in ArgoCD config at path: '%s'", config.CurrentContext, configPath)
// 	}

// 	// Find the user with matching name and get the auth token
// 	for _, user := range config.Users {
// 		if user.Name == currentContextUser {
// 			if user.AuthToken != "" {
// 				log.Debug().Msgf("Found auth token at path: '%s'", configPath)
// 				log.Info().Msg("🔑 Found auth token")
// 				return user.AuthToken, nil
// 			}
// 			return "", fmt.Errorf("user '%s' found but has no auth token in ArgoCD config at path: '%s'", user.Name, configPath)
// 		}
// 	}

// 	return "", fmt.Errorf("no auth token found in ArgoCD config at path: '%s' for user '%s'", configPath, currentContextUser)
// }
