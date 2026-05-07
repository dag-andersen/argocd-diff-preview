package argocd

import (
	"strings"
	"testing"
)

func TestIsOCIChartURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"plain oci url", "oci://ghcr.io/argoproj/argo-helm/argo-cd", true},
		{"oci url with leading whitespace", "  oci://ghcr.io/x", true},
		{"oci url with trailing whitespace", "oci://ghcr.io/x  ", true},
		{"https url", "https://argoproj.github.io/argo-helm", false},
		{"http url", "http://example.com/charts", false},
		{"empty string", "", false},
		{"uppercase scheme is not recognized", "OCI://ghcr.io/x", false},
		{"oci substring but not scheme", "https://oci.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOCIChartURL(tt.url); got != tt.want {
				t.Errorf("isOCIChartURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestOCIHost(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantHost  string
		wantError bool
	}{
		{"host with path", "oci://ghcr.io/argoproj/argo-helm/argo-cd", "ghcr.io", false},
		{"host only, no path", "oci://ghcr.io", "ghcr.io", false},
		{"host with port", "oci://registry.local:5000/charts/argo-cd", "registry.local:5000", false},
		{"leading and trailing whitespace", "  oci://ghcr.io/x  ", "ghcr.io", false},
		{"empty after scheme", "oci://", "", true},
		{"missing scheme is not validated", "ghcr.io/x", "ghcr.io", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ociHost(tt.url)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.url)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.url, err)
				return
			}
			if got != tt.wantHost {
				t.Errorf("ociHost(%q) = %q, want %q", tt.url, got, tt.wantHost)
			}
		})
	}
}

// TestInstallWithHelm_OCIRequiresExplicitVersion exercises the early-return
// guardrail in installWithHelm. The check fires before any Helm/k8s setup, so
// the test does not need a cluster — only the OCI URL and an empty Version
// (which is what triggers "install latest").
func TestInstallWithHelm_OCIRequiresExplicitVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"empty version", ""},
		{"latest keyword", "latest"},
		{"whitespace only", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ArgoCDInstallation{
				ChartURL: "oci://ghcr.io/argoproj/argo-helm/argo-cd",
				Version:  tt.version,
			}
			err := a.installWithHelm()
			if err == nil {
				t.Fatal("expected error for OCI without --argocd-chart-version, got nil")
			}
			if !strings.Contains(err.Error(), "explicit --argocd-chart-version is required") {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
