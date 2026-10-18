package plan

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

// object is a JSON object that keeps the order its keys were set in. The configuration is
// written for people to read and to compare with the one the shell backend writes, key by
// key in the same order; the values come from several places (an outbound decoded from a
// link, one written by hand), and their own order is kept as well.
type object struct {
	members []member
}

type member struct {
	key   string
	value any
}

// newObject makes an object of key, value pairs.
func newObject(pairs ...any) *object {
	result := new(object)
	for index := 0; index+1 < len(pairs); index += 2 {
		result.set(pairs[index].(string), pairs[index+1])
	}
	return result
}

func (o *object) set(key string, value any) {
	for index := range o.members {
		if o.members[index].key == key {
			o.members[index].value = value
			return
		}
	}
	o.members = append(o.members, member{key, value})
}

func (o *object) get(key string) (any, bool) {
	for _, entry := range o.members {
		if entry.key == key {
			return entry.value, true
		}
	}
	return nil, false
}

// append sets the keys of pairs, as newObject does.
func (o *object) append(pairs ...any) {
	for index := 0; index+1 < len(pairs); index += 2 {
		o.set(pairs[index].(string), pairs[index+1])
	}
}

// remove deletes a key; one that is not there is no error.
func (o *object) remove(key string) {
	for index := range o.members {
		if o.members[index].key == key {
			o.members = append(o.members[:index], o.members[index+1:]...)
			return
		}
	}
}

// insertAfter sets a key right after another one, or at the end when that one is not
// there. A key that is there already keeps its place.
func (o *object) insertAfter(after string, key string, value any) {
	if _, exists := o.get(key); exists {
		o.set(key, value)
		return
	}
	for index := range o.members {
		if o.members[index].key == after {
			o.members = append(o.members[:index+1], append([]member{{key, value}}, o.members[index+1:]...)...)
			return
		}
	}
	o.members = append(o.members, member{key, value})
}

// keys returns the keys in their order.
func (o *object) keys() []string {
	result := make([]string, 0, len(o.members))
	for _, entry := range o.members {
		result = append(result, entry.key)
	}
	return result
}

func (o *object) MarshalJSON() ([]byte, error) {
	var buffer bytes.Buffer
	err := encode(&buffer, o)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// encode writes a value built of objects, slices, strings, numbers and booleans, byte for
// byte as the JSON tool of the shell backend (jq 1.7) writes the same value.
func encode(buffer *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case string:
		writeString(buffer, typed)
	case json.Number:
		buffer.WriteString(canonicalNumber(string(typed)))
	case *object:
		buffer.WriteByte('{')
		for index, entry := range typed.members {
			if index > 0 {
				buffer.WriteByte(',')
			}
			writeString(buffer, entry.key)
			buffer.WriteByte(':')
			err := encode(buffer, entry.value)
			if err != nil {
				return err
			}
		}
		buffer.WriteByte('}')
	case []any:
		buffer.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				buffer.WriteByte(',')
			}
			err := encode(buffer, item)
			if err != nil {
				return err
			}
		}
		buffer.WriteByte(']')
	case []string:
		items := make([]any, len(typed))
		for index, item := range typed {
			items[index] = item
		}
		return encode(buffer, items)
	default:
		encoder := json.NewEncoder(buffer)
		encoder.SetEscapeHTML(false)
		err := encoder.Encode(typed)
		if err != nil {
			return err
		}
		// the encoder ends every value with a line feed
		buffer.Truncate(buffer.Len() - 1)
	}
	return nil
}

// writeString writes text as a JSON string: quotes, backslashes and control characters
// escaped, DEL among them, everything else as it is.
func writeString(buffer *bytes.Buffer, text string) {
	buffer.WriteByte('"')
	for _, character := range text {
		switch {
		case character == '"' || character == '\\':
			buffer.WriteByte('\\')
			buffer.WriteRune(character)
		case character == '\b':
			buffer.WriteString(`\b`)
		case character == '\t':
			buffer.WriteString(`\t`)
		case character == '\n':
			buffer.WriteString(`\n`)
		case character == '\f':
			buffer.WriteString(`\f`)
		case character == '\r':
			buffer.WriteString(`\r`)
		case character < 0x20 || character == 0x7f:
			buffer.WriteString(`\u00`)
			buffer.WriteByte(hexDigits[character>>4])
			buffer.WriteByte(hexDigits[character&0xf])
		default:
			// bytes that are no UTF-8 come out of the range as U+FFFD
			buffer.WriteRune(character)
		}
	}
	buffer.WriteByte('"')
}

const hexDigits = "0123456789abcdef"

// canonicalNumber writes a JSON number in the form of the General Decimal Arithmetic
// ("to-scientific-string"), which is how a number that was only passed through comes out
// of jq: 60 and 60.0 stay, 6e1 is 6E+1, 1e-7 is 1E-7. Digits are kept, none is rounded.
func canonicalNumber(literal string) string {
	mantissa, exponentText, _ := strings.Cut(strings.ToLower(literal), "e")
	exponent, err := strconv.Atoi(exponentText)
	if err != nil && exponentText != "" {
		return literal
	}
	var sign string
	if strings.HasPrefix(mantissa, "-") {
		sign, mantissa = "-", mantissa[1:]
	}
	whole, fraction, _ := strings.Cut(mantissa, ".")
	exponent -= len(fraction)
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		digits = "0"
	}
	adjusted := exponent + len(digits) - 1
	switch {
	case exponent == 0:
		return sign + digits
	case exponent < 0 && adjusted >= -6:
		if point := len(digits) + exponent; point > 0 {
			return sign + digits[:point] + "." + digits[point:]
		} else {
			return sign + "0." + strings.Repeat("0", -point) + digits
		}
	}
	result := sign + digits[:1]
	if len(digits) > 1 {
		result += "." + digits[1:]
	}
	if adjusted < 0 {
		return result + "E-" + strconv.Itoa(-adjusted)
	}
	return result + "E+" + strconv.Itoa(adjusted)
}

// indented writes a value as a document: two spaces a level, a line feed at the end.
func indented(value any) ([]byte, error) {
	var compact bytes.Buffer
	err := encode(&compact, value)
	if err != nil {
		return nil, err
	}
	var result bytes.Buffer
	err = json.Indent(&result, compact.Bytes(), "", "  ")
	if err != nil {
		return nil, err
	}
	result.WriteByte('\n')
	return result.Bytes(), nil
}

// parseValue reads one JSON value, objects as *object with their keys in order and
// numbers as written.
func parseValue(content []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	value, err := decodeValue(decoder)
	if err != nil {
		return nil, err
	}
	_, err = decoder.Token()
	if err != io.EOF {
		return nil, E.New("more than one JSON value")
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return token, nil
	}
	switch delimiter {
	case '{':
		result := new(object)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, isString := keyToken.(string)
			if !isString {
				return nil, E.New("a key that is no string")
			}
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			result.set(key, value)
		}
		_, err = decoder.Token()
		if err != nil {
			return nil, err
		}
		return result, nil
	case '[':
		result := []any{}
		for decoder.More() {
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		_, err = decoder.Token()
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	return nil, E.New("unexpected ", delimiter.String())
}

// argument takes a setting the way the shell backend passes one to its JSON tool: text
// that is a JSON value (a number, a quoted string, a list) is that value, anything else,
// "null" and "false" included, is a string. A TTL written as 60 is thus the number 60, and
// a mistyped one is a string the engine then refuses.
func argument(text string) any {
	value, err := parseValue([]byte(text))
	if err != nil {
		if whole, isWhole := wholeNumber(text); isWhole {
			return whole
		}
		return text
	}
	if value == nil || value == false {
		return text
	}
	return value
}

// wholeNumber reads digits that are no JSON number and that the JSON tool of the shell
// backend takes for one all the same: zeros in front, a plus sign. A form field that
// only asks for a number lets both through.
func wholeNumber(text string) (json.Number, bool) {
	text = strings.Trim(text, " \t\n\r")
	var sign string
	if text != "" && (text[0] == '+' || text[0] == '-') {
		if text[0] == '-' {
			sign = "-"
		}
		text = text[1:]
	}
	if text == "" {
		return "", false
	}
	for _, character := range []byte(text) {
		if character < '0' || character > '9' {
			return "", false
		}
	}
	text = strings.TrimLeft(text, "0")
	if text == "" {
		text = "0"
	}
	return json.Number(sign + text), true
}

// numberOf reads a setting that has to be a number, in any form the JSON tool of the
// shell backend takes.
func numberOf(text string) (json.Number, bool) {
	if text != strings.Trim(text, " \t\n\r") {
		// the shell backend converts this one inside the JSON tool, which takes no spaces
		return "", false
	}
	number, isNumber := argument(text).(json.Number)
	return number, isNumber
}

// extend adds to the value of a rule's field: one value is kept as it is, and becomes a
// list when a second is added.
func extend(current any, added any) any {
	currentList, currentIsList := current.([]any)
	addedList, addedIsList := added.([]any)
	switch {
	case currentIsList && addedIsList:
		return append(append([]any{}, currentList...), addedList...)
	case currentIsList:
		return append(append([]any{}, currentList...), added)
	case addedIsList:
		return append([]any{current}, addedList...)
	}
	return []any{current, added}
}

// patch sets a field of a rule, or adds to the value it has.
func patch(rule *object, key string, value any) {
	current, found := rule.get(key)
	if found {
		rule.set(key, extend(current, value))
	} else {
		rule.set(key, value)
	}
}

func stringsToAny(items []string) []any {
	result := make([]any, len(items))
	for index, item := range items {
		result[index] = item
	}
	return result
}

// Pretty writes JSON the way the shell backend's JSON tool prints a value it was given:
// two spaces a level, the keys in their order, a line feed at the end.
func Pretty(content []byte) ([]byte, error) {
	value, err := parseValue(content)
	if err != nil {
		return nil, err
	}
	return indented(value)
}

// Masked is Pretty with the values the function hidden names replaced by the word MASKED:
// a configuration that can be shown to others. hidden is given the path of the object a
// member is in, its keys from the top joined by dots (arrays leave no mark: the members
// of an outbound are at "outbounds", those of its TLS at "outbounds.tls"), and the key.
func Masked(content []byte, hidden func(path string, key string) bool) ([]byte, error) {
	value, err := parseValue(content)
	if err != nil {
		return nil, err
	}
	mask(value, "", hidden)
	return indented(value)
}

func mask(value any, path string, hidden func(path string, key string) bool) {
	switch typed := value.(type) {
	case *object:
		for index := range typed.members {
			key := typed.members[index].key
			if hidden(path, key) {
				typed.members[index].value = "MASKED"
				continue
			}
			inner := key
			if path != "" {
				inner = path + "." + key
			}
			mask(typed.members[index].value, inner, hidden)
		}
	case []any:
		for _, item := range typed {
			mask(item, path, hidden)
		}
	}
}
