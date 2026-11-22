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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"

	"github.com/fairwindsops/goldilocks/pkg/utils"
)

// Test_Summarizer_RoundingRelationships verifies that rounding the
// recommendations keeps the relationships the VPA guarantees: lowerBound <=
// target <= upperBound, and that the uncappedTarget stays on the correct side
// of target when a minAllowed/maxAllowed resource policy constrains target. It
// also checks that a resource list containing only one resource is rounded
// without inventing the other.
func Test_Summarizer_RoundingRelationships(t *testing.T) {
	cpu := func(s string) corev1.ResourceList {
		return corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(s)}
	}

	vpa := vpav1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{Name: "goldilocks-app", Namespace: "rounding"},
		Spec: vpav1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "app",
			},
		},
		Status: vpav1.VerticalPodAutoscalerStatus{
			Recommendation: &vpav1.RecommendedPodResources{
				ContainerRecommendations: []vpav1.RecommendedContainerResources{
					{
						ContainerName: "app",
						LowerBound:    cpu("1000000u"), // 1 core
						Target:        cpu("1234567u"), // 1235m after rounding
						UpperBound:    cpu("1500000u"), // 1500m after rounding
						// An uncapped recommendation below a minAllowed-constrained
						// target; rounding must not cross them over.
						UncappedTarget: cpu("1100000u"), // 1100m
						// No memory entries at all.
					},
				},
			},
		},
	}

	cs := containerFromSummary(t, roundingSummary(t, []vpav1.VerticalPodAutoscaler{vpa}))

	assert.Equal(t, "1", cs.LowerBound.Cpu().String())
	assert.Equal(t, "1235m", cs.Target.Cpu().String())
	assert.Equal(t, "1500m", cs.UpperBound.Cpu().String())
	assert.Equal(t, "1100m", cs.UncappedTarget.Cpu().String())

	// lowerBound <= target <= upperBound still holds.
	assert.LessOrEqual(t, cs.LowerBound.Cpu().Cmp(*cs.Target.Cpu()), 0)
	assert.LessOrEqual(t, cs.Target.Cpu().Cmp(*cs.UpperBound.Cpu()), 0)

	// The uncappedTarget remains below the constrained target.
	assert.LessOrEqual(t, cs.UncappedTarget.Cpu().Cmp(*cs.Target.Cpu()), 0)

	// Only CPU was present, so memory must not have been added to any list.
	for name, list := range map[string]corev1.ResourceList{
		"lowerBound":     cs.LowerBound,
		"target":         cs.Target,
		"upperBound":     cs.UpperBound,
		"uncappedTarget": cs.UncappedTarget,
	} {
		_, hasMemory := list[corev1.ResourceMemory]
		assert.Falsef(t, hasMemory, "%s must not gain a memory entry", name)
	}
}

// Test_NewContainerSummary_EmptyRecommendationStaysEmpty verifies that an
// entirely absent recommendation list is reported as nil rather than as an
// empty non-nil map.
func Test_NewContainerSummary_EmptyRecommendationStaysEmpty(t *testing.T) {
	cs, err := newContainerSummary(vpav1.RecommendedContainerResources{ContainerName: "app"}, corev1.Container{Name: "app"})
	require.NoError(t, err)
	assert.Nil(t, cs.LowerBound)
	assert.Nil(t, cs.Target)
	assert.Nil(t, cs.UpperBound)
	assert.Nil(t, cs.UncappedTarget)
}

// Test_NewContainerSummary_NegativeRecommendation verifies a negative
// recommendation is surfaced as an error rather than silently clamped.
func Test_NewContainerSummary_NegativeRecommendation(t *testing.T) {
	_, err := newContainerSummary(vpav1.RecommendedContainerResources{
		ContainerName: "app",
		Target: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("-1m"),
		},
	}, corev1.Container{Name: "app"})
	require.Error(t, err)
	assert.ErrorIs(t, err, utils.ErrNegativeQuantity)
}
