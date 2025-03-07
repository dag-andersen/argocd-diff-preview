package kind

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/dag-andersen/argocd-diff-preview/pkg/cluster"
)

func (k *Kind) GetName() string {
	return "kind"
}

// IsInstalled checks if kind is installed on the system
func IsInstalled() bool {
	_, err := exec.LookPath("kind")
	return err == nil
}

// CreateCluster creates a new kind cluster with the given name
func CreateCluster(clusterName string) error {
	// Check if docker is running
	if _, err := runCommand("docker", "ps"); err != nil {
		log.Error().Msg("❌ Docker is not running")
		return fmt.Errorf("docker is not running: %w", err)
	}

	log.Info().Msg("🚀 Creating cluster...")

	// Delete existing cluster if it exists
	if _, err := runCommand("kind", "delete", "cluster", "--name", clusterName); err != nil {
		return fmt.Errorf("failed to delete existing cluster: %w", err)
	}

	// Create new cluster
	if _, err := runCommand("kind", "create", "cluster", "--name", clusterName); err != nil {
		log.Error().Msg("❌ Failed to create cluster")
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	log.Info().Msg("🚀 Cluster created successfully")
	return nil
}

// ClusterExists checks if a cluster with the given name exists
func ClusterExists(clusterName string) bool {
	output, err := runCommand("kind", "get", "clusters")
	if err != nil {
		return false
	}

	clusters := strings.Split(strings.TrimSpace(output), "\n")
	for _, cluster := range clusters {
		if cluster == clusterName {
			return true
		}
	}

	log.Error().Msgf("❌ Cluster '%s' not found in: %s", clusterName, output)
	return false
}

// DeleteCluster deletes the kind cluster with the given name
func DeleteCluster(clusterName string, wait bool) {
	log.Info().Msg("💥 Deleting cluster...")

	if wait {
		if _, err := runCommand("kind", "delete", "cluster", "--name", clusterName); err != nil {
			log.Error().Msgf("❌ Failed to delete cluster: %v", err)
			return
		}
		log.Info().Msg("💥 Cluster deleted successfully")
	} else {
		cmd := exec.Command("kind", "delete", "cluster", "--name", clusterName)
		if err := cmd.Start(); err != nil {
			log.Error().Msgf("❌ Failed to start cluster deletion: %v", err)
		}
	}
}

// runCommand executes a command and returns its output
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// ensure Kind implements cluster.Provider
var _ cluster.Provider = (*Kind)(nil)

type Kind struct {
	clusterName string
}

func New(clusterName string) *Kind {
	return &Kind{clusterName: clusterName}
}

// Implement cluster.Provider interface
func (k *Kind) IsInstalled() bool {
	return IsInstalled()
}

func (k *Kind) CreateCluster() error {
	return CreateCluster(k.clusterName)
}

func (k *Kind) ClusterExists() bool {
	return ClusterExists(k.clusterName)
}

func (k *Kind) DeleteCluster(wait bool) {
	DeleteCluster(k.clusterName, wait)
}

// implement Provider methods...
