package types

import (
	"testing"
)

func TestNewBranch(t *testing.T) {
	tests := []struct {
		name       string
		branchName string
		branchType BranchType
		want       *Branch
	}{
		{
			name:       "Create base branch",
			branchName: "main",
			branchType: Base,
			want: &Branch{
				Name:       "main",
				folderName: "base-branch",
				branchType: Base,
			},
		},
		{
			name:       "Create target branch",
			branchName: "feature/new-feature",
			branchType: Target,
			want: &Branch{
				Name:       "feature/new-feature",
				folderName: "target-branch",
				branchType: Target,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewBranch(tt.branchName, tt.branchType)
			if got.Name != tt.want.Name {
				t.Errorf("NewBranch().Name = %v, want %v", got.Name, tt.want.Name)
			}
			if got.folderName != tt.want.folderName {
				t.Errorf("NewBranch().folderName = %v, want %v", got.folderName, tt.want.folderName)
			}
			if got.branchType != tt.want.branchType {
				t.Errorf("NewBranch().branchType = %v, want %v", got.branchType, tt.want.branchType)
			}
		})
	}
}

func TestBranch_FolderName(t *testing.T) {
	tests := []struct {
		name   string
		branch *Branch
		want   string
	}{
		{
			name: "Base branch folder name",
			branch: &Branch{
				Name:       "main",
				folderName: "base-branch",
				branchType: Base,
			},
			want: "base-branch",
		},
		{
			name: "Target branch folder name",
			branch: &Branch{
				Name:       "feature/new-feature",
				folderName: "target-branch",
				branchType: Target,
			},
			want: "target-branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.branch.FolderName(); got != tt.want {
				t.Errorf("Branch.FolderName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBranch_Type(t *testing.T) {
	tests := []struct {
		name   string
		branch *Branch
		want   BranchType
	}{
		{
			name: "Base branch type",
			branch: &Branch{
				Name:       "main",
				folderName: "base-branch",
				branchType: Base,
			},
			want: Base,
		},
		{
			name: "Target branch type",
			branch: &Branch{
				Name:       "feature/new-feature",
				folderName: "target-branch",
				branchType: Target,
			},
			want: Target,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.branch.Type(); got != tt.want {
				t.Errorf("Branch.Type() = %v, want %v", got, tt.want)
			}
		})
	}
}
