package reposerverextract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestLocalRefSourcesAvailableIgnoresOptionalMissingValueFile(t *testing.T) {
	branchFolder := t.TempDir()
	primary := v1alpha1.ApplicationSource{
		Helm: &v1alpha1.ApplicationSourceHelm{
			ValueFiles:              []string{"$values/optional.yaml"},
			IgnoreMissingValueFiles: true,
		},
	}
	refs := []v1alpha1.ApplicationSource{{Ref: "values"}}

	available, reason := localRefSourcesAvailable(branchFolder, primary, refs)
	if !available {
		t.Fatalf("optional missing value file should keep the local render path available: %s", reason)
	}
}

func TestLocalRefSourcesAvailableRejectsRequiredMissingValueFile(t *testing.T) {
	branchFolder := t.TempDir()
	primary := v1alpha1.ApplicationSource{
		Helm: &v1alpha1.ApplicationSourceHelm{
			ValueFiles: []string{"$values/required.yaml"},
		},
	}
	refs := []v1alpha1.ApplicationSource{{Ref: "values"}}

	available, reason := localRefSourcesAvailable(branchFolder, primary, refs)
	if available {
		t.Fatal("required missing value file should make the local render path unavailable")
	}
	if !strings.Contains(reason, "required.yaml") || !strings.Contains(reason, "does not exist") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestLocalRefSourcesAvailableRejectsMissingCheckout(t *testing.T) {
	branchFolder := filepath.Join(t.TempDir(), "missing-checkout")
	primary := v1alpha1.ApplicationSource{}
	refs := []v1alpha1.ApplicationSource{{Ref: "values"}}

	available, reason := localRefSourcesAvailable(branchFolder, primary, refs)
	if available {
		t.Fatal("missing checkout should make the local render path unavailable")
	}
	if !strings.Contains(reason, "missing-checkout") || !strings.Contains(reason, "does not exist") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestLocalRefSourcesAvailableDoesNotRequireRefContentPath(t *testing.T) {
	branchFolder := t.TempDir()
	refs := []v1alpha1.ApplicationSource{{Ref: "values", Path: "missing-content"}}
	available, reason := localRefSourcesAvailable(branchFolder, v1alpha1.ApplicationSource{}, refs)
	if !available {
		t.Fatalf("a ref source's content path does not affect its repository root: %s", reason)
	}

	available, _ = localContentAndRefSourcesAvailable(branchFolder, refs[0], refs)
	if available {
		t.Fatal("rendering the ref source's own content must still require its path")
	}
}

func TestLocalRefSourcesAvailableAcceptsExistingRequiredValueFile(t *testing.T) {
	branchFolder := t.TempDir()
	valueFile := filepath.Join(branchFolder, "required.yaml")
	if err := os.WriteFile(valueFile, []byte("key: value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	primary := v1alpha1.ApplicationSource{
		Helm: &v1alpha1.ApplicationSourceHelm{
			ValueFiles: []string{"$values/required.yaml"},
		},
	}
	refs := []v1alpha1.ApplicationSource{{Ref: "values"}}

	available, reason := localRefSourcesAvailable(branchFolder, primary, refs)
	if !available {
		t.Fatalf("existing required value file should keep the local render path available: %s", reason)
	}
}
