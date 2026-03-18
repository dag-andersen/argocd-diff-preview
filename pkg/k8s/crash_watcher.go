package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CrashWatcherState tracks restart counts and CrashLoopBackOff state across polls
// for the container crash watcher. Exported for testability.
type CrashWatcherState struct {
	// RestartCounts tracks "podName/containerName" -> last seen restart count
	RestartCounts map[string]int32
	// CrashLoopReported tracks whether we've already warned about CrashLoopBackOff for a given key
	CrashLoopReported map[string]bool
}

// NewCrashWatcherState creates a new CrashWatcherState with initialized maps.
func NewCrashWatcherState() *CrashWatcherState {
	return &CrashWatcherState{
		RestartCounts:     make(map[string]int32),
		CrashLoopReported: make(map[string]bool),
	}
}

// CrashEvent represents a single crash event detected by the watcher.
type CrashEvent struct {
	PodName       string
	ContainerName string
	EventType     string // "restart" or "crash-loop"
	PrevRestarts  int32  // only set for "restart" events
	CurrRestarts  int32  // only set for "restart" events
}

// DetectCrashEvents inspects container statuses from a list of pods and returns
// any new crash events (restarts or CrashLoopBackOff) since the last poll.
// It updates the state in-place so subsequent calls only report new events.
func (s *CrashWatcherState) DetectCrashEvents(pods []corev1.Pod) []CrashEvent {
	var events []CrashEvent

	for _, pod := range pods {
		allStatuses := append(pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses...)
		for _, cs := range allStatuses {
			key := fmt.Sprintf("%s/%s", pod.Name, cs.Name)

			prevCount, seen := s.RestartCounts[key]
			s.RestartCounts[key] = cs.RestartCount

			if seen && cs.RestartCount > prevCount {
				events = append(events, CrashEvent{
					PodName:       pod.Name,
					ContainerName: cs.Name,
					EventType:     "restart",
					PrevRestarts:  prevCount,
					CurrRestarts:  cs.RestartCount,
				})
				// Reset CrashLoopBackOff tracking so we report it again if it recurs after a restart
				s.CrashLoopReported[key] = false
			}

			// Check for CrashLoopBackOff
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				if !s.CrashLoopReported[key] {
					events = append(events, CrashEvent{
						PodName:       pod.Name,
						ContainerName: cs.Name,
						EventType:     "crash-loop",
					})
					s.CrashLoopReported[key] = true
				}
			} else {
				// Container is no longer in CrashLoopBackOff, reset so we can detect it again
				s.CrashLoopReported[key] = false
			}
		}
	}

	return events
}

// WatchForContainerRestarts starts a background goroutine that monitors pods matching
// the given label selector for container restarts. It logs a warning whenever a
// container's restart count increases or when a container enters CrashLoopBackOff.
// Close the returned channel to stop watching.
func (c *Client) WatchForContainerRestarts(namespace, labelSelector string, pollInterval time.Duration) chan struct{} {
	stopChan := make(chan struct{})

	go func() {
		clientset, err := kubernetes.NewForConfig(c.config)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create clientset for crash watcher")
			return
		}

		state := NewCrashWatcherState()

		// pollOnce lists pods and processes crash events, logging any warnings.
		// Returns false if the stop channel has been closed.
		pollOnce := func() bool {
			pods, err := clientset.CoreV1().Pods(namespace).List(
				context.Background(),
				metav1.ListOptions{LabelSelector: labelSelector},
			)
			if err != nil {
				log.Debug().Err(err).Msg("Crash watcher: failed to list pods")
				return true
			}

			events := state.DetectCrashEvents(pods.Items)
			for _, event := range events {
				switch event.EventType {
				case "restart":
					log.Warn().Msgf(
						"🚨🚨🚨🚨🚨 Container '%s' in pod '%s' has restarted (restarts: %d -> %d). This may cause rendering failures or timeouts.",
						event.ContainerName, event.PodName, event.PrevRestarts, event.CurrRestarts,
					)
				case "crash-loop":
					log.Warn().Msgf(
						"🚨🚨🚨🚨🚨 Container '%s' in pod '%s' is in CrashLoopBackOff. ArgoCD may not be functioning correctly.",
						event.ContainerName, event.PodName,
					)
				}
			}
			return true
		}

		// Poll immediately to establish baseline restart counts before the first tick
		pollOnce()

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				pollOnce()
			}
		}
	}()

	return stopChan
}
