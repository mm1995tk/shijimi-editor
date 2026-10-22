package lsp

import (
	"os"
	"testing"

	"github.com/mm1995tk/shijimi-editor/internal/i18n"
)

func TestMain(m *testing.M) {
	// The assertions read the Japanese strings.
	i18n.SetLang(i18n.Japanese)
	os.Exit(m.Run())
}
