package argocdconfig

import _ "embed"

//go:embed values-override.yaml
var ValuesOverride []byte
