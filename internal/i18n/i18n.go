// Package i18n holds every user-visible string of shijimi in one table per
// language (en.go, ja.go). Code asks for a string by key with T; the language
// is chosen once, from `ui-language` in config.toml, with SetLang.
package i18n

import (
	"fmt"
	"maps"
	"strings"
	"sync/atomic"

	"github.com/mm1995tk/shijimi-editor/internal/msg"
)

// Supported language codes.
const (
	English  = "en"
	Japanese = "ja"
)

// Default is the language used when `ui-language` is unset or unknown.
const Default = English

type selected struct {
	code  string
	table map[string]string
}

var current atomic.Pointer[selected]

func init() { SetLang(Default) }

// SetLang selects the UI language by its code ("en" or "ja"). An empty or
// unknown code selects the default, English.
func SetLang(code string) {
	if code == Japanese {
		current.Store(&selected{Japanese, ja})
		return
	}
	current.Store(&selected{English, en})
}

// Valid reports whether code names a supported language.
func Valid(code string) bool {
	return code == English || code == Japanese
}

// Lang returns the code of the language selected now.
func Lang() string { return current.Load().code }

// T returns the string for key in the selected language. A key missing from
// that table falls back to the English one, and then to the key itself, so a
// gap never blanks the screen.
func T(key string) string {
	if s, ok := current.Load().table[key]; ok {
		return s
	}
	if s, ok := en[key]; ok {
		return s
	}
	return key
}

// Text turns m into text in the selected language: the string for m.Key with
// its Args filled in. An arg that is a Msg or an error is translated first. A Msg
// with no key is its Args joined.
func Text(m msg.Msg) string {
	if m.Key == "" {
		var sb strings.Builder
		for _, a := range m.Args {
			sb.WriteString(argText(a))
		}
		return sb.String()
	}
	args := make([]any, len(m.Args))
	for i, a := range m.Args {
		args[i] = argValue(a)
	}
	return fmt.Sprintf(T(m.Key), args...)
}

// ErrText is the text of err: Text for an error made by msg, and its own text
// otherwise.
func ErrText(err error) string {
	if err == nil {
		return ""
	}
	if e, ok := err.(*msg.Error); ok {
		return Text(e.Msg)
	}
	return err.Error()
}

func argValue(a any) any {
	switch v := a.(type) {
	case msg.Msg:
		return Text(v)
	case error:
		return ErrText(v)
	}
	return a
}

func argText(a any) string {
	if s, ok := a.(string); ok {
		return s
	}
	return fmt.Sprint(argValue(a))
}

// Table is every string of the selected language, with the English strings
// under the ones it lacks (the same fallback as T). The GUI gets it with
// i18n.get and translates a Msg itself.
func Table() map[string]string {
	t := make(map[string]string, len(en))
	maps.Copy(t, en)
	maps.Copy(t, current.Load().table)
	return t
}
