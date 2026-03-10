package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makePod(name string, containerStatuses []corev1.ContainerStatus, initContainerStatuses []corev1.ContainerStatus) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.PodStatus{
			ContainerStatuses:     containerStatuses,
			InitContainerStatuses: initContainerStatuses,
		},
	}
}

func makeContainerStatus(name string, restartCount int32, waitingReason string) corev1.ContainerStatus {
	cs := corev1.ContainerStatus{
		Name:         name,
		RestartCount: restartCount,
	}
	if waitingReason != "" {
		cs.State = corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				Reason: waitingReason,
			},
		}
	}
	return cs
}

func TestDetectCrashEvents_FirstPollNoEvents(t *testing.T) {
	state := NewCrashWatcherState()
	pods := []corev1.Pod{
		makePod("argocd-server-abc", []corev1.ContainerStatus{
			makeContainerStatus("argocd-server", 0, ""),
		}, nil),
		makePod("argocd-repo-server-xyz", []corev1.ContainerStatus{
			makeContainerStatus("argocd-repo-server", 2, ""),
		}, nil),
	}

	events := state.DetectCrashEvents(pods)
	if len(events) != 0 {
		t.Errorf("Expected 0 events on first poll, got %d: %+v", len(events), events)
	}

	// Verify baseline was recorded
	if state.RestartCounts["argocd-server-abc/argocd-server"] != 0 {
		t.Errorf("Expected restart count baseline 0 for server, got %d", state.RestartCounts["argocd-server-abc/argocd-server"])
	}
	if state.RestartCounts["argocd-repo-server-xyz/argocd-repo-server"] != 2 {
		t.Errorf("Expected restart count baseline 2 for repo-server, got %d", state.RestartCounts["argocd-repo-server-xyz/argocd-repo-server"])
	}
}

func TestDetectCrashEvents_RestartDetected(t *testing.T) {
	state := NewCrashWatcherState()
	pods := []corev1.Pod{
		makePod("argocd-server-abc", []corev1.ContainerStatus{
			makeContainerStatus("argocd-server", 0, ""),
		}, nil),
	}

	// First poll: establish baseline
	state.DetectCrashEvents(pods)

	// Second poll: restart count increased
	pods[0].Status.ContainerStatuses[0].RestartCount = 2
	events := state.DetectCrashEvents(pods)

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d: %+v", len(events), events)
	}
	if events[0].EventType != "restart" {
		t.Errorf("Expected event type 'restart', got '%s'", events[0].EventType)
	}
	if events[0].PodName != "argocd-server-abc" {
		t.Errorf("Expected pod name 'argocd-server-abc', got '%s'", events[0].PodName)
	}
	if events[0].ContainerName != "argocd-server" {
		t.Errorf("Expected container name 'argocd-server', got '%s'", events[0].ContainerName)
	}
	if events[0].PrevRestarts != 0 {
		t.Errorf("Expected prev restarts 0, got %d", events[0].PrevRestarts)
	}
	if events[0].CurrRestarts != 2 {
		t.Errorf("Expected curr restarts 2, got %d", events[0].CurrRestarts)
	}
}

func TestDetectCrashEvents_NoEventWhenRestartCountUnchanged(t *testing.T) {
	state := NewCrashWatcherState()
	pods := []corev1.Pod{
		makePod("argocd-server-abc", []corev1.ContainerStatus{
			makeContainerStatus("argocd-server", 3, ""),
		}, nil),
	}

	// First poll
	state.DetectCrashEvents(pods)

	// Second poll: same restart count
	events := state.DetectCrashEvents(pods)
	if len(events) != 0 {
		t.Errorf("Expected 0 events when restart count unchanged, got %d: %+v", len(events), events)
	}
}

func TestDetectCrashEvents_CrashLoopBackOff(t *testing.T) {
	state := NewCrashWatcherState()
	pods := []corev1.Pod{
		makePod("argocd-repo-server-xyz", []corev1.ContainerStatus{
			makeContainerStatus("repo-server", 3, "CrashLoopBackOff"),
		}, nil),
	}

	// First poll: should report CrashLoopBackOff
	events := state.DetectCrashEvents(pods)
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d: %+v", len(events), events)
	}
	if events[0].EventType != "crash-loop" {
		t.Errorf("Expected event type 'crash-loop', got '%s'", events[0].EventType)
	}
	if events[0].ContainerName != "repo-server" {
		t.Errorf("Expected container name 'repo-server', got '%s'", events[0].ContainerName)
	}
}

func TestDetectCrashEvents_CrashLoopNotReportedTwice(t *testing.T) {
	state := NewCrashWatcherState()
	pods := []corev1.Pod{
		makePod("argocd-repo-server-xyz", []corev1.ContainerStatus{
			makeContainerStatus("repo-server", 3, "CrashLoopBackOff"),
		}, nil),
	}

	// First poll
	state.DetectCrashEvents(pods)

	// Second poll: still in CrashLoopBackOff, should not report again
	events := state.DetectCrashEvents(pods)
	if len(events) != 0 {
		t.Errorf("Expected 0 events on second poll (already reported), got %d: %+v", len(events), events)
	}
}

func TestDetectCrashEvents_CrashLoopReportedAgainAfterRecoveryAndRelapse(t *testing.T) {
	state := NewCrashWatcherState()

	// Poll 1: CrashLoopBackOff
	pods := []corev1.Pod{
		makePod("argocd-server-abc", []corev1.ContainerStatus{
			makeContainerStatus("argocd-server", 3, "CrashLoopBackOff"),
		}, nil),
	}
	events := state.DetectCrashEvents(pods)
	if len(events) != 1 {
		t.Fatalf("Expected 1 event on first crash-loop, got %d", len(events))
	}

	// Poll 2: recovered (running normally)
	pods[0].Status.ContainerStatuses[0] = makeContainerStatus("argocd-server", 3, "")
	events = state.DetectCrashEvents(pods)
	if len(events) != 0 {
		t.Errorf("Expected 0 events after recovery, got %d: %+v", len(events), events)
	}

	// Poll 3: relapsed into CrashLoopBackOff again
	pods[0].Status.ContainerStatuses[0] = makeContainerStatus("argocd-server", 4, "CrashLoopBackOff")
	events = state.DetectCrashEvents(pods)

	// Should get both a restart event (3->4) and a crash-loop event
	if len(events) != 2 {
		t.Fatalf("Expected 2 events on relapse, got %d: %+v", len(events), events)
	}

	hasRestart := false
	hasCrashLoop := false
	for _, e := range events {
		if e.EventType == "restart" {
			hasRestart = true
		}
		if e.EventType == "crash-loop" {
			hasCrashLoop = true
		}
	}
	if !hasRestart {
		t.Error("Expected a 'restart' event on relapse")
	}
	if !hasCrashLoop {
		t.Error("Expected a 'crash-loop' event on relapse")
	}
}

func TestDetectCrashEvents_InitContainerRestart(t *testing.T) {
	state := NewCrashWatcherState()
	pods := []corev1.Pod{
		makePod("argocd-server-abc", nil, []corev1.ContainerStatus{
			makeContainerStatus("init-certs", 0, ""),
		}),
	}

	// First poll
	state.DetectCrashEvents(pods)

	// Second poll: init container restarted
	pods[0].Status.InitContainerStatuses[0].RestartCount = 1
	events := state.DetectCrashEvents(pods)

	if len(events) != 1 {
		t.Fatalf("Expected 1 event for init container restart, got %d: %+v", len(events), events)
	}
	if events[0].ContainerName != "init-certs" {
		t.Errorf("Expected container name 'init-certs', got '%s'", events[0].ContainerName)
	}
	if events[0].EventType != "restart" {
		t.Errorf("Expected event type 'restart', got '%s'", events[0].EventType)
	}
}

func TestDetectCrashEvents_MultipleContainersMultiplePods(t *testing.T) {
	state := NewCrashWatcherState()
	pods := []corev1.Pod{
		makePod("argocd-server-abc", []corev1.ContainerStatus{
			makeContainerStatus("argocd-server", 0, ""),
		}, nil),
		makePod("argocd-repo-server-xyz", []corev1.ContainerStatus{
			makeContainerStatus("repo-server", 0, ""),
		}, nil),
	}

	// First poll
	state.DetectCrashEvents(pods)

	// Second poll: both containers restarted
	pods[0].Status.ContainerStatuses[0].RestartCount = 1
	pods[1].Status.ContainerStatuses[0].RestartCount = 3
	events := state.DetectCrashEvents(pods)

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d: %+v", len(events), events)
	}

	names := map[string]bool{}
	for _, e := range events {
		names[e.ContainerName] = true
		if e.EventType != "restart" {
			t.Errorf("Expected event type 'restart', got '%s'", e.EventType)
		}
	}
	if !names["argocd-server"] {
		t.Error("Expected restart event for 'argocd-server'")
	}
	if !names["repo-server"] {
		t.Error("Expected restart event for 'repo-server'")
	}
}

func TestDetectCrashEvents_EmptyPodList(t *testing.T) {
	state := NewCrashWatcherState()
	events := state.DetectCrashEvents([]corev1.Pod{})
	if len(events) != 0 {
		t.Errorf("Expected 0 events for empty pod list, got %d: %+v", len(events), events)
	}
}

func TestDetectCrashEvents_RestartResetsExistingCrashLoopReporting(t *testing.T) {
	state := NewCrashWatcherState()

	// Poll 1: CrashLoopBackOff
	pods := []corev1.Pod{
		makePod("argocd-server-abc", []corev1.ContainerStatus{
			makeContainerStatus("argocd-server", 3, "CrashLoopBackOff"),
		}, nil),
	}
	state.DetectCrashEvents(pods)

	// Poll 2: restart count goes up but still CrashLoopBackOff - restart event should reset crash-loop tracking
	// so we report crash-loop again
	pods[0].Status.ContainerStatuses[0].RestartCount = 4
	events := state.DetectCrashEvents(pods)

	// Should get restart event AND crash-loop event (because restart resets the crash-loop reporting)
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d: %+v", len(events), events)
	}
}
