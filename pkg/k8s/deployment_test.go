package k8s

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeWorkload(name, resource string, desired, ready, available int64, availableCondition bool) *unstructured.Unstructured {
	workload := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": name},
		"spec":     map[string]any{"replicas": desired},
		"status":   map[string]any{"readyReplicas": ready},
	}}
	if resource == "deployments" {
		workload.Object["status"].(map[string]any)["availableReplicas"] = available
		conditionStatus := "False"
		if availableCondition {
			conditionStatus = "True"
		}
		workload.Object["status"].(map[string]any)["conditions"] = []any{
			map[string]any{"type": "Available", "status": conditionStatus},
		}
	}
	return workload
}

func TestIsWorkloadReady(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		workload *unstructured.Unstructured
		want     bool
	}{
		{
			name:     "ready deployment",
			resource: "deployments",
			workload: makeWorkload("server", "deployments", 1, 1, 1, true),
			want:     true,
		},
		{
			name:     "deployment without available condition",
			resource: "deployments",
			workload: makeWorkload("server", "deployments", 1, 1, 1, false),
			want:     false,
		},
		{
			name:     "ready statefulset",
			resource: "statefulsets",
			workload: makeWorkload("controller", "statefulsets", 1, 1, 0, false),
			want:     true,
		},
		{
			name:     "statefulset with unready replica",
			resource: "statefulsets",
			workload: makeWorkload("controller", "statefulsets", 1, 0, 0, false),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isWorkloadReady(tt.workload, tt.resource); got != tt.want {
				t.Errorf("isWorkloadReady() = %v, want %v", got, tt.want)
			}
		})
	}
}
