package cluster

// Provider defines the interface for cluster management
type Provider interface {
	GetName() string
	IsInstalled() bool
	CreateCluster() error
	ClusterExists() bool
	DeleteCluster(wait bool)
}
