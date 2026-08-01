package k8s

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
)

// TestFallbackCondition validates that WebSocket failures trigger SPDY fallback
func TestFallbackCondition(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		shouldFallback bool
	}{
		{
			name:           "upgrade failure triggers fallback",
			err:            &httpstream.UpgradeFailureError{Cause: fmt.Errorf("upgrade failed")},
			shouldFallback: true,
		},
		{
			name:           "other errors do not trigger fallback",
			err:            fmt.Errorf("network timeout"),
			shouldFallback: false,
		},
		{
			name:           "nil error does not trigger fallback",
			err:            nil,
			shouldFallback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is the exact condition from PortForwardToPod:60
			result := httpstream.IsUpgradeFailure(tt.err) || httpstream.IsHTTPSProxyError(tt.err)

			if result != tt.shouldFallback {
				t.Errorf("expected %v, got %v for error: %v",
					tt.shouldFallback, result, tt.err)
			}
		})
	}
}

func TestChooseServicePort(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "argocd-repo-server"},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{Name: "metrics", Port: 8084},
				{Name: "server", Port: 8081},
			},
		},
	}

	tests := []struct {
		name              string
		preferredPortName string
		preferredPort     int32
		want              int32
	}{
		{
			name:              "prefers named port",
			preferredPortName: "server",
			preferredPort:     8084,
			want:              8081,
		},
		{
			name:              "falls back to preferred numeric port",
			preferredPortName: "grpc",
			preferredPort:     8081,
			want:              8081,
		},
		{
			name:              "falls back to first port",
			preferredPortName: "grpc",
			preferredPort:     9090,
			want:              8084,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := chooseServicePort(service, tt.preferredPortName, tt.preferredPort)
			if err != nil {
				t.Fatalf("chooseServicePort() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("chooseServicePort() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestServiceDNSAddress(t *testing.T) {
	got := serviceDNSAddress("argocd-repo-server", "argocd", 8081)
	want := "argocd-repo-server.argocd.svc:8081"

	if got != want {
		t.Fatalf("serviceDNSAddress() = %q, want %q", got, want)
	}
}
