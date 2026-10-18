package plan

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// What the shell backend's JSON tool makes of a setting passed to it as text.
func TestArgument(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]any{
		"60":            json.Number("60"),
		" 60 ":          json.Number("60"),
		"060":           json.Number("60"),
		"+60":           json.Number("60"),
		"-060":          json.Number("-60"),
		"000":           json.Number("0"),
		"60.0":          json.Number("60.0"),
		"6e1":           json.Number("6e1"),
		"60s":           "60s",
		"0x3c":          "0x3c",
		"+":             "+",
		"+-6":           "+-6",
		"6 0":           "6 0",
		"":              "",
		"null":          "null",
		"false":         "false",
		"true":          true,
		"192.168.1.5":   "192.168.1.5",
		"192.168.1.0/2": "192.168.1.0/2",
		`"quoted"`:      "quoted",
		"[1]":           []any{json.Number("1")},
	} {
		require.Equal(t, want, argument(text), text)
	}
}

func TestNumberOf(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"50", "050", "+50", "1e2", "50.5", "0.0"} {
		_, isNumber := numberOf(text)
		require.True(t, isNumber, text)
	}
	// forms a number parser of Go would take
	for _, text := range []string{"", " 50", "50 ", "1_000", "Inf", "NaN", "0x32", "0x1p-2", "true", `"50"`, "fifty"} {
		_, isNumber := numberOf(text)
		require.False(t, isNumber, text)
	}
}

// A rule's field holds one value as it is and becomes a list with the second one.
func TestPatch(t *testing.T) {
	t.Parallel()
	rule := newObject("action", "route")
	patch(rule, "rule_set", "a")
	content, err := indented(rule)
	require.NoError(t, err)
	require.Equal(t, "{\n  \"action\": \"route\",\n  \"rule_set\": \"a\"\n}\n", string(content))
	patch(rule, "rule_set", "b")
	patch(rule, "rule_set", []any{"c", "d"})
	value, _ := rule.get("rule_set")
	require.Equal(t, []any{"a", "b", "c", "d"}, value)
	require.Equal(t, []string{"action", "rule_set"}, rule.keys())
}

// Keys keep their order through reading and writing, a key set twice keeps its first
// place with the last value, and text is written without escapes for HTML.
func TestObjectRoundTrip(t *testing.T) {
	t.Parallel()
	parsed, err := parseValue([]byte(`{"z":1,"a":{"c":[1.50,1E+2,12345678901234567890123],"b":"<&> é"},"z":2}`))
	require.NoError(t, err)
	var compact []byte
	content, err := indented(parsed)
	require.NoError(t, err)
	for _, character := range content {
		if character != ' ' && character != '\n' {
			compact = append(compact, character)
		}
	}
	require.Equal(t, `{"z":2,"a":{"c":[1.50,1E+2,12345678901234567890123],"b":"<&>é"}}`, string(compact))
	_, err = parseValue([]byte(`{"a":1} {"b":2}`))
	require.Error(t, err)
	_, err = parseValue([]byte(`{"a":1,}`))
	require.Error(t, err)
}

// Numbers as jq 1.7 writes one it only passed through (checked against its output on the
// stand for the forms marked so; the rest follow the same rule of the decimal arithmetic).
func TestCanonicalNumber(t *testing.T) {
	t.Parallel()
	for literal, want := range map[string]string{
		"60":                      "60",      // stand
		"60.0":                    "60.0",    // stand
		"0.0":                     "0.0",     // stand
		"6e1":                     "6E+1",    // stand
		"1e2":                     "1E+2",    // stand
		"2e3":                     "2E+3",    // stand
		"1.08e3":                  "1.08E+3", // stand
		"2080.0":                  "2080.0",  // stand
		"1E+2":                    "1E+2",
		"1.50":                    "1.50",
		"-0":                      "-0",
		"0":                       "0",
		"5e-1":                    "0.5",
		"1e-7":                    "1E-7",
		"1e-6":                    "0.000001",
		"100e-2":                  "1.00",
		"-1.5e10":                 "-1.5E+10",
		"0.001":                   "0.001",
		"12345678901234567890123": "12345678901234567890123",
		"4294967295":              "4294967295",
	} {
		require.Equal(t, want, canonicalNumber(literal), literal)
	}
}

// Text as jq writes it: what has to be escaped and nothing more.
func TestWriteString(t *testing.T) {
	t.Parallel()
	content, err := indented("a\"b\\c/\b\t\n\f\r\x01\x1f\x7f <&> \u2028 é \U0001F600 \xff")
	require.NoError(t, err)
	require.Equal(t, `"a\"b\\c/\b\t\n\f\r\u0001\u001f\u007f <&> `+"\u2028"+` é `+"\U0001F600"+" \ufffd\"\n", string(content))
}
