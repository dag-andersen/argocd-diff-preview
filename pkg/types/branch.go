package types

// Branch represents a git branch and its local folder
type Branch struct {
	Name       string
	folderName string
}

// NewBranch creates a new Branch instance
func NewBranch(name string, folderName string) *Branch {
	return &Branch{
		Name:       name,
		folderName: folderName,
	}
}

// FolderName returns the folder name for the branch
func (b *Branch) FolderName() string {
	return b.folderName
}
