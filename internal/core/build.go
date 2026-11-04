package core

import "github.com/mm1995tk/shijimi-editor/internal/config"

// Build is what the UI that makes the core says about the build it runs in.
// The core does not know the release channel or where the version comes from;
// it uses these values and nothing else.
type Build struct {
	// Config tells the configuration which directory is the user's and what its
	// defaults are (see config.Options).
	Config config.Options
	// Version is the build's version as the status line shows it and as the
	// agent is told it, such as "v0.1 (be64521)".
	Version string
}
