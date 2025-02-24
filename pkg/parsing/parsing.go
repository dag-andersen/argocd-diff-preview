package parsing

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
	"gopkg.in/yaml.v3"
)

// GetYamlFiles returns all yaml files in the given directory that match the regex
func GetYamlFiles(directory string, regex *string) []string {
	log.Printf("🤖 Fetching all files in dir: %s", directory)

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
		if regex != nil {
			matched, err := regexp.MatchString(*regex, relPath)
			if err != nil || !matched {
				return nil
			}
		}

		yamlFiles = append(yamlFiles, relPath)
		return nil
	})

	if err != nil {
		log.Printf("⚠️ Error walking directory: %v", err)
		return []string{}
	}

	if regex != nil {
		log.Printf("🤖 Found %d yaml files in dir '%s' matching regex: %s",
			len(yamlFiles), directory, *regex)
	} else {
		log.Printf("🤖 Found %d yaml files in dir '%s'",
			len(yamlFiles), directory)
	}

	return yamlFiles
}

// ParseYaml parses yaml files and returns a slice of K8sResource
func ParseYaml(directory string, files []string) []types.K8sResource {
	var resources []types.K8sResource

	for _, file := range files {
		log.Printf("In dir '%s' found yaml file: %s", directory, file)

		// Open and read file
		f, err := os.Open(filepath.Join(directory, file))
		if err != nil {
			log.Printf("⚠️ Failed to open file '%s': %v", file, err)
			continue
		}
		defer f.Close()

		// Read file line by line and split on "---"
		var currentChunk strings.Builder
		scanner := bufio.NewScanner(f)

		for scanner.Scan() {
			line := scanner.Text()

			if line == "---" {
				// Process the current chunk if it's not empty
				if currentChunk.Len() > 0 {
					processYamlChunk(file, currentChunk.String(), &resources)
				}
				currentChunk.Reset()
			} else {
				currentChunk.WriteString(line)
				currentChunk.WriteString("\n")
			}
		}

		// Process the last chunk
		if currentChunk.Len() > 0 {
			processYamlChunk(file, currentChunk.String(), &resources)
		}
	}

	return resources
}

func processYamlChunk(filename, chunk string, resources *[]types.K8sResource) {
	var yamlData yaml.Node
	err := yaml.Unmarshal([]byte(chunk), &yamlData)
	if err != nil {
		log.Printf("⚠️ Failed to parse YAML in file '%s': %v", filename, err)
		return
	}

	*resources = append(*resources, types.K8sResource{
		FileName: filename,
		Yaml:     yamlData,
	})
}
