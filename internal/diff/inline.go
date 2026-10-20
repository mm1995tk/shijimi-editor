package diff

// Span is a range of runes [Start, End) within one line.
type Span struct{ Start, End int }

// MaxInlineRunes is the longest line Inline compares; longer lines are only
// shown as changed as a whole.
const MaxInlineRunes = 400

// Changes fewer than inlineJoinGap unchanged runes apart are drawn as one
// change, so the highlight does not break into many small pieces.
const inlineJoinGap = 3

// Inline compares a removed line with the line that replaced it, rune by
// rune, and returns the changed parts of each. ok is false when the lines are
// unrelated (fewer than half of the runes of the longer line are common) or
// too long to compare; the caller then marks the whole lines instead.
func Inline(old, cur string) (oldSpans, curSpans []Span, ok bool) {
	a, b := []rune(old), []rune(cur)
	if len(a) > MaxInlineRunes || len(b) > MaxInlineRunes {
		return nil, nil, false
	}
	// common >= max/2 ⇔ edits = n+m-2·common <= min(n, m)
	limit := min(len(a), len(b))
	blocks, found := compare(a, b, limit)
	if !found {
		return nil, nil, false
	}
	common := len(a)
	for _, bl := range blocks {
		common -= bl.OldEnd - bl.OldStart
	}
	if 2*common < max(len(a), len(b)) {
		return nil, nil, false
	}
	var joined []Block
	for _, bl := range blocks {
		if n := len(joined); n > 0 && bl.OldStart-joined[n-1].OldEnd < inlineJoinGap {
			joined[n-1].OldEnd, joined[n-1].NewEnd = bl.OldEnd, bl.NewEnd
			continue
		}
		joined = append(joined, bl)
	}
	for _, bl := range joined {
		if bl.OldEnd > bl.OldStart {
			oldSpans = append(oldSpans, Span{bl.OldStart, bl.OldEnd})
		}
		if bl.NewEnd > bl.NewStart {
			curSpans = append(curSpans, Span{bl.NewStart, bl.NewEnd})
		}
	}
	return oldSpans, curSpans, true
}
