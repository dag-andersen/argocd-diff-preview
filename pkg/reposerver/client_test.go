package reposerver

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsRetryableRenderError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "grpc unavailable",
			err:  status.Error(codes.Unavailable, "connection reset"),
			want: true,
		},
		{
			// Send on an aborted stream returns a bare io.EOF, wrapped twice by the time the retry loop sees it.
			name: "wrapped io.EOF from stream send",
			err:  fmt.Errorf("failed to stream tarball: %w", fmt.Errorf("failed to send chunk: %w", io.EOF)),
			want: true,
		},
		{
			name: "grpc invalid argument",
			err:  status.Error(codes.InvalidArgument, "bad request"),
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("kustomize build failed"),
			want: false,
		},
		{
			// Deterministic server rejection: the real status (Unknown)
			// replaces the bare io.EOF and must not be retried.
			name: "wrapped Unknown status from aborted stream",
			err:  fmt.Errorf("failed to stream tarball: %w", fmt.Errorf("stream aborted: %w (send: %v)", status.Error(codes.Unknown, "error receiving tgz file: file exceeded max size of 100000000 bytes"), io.EOF)),
			want: false,
		},
		{
			// Transient transport abort stays retryable.
			name: "wrapped Unavailable status from aborted stream",
			err:  fmt.Errorf("failed to stream tarball: %w", fmt.Errorf("stream aborted: %w (send: %v)", status.Error(codes.Unavailable, "transport is closing"), io.EOF)),
			want: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableRenderError(tc.err); got != tc.want {
				t.Errorf("isRetryableRenderError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
