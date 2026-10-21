package diff

import (
	"unicode"
	"unicode/utf8"
)

// isCombiningMark reports whether r is a Unicode combining character or modifier.
func isCombiningMark(r rune) bool {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r) {
		return true
	}
	// Variation Selectors
	if (r >= 0xFE00 && r <= 0xFE0F) || (r >= 0xE0100 && r <= 0xE01EF) {
		return true
	}
	// Emoji modifiers (Fitzpatrick skin tones)
	if r >= 0x1F3FB && r <= 0x1F3FF {
		return true
	}
	// Combining Enclosing Keycap
	if r == 0x20E3 {
		return true
	}
	// Burmese stacked consonant virama (U+1039) or asat (U+103A)
	if r == 0x1039 || r == 0x103A {
		return true
	}
	return false
}

// isRegionalIndicator reports whether r is an emoji regional indicator symbol (used for flags).
func isRegionalIndicator(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}

// SplitGraphemes segments a UTF-8 string into user-perceived character clusters
// (grapheme clusters), preventing corrupt splits of combining marks, Burmese scripts,
// and emoji sequences.
func SplitGraphemes(s string) []string {
	if len(s) == 0 {
		return nil
	}

	runes := []rune(s)
	n := len(runes)
	var clusters []string

	i := 0
	for i < n {
		start := i
		r := runes[i]
		i++

		// Rule: CRLF is a single cluster
		if r == '\r' && i < n && runes[i] == '\n' {
			i++
			clusters = append(clusters, string(runes[start:i]))
			continue
		}

		// Rule: Regional indicators form pairs (flags)
		if isRegionalIndicator(r) {
			if i < n && isRegionalIndicator(runes[i]) {
				i++
			}
			clusters = append(clusters, string(runes[start:i]))
			continue
		}

		// Extend cluster with combining marks and ZWJ continuations
		for i < n {
			next := runes[i]

			// If current is ZWJ, swallow ZWJ and the following base character + its combining marks
			if next == 0x200D { // ZWJ
				i++
				if i < n {
					i++ // swallow base rune following ZWJ
					// swallow any combining marks on that base rune
					for i < n && isCombiningMark(runes[i]) {
						i++
					}
				}
				continue
			}

			// If next is a combining mark, it attaches to the cluster
			if isCombiningMark(next) {
				i++
				continue
			}

			// Burmese stacked consonant: if previous was U+1039 (virama), the next consonant stacks
			if i > 0 && runes[i-1] == 0x1039 && unicode.Is(unicode.Lo, next) {
				i++
				continue
			}

			break
		}

		clusters = append(clusters, string(runes[start:i]))
	}

	// Double-check UTF-8 validation
	validClusters := make([]string, 0, len(clusters))
	for _, c := range clusters {
		if utf8.ValidString(c) {
			validClusters = append(validClusters, c)
		} else {
			// Fallback: rune-by-rune replacement
			validClusters = append(validClusters, string([]rune(c)))
		}
	}

	return validClusters
}
