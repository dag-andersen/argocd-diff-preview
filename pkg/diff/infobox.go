package diff

import (
	"fmt"
	"time"
)

type InfoBox struct {
	FullDuration               time.Duration
	ExtractDuration            time.Duration
	ArgoCDInstallationDuration time.Duration
	ClusterCreationDuration    time.Duration
	ApplicationCount           int
}

func (t InfoBox) String() string {
	return fmt.Sprintf("_Stats_:\n[Execution Time: %s], [Cluster Setup: %s], [ArgoCD Installation: %s], [Rendering: %s], [Applications: %d]",
		t.FullDuration.Round(time.Second), t.ClusterCreationDuration.Round(time.Second), t.ArgoCDInstallationDuration.Round(time.Second), t.ExtractDuration.Round(time.Second), t.ApplicationCount)
}
