package k3d

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/cluster"
	"github.com/rs/zerolog/log"
)

func (k *K3d) GetName() string {
	return "k3d"
}

// IsInstalled checks if the k3d binary is available in PATH.
func IsInstalled() bool {
	_, err := exec.LookPath("k3d")
	if err != nil {
		log.Debug().Msg("k3d command not found in PATH")
		return false
	}
	return true
}

// CreateCluster creates a new k3d cluster with the given name and options.
func CreateCluster(clusterName, k3dOptions string, wait time.Duration) error {
	// Check if docker is running
	if output, err := runCommand("docker", "ps"); err != nil {
		log.Error().Msg("❌ Docker is not running")
		return fmt.Errorf("docker is not running: %s", output)
	}

	log.Info().Msg("🚀 Creating k3d cluster...")

	// Delete existing cluster if it exists
	if output, err := runCommand("k3d", "cluster", "delete", clusterName); err != nil {
		return fmt.Errorf("failed to delete existing cluster: %s", output)
	}

	// Create new cluster
	args := []string{"cluster", "create"}
	if strings.TrimSpace(k3dOptions) != "" {
		args = append(args, strings.Fields(k3dOptions)...)
	}
	args = append(args, clusterName)

	if output, err := runCommand("k3d", args...); err != nil {
		if strings.TrimSpace(k3dOptions) == "" {
			log.Error().Msg("❌ Failed to create cluster")
		} else {
			log.Error().Msgf("❌ Failed to create cluster with options: %s", k3dOptions)
		}
		return fmt.Errorf("failed to create cluster: %s", output)
	}

	log.Info().Msg("🚀 Cluster created successfully")
	return nil
}

// ClusterExists checks if a cluster with the given name exists by parsing JSON output.
func ClusterExists(clusterName string) bool {
	// Request JSON output for reliable parsing
	output, err := runCommand("k3d", "cluster", "list", "--output", "json")
	if err != nil {
		log.Debug().Err(err).Msgf("Failed to list k3d clusters: %s", output)
		return false
	}

	// Define a struct to unmarshal the cluster name, we only need the name field.
	type k3dClusterInfo struct {
		Name string `json:"name"`
	}

	var clusters []k3dClusterInfo
	err = json.Unmarshal([]byte(output), &clusters)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to unmarshal k3d cluster list JSON: %s", output)
		return false
	}

	for _, cluster := range clusters {
		if cluster.Name == clusterName {
			return true
		}
	}

	log.Debug().Msgf("❌ Cluster '%s' not found in: %s", clusterName, output)
	return false
}

// DeleteCluster deletes the k3d cluster with the given name
func DeleteCluster(clusterName string, wait bool) {
	log.Info().Msg("💥 Deleting cluster...")

	if wait {
		if output, err := runCommand("k3d", "cluster", "delete", clusterName); err != nil {
			log.Error().Msgf("❌ Failed to delete cluster: %s", output)
			return
		}
		log.Info().Msg("💥 Cluster deleted successfully")
	} else {
		if output, err := runCommand("k3d", "cluster", "delete", clusterName); err != nil {
			log.Error().Msgf("❌ Failed to start cluster deletion: %s", output)
		}
	}
}

// runCommand executes a k3d command and returns its output or error.
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// ensure K3dCluster implements cluster.Provider
var _ cluster.Provider = (*K3d)(nil)

// K3d represents a k3d cluster configuration.
type K3d struct {
	clusterName string
	k3dOptions  string
}

func New(clusterName string, k3dOptions string) *K3d {
	return &K3d{clusterName: clusterName, k3dOptions: k3dOptions}
}

// Implement cluster.Provider interface
func (k *K3d) IsInstalled() bool {
	return IsInstalled()
}

func (k *K3d) CreateCluster() error {
	return CreateCluster(k.clusterName, k.k3dOptions, 120*time.Second)
}

func (k *K3d) ClusterExists() bool {
	return ClusterExists(k.clusterName)
}

func (k *K3d) DeleteCluster(wait bool) {
	DeleteCluster(k.clusterName, wait)
}
