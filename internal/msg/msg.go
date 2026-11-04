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

// Marshal turns a Msg into the text the JSON protocol carries. The process that
// serves the protocol sets it (to i18n.Text), because the wire format is still
// localized text. Without it a Msg is written as its String form.
var Marshal func(Msg) string

// MarshalJSON writes m as a string.
func (m Msg) MarshalJSON() ([]byte, error) {
	if Marshal != nil {
		return json.Marshal(Marshal(m))
	}
	return json.Marshal(m.String())
}

// UnmarshalJSON reads a string as untranslated text.
func (m *Msg) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*m = S(s)
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
