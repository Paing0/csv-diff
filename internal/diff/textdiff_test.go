package diff

import (
	"strings"
	"testing"

	"csv-diff/internal/model"
)

func TestSplitGraphemesBurmese(t *testing.T) {
	// Burmese word "မင်္ဂလာပါ"
	word := "မင်္ဂလာပါ"
	clusters := SplitGraphemes(word)
	reconstructed := strings.Join(clusters, "")
	if reconstructed != word {
		t.Errorf("expected %q, got %q", word, reconstructed)
	}

	// Verify stacked consonant "င်္ဂ" or "မင်္ဂ" does not break UTF-8 or virama
	for _, c := range clusters {
		if c == "\u1039" { // Isolated virama should not happen
			t.Errorf("found isolated virama in cluster: %v", clusters)
		}
	}
}

func TestSplitGraphemesEmojiZWJ(t *testing.T) {
	// Family emoji with ZWJ: 👨‍👩‍👧‍👦
	family := "👨‍👩‍👧‍👦"
	clusters := SplitGraphemes(family)
	if len(clusters) != 1 {
		t.Errorf("expected 1 grapheme cluster for ZWJ family emoji, got %d: %v", len(clusters), clusters)
	}
	if clusters[0] != family {
		t.Errorf("cluster mismatch: got %q, want %q", clusters[0], family)
	}
}

func TestSplitGraphemesCombiningAccents(t *testing.T) {
	// "e" + combining acute accent U+0301
	accented := "e\u0301"
	clusters := SplitGraphemes(accented)
	if len(clusters) != 1 {
		t.Errorf("expected 1 cluster for combining accent, got %d: %v", len(clusters), clusters)
	}
}

func TestDiffStringsIdentical(t *testing.T) {
	orig, mod := DiffStrings("hello world", "hello world")
	if len(orig) != 1 || orig[0].Type != model.ChunkEqual || orig[0].Text != "hello world" {
		t.Errorf("unexpected orig chunks: %v", orig)
	}
	if len(mod) != 1 || mod[0].Type != model.ChunkEqual || mod[0].Text != "hello world" {
		t.Errorf("unexpected mod chunks: %v", mod)
	}
}

func TestDiffStringsSimpleEdit(t *testing.T) {
	orig, mod := DiffStrings("kitten", "sitting")

	var origRebuilt, modRebuilt strings.Builder
	for _, c := range orig {
		origRebuilt.WriteString(c.Text)
	}
	for _, c := range mod {
		modRebuilt.WriteString(c.Text)
	}

	if origRebuilt.String() != "kitten" {
		t.Errorf("orig rebuilt mismatch: got %q, want 'kitten'", origRebuilt.String())
	}
	if modRebuilt.String() != "sitting" {
		t.Errorf("mod rebuilt mismatch: got %q, want 'sitting'", modRebuilt.String())
	}
}

func TestDiffStringsUnicodeBurmese(t *testing.T) {
	orig := "မင်္ဂလာပါ အားလုံး"
	mod := "မင်္ဂလာပါ လူကြီးမင်းတို့"

	origChunks, modChunks := DiffStrings(orig, mod)

	var origRebuilt, modRebuilt strings.Builder
	for _, c := range origChunks {
		origRebuilt.WriteString(c.Text)
	}
	for _, c := range modChunks {
		modRebuilt.WriteString(c.Text)
	}

	if origRebuilt.String() != orig {
		t.Errorf("Burmese orig mismatch: got %q, want %q", origRebuilt.String(), orig)
	}
	if modRebuilt.String() != mod {
		t.Errorf("Burmese mod mismatch: got %q, want %q", modRebuilt.String(), mod)
	}
}

func TestDiffStringsWhitespaceAndCase(t *testing.T) {
	// Leading / trailing whitespace and capitalization must NOT be trimmed or normalized
	orig := " Test "
	mod := " test "
	origChunks, modChunks := DiffStrings(orig, mod)

	var origRebuilt, modRebuilt strings.Builder
	for _, c := range origChunks {
		origRebuilt.WriteString(c.Text)
	}
	for _, c := range modChunks {
		modRebuilt.WriteString(c.Text)
	}

	if origRebuilt.String() != orig {
		t.Errorf("orig mismatch: got %q, want %q", origRebuilt.String(), orig)
	}
	if modRebuilt.String() != mod {
		t.Errorf("mod mismatch: got %q, want %q", modRebuilt.String(), mod)
	}

	// Must detect difference between 'T' and 't'
	hasDiff := false
	for _, c := range origChunks {
		if c.Type == model.ChunkDelete && c.Text == "T" {
			hasDiff = true
		}
	}
	if !hasDiff {
		t.Errorf("expected deletion of 'T', got %v", origChunks)
	}
}
