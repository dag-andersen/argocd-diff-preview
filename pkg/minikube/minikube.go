package minikube

import (
	"fmt"
	"os/exec"

	"github.com/rs/zerolog/log"

	"github.com/dag-andersen/argocd-diff-preview/pkg/cluster"
)

func (m *Minikube) GetName() string {
	return "minikube"
}

// IsInstalled checks if minikube is installed on the system
func IsInstalled() bool {
	_, err := exec.LookPath("minikube")
	return err == nil
}

// CreateCluster creates a new minikube cluster
func CreateCluster() error {
	// Check if docker is running
	if output, err := runCommand("docker", "ps"); err != nil {
		log.Error().Msg("❌ Docker is not running")
		return fmt.Errorf("docker is not running: %s", output)
	}

	log.Info().Msg("🚀 Creating cluster...")

	// Delete existing cluster if it exists
	if output, err := runCommand("minikube", "delete"); err != nil {
		return fmt.Errorf("failed to delete existing cluster: %s", output)
	}

	// Create new cluster
	if output, err := runCommand("minikube", "start"); err != nil {
		log.Error().Msg("❌ Failed to create cluster")
		return fmt.Errorf("failed to create cluster: %s", output)
	}

	log.Info().Msg("🚀 Cluster created successfully")
	return nil
}

// ClusterExists checks if a minikube cluster exists
func ClusterExists() bool {
	_, err := runCommand("minikube", "status")
	return err == nil
}

// DeleteCluster deletes the minikube cluster
func DeleteCluster(wait bool) {
	log.Info().Msg("💥 Deleting cluster...")

	if wait {
		output, err := runCommand("minikube", "delete")
		if err != nil {
			log.Error().Msgf("❌ Failed to delete cluster: %v", output)
			return
		}
		log.Info().Msgf("💥 Cluster deleted successfully: %s", output)
	} else {
		cmd := exec.Command("minikube", "delete")
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

// ensure Minikube implements cluster.Provider
var _ cluster.Provider = (*Minikube)(nil)

type Minikube struct{}

func New() *Minikube {
	return &Minikube{}
}

// Implement cluster.Provider interface
func (m *Minikube) IsInstalled() bool {
	return IsInstalled()
}

func (m *Minikube) CreateCluster() error {
	return CreateCluster()
}

func (m *Minikube) ClusterExists() bool {
	return ClusterExists()
}

func (m *Minikube) DeleteCluster(wait bool) {
	DeleteCluster(wait)
}
