package types

import (
	"fmt"
	"log"
	"strings"
)

// Operator represents the comparison operator for selectors
type Operator int

const (
	Eq Operator = iota // Equal
	Ne                 // Not Equal
)

// String returns the string representation of the Operator
func (o Operator) String() string {
	switch o {
	case Eq:
		return "="
	case Ne:
		return "!="
	default:
		return "unknown"
	}
}

// Selector represents a key-value selector with an operator
type Selector struct {
	Key      string
	Value    string
	Operator Operator
}

// String returns the string representation of the Selector
func (s *Selector) String() string {
	return fmt.Sprintf("%s%s%s", s.Key, s.Operator, s.Value)
}

// FromString creates a new Selector from a string representation
func FromString(s string) (*Selector, error) {
	notEqual := strings.Split(s, "!=")
	equalDouble := strings.Split(s, "==")
	equalSingle := strings.Split(s, "=")

	var selector *Selector

	switch {
	case len(notEqual) == 2:
		selector = &Selector{
			Key:      strings.TrimSpace(notEqual[0]),
			Value:    strings.TrimSpace(notEqual[1]),
			Operator: Ne,
		}
	case len(equalDouble) == 2:
		selector = &Selector{
			Key:      strings.TrimSpace(equalDouble[0]),
			Value:    strings.TrimSpace(equalDouble[1]),
			Operator: Eq,
		}
	case len(equalSingle) == 2:
		selector = &Selector{
			Key:      strings.TrimSpace(equalSingle[0]),
			Value:    strings.TrimSpace(equalSingle[1]),
			Operator: Eq,
		}
	default:
		log.Printf("❌ Invalid label selector format: %s", s)
		return nil, fmt.Errorf("invalid label selector format")
	}

	// Validate selector
	if selector.Key == "" ||
		strings.Contains(selector.Key, "!") ||
		strings.Contains(selector.Key, "=") ||
		selector.Value == "" ||
		strings.Contains(selector.Value, "!") ||
		strings.Contains(selector.Value, "=") {
		log.Printf("❌ Invalid label selector format: %s", s)
		return nil, fmt.Errorf("invalid label selector format")
	}

	return selector, nil
}
