// Package cli is the command-line side of the UIs that is not a screen:
// --help and --version. The control socket's client is in ctlclient.
package cli

import (
	"fmt"

	"github.com/mm1995tk/shijimi-editor/core/ctl"
	"github.com/mm1995tk/shijimi-editor/ui/internal/buildinfo"
	"github.com/mm1995tk/shijimi-editor/ui/internal/i18n"
)

// Version prints the version for --version.
func Version() {
	fmt.Println("shijimi " + buildinfo.Label())
}

// Help prints the usage for --help, in the language i18n is set to.
func Help() {
	fmt.Println(i18n.T("help.usage"))
	fmt.Println(i18n.T("help.single1"))
	fmt.Println(i18n.T("help.single2"))
	fmt.Println(i18n.T("help.single3"))
	fmt.Println("       shj diff [path]")
	fmt.Println(i18n.T("help.exec"))
	fmt.Println(i18n.T("help.skill"))
	fmt.Println(i18n.T("help.serve"))
	fmt.Println(i18n.T("help.attach"))
	fmt.Println("       shj -v, --version")
	fmt.Println(i18n.T("help.sub1") + ctl.EnvSocket + i18n.T("help.sub2"))
	fmt.Println(i18n.T("help.samename"))
}
