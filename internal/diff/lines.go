package diff

// Block is a run of changed lines: old[OldStart:OldEnd] was replaced by
// new[NewStart:NewEnd]. Either side may be empty (pure insertion/deletion).
type Block struct {
	OldStart, OldEnd int
	NewStart, NewEnd int
}

// maxEditDistance bounds the Myers search; beyond it the whole differing
// middle is reported as one block.
const maxEditDistance = 2000

// Lines compares two line lists and returns the changed blocks in order.
func Lines(old, cur []string) []Block {
	pre := 0
	for pre < len(old) && pre < len(cur) && old[pre] == cur[pre] {
		pre++
	}
	suf := 0
	for suf < len(old)-pre && suf < len(cur)-pre && old[len(old)-1-suf] == cur[len(cur)-1-suf] {
		suf++
	}
	a, b := old[pre:len(old)-suf], cur[pre:len(cur)-suf]
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	var blocks []Block
	if len(a) == 0 || len(b) == 0 {
		blocks = []Block{{0, len(a), 0, len(b)}}
	} else {
		blocks = myers(a, b)
	}
	for i := range blocks {
		blocks[i].OldStart += pre
		blocks[i].OldEnd += pre
		blocks[i].NewStart += pre
		blocks[i].NewEnd += pre
	}
	return blocks
}

// myers runs the O(ND) shortest-edit-script algorithm.
func myers(a, b []string) []Block {
	n, m := len(a), len(b)
	maxD := min(n+m, maxEditDistance)
	off := maxD + 1
	v := make([]int, 2*maxD+3)
	var trace [][]int
	found := -1
search:
	for d := 0; d <= maxD; d++ {
		snap := make([]int, len(v))
		copy(snap, v)
		trace = append(trace, snap)
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[off+k] = x
			if x >= n && y >= m {
				found = d
				break search
			}
		}
	}
	if found < 0 {
		return []Block{{0, n, 0, m}}
	}
	// Walk back to recover the edit positions.
	var dels, ins []int // indexes into a / b
	x, y := n, m
	for d := found; d > 0; d-- {
		vv := trace[d]
		k := x - y
		var pk int
		if k == -d || (k != d && vv[off+k-1] < vv[off+k+1]) {
			pk = k + 1
		} else {
			pk = k - 1
		}
		px := vv[off+pk]
		py := px - pk
		if pk == k+1 {
			ins = append(ins, py)
		} else {
			dels = append(dels, px)
		}
		x, y = px, py
	}
	// dels/ins were collected from the end; merge them into blocks.
	for i, j := 0, len(dels)-1; i < j; i, j = i+1, j-1 {
		dels[i], dels[j] = dels[j], dels[i]
	}
	for i, j := 0, len(ins)-1; i < j; i, j = i+1, j-1 {
		ins[i], ins[j] = ins[j], ins[i]
	}
	return mergeEdits(dels, ins, n, m)
}

// mergeEdits turns sorted deleted/inserted line indexes into blocks.
func mergeEdits(dels, ins []int, n, m int) []Block {
	var blocks []Block
	oi, ni := 0, 0
	// ox/ny: old/new positions consumed so far
	ox, ny := 0, 0
	for oi < len(dels) || ni < len(ins) {
		// advance through the matching region to the next edit
		nextOld, nextNew := n, m
		if oi < len(dels) {
			nextOld = dels[oi]
		}
		if ni < len(ins) {
			nextNew = ins[ni]
		}
		// matched lines before the next edit: both sides advance together
		skip := min(nextOld-ox, nextNew-ny)
		ox += skip
		ny += skip
		blk := Block{OldStart: ox, NewStart: ny}
		for {
			progressed := false
			if oi < len(dels) && dels[oi] == ox {
				ox++
				oi++
				progressed = true
			}
			if ni < len(ins) && ins[ni] == ny {
				ny++
				ni++
				progressed = true
			}
			if !progressed {
				break
			}
		}
		blk.OldEnd, blk.NewEnd = ox, ny
		blocks = append(blocks, blk)
	}
	return blocks
}
