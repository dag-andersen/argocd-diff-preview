package diff

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/utils/merkletrie"
)

func TestDiff_prettyName(t *testing.T) {
	tests := []struct {
		name     string
		diff     Diff
		expected string
	}{
		{
			name:     "Both names, different",
			diff:     Diff{newName: "new-app", oldName: "old-app"},
			expected: "old-app -> new-app",
		},
		{
			name:     "Both names, same",
			diff:     Diff{newName: "app", oldName: "app"},
			expected: "app",
		},
		{
			name:     "Only new name",
			diff:     Diff{newName: "new-app"},
			expected: "new-app",
		},
		{
			name:     "Only old name",
			diff:     Diff{oldName: "old-app"},
			expected: "old-app",
		},
		{
			name:     "No names",
			diff:     Diff{},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diff.prettyName(); got != tt.expected {
				t.Errorf("prettyName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDiff_prettyPath(t *testing.T) {
	tests := []struct {
		name     string
		diff     Diff
		expected string
	}{
		{
			name:     "Both paths, different",
			diff:     Diff{newSourcePath: "/path/to/new", oldSourcePath: "/path/to/old"},
			expected: "/path/to/old -> /path/to/new",
		},
		{
			name:     "Both paths, same",
			diff:     Diff{newSourcePath: "/path/to/app", oldSourcePath: "/path/to/app"},
			expected: "/path/to/app",
		},
		{
			name:     "Only new path",
			diff:     Diff{newSourcePath: "/path/to/new"},
			expected: "/path/to/new",
		},
		{
			name:     "Only old path",
			diff:     Diff{oldSourcePath: "/path/to/old"},
			expected: "/path/to/old",
		},
		{
			name:     "No paths",
			diff:     Diff{},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diff.prettyPath(); got != tt.expected {
				t.Errorf("prettyPath() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDiff_commentHeader(t *testing.T) {
	tests := []struct {
		name     string
		diff     Diff
		expected string
	}{
		{
			name:     "Insert",
			diff:     Diff{newName: "app", newSourcePath: "/path", action: merkletrie.Insert},
			expected: "@@ Application added: app (/path) @@\n",
		},
		{
			name:     "Delete",
			diff:     Diff{oldName: "app", oldSourcePath: "/path", action: merkletrie.Delete},
			expected: "@@ Application deleted: app (/path) @@\n",
		},
		{
			name:     "Modify",
			diff:     Diff{newName: "app", newSourcePath: "/path", action: merkletrie.Modify},
			expected: "@@ Application modified: app (/path) @@\n",
		},
		{
			name:     "Unknown action",
			diff:     Diff{newName: "app", newSourcePath: "/path", action: 99}, // Assuming 99 is not a valid action
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.diff.commentHeader(); got != tt.expected {
				t.Errorf("commentHeader() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDiff_buildSection(t *testing.T) {
	tests := []struct {
		name        string
		diff        Diff
		expectedFmt string // Use fmt string for easier comparison of structure
	}{
		{
			name: "Insert",
			diff: Diff{
				newName:       "new-app",
				newSourcePath: "/path/new",
				action:        merkletrie.Insert,
				content:       "+ line 1\n+ line 2",
			},
			expectedFmt: "<details>\n<summary>%s</summary>\n<br>\n\n```diff\n%s\n```\n\n</details>\n\n",
		},
		{
			name: "Modify with name change",
			diff: Diff{
				newName:       "app-v2",
				oldName:       "app-v1",
				newSourcePath: "/path/app",
				oldSourcePath: "/path/app",
				action:        merkletrie.Modify,
				content:       "- line 1\n+ line 1 mod",
			},
			expectedFmt: "<details>\n<summary>%s</summary>\n<br>\n\n```diff\n%s\n```\n\n</details>\n\n",
		},
		{
			name: "Delete",
			diff: Diff{
				oldName:       "old-app",
				oldSourcePath: "/path/old",
				action:        merkletrie.Delete,
				content:       "- line 1\n- line 2",
			},
			expectedFmt: "<details>\n<summary>%s</summary>\n<br>\n\n```diff\n%s\n```\n\n</details>\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := fmt.Sprintf("%s (%s)", tt.diff.prettyName(), tt.diff.prettyPath())
			content := strings.TrimSpace(fmt.Sprintf("%s%s", tt.diff.commentHeader(), tt.diff.content))
			expected := fmt.Sprintf(tt.expectedFmt, header, content)
			if got := tt.diff.buildSection(); got != expected {
				t.Errorf("buildSection() got =\n%v\nwant =\n%v", got, expected)
			}
		})
	}
}
