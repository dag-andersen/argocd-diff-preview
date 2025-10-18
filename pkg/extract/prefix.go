package extract

import (
	"crypto/sha256"
	"fmt"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/rs/zerolog/log"
)

// addApplicationPrefix prefixes the application name with the branch name and a unique ID
func addApplicationPrefix(a *argoapplication.ArgoResource, prefix string) error {
	if a.Branch == "" {
		log.Warn().Str(a.Kind.ShortName(), a.GetLongName()).Msg("⚠️ Can't prefix application name with prefix because branch is empty")
		return nil
	}

	var branchShortName string
	switch a.Branch {
	case git.Base:
		branchShortName = "b"
	case git.Target:
		branchShortName = "t"
	}

	maxKubernetesNameLength := 53

	prefixSize := len(prefix) + len(branchShortName) + len("--")
	var newId string
	if prefixSize+len(a.Id) > maxKubernetesNameLength {
		// hash id so it becomes shorter
		hashedId := fmt.Sprintf("%x", sha256.Sum256([]byte(a.Id)))
		hashPart := hashedId[:53-prefixSize]
		log.Debug().Msgf("Application name too long. Renamed '%s' to '%s'", a.Id, hashPart)
		newId = fmt.Sprintf("%s-%s-%s", prefix, branchShortName, hashPart)
	} else {
		newId = fmt.Sprintf("%s-%s-%s", prefix, branchShortName, a.Id)
	}

	a.Id = newId
	a.Yaml.SetName(newId)

	return nil
}

// removeApplicationPrefix removes the prefix from the application name
// returns the old id and an error
func removeApplicationPrefix(a *argoapplication.ArgoResource, prefix string) (string, error) {
	// remove the branch short name, and two dashes.
	oldId := a.Id
	newId := a.Id[len(prefix)+len("-x-"):]
	a.Id = newId
	a.Yaml.SetName(newId)
	return oldId, nil
}
