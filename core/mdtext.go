package core

import (
	"strings"

	"github.com/mm1995tk/shijimi-editor/core/screen"
)

// mdText holds the graphemes that the preview draws. A grapheme takes the
// style of its first byte, even when an inline marker or lexer token splits it.
type mdText []mdGlyph

type mdGlyph struct {
	text  string
	width int
	style *screen.Span
}

func mdGraphemes(spans []screen.Span) mdText {
	var plain strings.Builder
	ends := make([]int, len(spans))
	for i, sp := range spans {
		plain.WriteString(strings.ReplaceAll(sp.Text, "\t", "    "))
		ends[i] = plain.Len()
	}
	var glyphs mdText
	style, start := 0, 0
	for ch, width := range displayGraphemes(plain.String()) {
		for start >= ends[style] {
			style++
		}
		glyphs = append(glyphs, mdGlyph{ch, width, &spans[style]})
		start += len(ch)
	}
	return glyphs
}

func (text mdText) width() int {
	var width int
	for _, glyph := range text {
		width += glyph.width
	}
	return width
}

// prefix keeps as many whole graphemes as fit in w columns.
func (text mdText) prefix(w int) mdText {
	for i, glyph := range text {
		w -= glyph.width
		if w < 0 {
			return text[:i]
		}
	}
	return text
}

type mdWrappedLine struct {
	text  mdText
	space bool // a space separates this row from the next one when copied
}

// wrap prefers spaces in prose. In code every space belongs to a row,
// including each of the four spaces drawn for a tab.
func (text mdText) wrap(w int, atSpaces bool) []mdWrappedLine {
	w = max(1, w)
	var lines []mdWrappedLine
	for {
		n := len(text.prefix(w))
		if n == 0 && len(text) > 0 {
			n = 1 // a grapheme wider than the row occupies a row of its own
		}
		end, next := n, n
		if atSpaces && n < len(text) {
			if text[n].text != " " {
				for i := n - 1; i > 0; i-- {
					if text[i].text == " " {
						end, next = i, i
						break
					}
				}
			}
			for next < len(text) && text[next].text == " " {
				next++
			}
		}
		line := mdWrappedLine{text: text[:end], space: next > end}
		if atSpaces {
			for len(line.text) > 0 && line.text[len(line.text)-1].text == " " {
				line.text = line.text[:len(line.text)-1]
			}
		}
		lines = append(lines, line)
		if next == len(text) {
			return lines
		}
		text = text[next:]
	}
}

func (text mdText) spans() []screen.Span {
	var spans []screen.Span
	for len(text) > 0 {
		style := text[0].style
		var run strings.Builder
		for len(text) > 0 && text[0].style == style {
			run.WriteString(text[0].text)
			text = text[1:]
		}
		sp := *style
		sp.Text = run.String()
		spans = append(spans, sp)
	}
	return spans
}
