package extract

import (
	"regexp"
	"strings"
)

// ErrorKind represents different types of errors we look for
type ErrorKind string

// for errors from Argo CD
const (
	ErrorHelmTemplate         ErrorKind = "helm template ."
	ErrorAuthRequired         ErrorKind = "authentication required"
	ErrorAuthFailed           ErrorKind = "authentication failed"
	ErrorOCIRegistry          ErrorKind = "error logging into OCI registry"
	ErrorPathNotExist         ErrorKind = "path does not exist"
	ErrorYAMLToJSON           ErrorKind = "error converting YAML to JSON"
	ErrorHelmTemplateDesc     ErrorKind = "Unknown desc = `helm template ."
	ErrorKustomizeBuild       ErrorKind = "Unknown desc = `kustomize build"
	ErrorUnableToResolve      ErrorKind = "Unknown desc = Unable to resolve"
	ErrorInvalidChartRepo     ErrorKind = "is not a valid chart repository or cannot be reached"
	ErrorRepoNotFound         ErrorKind = "Unknown desc = repository not found"
	ErrorCommitSHA            ErrorKind = "to a commit SHA"
	ErrorFetchChart           ErrorKind = "error fetching chart: failed to fetch chart: failed to get command args to log"
	ErrorClusterVersionFailed ErrorKind = "ComparisonError: Failed to load target state: failed to get cluster version for cluster"
)

var errorMessages = []string{
	string(ErrorHelmTemplate),
	string(ErrorAuthRequired),
	string(ErrorAuthFailed),
	string(ErrorOCIRegistry),
	string(ErrorPathNotExist),
	string(ErrorYAMLToJSON),
	string(ErrorHelmTemplateDesc),
	string(ErrorKustomizeBuild),
	string(ErrorUnableToResolve),
	string(ErrorInvalidChartRepo),
	string(ErrorRepoNotFound),
	string(ErrorCommitSHA),
	string(ErrorFetchChart),
	string(ErrorClusterVersionFailed),
}

var helpMessages = map[ErrorKind]string{
	ErrorClusterVersionFailed: "This error usually happens if your cluster is configured with 'createClusterRoles: false' and '--use-argocd-api=true' is not set",
}

// GetHelpMessage returns a helpful message for a given error if one exists
func GetHelpMessage(err error) string {
	for errorKind, helpMessage := range helpMessages {
		if strings.Contains(err.Error(), string(errorKind)) {
			return helpMessage
		}
	}
	return ""
}

// custom Error error messages
const (
	errorApplicationNotFound ErrorKind = "application does not exist"
)

// Timeout errors
var timeoutMessages = []string{
	"Client.Timeout",
	"failed to get git client for repo",
	"rpc error: code = Unknown desc = Get \"https",
	"i/o timeout",
	"Could not resolve host: github.com",
	":8081: connect: connection refused",
	"Temporary failure in name resolution",
	"=git-upload-pack",
	"DeadlineExceeded",
	string(errorApplicationNotFound),
}

// Expected errors when running with 'createClusterRoles: false'
var expectedErrorPatterns = []string{
	`.*Failed to load live state: failed to get cluster info for .*?: error synchronizing cache state : failed to sync cluster .*?: failed to load initial state of resource.*`,
	// `.*Failed to load live state: namespace ".*" for .* ".*" is not managed`,
}
var compiledExpectedErrors []*regexp.Regexp

func init() {
	compiledExpectedErrors = make([]*regexp.Regexp, len(expectedErrorPatterns))
	for i, pattern := range expectedErrorPatterns {
		compiledExpectedErrors[i] = regexp.MustCompile(pattern)
	}
}

func isExpectedError(errorMessage string) bool {
	for _, regex := range compiledExpectedErrors {
		if regex.MatchString(errorMessage) {
			return true
		}
	}
	return false
}
