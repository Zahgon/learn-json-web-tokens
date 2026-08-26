// Package jsvalue reproduces the handful of JavaScript value coercions that the
// original implementation depends on.
//
// These are not conveniences. helpers.js stores a plain object in LevelDB, and
// the store's default "utf8" value encoding runs it through String(), which
// yields the literal text "[object Object]". A later JSON.parse of that text
// throws, which is what revokes a session after logout. Reproducing the
// coercion faithfully is therefore part of reproducing the behaviour.
package jsvalue

import (
	"math"
	"reflect"
	"strconv"
)

// Undefined is the sentinel for JavaScript's undefined, which String() renders
// as "undefined" rather than "null".
type undefinedType struct{}

// Undefined represents the JavaScript value `undefined`.
var Undefined = undefinedType{}

// String mirrors the JavaScript String() function for the value kinds that can
// reach it in this project.
func String(value any) string {
	if value == nil {
		return "null"
	}
	if _, ok := value.(undefinedType); ok {
		return "undefined"
	}

	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return Number(rv.Float())
	case reflect.Slice, reflect.Array:
		// Array.prototype.toString joins with "," and renders null/undefined
		// elements as the empty string.
		out := ""
		for i := 0; i < rv.Len(); i++ {
			if i > 0 {
				out += ","
			}
			elem := rv.Index(i).Interface()
			if elem == nil {
				continue
			}
			if _, ok := elem.(undefinedType); ok {
				continue
			}
			out += String(elem)
		}
		return out
	case reflect.Ptr, reflect.Interface:
		if rv.IsNil() {
			return "null"
		}
		return String(rv.Elem().Interface())
	}

	// Objects (maps and structs) stringify to the well known tag. This is the
	// coercion the logout path relies on.
	return "[object Object]"
}

// Number mirrors JavaScript's number-to-string conversion for the range this
// project produces: integral values never gain a fractional part or exponent.
func Number(f float64) string {
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if f == 0 {
		// String(-0) is "0" in JavaScript, where strconv would give "-0".
		return "0"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// Truthy mirrors the JavaScript truthiness test. The falsy set is exactly
// false, 0, -0, NaN, "", null and undefined; everything else is truthy,
// including empty objects and empty arrays.
func Truthy(value any) bool {
	if value == nil {
		return false
	}
	if _, ok := value.(undefinedType); ok {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() != 0
	case reflect.Float32, reflect.Float64:
		f := rv.Float()
		return f != 0 && !math.IsNaN(f)
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return !rv.IsNil()
	}

	return true
}
