package cli

import (
	"github.com/mm1995tk/shijimi-editor/internal/buildinfo"
	"github.com/mm1995tk/shijimi-editor/internal/config"
	"github.com/mm1995tk/shijimi-editor/internal/core"
	uicfg "github.com/mm1995tk/shijimi-editor/internal/ui/config"
	guicfg "github.com/mm1995tk/shijimi-editor/internal/ui/gui/config"
	tuicfg "github.com/mm1995tk/shijimi-editor/internal/ui/tui/config"
)

// coreBuild is what the core is told about this build: the user's
// configuration directory and the defaults that follow the release channel,
// and the version the status line and the agent show. The core reads nothing
// of the build itself.
func coreBuild() core.Build {
	return core.Build{
		Config:  configOptions(),
		Version: buildinfo.Label(),
	}
}

// configOptions is what the configuration is told about this build: the
// directory, and the settings of the UIs (the one `shj` runs is one of them,
// and `shj serve` another; all are accepted in every file).
func configOptions() config.Options {
	return config.Options{
		DirSuffix: buildinfo.Suffix(),
		Sections:  []config.Section{uicfg.Section, tuicfg.Section(buildinfo.IsNightly()), guicfg.Section},
	}
}
