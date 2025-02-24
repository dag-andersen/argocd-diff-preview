package types

import "fmt"

// Branch represents a git branch and its local folder
type Branch struct {
	Name       string
	folderName string
	branchType BranchType
}

type BranchType string

const (
	Base   BranchType = "base"
	Target BranchType = "target"
)

// NewBranch creates a new Branch instance
func NewBranch(name string, branchType BranchType) *Branch {
	return &Branch{
		Name:       name,
		folderName: fmt.Sprintf("%s-branch", branchType),
		branchType: branchType,
	}
}

// FolderName returns the folder name for the branch
func (b *Branch) FolderName() string {
	return b.folderName
}

func (b *Branch) Type() BranchType {
	return b.branchType
}
