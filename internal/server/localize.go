package server

import (
	"github.com/mm1995tk/shijimi-editor/internal/i18n"
	"github.com/mm1995tk/shijimi-editor/internal/msg"
	"github.com/mm1995tk/shijimi-editor/internal/screen"
)

// The core and screen.Screen hold every message as a msg.Msg (an i18n key and
// its values); the language is the UI's to apply. The GUI protocol still carries
// the text in the language `shj serve` runs in, so the server translates a Msg
// as it writes the JSON. The next change sends the Msg itself, with the tables.
func init() { msg.Marshal = i18n.Text }

// Text is core.Port's: m in the language of this process.
func (s *Server) Text(m msg.Msg) string { return i18n.Text(m) }

// SetLanguage is core.Port's: the config of a core picked a language, and the
// process translates with it.
func (s *Server) SetLanguage(code string) { i18n.SetLang(code) }

// wireOverlays puts the overlays in the form the protocol has always had. The
// core gives a confirmation's question as the Title of the overlay; the wire
// format carries it as the one line of the overlay.
func wireOverlays(sc *screen.Screen) {
	for i := range sc.Overlay {
		ov := &sc.Overlay[i]
		if ov.Kind == screen.OverlayConfirm && !ov.Title.IsZero() {
			ov.Lines = []screen.Line{{Spans: []screen.Span{{Text: i18n.Text(ov.Title)}}}}
			ov.Title = msg.Msg{}
		}
	}
}
