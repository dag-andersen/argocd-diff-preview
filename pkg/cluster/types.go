package cluster

import "time"

// Provider defines the interface for cluster management
type Provider interface {
	GetName() string
	IsInstalled() bool
	CreateCluster() (time.Duration, error)
	ClusterExists() bool
	DeleteCluster(wait bool)
}
