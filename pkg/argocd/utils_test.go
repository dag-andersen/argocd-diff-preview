package argocd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyManifestsFromFolder(t *testing.T) {
	t.Run("missing folder is a no-op", func(t *testing.T) {
		count, found, err := applyManifestsFromFolder(filepath.Join(t.TempDir(), "missing"), "preinstall", func(string) (int, error) {
			t.Fatal("apply must not be called")
			return 0, nil
		})

		require.NoError(t, err)
		assert.False(t, found)
		assert.Zero(t, count)
	})

	t.Run("applies direct files in filename order and skips directories", func(t *testing.T) {
		folder := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(folder, "20-second.yaml"), []byte("second"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(folder, "10-first.yaml"), []byte("first"), 0o600))
		require.NoError(t, os.Mkdir(filepath.Join(folder, "15-skipped"), 0o700))

		var applied []string
		count, found, err := applyManifestsFromFolder(folder, "preinstall", func(path string) (int, error) {
			applied = append(applied, filepath.Base(path))
			return 2, nil
		})

		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, 4, count)
		assert.Equal(t, []string{"10-first.yaml", "20-second.yaml"}, applied)
	})

	t.Run("stops after the first apply error", func(t *testing.T) {
		folder := t.TempDir()
		for _, name := range []string{"10-first.yaml", "20-fails.yaml", "30-skipped.yaml"} {
			require.NoError(t, os.WriteFile(filepath.Join(folder, name), []byte(name), 0o600))
		}

		var applied []string
		count, found, err := applyManifestsFromFolder(folder, "preinstall", func(path string) (int, error) {
			name := filepath.Base(path)
			applied = append(applied, name)
			if name == "20-fails.yaml" {
				return 0, fmt.Errorf("apply failed")
			}
			return 1, nil
		})

		assert.ErrorContains(t, err, "failed to apply preinstall 20-fails.yaml")
		assert.True(t, found)
		assert.Equal(t, 1, count)
		assert.Equal(t, []string{"10-first.yaml", "20-fails.yaml"}, applied)
	})
}
