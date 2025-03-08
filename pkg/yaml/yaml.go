package yaml

import (
	"strconv"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// GetYamlValue gets a value from a YAML node by path
func GetYamlValue(node *yaml.Node, path []string) *yaml.Node {
	if node == nil || len(path) == 0 {
		return node
	}

	if node.Kind == yaml.DocumentNode {
		if len(node.Content) > 0 {
			return GetYamlValue(node.Content[0], path)
		}
		return nil
	}

	switch node.Kind {
	case yaml.MappingNode:
		// Handle dictionary/mapping nodes
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == path[0] {
				if len(path) == 1 {
					return node.Content[i+1]
				}
				return GetYamlValue(node.Content[i+1], path[1:])
			}
		}
	case yaml.SequenceNode:
		// If we're looking for a key in a sequence, search through all elements
		if _, err := strconv.Atoi(path[0]); err != nil {
			// Not a numeric index, search through all sequence elements
			for _, item := range node.Content {
				if result := GetYamlValue(item, path); result != nil {
					return result
				}
			}
		} else {
			// Handle array/sequence nodes with numeric index
			if idx, err := strconv.Atoi(path[0]); err == nil && idx >= 0 && idx < len(node.Content) {
				if len(path) == 1 {
					return node.Content[idx]
				}
				return GetYamlValue(node.Content[idx], path[1:])
			}
		}
	}

	return nil
}

// SetYamlValue sets a value in a YAML node by path
func SetYamlValue(node *yaml.Node, path []string, value string) {
	if node == nil || len(path) == 0 {
		log.Debug().Msg("Can't set value because node is nil or path is empty")
		return
	}

	if node.Kind == yaml.DocumentNode {
		if len(node.Content) > 0 {
			SetYamlValue(node.Content[0], path, value)
		}
		return
	}

	if node.Kind != yaml.MappingNode {
		log.Debug().Msg("Can't set value because node is not a mapping node")
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == path[0] {
			if len(path) == 1 {
				// Create new node if it doesn't exist
				if node.Content[i+1] == nil {
					node.Content[i+1] = &yaml.Node{
						Kind:  yaml.ScalarNode,
						Value: value,
					}
				} else {
					// Update existing node
					node.Content[i+1].Kind = yaml.ScalarNode
					node.Content[i+1].Value = value
					node.Content[i+1].Tag = "!!str"
				}
				return
			}
			SetYamlValue(node.Content[i+1], path[1:], value)
			return
		}
	}

	// Key not found, create new key-value pair
	if len(path) == 1 {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: path[0]},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value},
		)
	} else {
		newMap := &yaml.Node{Kind: yaml.MappingNode}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: path[0]},
			newMap,
		)
		SetYamlValue(newMap, path[1:], value)
	}
}

// RemoveYamlValue removes a value from a YAML node by path
func RemoveYamlValue(node *yaml.Node, path []string) {
	if node == nil || len(path) == 0 {
		return
	}

	if node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == path[0] {
			if len(path) == 1 {
				// Only remove if we found the exact path
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				return
			}
			// Continue searching deeper
			RemoveYamlValue(node.Content[i+1], path[1:])
			return
		}
	}
}

// KeyExists checks if a key exists in a YAML node
func KeyExists(node *yaml.Node, path []string) bool {
	if node == nil || len(path) == 0 {
		return false
	}

	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == path[0] {
				if len(path) == 1 {
					return true
				}
				return KeyExists(node.Content[i+1], path[1:])
			}
		}
	}

	return false
}

// YamlToString converts a YAML node to a string
func YamlToString(input *yaml.Node) string {
	bytes, err := yaml.Marshal(input)
	if err != nil {
		return ""
	}
	return string(bytes)
}

// YamlEqual checks if two YAML nodes are equal
func YamlEqual(a, b *yaml.Node) bool {
	aStr, err := yaml.Marshal(a)
	if err != nil {
		return false
	}
	bStr, err := yaml.Marshal(b)
	if err != nil {
		return false
	}
	return string(aStr) == string(bStr)
}

// DeepCopyYaml creates a deep copy of YAML
func DeepCopyYaml(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}

	bytes, err := yaml.Marshal(node)
	if err != nil {
		return nil
	}

	var newNode yaml.Node
	if err := yaml.Unmarshal(bytes, &newNode); err != nil {
		return nil
	}

	return &newNode
}
