package cmd

import (
	"github.com/mm1995tk/shijimi-editor/core"
	"github.com/mm1995tk/shijimi-editor/core/config"
	"github.com/mm1995tk/shijimi-editor/ui/cli"
	guicfg "github.com/mm1995tk/shijimi-editor/ui/gui/config"
	tuicfg "github.com/mm1995tk/shijimi-editor/ui/tui/config"
)

// coreBuild is what the core is told about this build: the user's
// configuration directory and the defaults that follow the release channel,
// and the version the status line and the agent show. The core reads nothing
// of the build itself.
func coreBuild() core.Build {
	return core.Build{
		Config:  configOptions(),
		Version: cli.Label(),
	}
}

// configOptions is what the configuration is told about this build: the
// directory, and the settings of the UIs (the one `shj` runs is one of them,
// and `kisui serve` another; all are accepted in every file).
func configOptions() config.Options {
	return config.Options{
		DirSuffix: cli.Suffix(),
		Sections:  []config.Section{cli.Section, tuicfg.Section(cli.IsNightly()), guicfg.Section},
	}
}
