package querystring

import (
	"reflect"
	"testing"
)

// The expectations below were captured from Node.js itself, so this file pins
// the port to the behaviour of the module it replaces rather than to an
// interpretation of it.

func TestParse(t *testing.T) {
	cases := []struct {
		name     string
		query    string
		expected Values
	}{
		{"empty string yields an empty object", "", Values{}},
		{"a lone separator yields an empty object", "&", Values{}},
		{"empty pairs are dropped", "a=1&&b=2", Values{"a": "1", "b": "2"}},
		{"a key with no separator gets an empty value", "a", Values{"a": ""}},
		{"a key with a trailing separator gets an empty value", "a=", Values{"a": ""}},
		{"an empty key is preserved when the value is not empty", "=b", Values{"": "b"}},
		{"a repeated key becomes an array", "a=1&a=2&a=3", Values{"a": []string{"1", "2", "3"}}},
		{"percent and plus escapes are decoded", "x=%20%2B+y", Values{"x": " + y"}},
		{"only the first separator splits a pair", "a=1=2", Values{"a": "1=2"}},
		{
			"the concatenated body of the shared-emitter dispatch",
			"username=badguy&password=kragleusername=masterbuilder&password=itsnosecret",
			Values{
				"username": "badguy",
				"password": []string{"kragleusername=masterbuilder", "itsnosecret"},
			},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actual := Parse(test.query)
			if !reflect.DeepEqual(actual, test.expected) {
				t.Errorf("Parse(%q) = %#v, want %#v", test.query, actual, test.expected)
			}
		})
	}
}

// A repeated key has to survive parsing as a []string, because the credential
// check compares with a string type assertion and an array can never match.
func TestParseRepeatedKeyIsNotAString(t *testing.T) {
	password := Parse("password=one&password=two")["password"]
	if _, isString := password.(string); isString {
		t.Fatalf("a repeated key parsed as a string, want []string: %#v", password)
	}
	if _, isSlice := password.([]string); !isSlice {
		t.Fatalf("a repeated key parsed as %T, want []string", password)
	}
}

func TestStringify(t *testing.T) {
	cases := []struct {
		name     string
		values   Values
		expected string
	}{
		{"an empty object yields an empty string", Values{}, ""},
		{"keys are emitted in sorted order", Values{"b": "2", "a": "1"}, "a=1&b=2"},
		{"an array value repeats its key", Values{"a": []string{"1", "2"}}, "a=1&a=2"},
		{"reserved characters are escaped", Values{"x": " +"}, "x=%20%2B"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if actual := Stringify(test.values); actual != test.expected {
				t.Errorf("Stringify(%#v) = %q, want %q", test.values, actual, test.expected)
			}
		})
	}
}

// Form submissions are order sensitive, so the pair form must not sort.
func TestStringifyPairsKeepsOrder(t *testing.T) {
	actual := StringifyPairs([][2]string{
		{"username", "masterbuilder"},
		{"password", "itsnosecret"},
	})
	expected := "username=masterbuilder&password=itsnosecret"
	if actual != expected {
		t.Errorf("StringifyPairs() = %q, want %q", actual, expected)
	}
}

func TestEscape(t *testing.T) {
	cases := []struct {
		value    string
		expected string
	}{
		{"", ""},
		{"abcXYZ019", "abcXYZ019"},
		{"!'()*-._~", "!'()*-._~"},
		{" ", "%20"},
		{"+", "%2B"},
		{"&=", "%26%3D"},
		{"/", "%2F"},
	}

	for _, test := range cases {
		if actual := Escape(test.value); actual != test.expected {
			t.Errorf("Escape(%q) = %q, want %q", test.value, actual, test.expected)
		}
	}
}

func TestUnescape(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		expected string
	}{
		{"a value with nothing to decode is returned as is", "plain", "plain"},
		{"plus becomes a space", "a+b", "a b"},
		{"percent escapes are decoded", "%20", " "},
		{"lowercase hex is accepted", "%2f", "/"},
		{"a truncated escape is left alone", "%2", "%2"},
		{"a trailing percent is left alone", "abc%", "abc%"},
		{"a non-hex escape is left alone", "%zz", "%zz"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if actual := Unescape(test.value); actual != test.expected {
				t.Errorf("Unescape(%q) = %q, want %q", test.value, actual, test.expected)
			}
		})
	}
}

func TestEscapeUnescapeRoundTrip(t *testing.T) {
	for _, value := range []string{"", "plain", "a b", "a+b", "%", "&=/?#", "masterbuilder"} {
		if actual := Unescape(Escape(value)); actual != value {
			t.Errorf("Unescape(Escape(%q)) = %q, want %q", value, actual, value)
		}
	}
}

func TestParseStopsAtTheKeyLimit(t *testing.T) {
	query := ""
	for i := 0; i < maxKeys+50; i++ {
		if i > 0 {
			query += "&"
		}
		query += "k" + string(rune('a'+i%26)) + Escape(string(rune(i))) + "=v"
	}

	if keys := len(Parse(query)); keys > maxKeys {
		t.Errorf("Parse kept %d keys, want at most %d", keys, maxKeys)
	}
}
