package argoapplicaiton

// ApplicationKind represents the type of Argo CD application
type ApplicationKind int

const (
	Application ApplicationKind = iota
	ApplicationSet
)

// String returns the string representation of ApplicationKind
func (k ApplicationKind) String() string {
	switch k {
	case Application:
		return "Application"
	case ApplicationSet:
		return "ApplicationSet"
	default:
		return "Unknown"
	}
}
