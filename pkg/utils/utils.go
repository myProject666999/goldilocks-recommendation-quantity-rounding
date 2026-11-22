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

package utils

import (
	"errors"
	"fmt"
	"math/big"

	inf "gopkg.in/inf.v0"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	// LabelBase is the string that will be used for labels on namespaces
	LabelOrAnnotationBase = "goldilocks.fairwinds.com"
	// VpaEnabledLabel is the label used to indicate that Goldilocks is enabled.
	VpaEnabledLabel = LabelOrAnnotationBase + "/" + "enabled"
	// VpaUpdateModeKey is the label used to indicate the vpa update mode.
	VpaUpdateModeKey = LabelOrAnnotationBase + "/" + "vpa-update-mode"
	// VpaMinReplicas is the annotation to use to define minimum replicas for eviction of a VPA
	VpaMinReplicasAnnotation = LabelOrAnnotationBase + "/" + "vpa-min-replicas"
	// DeploymentExcludeContainersAnnotation is the label used to exclude container names from being reported.
	WorkloadExcludeContainersAnnotation = LabelOrAnnotationBase + "/" + "exclude-containers"
	// VpaResourcePolicyAnnotation is the annotation use to define the json configuration of PodResourcePolicy section of a vpa
	VpaResourcePolicyAnnotation = LabelOrAnnotationBase + "/" + "vpa-resource-policy"
)

// VPALabels is a set of default labels that get placed on every VPA.
var VPALabels = map[string]string{
	"creator": "Fairwinds",
	"source":  "goldilocks",
}

// An Event represents an update of a Kubernetes object and contains metadata about the update.
type Event struct {
	Key          string // A key identifying the object.  This is in the format <object-type>/<object-name>
	EventType    string // The type of event - update, delete, or create
	Namespace    string // The namespace of the event's object
	ResourceType string // The type of resource that was updated.
}

// UniqueString returns a unique string from a slice.
func UniqueString(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// Difference returns the difference betwee two string slices.
func Difference(a, b []string) (diff []string) {
	m := make(map[string]bool)

	for _, item := range b {
		m[item] = true
	}

	for _, item := range a {
		if _, ok := m[item]; !ok {
			diff = append(diff, item)
		}
	}
	return
}

// ErrNegativeQuantity is returned when a resource quantity Goldilocks would
// round is negative. Negative requests and limits are invalid in Kubernetes,
// so they are reported instead of being silently corrected.
var ErrNegativeQuantity = errors.New("resource quantity must not be negative")

const (
	// memorySmallUnit is the rounding step for memory recommendations below
	// one MiB: values are rounded up to a whole Ki (1024 bytes).
	memorySmallUnit = int64(1024)
	// memoryLargeUnit is the rounding step for memory recommendations of one
	// MiB or more: values are rounded up to a whole Mi (1024 * 1024 bytes).
	memoryLargeUnit = int64(1024 * 1024)
)

// FormatResourceList returns a copy of rl with the CPU and memory entries
// rounded up for display. The value passed in is never modified; callers that
// need rounded values must use the returned list (the workload's actual
// requests and limits are reported verbatim, not through this function).
//
// Rounding never lowers a value and is applied independently per resource:
//
//   - CPU is rounded up to a whole millicore. A value that is already an exact
//     whole number of cores is emitted as an integer ("2"), everything else in
//     millicores ("1235m").
//   - Memory is emitted in BinarySI (Ki, Mi, Gi). Values below one MiB are
//     rounded up to a whole Ki, values of one MiB or more up to a whole Mi.
//
// Missing resources are not added and zero values are left untouched. A
// negative CPU or memory value returns ErrNegativeQuantity. Other resource
// names are copied through unchanged.
//
// The rounding is monotonic (x <= y implies round(x) <= round(y)) and
// idempotent (rounding an already rounded value changes nothing), so ordering
// relationships such as lowerBound <= target <= upperBound survive it.
func FormatResourceList(rl v1.ResourceList) (v1.ResourceList, error) {
	if rl == nil {
		return nil, nil
	}

	formatted := make(v1.ResourceList, len(rl))
	for name, quantity := range rl {
		formatted[name] = quantity
	}

	for _, name := range []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory} {
		quantity, exists := rl[name]
		if !exists {
			continue
		}
		rounded, err := FormatResource(name, quantity)
		if err != nil {
			return nil, err
		}
		formatted[name] = rounded
	}

	return formatted, nil
}

// FormatResource rounds a single resource quantity using the same rules as
// FormatResourceList. Resource names other than CPU and memory are returned
// unchanged.
func FormatResource(name v1.ResourceName, quantity resource.Quantity) (resource.Quantity, error) {
	switch name {
	case v1.ResourceCPU, v1.ResourceMemory:
		if quantity.Sign() < 0 {
			return resource.Quantity{}, fmt.Errorf("%w: %s=%s", ErrNegativeQuantity, name, quantity.String())
		}
		// Zero is left exactly as it was provided.
		if quantity.IsZero() {
			return quantity, nil
		}
		if name == v1.ResourceCPU {
			return roundCPU(quantity), nil
		}
		return roundMemory(quantity), nil
	default:
		return quantity, nil
	}
}

// roundCPU rounds a CPU quantity up to a whole millicore. The result is
// represented in DecimalSI at milli scale, so Quantity's canonical
// serialization emits whole cores as integers ("2") and everything else in
// millicores ("1235m", "2m", "250m"). All arithmetic goes through inf.Dec /
// big.Int, so values whose millicore count exceeds math.MaxInt64 cannot
// overflow.
func roundCPU(quantity resource.Quantity) resource.Quantity {
	// milli is the quantity in millicores; ceilMilli is its ceiling.
	milli := new(inf.Dec).Mul(quantity.AsDec(), inf.NewDec(1000, 0))
	ceilMilli := new(inf.Dec).QuoRound(milli, inf.NewDec(1, 0), 0, inf.RoundCeil)

	// ceilMilli is an integer count of millicores. Representing it at scale 3
	// keeps the value exact while letting Quantity's canonical serialization
	// collapse whole cores to an integer ("2") and leave the rest in
	// millicores ("1235m", "2m").
	wholeMilli := new(big.Int).Set(ceilMilli.UnscaledBig())
	exact := inf.NewDecBig(wholeMilli, 3)

	return canonicalQuantity(exact, resource.DecimalSI)
}

// roundMemory rounds a memory quantity up to a whole Ki below one MiB and to a
// whole Mi at or above one MiB, represented in BinarySI.
func roundMemory(quantity resource.Quantity) resource.Quantity {
	step := memoryLargeUnit
	if quantity.Cmp(*resource.NewQuantity(memoryLargeUnit, resource.BinarySI)) < 0 {
		step = memorySmallUnit
	}

	// units is ceil(bytes / step); rounded is the result back in bytes.
	stepDec := inf.NewDec(step, 0)
	units := new(inf.Dec).QuoRound(quantity.AsDec(), stepDec, 0, inf.RoundCeil)
	rounded := new(inf.Dec).Mul(units, stepDec)

	return canonicalQuantity(rounded, resource.BinarySI)
}

// canonicalQuantity builds a Quantity for an exact value. It round-trips the
// value through its canonical serialization so the result carries the same
// internal representation (and therefore the same JSON and String output) as
// resource.MustParse of that text, without touching any private field. If the
// text cannot be reproduced exactly -- for example because Quantity caps
// BinarySI values above math.MaxInt64 when parsing -- the exact decimal value
// is used instead and still serializes canonically.
func canonicalQuantity(exact *inf.Dec, format resource.Format) resource.Quantity {
	preview := resource.NewDecimalQuantity(*exact, format)
	text := preview.String()

	if parsed, err := resource.ParseQuantity(text); err == nil && parsed.Format == format {
		// Compare on a copy: Quantity.Cmp may promote its receiver to the
		// inf.Dec representation, and the parsed value must keep the
		// int64 fast-path form resource.MustParse would produce.
		probe := parsed.DeepCopy()
		if probe.Cmp(*preview) == 0 {
			return parsed
		}
	}

	return *preview
}
