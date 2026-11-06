package cmd

import (
	"os"
	"testing"

	"github.com/mm1995tk/shijimi-editor/core/acptest"
)

func TestMain(m *testing.M) {
	// A core may start the agent as a child of the test binary.
	acptest.Main()
	// Keep the developer's own config out of the cores.
	dir, err := os.MkdirTemp("", "shijimi-cli-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	os.Setenv("HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
