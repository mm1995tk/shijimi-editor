// Package channel tells which release channel this binary was built for.
// The stable shj and the nightly shj-nightly are the same program; the
// channel only changes the names that keep the two apart when both are
// installed: the user's configuration directory, the socket directory, the
// version label and the defaults of the [experimental] settings.
package channel

// Channel is set at build time with
// -ldflags "-X github.com/mm1995tk/shijimi-editor/internal/channel.Channel=nightly".
// Empty means the stable channel.
var Channel = ""

// Nightly is the name of the nightly channel.
const Nightly = "nightly"

// IsNightly reports whether this is a nightly build.
func IsNightly() bool { return Channel == Nightly }

// Suffix returns "-" followed by the channel name, such as "-nightly", or ""
// on the stable channel. It is appended to names shared with the stable
// build, such as "shijimi-editor" + Suffix() for the configuration directory.
func Suffix() string {
	if Channel == "" {
		return ""
	}
	return "-" + Channel
}
