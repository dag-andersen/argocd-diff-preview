package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBranch(t *testing.T) {
	tests := []struct {
		name       string
		branchName string
		branchType BranchType
		expected   *Branch
	}{
		{
			name:       "base branch",
			branchName: "main",
			branchType: Base,
			expected: &Branch{
				Name:       "main",
				folderName: "base-branch",
				branchType: Base,
			},
		},
		{
			name:       "target branch",
			branchName: "feature",
			branchType: Target,
			expected: &Branch{
				Name:       "feature",
				folderName: "target-branch",
				branchType: Target,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch := NewBranch(tt.branchName, tt.branchType)
			assert.Equal(t, tt.expected.Name, branch.Name)
			assert.Equal(t, tt.expected.folderName, branch.folderName)
			assert.Equal(t, tt.expected.branchType, branch.branchType)
		})
	}
}

func TestBranchMethods(t *testing.T) {
	branch := NewBranch("main", Base)

	t.Run("FolderName", func(t *testing.T) {
		assert.Equal(t, "base-branch", branch.FolderName())
	})

	t.Run("Type", func(t *testing.T) {
		assert.Equal(t, Base, branch.Type())
	})
}
