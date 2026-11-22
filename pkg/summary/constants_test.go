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
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	"github.com/fairwindsops/goldilocks/pkg/utils"
)

// These are all variables for testing. These resources, when applied to a cluster, should generate the testSummary at the end.

var updateMode = vpav1.UpdateModeOff

// lowerBound is used for all lower bounds in VPAs with recommendations. The
// values are deliberately not round numbers so that the summary's rounding is
// observable: 10500u is 10.5 millicores and 10486784 bytes is 10Mi + 1Ki, so
// both round up (to 11m and 11Mi respectively).
var lowerBound = v1.ResourceList{
	v1.ResourceCPU:    resource.MustParse("10500u"),
	v1.ResourceMemory: resource.MustParse("10486784"),
}

// upperBound is used for all upper bounds in VPAs with recommendations.
// 1500000u rounds up to 1500m and 200M (200000000 bytes) rounds up to 191Mi.
var upperBound = v1.ResourceList{
	v1.ResourceCPU:    resource.MustParse("1500000u"),
	v1.ResourceMemory: resource.MustParse("200M"),
}

// targetResources is used for targets on VPAs with recommendations. 1234567u
// rounds up to 1235m and 100M (100000000 bytes) rounds up to 96Mi.
var targetResources = v1.ResourceList{
	v1.ResourceCPU:    resource.MustParse("1234567u"),
	v1.ResourceMemory: resource.MustParse("100M"),
}

// roundedLowerBound, roundedUpperBound and roundedTarget are what the summary
// reports for the recommendations above after rounding.
var roundedLowerBound = v1.ResourceList{
	v1.ResourceCPU:    resource.MustParse("11m"),
	v1.ResourceMemory: resource.MustParse("11Mi"),
}

var roundedUpperBound = v1.ResourceList{
	v1.ResourceCPU:    resource.MustParse("1500m"),
	v1.ResourceMemory: resource.MustParse("191Mi"),
}

var roundedTarget = v1.ResourceList{
	v1.ResourceCPU:    resource.MustParse("1235m"),
	v1.ResourceMemory: resource.MustParse("96Mi"),
}

// currentResources are the requests and limits set on the test workloads. They
// are not round numbers either, but unlike the recommendations they are
// reported verbatim in the summary: current configuration must reflect what is
// actually deployed rather than a rounded approximation of it.
var currentResources = v1.ResourceList{
	v1.ResourceCPU:    resource.MustParse("1500m"),
	v1.ResourceMemory: resource.MustParse("100M"),
}

// Basic VPA and Deployment

var testVPABasic = &vpav1.VerticalPodAutoscaler{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "goldilocks-test-basic",
		Labels:    utils.VPALabels,
		Namespace: "testing",
	},
	Spec: vpav1.VerticalPodAutoscalerSpec{
		TargetRef: &autoscalingv1.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "test-basic",
		},
		UpdatePolicy: &vpav1.PodUpdatePolicy{
			UpdateMode: &updateMode,
		},
	},
	Status: vpav1.VerticalPodAutoscalerStatus{
		Conditions: []vpav1.VerticalPodAutoscalerCondition{
			{
				Message: "idk",
			},
		},
	},
}

var testDeploymentBasicUnstructured = &unstructured.Unstructured{
	Object: map[string]any{
		"kind":       "Deployment",
		"apiVersion": "apps/v1",
		"metadata": map[string]any{
			"name":      "test-basic",
			"namespace": "testing",
		},
		"spec": map[string]any{},
	},
}

var testDeploymentBasicReplicaSetUnstructured = &unstructured.Unstructured{
	Object: map[string]any{
		"kind":       "ReplicaSet",
		"apiVersion": "apps/v1",
		"metadata": map[string]any{
			"ownerReferences": []any{
				map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"controller": true,
					"name":       "test-basic",
				},
			},
			"name":      "test-basic-0123456789",
			"namespace": "testing",
		},
		"spec": map[string]any{},
	},
}

var testDeploymentBasicPodUnstructured = &unstructured.Unstructured{
	Object: map[string]any{
		"kind":       "Pod",
		"apiVersion": "v1",
		"metadata": map[string]any{
			"ownerReferences": []any{
				map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"controller": true,
					"name":       "test-basic-0123456789",
				},
			},
			"name":      "test-basic-0123456789-01234",
			"namespace": "testing",
		},
		"spec": map[string]any{},
	},
}

// Not labelled for use by Goldilocks

var testVPANoLabels = &vpav1.VerticalPodAutoscaler{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-vpa-no-labels",
		Namespace: "testing",
	},
	Spec: vpav1.VerticalPodAutoscalerSpec{
		TargetRef: &autoscalingv1.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "test-vpa-no-labels",
		},
		UpdatePolicy: &vpav1.PodUpdatePolicy{
			UpdateMode: &updateMode,
		},
	},
}

// A complete VPA with recommendations

var testVPAWithReco = &vpav1.VerticalPodAutoscaler{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "goldilocks-test-vpa-with-reco",
		Namespace: "testing",
		Labels:    utils.VPALabels,
	},
	Spec: vpav1.VerticalPodAutoscalerSpec{
		TargetRef: &autoscalingv1.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "test-vpa-with-reco",
		},
		UpdatePolicy: &vpav1.PodUpdatePolicy{
			UpdateMode: &updateMode,
		},
	},
	Status: vpav1.VerticalPodAutoscalerStatus{
		Recommendation: &vpav1.RecommendedPodResources{
			ContainerRecommendations: []vpav1.RecommendedContainerResources{
				{
					ContainerName: "container",
					Target:        targetResources,
					UpperBound:    upperBound,
					LowerBound:    lowerBound,
				},
			},
		},
	},
}

var testDeploymentWithRecoUnstructured = &unstructured.Unstructured{
	Object: map[string]any{
		"kind":       "Deployment",
		"apiVersion": "apps/v1",
		"metadata": map[string]any{
			"name":      "test-vpa-with-reco",
			"namespace": "testing",
		},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": "container",
							"resources": map[string]any{
								"limits": map[string]any{
									"cpu":    "1500m",
									"memory": "100M",
								},
								"requests": map[string]any{
									"cpu":    "1500m",
									"memory": "100M",
								},
							},
						},
					},
				},
			},
		},
	},
}

var testDeploymentWithRecoReplicaSetUnstructured = &unstructured.Unstructured{
	Object: map[string]any{
		"kind":       "ReplicaSet",
		"apiVersion": "apps/v1",
		"metadata": map[string]any{
			"ownerReferences": []any{
				map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"controller": true,
					"name":       "test-vpa-with-reco",
				},
			},
			"name":      "test-vpa-with-reco-0123456789",
			"namespace": "testing",
		},
		"spec": map[string]any{},
	},
}

var testDeploymentWithRecoPodUnstructured = &unstructured.Unstructured{
	Object: map[string]any{
		"kind":       "Pod",
		"apiVersion": "v1",
		"metadata": map[string]any{
			"ownerReferences": []any{
				map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"controller": true,
					"name":       "test-vpa-with-reco-0123456789",
				},
			},
			"name":      "test-vpa-with-reco-0123456789-01234",
			"namespace": "testing",
		},
		"spec": map[string]any{},
	},
}

// The summary of these objects

var testSummary = Summary{
	Namespaces: map[string]namespaceSummary{
		"testing": {
			Namespace:       "testing",
			IsOnlyNamespace: true,
			Workloads: map[string]workloadSummary{
				"test-basic": {
					ControllerName: "test-basic",
					ControllerType: "Deployment",
					Containers:     map[string]ContainerSummary{},
				},
				"test-vpa-with-reco": {
					ControllerName: "test-vpa-with-reco",
					ControllerType: "Deployment",
					Containers: map[string]ContainerSummary{
						"container": {
							ContainerName: "container",
							LowerBound:    roundedLowerBound,
							UpperBound:    roundedUpperBound,
							Target:        roundedTarget,
							Limits:        currentResources,
							Requests:      currentResources,
						},
					},
				},
			},
		},
	},
}

// DaemonSet test and VPA

var testDaemonSettWithRecoUnstructured = &unstructured.Unstructured{
	Object: map[string]any{
		"kind":       "DaemonSet",
		"apiVersion": "apps/v1",
		"metadata": map[string]any{
			"name":      "test-ds-with-reco",
			"namespace": "testing-daemonset",
		},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": "container",
							"resources": map[string]any{
								"limits": map[string]any{
									"cpu":    "1500m",
									"memory": "100M",
								},
								"requests": map[string]any{
									"cpu":    "1500m",
									"memory": "100M",
								},
							},
						},
					},
				},
			},
		},
	},
}

var testDaemonSetWithRecoPodUnstructured = &unstructured.Unstructured{
	Object: map[string]any{
		"kind":       "Pod",
		"apiVersion": "v1",
		"metadata": map[string]any{
			"ownerReferences": []any{
				map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "DaemonSet",
					"controller": true,
					"name":       "test-ds-with-reco",
				},
			},
			"name":      "test-ds-with-reco-01234",
			"namespace": "testing-daemonset",
		},
		"spec": map[string]any{},
	},
}

var testDaemonSetVPAWithReco = &vpav1.VerticalPodAutoscaler{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "goldilocks-test-ds-with-reco",
		Namespace: "testing-daemonset",
		Labels:    utils.VPALabels,
	},
	Spec: vpav1.VerticalPodAutoscalerSpec{
		TargetRef: &autoscalingv1.CrossVersionObjectReference{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
			Name:       "test-ds-with-reco",
		},
		UpdatePolicy: &vpav1.PodUpdatePolicy{
			UpdateMode: &updateMode,
		},
	},
	Status: vpav1.VerticalPodAutoscalerStatus{
		Recommendation: &vpav1.RecommendedPodResources{
			ContainerRecommendations: []vpav1.RecommendedContainerResources{
				{
					ContainerName: "container",
					Target:        targetResources,
					UpperBound:    upperBound,
					LowerBound:    lowerBound,
				},
			},
		},
	},
}

// The summary of the daemonset

var testSummaryDaemonSet = Summary{
	Namespaces: map[string]namespaceSummary{
		"testing-daemonset": {
			Namespace:       "testing-daemonset",
			IsOnlyNamespace: true,
			Workloads: map[string]workloadSummary{
				"test-ds-with-reco": {
					ControllerName: "test-ds-with-reco",
					ControllerType: "DaemonSet",
					Containers: map[string]ContainerSummary{
						"container": {
							ContainerName: "container",
							LowerBound:    roundedLowerBound,
							UpperBound:    roundedUpperBound,
							Target:        roundedTarget,
							Limits:        currentResources,
							Requests:      currentResources,
						},
					},
				},
			},
		},
	},
}

// testVPANoTargetRef is a VPA whose spec omits targetRef, which the VPA CRD allows
var testVPANoTargetRef = &vpav1.VerticalPodAutoscaler{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test-vpa-no-target-ref",
		Labels:    utils.VPALabels,
		Namespace: "testing",
	},
	Spec: vpav1.VerticalPodAutoscalerSpec{
		UpdatePolicy: &vpav1.PodUpdatePolicy{
			UpdateMode: &updateMode,
		},
	},
}
