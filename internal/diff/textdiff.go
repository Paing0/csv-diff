package diff

import (
	"strings"

	"csv-diff/internal/model"
)

const maxMyersD = 400 // Limit edit search to keep cell diffs instantaneous

// DiffStrings computes fine-grained differences between two UTF-8 strings.
// It splits strings into grapheme clusters to prevent corrupting Unicode characters.
func DiffStrings(original, modified string) (origChunks []model.DiffChunk, modChunks []model.DiffChunk) {
	if original == modified {
		chunk := model.DiffChunk{Type: model.ChunkEqual, Text: original}
		return []model.DiffChunk{chunk}, []model.DiffChunk{chunk}
	}

	if original == "" {
		modChunk := model.DiffChunk{Type: model.ChunkInsert, Text: modified}
		return nil, []model.DiffChunk{modChunk}
	}

	if modified == "" {
		origChunk := model.DiffChunk{Type: model.ChunkDelete, Text: original}
		return []model.DiffChunk{origChunk}, nil
	}

	origTokens := SplitGraphemes(original)
	modTokens := SplitGraphemes(modified)

	// If tokens are too large for fine-grained diff, fall back to whole-cell diff
	if len(origTokens)+len(modTokens) > 2000 {
		return fallbackDiff(original, modified)
	}

	edits := myersDiff(origTokens, modTokens)
	if edits == nil {
		// Fallback if edit distance exceeded maxMyersD
		return fallbackDiff(original, modified)
	}

	return buildChunks(edits)
}

func fallbackDiff(original, modified string) ([]model.DiffChunk, []model.DiffChunk) {
	var origChunks []model.DiffChunk
	var modChunks []model.DiffChunk
	if original != "" {
		origChunks = []model.DiffChunk{{Type: model.ChunkDelete, Text: original}}
	}
	if modified != "" {
		modChunks = []model.DiffChunk{{Type: model.ChunkInsert, Text: modified}}
	}
	return origChunks, modChunks
}

type editOp struct {
	op   model.DiffChunkType
	text string
}

// myersDiff implements the Myers diff algorithm on token slices.
func myersDiff(a, b []string) []editOp {
	n := len(a)
	m := len(b)
	maxD := n + m
	if maxD > maxMyersD {
		maxD = maxMyersD
	}

	offset := maxD
	v := make([]int, 2*maxD+1)
	trace := make([][]int, 0, maxD+1)

	for d := 0; d <= maxD; d++ {
		vCopy := make([]int, len(v))
		copy(vCopy, v)
		trace = append(trace, vCopy)

		for k := -d; k <= d; k += 2 {
			var x int
			kIdx := k + offset
			if k == -d || (k != d && v[kIdx-1] < v[kIdx+1]) {
				x = v[kIdx+1] // move down (insert from b)
			} else {
				x = v[kIdx-1] + 1 // move right (delete from a)
			}
			y := x - k

			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[kIdx] = x

			if x >= n && y >= m {
				// Reconstruct path
				return backtrackMyers(trace, a, b, d, k, offset)
			}
		}
	}

	return nil // exceeded maxD
}

func backtrackMyers(trace [][]int, a, b []string, d, k, offset int) []editOp {
	x := len(a)
	y := len(b)
	var script []editOp

	for step := d; step > 0; step-- {
		v := trace[step]
		kIdx := k + offset

		var prevK int
		if k == -step || (k != step && v[kIdx-1] < v[kIdx+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}

		prevX := v[prevK+offset]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			script = append(script, editOp{op: model.ChunkEqual, text: a[x-1]})
			x--
			y--
		}

		if step > 0 {
			if x == prevX {
				script = append(script, editOp{op: model.ChunkInsert, text: b[prevY]})
				y--
			} else {
				script = append(script, editOp{op: model.ChunkDelete, text: a[prevX]})
				x--
			}
		}
		k = prevK
	}

	for x > 0 && y > 0 {
		script = append(script, editOp{op: model.ChunkEqual, text: a[x-1]})
		x--
		y--
	}

	// Reverse script to forward order
	for i, j := 0, len(script)-1; i < j; i, j = i+1, j-1 {
		script[i], script[j] = script[j], script[i]
	}

	return script
}

func buildChunks(edits []editOp) ([]model.DiffChunk, []model.DiffChunk) {
	var origChunks []model.DiffChunk
	var modChunks []model.DiffChunk

	var currOrigType model.DiffChunkType
	var currOrigBuf strings.Builder

	var currModType model.DiffChunkType
	var currModBuf strings.Builder

	flushOrig := func() {
		if currOrigBuf.Len() > 0 {
			origChunks = append(origChunks, model.DiffChunk{
				Type: currOrigType,
				Text: currOrigBuf.String(),
			})
			currOrigBuf.Reset()
		}
	}

	flushMod := func() {
		if currModBuf.Len() > 0 {
			modChunks = append(modChunks, model.DiffChunk{
				Type: currModType,
				Text: currModBuf.String(),
			})
			currModBuf.Reset()
		}
	}

	for _, edit := range edits {
		switch edit.op {
		case model.ChunkEqual:
			// Belongs to both
			if currOrigType != model.ChunkEqual {
				flushOrig()
				currOrigType = model.ChunkEqual
			}
			currOrigBuf.WriteString(edit.text)

			if currModType != model.ChunkEqual {
				flushMod()
				currModType = model.ChunkEqual
			}
			currModBuf.WriteString(edit.text)

		case model.ChunkDelete:
			// Belongs to original only
			if currOrigType != model.ChunkDelete {
				flushOrig()
				currOrigType = model.ChunkDelete
			}
			currOrigBuf.WriteString(edit.text)

		case model.ChunkInsert:
			// Belongs to modified only
			if currModType != model.ChunkInsert {
				flushMod()
				currModType = model.ChunkInsert
			}
			currModBuf.WriteString(edit.text)
		}
	}

	flushOrig()
	flushMod()

	return origChunks, modChunks
}
