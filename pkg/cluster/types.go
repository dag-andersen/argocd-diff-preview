package cluster

// Provider defines the interface for cluster management
type Provider interface {
	IsInstalled() bool
	CreateCluster() error
	ClusterExists() bool
	DeleteCluster(wait bool)
}
