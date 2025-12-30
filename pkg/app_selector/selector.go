package app_selector

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// Operator represents the comparison operator for selectors
type Operator int

const (
	// Eq represents the equality operator
	Eq Operator = iota
	// Ne represents the inequality operator
	Ne
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

// InvalidSelectorError represents an error in selector format
type InvalidSelectorError struct {
	Selector string
	Reason   string
}

func (e *InvalidSelectorError) Error() string {
	return fmt.Sprintf("invalid selector '%s': %s", e.Selector, e.Reason)
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
		// Only use single equals if it's not actually a double equals
		if !strings.Contains(s, "==") {
			selector = &Selector{
				Key:      strings.TrimSpace(equalSingle[0]),
				Value:    strings.TrimSpace(equalSingle[1]),
				Operator: Eq,
			}
		} else {
			log.Error().Msgf("❌ Invalid label selector format: %s", s)
			return nil, &InvalidSelectorError{
				Selector: s,
				Reason:   "invalid format",
			}
		}
	default:
		log.Error().Msgf("❌ Invalid label selector format: %s", s)
		return nil, &InvalidSelectorError{
			Selector: s,
			Reason:   "invalid format",
		}
	}

	// Validate selector
	if selector.Key == "" || selector.Value == "" {
		log.Error().Msgf("❌ Invalid label selector format: empty key or value: %s", s)
		return nil, &InvalidSelectorError{
			Selector: s,
			Reason:   "empty key or value",
		}
	}

	// Check for invalid characters in key
	if strings.Contains(selector.Key, "!") || strings.Contains(selector.Key, "=") {
		log.Error().Msgf("❌ Invalid label selector format: key contains invalid characters: %s", s)
		return nil, &InvalidSelectorError{
			Selector: s,
			Reason:   "key contains invalid characters",
		}
	}

	// Check for invalid characters in value
	if strings.Contains(selector.Value, "!") || strings.Contains(selector.Value, "=") {
		log.Error().Msgf("❌ Invalid label selector format: value contains invalid characters: %s", s)
		return nil, &InvalidSelectorError{
			Selector: s,
			Reason:   "value contains invalid characters",
		}
	}

	return selector, nil
}
