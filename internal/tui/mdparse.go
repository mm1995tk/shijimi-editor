package tui

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
