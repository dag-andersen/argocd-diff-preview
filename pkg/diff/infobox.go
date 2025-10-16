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
	return fmt.Sprintf("_Stats_:\n%s", t.Stats())
}

func (t InfoBox) Stats() string {
	return fmt.Sprintf("[Applications: %d], [Full Run: %s], [Rendering: %s], [Cluster: %s], [Argo CD: %s]",
		t.ApplicationCount, t.FullDuration.Round(time.Second), t.ExtractDuration.Round(time.Second), t.ClusterCreationDuration.Round(time.Second), t.ArgoCDInstallationDuration.Round(time.Second))
}
