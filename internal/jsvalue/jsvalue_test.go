package jsvalue

import (
	"math"
	"testing"
)

func TestString(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		expected string
	}{
		{"null", nil, "null"},
		{"undefined", Undefined, "undefined"},
		{"a string passes through", "masterbuilder", "masterbuilder"},
		{"an empty string passes through", "", ""},
		{"true", true, "true"},
		{"false", false, "false"},
		{"an int", 42, "42"},
		{"a negative int", -7, "-7"},
		{"an epoch millisecond stamp", int64(1787724658777), "1787724658777"},
		{"an unsigned int", uint(9), "9"},
		{"an integral float has no fractional part", float64(1787724658777), "1787724658777"},
		{"a fractional float", 0.5, "0.5"},
		{"an array joins with commas", []any{1, 2, 3}, "1,2,3"},
		{"a string array joins with commas", []string{"a", "b"}, "a,b"},
		{"an empty array is an empty string", []any{}, ""},
		{"null array elements render as empty", []any{1, nil, 3}, "1,,3"},
		{"undefined array elements render as empty", []any{1, Undefined, 3}, "1,,3"},
		{"a nested array flattens", []any{[]any{1, 2}, 3}, "1,2,3"},
		{"a map is the object tag", map[string]any{"valid": false}, "[object Object]"},
		{"an empty map is the object tag", map[string]any{}, "[object Object]"},
		{"a struct is the object tag", struct{ A int }{1}, "[object Object]"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if actual := String(test.value); actual != test.expected {
				t.Errorf("String(%#v) = %q, want %q", test.value, actual, test.expected)
			}
		})
	}
}

// This is the coercion that revokes a session: logout writes a map, the store
// renders it with String(), and the resulting text is not valid JSON.
func TestStringOfAnObjectIsNotValidJSON(t *testing.T) {
	record := map[string]any{"valid": false, "created": 1787724658777}
	if actual := String(record); actual != "[object Object]" {
		t.Fatalf("String(record) = %q, want %q", actual, "[object Object]")
	}
}

func TestStringOfPointers(t *testing.T) {
	value := "inner"
	if actual := String(&value); actual != "inner" {
		t.Errorf("String(&value) = %q, want %q", actual, "inner")
	}

	var missing *string
	if actual := String(missing); actual != "null" {
		t.Errorf("String((*string)(nil)) = %q, want %q", actual, "null")
	}
}

func TestNumber(t *testing.T) {
	cases := []struct {
		name     string
		value    float64
		expected string
	}{
		{"NaN", math.NaN(), "NaN"},
		{"positive infinity", math.Inf(1), "Infinity"},
		{"negative infinity", math.Inf(-1), "-Infinity"},
		{"zero", 0, "0"},
		{"negative zero prints without a sign", math.Copysign(0, -1), "0"},
		{"an integral value has no fractional part", 1787724658777, "1787724658777"},
		{"a negative integral value", -604800, "-604800"},
		{"a fractional value", 1.5, "1.5"},
		{"a value at the exponent threshold", 1e21, "1e+21"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if actual := Number(test.value); actual != test.expected {
				t.Errorf("Number(%v) = %q, want %q", test.value, actual, test.expected)
			}
		})
	}
}

func TestTruthy(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		expected bool
	}{
		{"null is falsy", nil, false},
		{"undefined is falsy", Undefined, false},
		{"false is falsy", false, false},
		{"true is truthy", true, true},
		{"an empty string is falsy", "", false},
		{"a non-empty string is truthy", "a", true},
		{"the string zero is truthy", "0", true},
		{"the integer zero is falsy", 0, false},
		{"a non-zero integer is truthy", 1, true},
		{"a negative integer is truthy", -1, true},
		{"the unsigned zero is falsy", uint(0), false},
		{"the float zero is falsy", 0.0, false},
		{"negative zero is falsy", math.Copysign(0, -1), false},
		{"NaN is falsy", math.NaN(), false},
		{"a non-zero float is truthy", 0.1, true},
		{"an empty array is truthy", []any{}, true},
		{"an empty map is truthy", map[string]any{}, true},
		{"a populated map is truthy", map[string]any{"valid": true}, true},
		{"a nil slice is falsy", []any(nil), false},
		{"a nil map is falsy", map[string]any(nil), false},
		{"a struct is truthy", struct{}{}, true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if actual := Truthy(test.value); actual != test.expected {
				t.Errorf("Truthy(%#v) = %v, want %v", test.value, actual, test.expected)
			}
		})
	}
}

func TestTruthyOfPointers(t *testing.T) {
	value := 0
	if !Truthy(&value) {
		t.Error("Truthy(&value) = false, want true: a pointer to zero is still an object")
	}

	var missing *int
	if Truthy(missing) {
		t.Error("Truthy((*int)(nil)) = true, want false")
	}
}
