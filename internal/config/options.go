package config

// Options is what the build decides about the settings. The configuration
// does not know what a release channel is: the UI that makes the core says
// which directory is the user's and what the defaults are.
type Options struct {
	// DirSuffix is appended to "shijimi-editor" for the user's configuration
	// directory, such as "-nightly". Empty is the plain `shijimi-editor`. When a
	// directory with a suffix does not exist, `shijimi-editor` is read instead
	// (see userDir).
	DirSuffix string
	// Experimental is the default of the [experimental] settings that are not
	// written in a file.
	Experimental bool
}
