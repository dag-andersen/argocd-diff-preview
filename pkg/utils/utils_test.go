package utils

import (
	"testing"
)

func TestSplitYAMLDocuments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single document",
			input:    "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test",
			expected: []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test"},
		},
		{
			name: "two documents with clean separators",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test1
---
apiVersion: v1
kind: Secret
metadata:
  name: test2`,
			expected: []string{
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test1",
				"apiVersion: v1\nkind: Secret\nmetadata:\n  name: test2",
			},
		},
		{
			name: "documents with whitespace after ---",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test1
---   
apiVersion: v1
kind: Secret
metadata:
  name: test2
---		# with tabs
apiVersion: v1
kind: Service
metadata:
  name: test3`,
			expected: []string{
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test1",
				"apiVersion: v1\nkind: Secret\nmetadata:\n  name: test2",
				"apiVersion: v1\nkind: Service\nmetadata:\n  name: test3",
			},
		},
		{
			name: "documents with --- in content should not split",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  description: "This contains --- in the middle"
  data:
    key: "value with --- separator"
---
apiVersion: v1
kind: Secret
metadata:
  name: test2`,
			expected: []string{
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  description: "This contains --- in the middle"
  data:
    key: "value with --- separator"`,
				"apiVersion: v1\nkind: Secret\nmetadata:\n  name: test2",
			},
		},
		{
			name: "documents with indented --- should not split",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  data:
    key: |
      some content
      ---
      more content
---
apiVersion: v1
kind: Secret
metadata:
  name: test2`,
			expected: []string{
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  data:
    key: |
      some content
      ---
      more content`,
				"apiVersion: v1\nkind: Secret\nmetadata:\n  name: test2",
			},
		},
		{
			name: "documents with --- at end of line should not split",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  description: "This ends with ---"
---
apiVersion: v1
kind: Secret
metadata:
  name: test2`,
			expected: []string{
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  description: "This ends with ---"`,
				"apiVersion: v1\nkind: Secret\nmetadata:\n  name: test2",
			},
		},
		{
			name: "empty documents should be filtered out",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test1
---
---
apiVersion: v1
kind: Secret
metadata:
  name: test2
---

apiVersion: v1
kind: Service
metadata:
  name: test3`,
			expected: []string{
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test1",
				"apiVersion: v1\nkind: Secret\nmetadata:\n  name: test2",
				"apiVersion: v1\nkind: Service\nmetadata:\n  name: test3",
			},
		},
		{
			name: "documents with mixed whitespace after ---",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test1
---  ` + "\t" + `  
apiVersion: v1
kind: Secret
metadata:
  name: test2
---		# comment with tabs
apiVersion: v1
kind: Service
metadata:
  name: test3`,
			expected: []string{
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test1",
				"apiVersion: v1\nkind: Secret\nmetadata:\n  name: test2",
				"apiVersion: v1\nkind: Service\nmetadata:\n  name: test3",
			},
		},
		{
			name: "single document with --- in description",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  description: |
    This is a description that contains ---
    multiple lines with --- separators
    but should not be split`,
			expected: []string{`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  description: |
    This is a description that contains ---
    multiple lines with --- separators
    but should not be split`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitYAMLDocuments(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d documents, got %d", len(tt.expected), len(result))
				return
			}

			for i, doc := range result {
				if doc != tt.expected[i] {
					t.Errorf("Document %d mismatch:\nExpected:\n%s\n\nGot:\n%s", i, tt.expected[i], doc)
				}
			}
		})
	}
}

func TestSplitYAMLDocumentsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "only separators",
			input:    "---\n---\n---",
			expected: []string{},
		},
		{
			name:     "separators with whitespace",
			input:    "---\n   \n---\n\t\n---",
			expected: []string{},
		},
		{
			name:     "single separator",
			input:    "---",
			expected: []string{},
		},
		{
			name:     "separator with whitespace",
			input:    "---   ",
			expected: []string{},
		},
		{
			name: "document with only whitespace",
			input: `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
---
   
---
apiVersion: v1
kind: Secret
metadata:
  name: test2`,
			expected: []string{
				"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test",
				"apiVersion: v1\nkind: Secret\nmetadata:\n  name: test2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitYAMLDocuments(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d documents, got %d", len(tt.expected), len(result))
				return
			}

			for i, doc := range result {
				if doc != tt.expected[i] {
					t.Errorf("Document %d mismatch:\nExpected:\n%s\n\nGot:\n%s", i, tt.expected[i], doc)
				}
			}
		})
	}
}
