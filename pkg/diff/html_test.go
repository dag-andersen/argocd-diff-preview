package diff

import (
	"strings"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/matching"
	"time"
)

// Tests for HTMLSection.printHTMLSection

func TestHTMLSection_PrintHTMLSection_Basic(t *testing.T) {
	section := HTMLSection{
		appName:  "my-app",
		filePath: "path/to/app",
		appURL:   "",
		resources: []ResourceSection{
			{
				Header:  "Deployment: my-deploy (default)",
				Content: "+replicas: 3\n-replicas: 1\n",
			},
		},
	}

	result := section.printHTMLSection()

	// Should contain the summary
	if !strings.Contains(result, "my-app (path/to/app)") {
		t.Errorf("expected summary with app name and path, got:\n%s", result)
	}
	// Should NOT contain link when no URL
	if strings.Contains(result, "<a href=") {
		t.Errorf("should not contain link when appURL is empty, got:\n%s", result)
	}
	// Should contain resource header
	if !strings.Contains(result, "Deployment: my-deploy (default)") {
		t.Errorf("expected resource header, got:\n%s", result)
	}
	// Should contain diff lines with correct classes
	if !strings.Contains(result, "added_line") {
		t.Errorf("expected added_line class for + line, got:\n%s", result)
	}
	if !strings.Contains(result, "removed_line") {
		t.Errorf("expected removed_line class for - line, got:\n%s", result)
	}
}

func TestHTMLSection_PrintHTMLSection_WithURL(t *testing.T) {
	section := HTMLSection{
		appName:  "my-app",
		filePath: "path/to/app",
		appURL:   "https://argocd.example.com/applications/my-app",
		resources: []ResourceSection{
			{Header: "Deployment: test (ns)", Content: " unchanged\n"},
		},
	}

	result := section.printHTMLSection()

	if !strings.Contains(result, `<a href="https://argocd.example.com/applications/my-app">link</a>`) {
		t.Errorf("expected link in summary, got:\n%s", result)
	}
}

func TestHTMLSection_PrintHTMLSection_EmptyResources(t *testing.T) {
	section := HTMLSection{
		appName:     "my-app",
		filePath:    "path/to/app",
		resources:   []ResourceSection{},
		emptyReason: matching.EmptyReasonNoResources,
	}

	result := section.printHTMLSection()

	if !strings.Contains(result, "Application rendered no resources") {
		t.Errorf("expected 'Application rendered no resources' for empty resources, got:\n%s", result)
	}
}

func TestHTMLSection_PrintHTMLSection_SkippedResource(t *testing.T) {
	section := HTMLSection{
		appName:  "my-app",
		filePath: "path/to/app",
		resources: []ResourceSection{
			{Header: "Secret: my-secret (default)", IsSkipped: true},
		},
	}

	result := section.printHTMLSection()

	if !strings.Contains(result, "Skipped") {
		t.Errorf("expected 'Skipped' for skipped resource, got:\n%s", result)
	}
	// Should NOT contain a diff table
	if strings.Contains(result, "diff_container") {
		t.Errorf("should not contain diff table for skipped resource, got:\n%s", result)
	}
}

func TestHTMLSection_PrintHTMLSection_CommentLine(t *testing.T) {
	section := HTMLSection{
		appName:  "my-app",
		filePath: "path/to/app",
		resources: []ResourceSection{
			{Header: "Deployment: test (ns)", Content: "@@ skipped 5 lines (10 -> 15) @@\n"},
		},
	}

	result := section.printHTMLSection()

	if !strings.Contains(result, "comment_line") {
		t.Errorf("expected comment_line class for @@ line, got:\n%s", result)
	}
}

func TestHTMLSection_PrintHTMLSection_NormalLine(t *testing.T) {
	section := HTMLSection{
		appName:  "my-app",
		filePath: "path/to/app",
		resources: []ResourceSection{
			{Header: "Deployment: test (ns)", Content: " apiVersion: apps/v1\n"},
		},
	}

	result := section.printHTMLSection()

	if !strings.Contains(result, "normal_line") {
		t.Errorf("expected normal_line class for context line, got:\n%s", result)
	}
}

func TestHTMLSection_PrintHTMLSection_HTMLEscaping(t *testing.T) {
	section := HTMLSection{
		appName:  "my-app<script>alert(1)</script>",
		filePath: "path/to/<app>",
		appURL:   "https://example.com/app?a=1&b=2",
		resources: []ResourceSection{
			{
				Header:  "ConfigMap: <test> (default)",
				Content: "+key: <value>&data\n",
			},
		},
	}

	result := section.printHTMLSection()

	// App name and file path should be escaped
	if strings.Contains(result, "<script>") {
		t.Errorf("expected HTML escaping for app name, got:\n%s", result)
	}
	if strings.Contains(result, "<app>") && !strings.Contains(result, "&lt;app&gt;") {
		t.Errorf("expected HTML escaping for file path, got:\n%s", result)
	}
	// Content should be escaped
	if strings.Contains(result, "<value>") && !strings.Contains(result, "&lt;value&gt;") {
		t.Errorf("expected HTML escaping for content, got:\n%s", result)
	}
}

func TestHTMLSection_PrintHTMLSection_MultipleResources(t *testing.T) {
	section := HTMLSection{
		appName:  "my-app",
		filePath: "path",
		resources: []ResourceSection{
			{Header: "Deployment: app (default)", Content: "+replicas: 3\n"},
			{Header: "Secret: creds (default)", IsSkipped: true},
			{Header: "ConfigMap: cfg (default)", Content: "-key: old\n+key: new\n"},
		},
	}

	result := section.printHTMLSection()

	// All three resource headers should be present
	if !strings.Contains(result, "Deployment: app (default)") {
		t.Error("expected Deployment header")
	}
	if !strings.Contains(result, "Secret: creds (default)") {
		t.Error("expected Secret header")
	}
	if !strings.Contains(result, "ConfigMap: cfg (default)") {
		t.Error("expected ConfigMap header")
	}
}

// Tests for HTMLOutput.printDiff

func TestHTMLOutput_PrintDiff_Basic(t *testing.T) {
	output := HTMLOutput{
		title:   "Test Diff",
		summary: "Total: 1 files changed",
		sections: []HTMLSection{
			{
				appName:  "my-app",
				filePath: "path/to/app",
				resources: []ResourceSection{
					{Header: "Deployment: app (ns)", Content: "+replicas: 3\n"},
				},
			},
		},
		statsInfo: StatsInfo{
			ApplicationCount: 1,
			FullDuration:     time.Second * 10,
		},
	}

	result := output.printDiff()

	if !strings.Contains(result, "<h1>Test Diff</h1>") {
		t.Errorf("expected title in output, got:\n%s", result)
	}
	if !strings.Contains(result, "Total: 1 files changed") {
		t.Errorf("expected summary in output, got:\n%s", result)
	}
	if !strings.Contains(result, "my-app") {
		t.Errorf("expected app name in output, got:\n%s", result)
	}
	if !strings.Contains(result, "Applications: 1") {
		t.Errorf("expected stats in output, got:\n%s", result)
	}
	if !strings.HasSuffix(result, "\n") {
		t.Error("expected output to end with newline")
	}
}

func TestHTMLOutput_PrintDiff_NoSections(t *testing.T) {
	output := HTMLOutput{
		title:    "Empty Diff",
		summary:  "No changes",
		sections: []HTMLSection{},
	}

	result := output.printDiff()

	if !strings.Contains(result, "No changes found") {
		t.Errorf("expected 'No changes found' for empty sections, got:\n%s", result)
	}
}

func TestHTMLOutput_PrintDiff_SelectionInfo(t *testing.T) {
	output := HTMLOutput{
		title:    "Test",
		summary:  "Summary",
		sections: []HTMLSection{},
		selectionInfo: SelectionInfo{
			Base:   AppSelectionInfo{SkippedApplications: 2, SkippedApplicationSets: 1},
			Target: AppSelectionInfo{SkippedApplications: 5, SkippedApplicationSets: 1},
		},
	}

	result := output.printDiff()

	if !strings.Contains(result, "_Skipped resources_") {
		t.Errorf("expected skipped resources info, got:\n%s", result)
	}
	if !strings.Contains(result, "Applications: `2` (base) -> `5` (target)") {
		t.Errorf("expected application counts, got:\n%s", result)
	}
}

func TestHTMLOutput_PrintDiff_NoSelectionInfo(t *testing.T) {
	output := HTMLOutput{
		title:   "Test",
		summary: "Summary",
		sections: []HTMLSection{
			{appName: "app", filePath: "path", resources: []ResourceSection{{Header: "H", Content: "+x\n"}}},
		},
		selectionInfo: SelectionInfo{
			Base:   AppSelectionInfo{SkippedApplications: 3, SkippedApplicationSets: 1},
			Target: AppSelectionInfo{SkippedApplications: 3, SkippedApplicationSets: 1},
		},
	}

	result := output.printDiff()

	// Counts are equal, so no selection info should be shown
	if strings.Contains(result, "_Skipped resources_") {
		t.Errorf("should not contain selection info when counts are equal, got:\n%s", result)
	}
}

func TestHTMLOutput_PrintDiff_TemplatePlaceholders(t *testing.T) {
	output := HTMLOutput{
		title:   "My Title",
		summary: "My Summary",
		sections: []HTMLSection{
			{appName: "app", filePath: "path", resources: []ResourceSection{{Header: "H", Content: "+x\n"}}},
		},
		statsInfo: StatsInfo{ApplicationCount: 1},
	}

	result := output.printDiff()

	// All placeholders should be replaced
	placeholders := []string{"%title%", "%summary%", "%app_diffs%", "%selection_changes%", "%info_box%"}
	for _, p := range placeholders {
		if strings.Contains(result, p) {
			t.Errorf("placeholder %q was not replaced in output", p)
		}
	}
}

func TestHTMLOutput_PrintDiff_ValidHTML(t *testing.T) {
	output := HTMLOutput{
		title:   "Test",
		summary: "Summary",
		sections: []HTMLSection{
			{appName: "app", filePath: "path", resources: []ResourceSection{{Header: "H", Content: "+line\n"}}},
		},
	}

	result := output.printDiff()

	if !strings.HasPrefix(strings.TrimSpace(result), "<html>") {
		t.Error("expected output to start with <html>")
	}
	if !strings.Contains(result, "</html>") {
		t.Error("expected output to contain </html>")
	}
}
