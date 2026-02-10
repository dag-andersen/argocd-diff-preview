package diff

import (
	"fmt"
	"strings"
)

// ResourceInfo holds identifying information for a K8s resource
type ResourceInfo struct {
	Kind      string
	Name      string
	Namespace string
}

// FormatHeader returns the formatted header string for a resource
// e.g., "Deployment/my-deploy (default)" or "Deployment/my-deploy" if no namespace
// Note: Does NOT include markdown formatting like "####" - that's added by the output formatter
func (r *ResourceInfo) FormatHeader() string {
	if r == nil {
		return ""
	}

	var parts []string

	// Build Kind/Name part
	if r.Kind != "" && r.Name != "" {
		parts = append(parts, fmt.Sprintf("%s/%s", r.Kind, r.Name))
	} else if r.Kind != "" {
		parts = append(parts, r.Kind)
	} else if r.Name != "" {
		parts = append(parts, r.Name)
	} else {
		return ""
	}

	result := strings.Join(parts, "")

	// Add namespace in parentheses if present
	if r.Namespace != "" {
		result = fmt.Sprintf("%s (%s)", result, r.Namespace)
	}

	return result
}

// resourceRange represents the line range for a resource
type resourceRange struct {
	startLine int
	info      ResourceInfo
}

// ResourceIndex maps line numbers to resources
type ResourceIndex struct {
	resources []resourceRange // sorted by startLine
}

// BuildResourceIndex parses YAML content and builds a line-to-resource mapping
// It handles multi-document YAML (separated by ---) and extracts kind, metadata.name, metadata.namespace
func BuildResourceIndex(yamlContent string) *ResourceIndex {
	if yamlContent == "" {
		return &ResourceIndex{}
	}

	lines := strings.Split(yamlContent, "\n")
	var resources []resourceRange

	// Track current resource being parsed
	currentResourceStart := 0
	var currentLines []string

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check for document separator
		if trimmedLine == "---" {
			// If we have accumulated lines, parse them as a resource
			if len(currentLines) > 0 {
				info := parseResourceInfo(currentLines)
				resources = append(resources, resourceRange{
					startLine: currentResourceStart,
					info:      info,
				})
			}
			// Start new resource after the separator
			currentResourceStart = i + 1
			currentLines = nil
			continue
		}

		currentLines = append(currentLines, line)
	}

	// Parse the last resource if there are remaining lines
	if len(currentLines) > 0 {
		info := parseResourceInfo(currentLines)
		resources = append(resources, resourceRange{
			startLine: currentResourceStart,
			info:      info,
		})
	}

	return &ResourceIndex{resources: resources}
}

// GetResourceForLine returns the ResourceInfo for a given line number (0-based)
// Returns nil if line is before any resource or content is empty
func (idx *ResourceIndex) GetResourceForLine(lineNum int) *ResourceInfo {
	if idx == nil || len(idx.resources) == 0 {
		return nil
	}

	// Find the resource that contains this line using binary search
	// We want the last resource whose startLine <= lineNum
	left, right := 0, len(idx.resources)-1
	result := -1

	for left <= right {
		mid := (left + right) / 2
		if idx.resources[mid].startLine <= lineNum {
			result = mid
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	if result == -1 {
		return nil
	}

	return &idx.resources[result].info
}

// parseResourceInfo extracts kind, name, and namespace from YAML lines
// Uses simple string parsing to avoid full YAML unmarshaling overhead
func parseResourceInfo(lines []string) ResourceInfo {
	info := ResourceInfo{}

	inMetadata := false
	metadataIndent := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Calculate indentation
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Check for kind at root level
		if strings.HasPrefix(trimmed, "kind:") && indent == 0 {
			info.Kind = extractYAMLValue(trimmed, "kind:")
			continue
		}

		// Check for metadata section at root level
		if strings.HasPrefix(trimmed, "metadata:") && indent == 0 {
			inMetadata = true
			metadataIndent = indent
			continue
		}

		// If we're in metadata section
		if inMetadata {
			// Check if we've exited metadata (same or less indentation as metadata key)
			if indent <= metadataIndent && !strings.HasPrefix(trimmed, "name:") && !strings.HasPrefix(trimmed, "namespace:") {
				// Could be a new top-level key
				if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
					inMetadata = false
					continue
				}
			}

			// Look for name and namespace within metadata
			if strings.HasPrefix(trimmed, "name:") && info.Name == "" {
				info.Name = extractYAMLValue(trimmed, "name:")
			} else if strings.HasPrefix(trimmed, "namespace:") && info.Namespace == "" {
				info.Namespace = extractYAMLValue(trimmed, "namespace:")
			}

			// Stop parsing metadata if we have both name and namespace
			if info.Name != "" && info.Namespace != "" {
				inMetadata = false
			}
		}

		// Early exit if we have all the info we need
		if info.Kind != "" && info.Name != "" && info.Namespace != "" {
			break
		}
	}

	return info
}

// extractYAMLValue extracts the value from a "key: value" YAML line
func extractYAMLValue(line, prefix string) string {
	value := strings.TrimPrefix(line, prefix)
	value = strings.TrimSpace(value)

	// Remove quotes if present
	if len(value) >= 2 {
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}
	}

	return value
}
