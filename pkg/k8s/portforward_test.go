package k8s

import (
	"fmt"
	"testing"

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
