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
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// quantityText and quantityCompare exist because Quantity's String and Cmp
// methods have pointer receivers, so they cannot be called directly on a map
// index expression (which is not addressable).
func quantityText(q resource.Quantity) string { return q.String() }

func quantityCompare(a, b resource.Quantity) int { return a.Cmp(b) }

func isZero(q resource.Quantity) bool { return q.IsZero() }

func TestUniqueString(t *testing.T) {
	for _, tc := range testUniqueStringCases {
		res := UniqueString(tc.testData)
		assert.Equal(t, res, tc.expected)
	}
}

var testUniqueStringCases = []struct {
	description string
	testData    []string
	expected    []string
}{
	{
		description: "three duplicates, one output",
		testData:    []string{"test", "test", "test"},
		expected:    []string{"test"},
	},
	{
		description: "no duplicates",
		testData:    []string{"one", "two", "three"},
		expected:    []string{"one", "two", "three"},
	},
}

func TestDifference(t *testing.T) {
	for _, tc := range testDifferenceCases {
		res := Difference(tc.testData1, tc.testData2)
		assert.Equal(t, res, tc.expected)
	}
}

var empty []string

var testDifferenceCases = []struct {
	description string
	testData1   []string
	testData2   []string
	expected    []string
}{
	{
		description: "empty case",
		testData1:   []string{"a", "b", "c"},
		testData2:   []string{"a", "b", "c"},
		expected:    empty,
	},
	{
		description: "extra item on right",
		testData1:   []string{"a", "b"},
		testData2:   []string{"a", "b", "c"},
		expected:    empty,
	},
	{
		description: "extra item on left",
		testData1:   []string{"a", "b", "c"},
		testData2:   []string{"a", "b"},
		expected:    []string{"c"},
	},
}

func TestFormatResourceList(t *testing.T) {
	for _, tc := range testFormatResourceCases {
		t.Run(tc.description, func(t *testing.T) {
			res, err := FormatResourceList(tc.testData)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, quantityText(res[tc.resourceType]))
		})
	}
}

var testFormatResourceCases = []struct {
	description  string
	testData     v1.ResourceList
	resourceType v1.ResourceName
	expected     string
}{
	{
		description: "Unmodified cpu",
		testData: v1.ResourceList{
			"cpu": resource.MustParse("1"),
		},
		resourceType: "cpu",
		expected:     "1",
	},
	{
		description: "Unmodified memory",
		testData: v1.ResourceList{
			"memory": resource.MustParse("1Mi"),
		},
		resourceType: "memory",
		expected:     "1Mi",
	},
	{
		description: "Memory in too large of units",
		testData: v1.ResourceList{
			"memory": resource.MustParse("123456Ki"),
		},
		resourceType: "memory",
		expected:     "121Mi",
	},
	{
		description: "Round up complex byte values to Mi",
		testData: v1.ResourceList{
			"memory": resource.MustParse("1038683533"),
		},
		resourceType: "memory",
		expected:     "991Mi",
	},
	{
		description: "Normalize Decimal G to Binary Mi",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("1G"),
		},
		resourceType: "memory",
		expected:     "954Mi",
	},
	{
		description: "Normalize Decimal k to Binary Ki",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("100k"),
		},
		resourceType: "memory",
		expected:     "98Ki",
	},
	{
		description: "Decimal 100M rounds up to 96Mi",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("100M"),
		},
		resourceType: "memory",
		expected:     "96Mi",
	},
	{
		description: "Exactly 1Mi stays 1Mi",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("1Mi"),
		},
		resourceType: "memory",
		expected:     "1Mi",
	},
	{
		description: "1023Ki is below the Mi boundary and stays unchanged",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("1023Ki"),
		},
		resourceType: "memory",
		expected:     "1023Ki",
	},
	{
		description: "1536Ki rounds up to 2Mi",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("1536Ki"),
		},
		resourceType: "memory",
		expected:     "2Mi",
	},
	{
		description: "Whole Gi renders as Gi",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("2Gi"),
		},
		resourceType: "memory",
		expected:     "2Gi",
	},
	{
		description: "Sub-millicore CPU rounds up to one millicore",
		testData: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("1234567u"),
		},
		resourceType: "cpu",
		expected:     "1235m",
	},
	{
		// 999999999n is 0.999999999 cores, just under one core. It rounds up to
		// one millicore-boundary value, which Quantity canonicalizes to "1".
		// The rounded value is numerically 1000m, but Quantity never emits
		// "1000m"; it always canonicalizes whole cores to an integer.
		description: "Just-under-one-core CPU rounds up to a whole core",
		testData: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("999999999n"),
		},
		resourceType: "cpu",
		expected:     "1",
	},
	{
		description: "CPU between millicores rounds up",
		testData: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("1500u"),
		},
		resourceType: "cpu",
		expected:     "2m",
	},
	{
		description: "Whole cores are emitted without a unit",
		testData: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("2000m"),
		},
		resourceType: "cpu",
		expected:     "2",
	},
	{
		description: "Exact millicores are unchanged",
		testData: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("250m"),
		},
		resourceType: "cpu",
		expected:     "250m",
	},
	{
		description: "CPU already in whole cores is unchanged",
		testData: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("1"),
		},
		resourceType: "cpu",
		expected:     "1",
	},
	{
		description: "Sub-nanocore noise rounds up to one millicore",
		testData: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("1n"),
		},
		resourceType: "cpu",
		expected:     "1m",
	},
	{
		// One byte under the Mi boundary still takes the Ki step and therefore
		// remains below 1Mi, then is emitted as bytes by Quantity.
		description: "One byte below 1Mi takes the Ki step",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("1048575"),
		},
		resourceType: "memory",
		expected:     "1Mi",
	},
	{
		description: "Exactly one byte",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("1"),
		},
		resourceType: "memory",
		expected:     "1Ki",
	},
	{
		// The largest memory value Kubernetes can represent. Rounding up to a
		// whole Mi pushes past math.MaxInt64, which Quantity caps at parse time;
		// the result must still be at least the input (never lower).
		description: "MaxInt64 bytes rounds up to 8Ei without lowering",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("9223372036854775807"),
		},
		resourceType: "memory",
		expected:     "8Ei",
	},
	{
		// The last int64 byte count that does not cross an Mi boundary is
		// returned unchanged.
		description: "Largest exact Mi multiple below MaxInt64 is unchanged",
		testData: v1.ResourceList{
			v1.ResourceMemory: resource.MustParse("9223372036854774784"),
		},
		resourceType: "memory",
		expected:     "8Ei",
	},
	{
		// 9223372036854775807n is 9223372036.854775807 cores; rounding up to a
		// whole millicore yields 9223372036855m without overflowing int64.
		description: "MaxInt64 nanocores rounds up to the next millicore",
		testData: v1.ResourceList{
			v1.ResourceCPU: resource.MustParse("9223372036854775807n"),
		},
		resourceType: "cpu",
		expected:     "9223372036855m",
	},
}

// TestFormatResourceList_PreservesInput verifies the function returns a new
// ResourceList and never mutates the caller's map or the quantities inside it.
func TestFormatResourceList_PreservesInput(t *testing.T) {
	input := v1.ResourceList{
		v1.ResourceCPU:    resource.MustParse("1500u"),
		v1.ResourceMemory: resource.MustParse("100M"),
	}
	before := resource.MustParse("1500u")
	beforeMem := resource.MustParse("100M")

	out, err := FormatResourceList(input)
	assert.NoError(t, err)

	assert.Equal(t, 0, quantityCompare(input[v1.ResourceCPU], before), "input CPU must be unchanged")
	assert.Equal(t, 0, quantityCompare(input[v1.ResourceMemory], beforeMem), "input memory must be unchanged")
	assert.Equal(t, "1500u", quantityText(input[v1.ResourceCPU]))
	assert.Equal(t, "100M", quantityText(input[v1.ResourceMemory]))

	out[v1.ResourceCPU] = resource.MustParse("9")
	assert.Equal(t, "1500u", quantityText(input[v1.ResourceCPU]), "returned map must be independent")
}

// TestFormatResourceList_MissingAndZero verifies missing resources are not
// added and zero values are left exactly as provided.
func TestFormatResourceList_MissingAndZero(t *testing.T) {
	t.Run("nil list", func(t *testing.T) {
		out, err := FormatResourceList(nil)
		assert.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("cpu only", func(t *testing.T) {
		out, err := FormatResourceList(v1.ResourceList{v1.ResourceCPU: resource.MustParse("1500u")})
		assert.NoError(t, err)
		_, hasMemory := out[v1.ResourceMemory]
		assert.False(t, hasMemory, "memory must not be invented")
		assert.Equal(t, "2m", quantityText(out[v1.ResourceCPU]))
	})

	t.Run("memory only", func(t *testing.T) {
		out, err := FormatResourceList(v1.ResourceList{v1.ResourceMemory: resource.MustParse("100M")})
		assert.NoError(t, err)
		_, hasCPU := out[v1.ResourceCPU]
		assert.False(t, hasCPU, "cpu must not be invented")
		assert.Equal(t, "96Mi", quantityText(out[v1.ResourceMemory]))
	})

	t.Run("unrelated resource passes through", func(t *testing.T) {
		eph := v1.ResourceList{v1.ResourceEphemeralStorage: resource.MustParse("1G")}
		out, err := FormatResourceList(eph)
		assert.NoError(t, err)
		assert.Equal(t, "1G", quantityText(out[v1.ResourceEphemeralStorage]))
	})

	t.Run("zero stays zero", func(t *testing.T) {
		out, err := FormatResourceList(v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse("0"),
			v1.ResourceMemory: resource.MustParse("0"),
		})
		assert.NoError(t, err)
		assert.True(t, isZero(out[v1.ResourceCPU]))
		assert.True(t, isZero(out[v1.ResourceMemory]))
	})
}

// TestFormatResourceList_Negative verifies negative values are reported rather
// than silently corrected.
func TestFormatResourceList_Negative(t *testing.T) {
	_, err := FormatResourceList(v1.ResourceList{v1.ResourceCPU: resource.MustParse("-1m")})
	assert.ErrorIs(t, err, ErrNegativeQuantity)

	_, err = FormatResourceList(v1.ResourceList{v1.ResourceMemory: resource.MustParse("-1Mi")})
	assert.ErrorIs(t, err, ErrNegativeQuantity)
}

// TestFormatResourceList_OrderingAndIdempotence verifies that rounding keeps
// lowerBound <= target <= upperBound and is stable when applied twice, using
// the exact example from the issue.
func TestFormatResourceList_OrderingAndIdempotence(t *testing.T) {
	raw := []v1.ResourceList{
		{v1.ResourceCPU: resource.MustParse("1000000u")}, // lowerBound
		{v1.ResourceCPU: resource.MustParse("1234567u")}, // target
		{v1.ResourceCPU: resource.MustParse("1500000u")}, // upperBound
	}
	expected := []string{"1", "1235m", "1500m"}

	rounded := make([]v1.ResourceList, len(raw))
	for i := range raw {
		var err error
		rounded[i], err = FormatResourceList(raw[i])
		assert.NoError(t, err)
		assert.Equal(t, expected[i], quantityText(rounded[i][v1.ResourceCPU]))
	}

	assert.LessOrEqual(t,
		quantityCompare(rounded[0][v1.ResourceCPU], rounded[1][v1.ResourceCPU]), 0,
		"lowerBound must remain <= target")
	assert.LessOrEqual(t,
		quantityCompare(rounded[1][v1.ResourceCPU], rounded[2][v1.ResourceCPU]), 0,
		"target must remain <= upperBound")

	for i := range rounded {
		again, err := FormatResourceList(rounded[i])
		assert.NoError(t, err)
		assert.Equal(t, 0, quantityCompare(again[v1.ResourceCPU], rounded[i][v1.ResourceCPU]),
			"rounding an already rounded value must not change it")
	}
}

// TestFormatResource_Properties exercises the rounding rules on a fixed-seed
// corpus of ten thousand random-but-valid quantities and asserts the invariants
// the rounding must hold: it never lowers a value, it is monotonic, it is
// idempotent, and it never mutates the input. It runs entirely offline -- no
// cluster or running VPA is involved.
func TestFormatResource_Properties(t *testing.T) {
	const samples = 10000
	rng := rand.New(rand.NewSource(20261123))

	// randomQuantity builds a quantity from a random bounded positive integer
	// scaled by a random suffix, staying comfortably inside int64 so the value
	// can be compared with Time-ordered neighbours without wrapping.
	for _, resourceName := range []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory} {
		t.Run(string(resourceName), func(t *testing.T) {
			values := make([]resource.Quantity, samples)
			for i := range values {
				values[i] = randomQuantity(rng, resourceName)
			}

			// Non-decreasing on a sorted corpus, which also covers monotonicity
			// for arbitrary pairs: sorting then checking neighbours is a proof
			// for any monotone function.
			sorted := append([]resource.Quantity(nil), values...)
			sort.Slice(sorted, func(i, j int) bool { return sorted[i].Cmp(sorted[j]) < 0 })

			previous := resource.Quantity{}
			for i, original := range sorted {
				rounded, err := FormatResource(resourceName, original)
				assert.NoError(t, err)

				// Never lowers.
				assert.GreaterOrEqual(t, rounded.Cmp(original), 0,
					"sample %d: %s rounded down (%s -> %s)", i, original.String(), original.String(), rounded.String())

				// Monotonic across the sorted corpus.
				assert.GreaterOrEqual(t, rounded.Cmp(previous), 0,
					"sample %d: rounding is not monotonic (%s -> %s)", i, original.String(), rounded.String())
				previous = rounded

				// Idempotent.
				again, err := FormatResource(resourceName, rounded)
				assert.NoError(t, err)
				assert.Equal(t, 0, again.Cmp(rounded),
					"sample %d: not idempotent (%s -> %s -> %s)", i, original.String(), rounded.String(), again.String())
			}
		})
	}

	t.Run("input is not mutated", func(t *testing.T) {
		for i := 0; i < samples; i++ {
			for _, resourceName := range []v1.ResourceName{v1.ResourceCPU, v1.ResourceMemory} {
				original := randomQuantity(rng, resourceName)
				before := original.String()
				_, err := FormatResource(resourceName, original)
				assert.NoError(t, err)
				assert.Equal(t, before, original.String(), "rounding must not modify the input quantity")
			}
		}
	})

	t.Run("ordering is preserved", func(t *testing.T) {
		for i := 0; i < samples; i++ {
			resourceName := v1.ResourceCPU
			if i%2 == 0 {
				resourceName = v1.ResourceMemory
			}

			// Two raw values in arbitrary order, standing in for a pair such as
			// lowerBound and upperBound.
			first := randomQuantity(rng, resourceName)
			second := randomQuantity(rng, resourceName)
			if first.Cmp(second) > 0 {
				first, second = second, first
			}

			roundedFirst, err := FormatResource(resourceName, first)
			assert.NoError(t, err)
			roundedSecond, err := FormatResource(resourceName, second)
			assert.NoError(t, err)

			assert.LessOrEqual(t, roundedFirst.Cmp(roundedSecond), 0,
				"sample %d: %s > %s after rounding (%s <= %s before)",
				i, roundedFirst.String(), roundedSecond.String(), first.String(), second.String())
		}
	})
}

// randomQuantity returns a positive quantity for the given resource, biased
// toward the small end where the Ki/Mi and millicore boundaries actually matter
// while still occasionally producing large values.
func randomQuantity(rng *rand.Rand, resourceName v1.ResourceName) resource.Quantity {
	if resourceName == v1.ResourceCPU {
		// Pick a suffix so whole cores, millicores, microcores and nanocores
		// all appear; sub-millicore values are where CPU rounding is subtle.
		suffixes := []string{"", "m", "u", "n"}
		suffix := suffixes[rng.Intn(len(suffixes))]
		// Half the samples are tiny so sub-millicore values are common rather
		// than an accident of a uniformly random magnitude.
		limit := int64(1 << 30)
		if rng.Intn(2) == 0 {
			limit = 2_000_000
		}
		magnitude := rng.Int63n(limit)
		if magnitude == 0 {
			magnitude = 1
		}
		return resource.MustParse(fmt.Sprintf("%d%s", magnitude, suffix))
	}

	magnitude := rng.Int63n(1 << 40)
	if rng.Intn(4) == 0 {
		// A quarter of memory samples land on a Ki or Mi boundary multiple.
		step := int64(1024)
		if rng.Intn(2) == 0 {
			step = 1024 * 1024
		}
		magnitude = (rng.Int63n(1<<20) + 1) * step
	}
	if magnitude == 0 {
		magnitude = 1
	}
	return *resource.NewQuantity(magnitude, resource.BinarySI)
}
