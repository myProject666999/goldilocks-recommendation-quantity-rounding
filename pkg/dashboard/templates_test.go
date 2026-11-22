package dashboard

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/fairwindsops/goldilocks/pkg/summary"
)

// TestContainerTemplate_RendersStatus verifies that the container template
// (which calls getStatus/getStatusRange with a resource name) parses and
// renders, and that a current memory request of 100M compared with a rounded
// target of 96Mi is shown as equal rather than less than.
func TestContainerTemplate_RendersStatus(t *testing.T) {
	tmpl, err := getTemplate(ContainerTemplateName, Options{}, "container")
	require.NoError(t, err)

	quantity := func(s string) resource.Quantity { return resource.MustParse(s) }
	cSummary := summary.ContainerSummary{
		ContainerName: "app",
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    quantity("1500m"),
			corev1.ResourceMemory: quantity("100M"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    quantity("1500m"),
			corev1.ResourceMemory: quantity("100M"),
		},
		LowerBound: corev1.ResourceList{
			corev1.ResourceCPU:    quantity("1"),
			corev1.ResourceMemory: quantity("48Mi"),
		},
		UpperBound: corev1.ResourceList{
			corev1.ResourceCPU:    quantity("1500m"),
			corev1.ResourceMemory: quantity("191Mi"),
		},
		Target: corev1.ResourceList{
			corev1.ResourceCPU:    quantity("1235m"),
			corev1.ResourceMemory: quantity("96Mi"),
		},
	}

	buf := &bytes.Buffer{}
	err = tmpl.ExecuteTemplate(buf, "container", cSummary)
	require.NoError(t, err)

	rendered := buf.String()
	// The current value is displayed verbatim, not rounded to 96Mi.
	assert.Contains(t, rendered, "100M")
	// Its status against the rounded target is "equal" (visually hidden text).
	assert.Contains(t, rendered, `<span class="visually-hidden">equal</span>`)
	assert.NotContains(t, rendered, `<span class="visually-hidden">less than</span>`)
	assert.False(t, strings.Contains(rendered, "<no value>"))
}
