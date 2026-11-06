// Package msg carries a user-visible message as data: an i18n key and the
// values to fill into it. The core and its parts (buffer, lsp, acp, proxy)
// build Msg values and never translate them; a UI turns one into text with
// i18n.Text, in the language it has selected. This is the same idea as sending
// a theme colour by name.
//
// A Msg whose Key is empty is a concatenation: its Args are joined, each Msg or
// error among them translated first. S wraps text that is not translated (a
// file name, a message from another program) in that form.
package msg

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Msg is an i18n key with the values for its format verbs.
type Msg struct {
	Key  string
	Args []any
}

// T is the message for key with args. An arg may itself be a Msg or an error.
func T(key string, args ...any) Msg { return Msg{Key: key, Args: args} }

// S is s as it is, without translation.
func S(s string) Msg {
	if s == "" {
		return Msg{}
	}
	return Msg{Args: []any{s}}
}

// Cat joins parts: strings are kept as they are, Msg and error values are
// translated when the message is shown.
func Cat(parts ...any) Msg { return Msg{Args: parts} }

// IsZero reports whether m is empty. It also makes `omitzero` work in JSON.
func (m Msg) IsZero() bool { return m.Key == "" && len(m.Args) == 0 }

// String is for logs and test failures: the key and its values, not text in a
// language. A UI uses i18n.Text.
func (m Msg) String() string {
	var sb strings.Builder
	if m.Key != "" {
		sb.WriteString(m.Key)
		sb.WriteByte('(')
	}
	for i, a := range m.Args {
		if i > 0 && m.Key != "" {
			sb.WriteString(", ")
		}
		fmt.Fprint(&sb, a)
	}
	if m.Key != "" {
		sb.WriteByte(')')
	}
	return sb.String()
}

// wire is the JSON form of a Msg that is not plain text: the key and the
// values, for the UI to translate. Key is absent for a concatenation.
type wire struct {
	Key  string `json:"key,omitempty"`
	Args []any  `json:"args"`
}

// MarshalJSON writes m as JSON. Text with no key (S, or a Cat of
// strings) is a JSON string. Anything else is {"key", "args"}. An arg that is a Msg or an
// error becomes a nested object (or a string, when it is plain text); numbers,
// booleans and strings stay as they are; any other value is its fmt.Sprint text.
func (m Msg) MarshalJSON() ([]byte, error) {
	if s, ok := m.plain(); ok {
		return json.Marshal(s)
	}
	w := wire{Key: m.Key, Args: make([]any, len(m.Args))}
	for i, a := range m.Args {
		w.Args[i] = wireArg(a)
	}
	return json.Marshal(w)
}

// plain is the text of m when it has no key and only string args.
func (m Msg) plain() (string, bool) {
	if m.Key != "" {
		return "", false
	}
	var sb strings.Builder
	for _, a := range m.Args {
		s, ok := a.(string)
		if !ok {
			return "", false
		}
		sb.WriteString(s)
	}
	return sb.String(), true
}

func wireArg(a any) any {
	switch v := a.(type) {
	case Msg:
		return v
	case *Error:
		return v.Msg
	case error:
		return v.Error()
	case fmt.Stringer:
		return v.String()
	case string, bool:
		return v
	}
	switch reflect.ValueOf(a).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return a
	}
	return fmt.Sprint(a)
}

// UnmarshalJSON reads what MarshalJSON writes. A string is untranslated text.
func (m *Msg) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*m = S(s)
		return nil
	}
	var w struct {
		Key  string            `json:"key"`
		Args []json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	out := Msg{Key: w.Key, Args: make([]any, len(w.Args))}
	for i, raw := range w.Args {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		if _, ok := v.(map[string]any); ok {
			var sub Msg
			if err := json.Unmarshal(raw, &sub); err != nil {
				return err
			}
			v = sub
		}
		out.Args[i] = v
	}
	*m = out
	return nil
}

// Error is an error whose text is a Msg. Args that are errors are wrapped, so
// errors.Is and errors.As see through it.
type Error struct{ Msg Msg }

// Err is an error for key and args.
func Err(key string, args ...any) error { return &Error{T(key, args...)} }

// ErrOf is an error carrying m.
func ErrOf(m Msg) error { return &Error{m} }

// Error is the String form of the message, for logs. A UI uses i18n.ErrText.
func (e *Error) Error() string { return e.Msg.String() }

// Unwrap lists the errors among the args.
func (e *Error) Unwrap() []error {
	var out []error
	for _, a := range e.Msg.Args {
		if err, ok := a.(error); ok {
			out = append(out, err)
		}
	}
	return out
}

// Of is the message of err: the Msg of a *Error, and err's own text otherwise.
func Of(err error) Msg {
	if err == nil {
		return Msg{}
	}
	if e, ok := err.(*Error); ok {
		return e.Msg
	}
	return Cat(err)
}

// Add is m followed by parts. Text added to text is merged, so a message
// grown one chunk at a time (a streamed reply) stays flat.
func (m Msg) Add(parts ...any) Msg {
	if m.Key != "" {
		return Cat(append([]any{m}, parts...)...)
	}
	args := append([]any(nil), m.Args...)
	for _, p := range parts {
		if pm, ok := p.(Msg); ok && pm.Key == "" {
			args = append(args, pm.Args...)
			continue
		}
		args = append(args, p)
	}
	// Merge neighbouring strings.
	out := args[:0]
	for _, a := range args {
		if s, ok := a.(string); ok && len(out) > 0 {
			if prev, ok := out[len(out)-1].(string); ok {
				out[len(out)-1] = prev + s
				continue
			}
		}
		out = append(out, a)
	}
	return Msg{Args: out}
}
