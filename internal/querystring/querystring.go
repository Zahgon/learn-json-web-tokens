// Package querystring is a port of Node.js's built-in `querystring` module,
// covering the parse and stringify behaviour this project relies on.
//
// The duplicate-key rule matters: a key seen more than once becomes an array
// rather than a string, and the credential check in helpers.js compares with
// strict equality, so an array never matches.
package querystring

import (
	"sort"
	"strings"
)

// Values is the object returned by Parse. A key seen once holds a string; a key
// seen more than once holds a []string.
type Values map[string]any

// maxKeys matches the module's default limit.
const maxKeys = 1000

// Parse splits a query string on "&" and "=", percent-decoding both sides and
// turning "+" into a space. Pairs whose key and value are both empty are
// dropped, matching the reference implementation.
func Parse(query string) Values {
	values := Values{}
	if query == "" {
		return values
	}

	keys := 0
	for _, pair := range strings.Split(query, "&") {
		if pair == "" {
			continue
		}
		if keys >= maxKeys {
			break
		}

		var rawKey, rawValue string
		if index := strings.IndexByte(pair, '='); index >= 0 {
			rawKey, rawValue = pair[:index], pair[index+1:]
		} else {
			rawKey = pair
		}

		key := Unescape(rawKey)
		value := Unescape(rawValue)
		if key == "" && value == "" {
			continue
		}

		existing, seen := values[key]
		if !seen {
			values[key] = value
			keys++
			continue
		}

		switch previous := existing.(type) {
		case string:
			values[key] = []string{previous, value}
		case []string:
			values[key] = append(previous, value)
		}
	}

	return values
}

// Stringify renders values back into a query string. Keys are emitted in sorted
// order so that the output is deterministic.
func Stringify(values Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var out strings.Builder
	for _, key := range keys {
		switch value := values[key].(type) {
		case string:
			appendPair(&out, key, value)
		case []string:
			for _, item := range value {
				appendPair(&out, key, item)
			}
		}
	}

	return out.String()
}

// StringifyPairs renders ordered key/value pairs, which is what a form
// submission looks like on the wire.
func StringifyPairs(pairs [][2]string) string {
	var out strings.Builder
	for _, pair := range pairs {
		appendPair(&out, pair[0], pair[1])
	}
	return out.String()
}

func appendPair(out *strings.Builder, key, value string) {
	if out.Len() > 0 {
		out.WriteByte('&')
	}
	out.WriteString(Escape(key))
	out.WriteByte('=')
	out.WriteString(Escape(value))
}

// unreserved lists the bytes querystring.escape leaves untouched.
func unreserved(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	}
	switch b {
	case '!', '\'', '(', ')', '*', '-', '.', '_', '~':
		return true
	}
	return false
}

const hexDigits = "0123456789ABCDEF"

// Escape percent-encodes a component the way querystring.escape does.
func Escape(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		b := value[i]
		if unreserved(b) {
			out.WriteByte(b)
			continue
		}
		out.WriteByte('%')
		out.WriteByte(hexDigits[b>>4])
		out.WriteByte(hexDigits[b&0x0f])
	}
	return out.String()
}

// Unescape reverses Escape. "+" becomes a space and malformed escapes are left
// exactly as they were found, matching the reference implementation.
func Unescape(value string) string {
	if !strings.ContainsAny(value, "+%") {
		return value
	}

	var out strings.Builder
	for i := 0; i < len(value); i++ {
		b := value[i]
		switch {
		case b == '+':
			out.WriteByte(' ')
		case b == '%' && i+2 < len(value):
			high, highOK := fromHex(value[i+1])
			low, lowOK := fromHex(value[i+2])
			if highOK && lowOK {
				out.WriteByte(high<<4 | low)
				i += 2
				continue
			}
			out.WriteByte(b)
		default:
			out.WriteByte(b)
		}
	}
	return out.String()
}

func fromHex(b byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	}
	return 0, false
}
