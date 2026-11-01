package core

import (
	"regexp"
	"strings"
)

var (
	mdOrderedRe = regexp.MustCompile(`^(\s*)(\d+)[.)]\s+(.*)$`)
	mdBulletRe  = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	mdHeadingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	mdFenceRe   = regexp.MustCompile("^\\s*(```+|~~~+)\\s*([^\\s`]*)")
	mdTableSep  = regexp.MustCompile(`^\s*\|?\s*:?-{1,}:?\s*(\|\s*:?-{1,}:?\s*)*\|?\s*$`)
)

type listMatch struct {
	indent int
	marker string
	text   string
}

func listItem(line string) *listMatch {
	if m := mdBulletRe.FindStringSubmatch(line); m != nil {
		return &listMatch{len(m[1]), "•", m[2]}
	}
	if m := mdOrderedRe.FindStringSubmatch(line); m != nil {
		return &listMatch{len(m[1]), m[2] + ".", m[3]}
	}
	return nil
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// isRule reports whether line is a thematic break (---, ***, ___).
func isRule(line string) bool {
	s := strings.ReplaceAll(strings.TrimSpace(line), " ", "")
	if len(s) < 3 || strings.Trim(s, string(s[0])) != "" {
		return false
	}
	return s[0] == '-' || s[0] == '*' || s[0] == '_'
}

// MdKind says what an MdBlock is.
type MdKind int

const (
	MdGap        MdKind = iota // a blank line between blocks
	MdPara                     // Text, the lines of a paragraph joined by spaces
	MdFence                    // Code and Lang, a fenced block (or YAML front matter)
	MdRule                     // a thematic break
	MdHeading                  // Level and Text
	MdQuote                    // Inner, the blocks of the quoted text
	MdTableBlock               // Rows and Aligns
	MdList                     // Prefix (indent, marker, checkbox) and Text
)

// MdBlock is one block of a Markdown document, before it has a width. Text and
// code are as written; how they wrap, where a table's columns are and how wide
// a rule is belong to the TUI (formatMarkdown). A block other than MdGap is
// followed by a gap when it is formatted, except an MdList, which says so with
// GapAfter, and an MdHeading, which is preceded by one too.
type MdBlock struct {
	Kind     MdKind
	Text     string
	Lang     string
	Code     []string
	Level    int
	Prefix   string
	GapAfter bool
	Rows     [][]string
	Aligns   []string
	Inner    []MdBlock
}

// ParseMarkdown splits src into blocks. frontMatter shows a leading block
// fenced by "---" lines as YAML instead of rules around a paragraph.
func ParseMarkdown(src string, frontMatter bool) []MdBlock {
	var out []MdBlock
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	start := 0
	if frontMatter && len(lines) > 0 && strings.TrimRight(lines[0], " \t") == "---" {
		for j := 1; j < len(lines); j++ {
			if t := strings.TrimRight(lines[j], " \t"); t == "---" || t == "..." {
				out = append(out, MdBlock{Kind: MdFence, Code: lines[1:j], Lang: "yaml"})
				start = j + 1
				break
			}
		}
	}
	var para []string
	flushPara := func() {
		if len(para) == 0 {
			return
		}
		out = append(out, MdBlock{Kind: MdPara, Text: strings.Join(para, " ")})
		para = nil
	}

	for i := start; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")

		if m := mdFenceRe.FindStringSubmatch(line); m != nil {
			flushPara()
			fence, lang := m[1], m[2]
			var code []string
			for i++; i < len(lines); i++ {
				if t := strings.TrimSpace(lines[i]); len(t) >= len(fence) && strings.Trim(t, string(fence[0])) == "" {
					break
				}
				code = append(code, lines[i])
			}
			out = append(out, MdBlock{Kind: MdFence, Code: code, Lang: lang})
			continue
		}

		switch {
		case strings.TrimSpace(line) == "":
			flushPara()
			out = append(out, MdBlock{Kind: MdGap})
			continue
		case isRule(line):
			flushPara()
			out = append(out, MdBlock{Kind: MdRule})
			continue
		}

		if m := mdHeadingRe.FindStringSubmatch(line); m != nil {
			flushPara()
			out = append(out, MdBlock{Kind: MdHeading, Level: len(m[1]), Text: m[2]})
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), ">") {
			flushPara()
			var q []string
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), ">"); i++ {
				q = append(q, strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">"), " "))
			}
			i--
			out = append(out, MdBlock{Kind: MdQuote, Inner: ParseMarkdown(strings.Join(q, "\n"), false)})
			continue
		}

		if i+1 < len(lines) && strings.Contains(line, "|") && mdTableSep.MatchString(lines[i+1]) && strings.Contains(lines[i+1], "-") {
			flushPara()
			rows := [][]string{splitRow(line)}
			aligns := splitRow(lines[i+1])
			for i += 2; i < len(lines) && strings.Contains(lines[i], "|") && strings.TrimSpace(lines[i]) != ""; i++ {
				rows = append(rows, splitRow(lines[i]))
			}
			i--
			out = append(out, MdBlock{Kind: MdTableBlock, Rows: rows, Aligns: aligns})
			continue
		}

		if lm := listItem(line); lm != nil {
			flushPara()
			prefix := strings.Repeat("  ", lm.indent/2) + lm.marker + " "
			// continuation lines belonging to this item
			text := lm.text
			for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" && listItem(lines[i+1]) == nil && strings.HasPrefix(lines[i+1], " ") && !mdFenceRe.MatchString(lines[i+1]) {
				i++
				text += " " + strings.TrimSpace(lines[i])
			}
			if strings.HasPrefix(text, "[ ] ") || strings.HasPrefix(text, "[x] ") || strings.HasPrefix(text, "[X] ") {
				box := "☐ "
				if text[1] != ' ' {
					box = "☑ "
				}
				text = text[4:]
				prefix += box
			}
			out = append(out, MdBlock{Kind: MdList, Prefix: prefix, Text: text, GapAfter: i+1 >= len(lines) || listItem(lines[i+1]) == nil})
			continue
		}

		para = append(para, strings.TrimSpace(line))
	}
	flushPara()
	return out
}
