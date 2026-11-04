package cli

import (
	"github.com/mm1995tk/shijimi-editor/core/config"
	"github.com/mm1995tk/shijimi-editor/core/msg"
	"github.com/mm1995tk/shijimi-editor/ui/internal/buildinfo"
	uicfg "github.com/mm1995tk/shijimi-editor/ui/internal/config"
	"github.com/mm1995tk/shijimi-editor/ui/internal/i18n"
)

// What cmd (the dispatcher that starts a UI) needs of the parts the UIs share
// (ui/internal/...), which only the packages under ui/ may import. cmd names
// them through this file and adds nothing of its own.

// Section is the settings the TUI and the GUI share, for
// config.Options.Sections.
var Section = uicfg.Section

// SetLangFrom selects the UI language that cfg names. It is also right for a
// cfg that Load returned with a mistake: the settings that did read give the
// language the mistakes are reported in.
func SetLangFrom(cfg *config.Config) { i18n.SetLang(uicfg.Of(cfg).UILanguage) }

// T is the string for key in the selected language.
func T(key string) string { return i18n.T(key) }

// Text is m in the selected language.
func Text(m msg.Msg) string { return i18n.Text(m) }

// ErrText is err in the selected language.
func ErrText(err error) string { return i18n.ErrText(err) }

// Nightly is the name of the nightly channel.
const Nightly = buildinfo.Nightly

// IsNightly reports whether this is a nightly build.
func IsNightly() bool { return buildinfo.IsNightly() }

// Suffix is "-nightly" on the nightly channel and "" on the stable one: the
// names that keep the two apart (the configuration and socket directories).
func Suffix() string { return buildinfo.Suffix() }

// Label is the version as shown on the status line.
func Label() string { return buildinfo.Label() }

// Resolve is the version and commit of this build. Either may be empty.
func Resolve() (version, commit string) { return buildinfo.Resolve() }
