package extract

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

    "github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
    "github.com/dag-andersen/argocd-diff-preview/pkg/git"
)

func TestParseIgnoreDifferencesFromApp_ApplicationSpec(t *testing.T) {
    // Build an Application with spec.ignoreDifferences
    appObj := map[string]interface{}{
        "apiVersion": "argoproj.io/v1alpha1",
        "kind":       "Application",
        "metadata": map[string]interface{}{
            "name":      "test-controller",
            "namespace": "argocd",
        },
        "spec": map[string]interface{}{
            "ignoreDifferences": []interface{}{
                map[string]interface{}{
                    "group":         "admissionregistration.k8s.io",
                    "kind":          "ValidatingWebhookConfiguration",
                    "name":          "example-webhook-validations",
                    "jsonPointers":  []interface{}{"/webhooks/0/clientConfig/caBundle"},
                },
                map[string]interface{}{
                    "group":         "",
                    "kind":          "Secret",
                    "name":          "example-webhook-ca-keypair",
                    "namespace":     "example-system",
                    "jsonPointers":  []interface{}{"/data"},
                },
            },
        },
    }

    ar := argoapplication.NewArgoResource(&unstructured.Unstructured{Object: appObj}, argoapplication.Application, "test-controller", "test-controller", "app.yaml", git.Target)

    rules := parseIgnoreDifferencesFromApp(*ar)

    require.Len(t, rules, 2)

    // First rule assertions
    r1 := rules[0]
    assert.Equal(t, "admissionregistration.k8s.io", r1.Group)
    assert.Equal(t, "ValidatingWebhookConfiguration", r1.Kind)
    assert.Equal(t, "example-webhook-validations", r1.Name)
    require.Len(t, r1.JSONPointers, 1)
    assert.Equal(t, "/webhooks/0/clientConfig/caBundle", r1.JSONPointers[0])

    // Second rule assertions
    r2 := rules[1]
    assert.Equal(t, "", r2.Group)
    assert.Equal(t, "Secret", r2.Kind)
    assert.Equal(t, "example-webhook-ca-keypair", r2.Name)
    assert.Equal(t, "example-system", r2.Namespace)
    require.Len(t, r2.JSONPointers, 1)
    assert.Equal(t, "/data", r2.JSONPointers[0])
}

func TestApplyIgnoreDifferencesToManifests_MaskingAndDeletion(t *testing.T) {
    // Build rules equivalent to user example
    rules := []ignoreDifferenceRule{
        {
            Group:        "admissionregistration.k8s.io",
            Kind:         "ValidatingWebhookConfiguration",
            Name:         "example-webhook-validations",
            JSONPointers: []string{"/webhooks/0/clientConfig/caBundle"},
        },
        {
            Group:        "",
            Kind:         "Secret",
            Name:         "example-webhook-ca-keypair",
            Namespace:    "example-system",
            JSONPointers: []string{"/data"},
        },
    }

    // Build manifests
    webhook := unstructured.Unstructured{Object: map[string]interface{}{
        "apiVersion": "admissionregistration.k8s.io/v1",
        "kind":       "ValidatingWebhookConfiguration",
        "metadata": map[string]interface{}{
            "name": "example-webhook-validations",
        },
        "webhooks": []interface{}{
            map[string]interface{}{
                "clientConfig": map[string]interface{}{
                    "caBundle": "SOMEBASE64CERT",
                },
            },
        },
    }}

    secret1 := unstructured.Unstructured{Object: map[string]interface{}{
        "apiVersion": "v1",
        "kind":       "Secret",
        "metadata": map[string]interface{}{
            "name":      "example-webhook-ca-keypair",
            "namespace": "example-system",
        },
        "data": map[string]interface{}{
            "tls.crt": "BASE64DATA",
            "tls.key": "BASE64KEY",
        },
    }}

    // Should not match: different name
    secret2 := unstructured.Unstructured{Object: map[string]interface{}{
        "apiVersion": "v1",
        "kind":       "Secret",
        "metadata": map[string]interface{}{
            "name":      "other-secret",
            "namespace": "example-system",
        },
        "data": map[string]interface{}{
            "password": "abc",
        },
    }}

    manifests := []unstructured.Unstructured{webhook, secret1, secret2}

    applyIgnoreDifferencesToManifests(manifests, rules)

    // Assert webhook caBundle removed
    webhooks, foundSlice, err := unstructured.NestedSlice(manifests[0].Object, "webhooks")
    require.NoError(t, err)
    require.True(t, foundSlice)
    require.GreaterOrEqual(t, len(webhooks), 1)
    firstWebhook, ok := webhooks[0].(map[string]interface{})
    require.True(t, ok)
    got, found, err := unstructured.NestedString(firstWebhook, "clientConfig", "caBundle")
    require.NoError(t, err)
    assert.False(t, found)
    assert.Empty(t, got)

    // Assert secret1 data removed
    _, foundMap, err := unstructured.NestedMap(manifests[1].Object, "data")
    require.NoError(t, err)
    assert.False(t, foundMap)

    // Assert secret2 remains intact
    val, found, err := unstructured.NestedString(manifests[2].Object, "data", "password")
    require.NoError(t, err)
    assert.True(t, found)
    assert.Equal(t, "abc", val)
}


