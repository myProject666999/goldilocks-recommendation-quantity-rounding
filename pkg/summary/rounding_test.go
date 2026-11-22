// Copyright 2019 FairwindsOps Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package summary

import (
	"encoding/json"
	"testing"

	controllerUtils "github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	"github.com/fairwindsops/goldilocks/pkg/kube"
)

// roundingVPA builds a VPA whose container recommendation and workload
// resources are deliberately written in decimal units. It exists to exercise
// the boundary between "recommended" values (which Goldilocks rounds) and the
// workload's current configuration (which it must leave alone).
func roundingVPA(name string, status vpav1.VerticalPodAutoscalerStatus) vpav1.VerticalPodAutoscaler {
	return vpav1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "rounding",
		},
		Spec: vpav1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "app",
			},
		},
		Status: status,
	}
}

// roundingWorkload builds the workload a roundingVPA targets. Its current
// requests are 1500m CPU and 100M memory: values that rounding would change.
func roundingWorkload(name string) *controllerUtils.Workload {
	return &controllerUtils.Workload{
		TopController: unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "app",
				"namespace": "rounding",
			},
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "app",
								"resources": map[string]any{
									"requests": map[string]any{
										"cpu":    "1500m",
										"memory": "100M",
									},
									"limits": map[string]any{
										"cpu":    "1500m",
										"memory": "100M",
									},
								},
							},
						},
					},
				},
			},
		}},
	}
}

func roundingSummary(t *testing.T, vpas []vpav1.VerticalPodAutoscaler) Summary {
	t.Helper()

	// NewSummarizer resolves default clients through package-level singletons
	// that fail fast without a kubeconfig; install the same fakes the other
	// tests use. GetSummary never calls them here because both caches are
	// populated below.
	kube.GetMockVPAClient()
	kube.GetMockClient()
	kube.GetMockControllerUtilsClient(kube.GetMockDynamicClient())

	summarizer := NewSummarizerForVPAs(vpas)
	// Supply the workload cache directly so these tests do not need a cluster.
	summarizer.workloadForVPANamed = map[string]*controllerUtils.Workload{}
	for i := range vpas {
		summarizer.workloadForVPANamed[vpas[i].Name] = roundingWorkload(vpas[i].Name)
	}

	got, err := summarizer.GetSummary()
	require.NoError(t, err)
	return got
}

func containerFromSummary(t *testing.T, summary Summary) ContainerSummary {
	t.Helper()

	cs, ok := summary.Namespaces["rounding"].Workloads["app"].Containers["app"]
	require.True(t, ok, "expected container app in summary")
	return cs
}

func quantityString(q *resource.Quantity) string {
	return q.String()
}

// Test_Summarizer_JSONFieldValues documents which fields of the summary JSON
// change and how. The recommendations (lowerBound, upperBound, target,
// uncappedTarget) are rounded; the workload's current configuration (requests,
// limits) is reported verbatim.
func Test_Summarizer_JSONFieldValues(t *testing.T) {
	vpa := roundingVPA("goldilocks-app", vpav1.VerticalPodAutoscalerStatus{
		Recommendation: &vpav1.RecommendedPodResources{
			ContainerRecommendations: []vpav1.RecommendedContainerResources{
				{
					ContainerName: "app",
					Target: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1234567u"),
						corev1.ResourceMemory: resource.MustParse("100M"),
					},
					UpperBound: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1500000u"),
						corev1.ResourceMemory: resource.MustParse("200M"),
					},
					LowerBound: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("10500u"),
						corev1.ResourceMemory: resource.MustParse("10486784"),
					},
					// No UncappedTarget: it must stay absent, not be invented.
				},
			},
		},
	})

	cs := containerFromSummary(t, roundingSummary(t, []vpav1.VerticalPodAutoscaler{vpa}))
	encoded, err := json.Marshal(cs)
	require.NoError(t, err)

	var fields map[string]any
	require.NoError(t, json.Unmarshal(encoded, &fields))

	assert.Equal(t, map[string]any{"cpu": "1235m", "memory": "96Mi"}, fields["target"])
	assert.Equal(t, map[string]any{"cpu": "1500m", "memory": "191Mi"}, fields["upperBound"])
	assert.Equal(t, map[string]any{"cpu": "11m", "memory": "11Mi"}, fields["lowerBound"])
	assert.Nil(t, fields["uncappedTarget"], "an absent recommendation must stay absent")
	assert.Equal(t, map[string]any{"cpu": "1500m", "memory": "100M"}, fields["requests"])
	assert.Equal(t, map[string]any{"cpu": "1500m", "memory": "100M"}, fields["limits"])
}

// Test_Summarizer_CurrentValuesAreNotRounded verifies the first reported
// impact: the workload's current requests/limits are run through the same
// rounding as recommendations. A current memory request of 100M
// (100000000 bytes) is currently displayed as 96Mi (100663296 bytes). Current
// configuration must stay verbatim so the dashboard and `summary` JSON report
// what is actually deployed.
func Test_Summarizer_CurrentValuesAreNotRounded(t *testing.T) {
	vpa := roundingVPA("goldilocks-app", vpav1.VerticalPodAutoscalerStatus{
		Recommendation: &vpav1.RecommendedPodResources{
			ContainerRecommendations: []vpav1.RecommendedContainerResources{
				{
					ContainerName: "app",
					Target: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1234567u"),
						corev1.ResourceMemory: resource.MustParse("100M"),
					},
					UpperBound: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1500000u"),
						corev1.ResourceMemory: resource.MustParse("200M"),
					},
					LowerBound: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1000000u"),
						corev1.ResourceMemory: resource.MustParse("50M"),
					},
				},
			},
		},
	})

	cs := containerFromSummary(t, roundingSummary(t, []vpav1.VerticalPodAutoscaler{vpa}))

	// Current configuration: untouched, decimal units included.
	assert.Equal(t, "1500m", quantityString(cs.Requests.Cpu()))
	assert.Equal(t, "100M", quantityString(cs.Requests.Memory()))
	assert.Equal(t, "1500m", quantityString(cs.Limits.Cpu()))
	assert.Equal(t, "100M", quantityString(cs.Limits.Memory()))

	// Recommendations: rounded up, in binary units, CPU in whole millicores.
	assert.Equal(t, "1235m", quantityString(cs.Target.Cpu()))
	assert.Equal(t, "96Mi", quantityString(cs.Target.Memory()))
	assert.Equal(t, "1", quantityString(cs.LowerBound.Cpu()))
	assert.Equal(t, "48Mi", quantityString(cs.LowerBound.Memory()))
	assert.Equal(t, "1500m", quantityString(cs.UpperBound.Cpu()))
	assert.Equal(t, "191Mi", quantityString(cs.UpperBound.Memory()))
}

// Test_Summarizer_DoesNotMutateCachedVPAs verifies the second reported impact:
// the Summarizer caches the VPA slice it was given, and the recommendation
// resource lists inside Status are Go maps. Rounding them in place therefore
// rewrites the caller's (and the cache's) VPA objects, so repeated summaries
// observe already-rounded data instead of the recommendations the VPA
// actually produced.
func Test_Summarizer_DoesNotMutateCachedVPAs(t *testing.T) {
	vpa := roundingVPA("goldilocks-app", vpav1.VerticalPodAutoscalerStatus{
		Recommendation: &vpav1.RecommendedPodResources{
			ContainerRecommendations: []vpav1.RecommendedContainerResources{
				{
					ContainerName: "app",
					Target: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1500u"),
						corev1.ResourceMemory: resource.MustParse("100M"),
					},
					UpperBound: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1500000u"),
						corev1.ResourceMemory: resource.MustParse("200M"),
					},
					LowerBound: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1000000u"),
						corev1.ResourceMemory: resource.MustParse("50M"),
					},
				},
			},
		},
	})

	vpas := []vpav1.VerticalPodAutoscaler{vpa}
	before, err := json.Marshal(vpas)
	require.NoError(t, err)

	first := roundingSummary(t, vpas)
	second := roundingSummary(t, vpas)

	after, err := json.Marshal(vpas)
	require.NoError(t, err)
	assert.JSONEq(t, string(before), string(after), "GetSummary must not mutate the VPAs it was given")

	firstJSON, err := json.Marshal(first)
	require.NoError(t, err)
	secondJSON, err := json.Marshal(second)
	require.NoError(t, err)
	assert.JSONEq(t, string(firstJSON), string(secondJSON), "repeated summaries must agree")
}
