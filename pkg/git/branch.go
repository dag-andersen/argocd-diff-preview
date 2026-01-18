package git

import "fmt"

// Branch represents a git branch and its local folder
type Branch struct {
	Name       string
	folderName string
	branchType BranchType
}

// BranchType represents the type of branch (base or target)
type BranchType string

const (
	// Base represents the base branch for comparison
	Base BranchType = "base"
	// Target represents the target branch for comparison
	Target BranchType = "target"
)

func (b BranchType) ShortName() string {
	switch b {
	case Base:
		return "b"
	case Target:
		return "t"
	}
	return "?"
}

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

// Type returns the type of the branch
func (b *Branch) Type() BranchType {
	return b.branchType
}
