package k8s

import (
	"context"
	"fmt"
	"sync"
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

// Message returns a user-facing description of the crash event.
func (e CrashEvent) Message() string {
	switch e.EventType {
	case "restart":
		return fmt.Sprintf(
			"Container '%s' in pod '%s' restarted (restarts: %d -> %d). This may cause rendering failures or timeouts.",
			e.ContainerName, e.PodName, e.PrevRestarts, e.CurrRestarts,
		)
	case "crash-loop":
		return fmt.Sprintf(
			"Container '%s' in pod '%s' is in CrashLoopBackOff. ArgoCD may not be functioning correctly.",
			e.ContainerName, e.PodName,
		)
	default:
		return fmt.Sprintf("Container '%s' in pod '%s' reported an unknown crash event.", e.ContainerName, e.PodName)
	}
}

// CrashEventRecorder stores crash events so they can be printed again near the
// end of the CLI output, where users are more likely to see them.
type CrashEventRecorder struct {
	mu     sync.Mutex
	events []CrashEvent
}

// Add records crash events detected by the watcher.
func (r *CrashEventRecorder) Add(events []CrashEvent) {
	if r == nil || len(events) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, events...)
}

// Events returns a snapshot of all recorded crash events.
func (r *CrashEventRecorder) Events() []CrashEvent {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	events := make([]CrashEvent, len(r.events))
	copy(events, r.events)
	return events
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
// Close the returned channel to stop watching. The returned recorder contains
// all detected events so callers can summarize them later.
func (c *Client) WatchForContainerRestarts(namespace, labelSelector string, pollInterval time.Duration) (chan struct{}, *CrashEventRecorder) {
	stopChan := make(chan struct{})
	recorder := &CrashEventRecorder{}

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
			recorder.Add(events)
			for _, event := range events {
				log.Warn().Msgf("🚨🚨🚨🚨🚨 %s", event.Message())
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

	return stopChan, recorder
}
