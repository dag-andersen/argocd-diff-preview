package fileparsing

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// GetYamlFiles gets all YAML files in a directory
func GetYamlFiles(directory string, fileRegex *regexp.Regexp) []string {
	log.Debug().Msgf("Fetching all files in dir: %s", directory)

	var yamlFiles []string
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check if file has .yaml or .yml extension
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Convert path to relative path
		relPath, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}

		// Check regex if provided
		if fileRegex != nil {
			if !fileRegex.MatchString(relPath) {
				return nil
			}
		}

		yamlFiles = append(yamlFiles, relPath)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("⚠️ Error reading directory")
		return []string{}
	}

	if fileRegex != nil {
		log.Debug().Msgf("Found %d yaml files in dir '%s' matching regex: %s",
			len(yamlFiles), directory, fileRegex.String())
	} else {
		log.Debug().Msgf("Found %d yaml files in dir '%s'",
			len(yamlFiles), directory)
	}

	return yamlFiles
}

// ParseYaml parses YAML files into Resources
func ParseYaml(dir string, files []string, branch git.BranchType) []Resource {
	var resources []Resource

	for _, file := range files {
		log.Debug().Msgf("In dir '%s' found yaml file: %s", dir, file)

		// Open and read file
		f, err := os.Open(filepath.Join(dir, file))
		if err != nil {
			log.Warn().Err(err).Msgf("⚠️ Failed to open file '%s'", file)
			continue
		}
		defer func() {
			if err := f.Close(); err != nil {
				log.Warn().Err(err).Msgf("⚠️ Failed to close file '%s'", file)
			}
		}()

		// Read file line by line and split on "---"
		var currentChunk strings.Builder
		scanner := bufio.NewScanner(f)

		for scanner.Scan() {
			line := scanner.Text()

			if line == "---" {
				// Process the current chunk if it's not empty
				if currentChunk.Len() > 0 {
					if resource, ok := processYamlChunk(file, currentChunk.String(), branch); ok {
						resources = append(resources, *resource)
					}
				}
				currentChunk.Reset()
			} else {
				currentChunk.WriteString(line)
				currentChunk.WriteString("\n")
			}
		}

		// Process the last chunk
		if currentChunk.Len() > 0 {
			if resource, ok := processYamlChunk(file, currentChunk.String(), branch); ok {
				resources = append(resources, *resource)
			}
		}
	}

	return resources
}

// processYamlChunk parses a YAML chunk into a Resource
// A chunk is a single YAML object, e.g. a Deployment, Service, etc.
// Returns (resource, shouldInclude) where shouldInclude is true if the chunk is a valid resource
func processYamlChunk(filename, chunk string, branch git.BranchType) (*Resource, bool) {
	// Skip empty chunks or chunks with only whitespace
	if strings.TrimSpace(chunk) == "" {
		return nil, false
	}

	// Create a new map to hold the parsed YAML
	var yamlObj map[string]any
	err := yaml.Unmarshal([]byte(chunk), &yamlObj)
	if err != nil {
		log.Debug().Err(err).Msgf("⚠️ Failed to parse YAML in file '%s'", filename)
		return nil, false
	}

	// Skip empty objects
	if len(yamlObj) == 0 {
		return nil, false
	}

	// Convert the map to an unstructured.Unstructured
	yamlData := unstructured.Unstructured{Object: yamlObj}

	// Check if this is a valid Kubernetes resource
	apiVersion, found, _ := unstructured.NestedString(yamlObj, "apiVersion")
	kind, kindFound, _ := unstructured.NestedString(yamlObj, "kind")

	if !found || !kindFound || apiVersion == "" || kind == "" {
		log.Debug().Msgf("⚠️ Skipping invalid Kubernetes resource in file '%s' (missing apiVersion or kind)", filename)
		return nil, false
	}

	return &Resource{
		FileName: filename,
		Yaml:     yamlData,
		Branch:   branch,
	}, true
}
