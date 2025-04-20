package argoapplicaiton

// ApplicationKind represents the type of Argo CD application
type ApplicationKind int

const (
	Application ApplicationKind = iota
	ApplicationSet
)

// ShortName returns the string representation of ApplicationKind
func (k ApplicationKind) ShortName() string {
	switch k {
	case Application:
		return "App"
	case ApplicationSet:
		return "AppSet"
	default:
		return "Unknown"
	}
}
