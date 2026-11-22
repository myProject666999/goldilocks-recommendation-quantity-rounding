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

package helpers

import (
	"reflect"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/fairwindsops/goldilocks/pkg/utils"
)

func PrintResource(quant resource.Quantity) string {
	if quant.IsZero() {
		return "Not Set"
	}
	return quant.String()
}

// comparisonValue rounds a current (deployed) quantity with the same steps the
// summary uses for recommendations, so that a difference caused purely by
// display rounding -- current 100M versus a recommendation shown as 96Mi --
// does not read as "less than". Only the comparison is rounded; callers still
// display the original value with PrintResource. If the value cannot be rounded
// (for example it is negative) it is compared as-is.
func comparisonValue(resourceName corev1.ResourceName, quant resource.Quantity) resource.Quantity {
	rounded, err := utils.FormatResource(resourceName, quant)
	if err != nil {
		return quant
	}
	return rounded
}

// GetStatus compares a current quantity against a recommendation for display.
// Both sides are compared after rounding the current value with the
// recommendation's steps; the recommendation passed in is already rounded by
// the summary, and rounding is idempotent.
func GetStatus(resourceName corev1.ResourceName, existing resource.Quantity, recommendation resource.Quantity, style string) string {
	if existing.IsZero() {
		switch style {
		case "text":
			return "error - not set"
		case "icon":
			return "fa-exclamation error"
		default:
			return ""
		}
	}

	roundedExisting := comparisonValue(resourceName, existing)
	comparison := roundedExisting.Cmp(comparisonValue(resourceName, recommendation))
	if comparison == 0 {
		switch style {
		case "text":
			return "equal"
		case "icon":
			return "fa-equals success"
		default:
			return ""
		}
	}
	if comparison < 0 {
		switch style {
		case "text":
			return "less than"
		case "icon":
			return "fa-less-than warning"
		default:
			return ""
		}
	}
	if comparison > 0 {
		switch style {
		case "text":
			return "greater than"
		case "icon":
			return "fa-greater-than warning"
		default:
			return ""
		}
	}
	return ""
}

// GetStatusRange compares a current quantity against the recommended lower and
// upper bounds. As with GetStatus, the current value and both bounds are
// compared after being rounded with the recommendation steps; the values
// themselves are displayed unrounded by PrintResource.
func GetStatusRange(resourceName corev1.ResourceName, existing, lower, upper resource.Quantity, style string, resourceType string) string {
	if existing.IsZero() {
		switch style {
		case "text":
			return "error - not set"
		case "icon":
			return "fa-exclamation error"
		default:
			return ""
		}
	}

	roundedExisting := comparisonValue(resourceName, existing)
	comparisonLower := roundedExisting.Cmp(comparisonValue(resourceName, lower))
	comparisonUpper := roundedExisting.Cmp(comparisonValue(resourceName, upper))

	if comparisonLower < 0 {
		switch style {
		case "text":
			return "less than"
		case "icon":
			return "fa-less-than warning"
		}
	}

	if comparisonUpper > 0 {
		switch style {
		case "text":
			return "greater than"
		case "icon":
			return "fa-greater-than warning"
		}
	}

	switch resourceType {
	case "request":
		if comparisonLower == 0 {
			switch style {
			case "text":
				return "equal"
			case "icon":
				return "fa-equals success"
			}
		}
	case "limit":
		if comparisonUpper == 0 {
			switch style {
			case "text":
				return "equal"
			case "icon":
				return "fa-equals success"
			}
		}
	}

	switch style {
	case "text":
		return "not equal"
	case "icon":
		return "fa-exclamation error"
	}

	return ""
}

func ResourceName(name string) corev1.ResourceName {
	return corev1.ResourceName(name)
}

func GetUUID() string {
	return uuid.New().String()
}

func HasField(v any, name string) bool {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return false
	}
	return rv.FieldByName(name).IsValid()
}
