package csvparser

import (
	"strings"
	"testing"
)

func TestParseStandardCSV(t *testing.T) {
	input := "id,name,role\n1,Alice,Engineer\n2,Bob,Designer\n"
	parsed, err := Parse(strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.Header) != 3 {
		t.Fatalf("expected 3 headers, got %d", len(parsed.Header))
	}
	if parsed.Header[0] != "id" || parsed.Header[1] != "name" || parsed.Header[2] != "role" {
		t.Errorf("unexpected headers: %v", parsed.Header)
	}
	if len(parsed.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(parsed.Rows))
	}
	if parsed.Rows[0][1] != "Alice" {
		t.Errorf("expected Alice, got %s", parsed.Rows[0][1])
	}
}

func TestParseUTF8BOM(t *testing.T) {
	input := "\xef\xbb\xbfid,name\n1,John\n"
	parsed, err := Parse(strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Header[0] != "id" {
		t.Errorf("expected 'id' without BOM, got %q", parsed.Header[0])
	}
}

func TestParseCRLFandLF(t *testing.T) {
	inputCRLF := "id,val\r\n1,A\r\n2,B\r\n"
	parsedCRLF, err := Parse(strings.NewReader(inputCRLF), DefaultParseOptions())
	if err != nil {
		t.Fatalf("CRLF parse error: %v", err)
	}

	inputLF := "id,val\n1,A\n2,B\n"
	parsedLF, err := Parse(strings.NewReader(inputLF), DefaultParseOptions())
	if err != nil {
		t.Fatalf("LF parse error: %v", err)
	}

	if len(parsedCRLF.Rows) != len(parsedLF.Rows) {
		t.Errorf("CRLF rows %d != LF rows %d", len(parsedCRLF.Rows), len(parsedLF.Rows))
	}
}

func TestParseMultilineAndQuotes(t *testing.T) {
	input := "id,note\n1,\"Line 1\nLine 2\"\n2,\"He said \"\"Hello\"\"\"\n3,\"Comma, in, field\"\n"
	parsed, err := Parse(strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(parsed.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(parsed.Rows))
	}
	if parsed.Rows[0][1] != "Line 1\nLine 2" {
		t.Errorf("multiline mismatch: %q", parsed.Rows[0][1])
	}
	if parsed.Rows[1][1] != `He said "Hello"` {
		t.Errorf("escaped quotes mismatch: %q", parsed.Rows[1][1])
	}
	if parsed.Rows[2][1] != "Comma, in, field" {
		t.Errorf("quoted comma mismatch: %q", parsed.Rows[2][1])
	}
}

func TestParseVariableColumnCount(t *testing.T) {
	input := "col1,col2\n1,2,3,4\n5\n"
	parsed, err := Parse(strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.MaxColumnCount != 4 {
		t.Errorf("expected max cols 4, got %d", parsed.MaxColumnCount)
	}
	if len(parsed.Header) != 4 {
		t.Errorf("expected 4 headers (2 synthetic), got %d", len(parsed.Header))
	}
	if parsed.Header[2] != "Column 3" || parsed.Header[3] != "Column 4" {
		t.Errorf("synthetic header mismatch: %v", parsed.Header)
	}
}

func TestParseEmptyAndHeaderOnly(t *testing.T) {
	// Empty
	parsedEmpty, err := Parse(strings.NewReader(""), DefaultParseOptions())
	if err != nil {
		t.Fatalf("empty parse error: %v", err)
	}
	if len(parsedEmpty.Rows) != 0 || len(parsedEmpty.Header) != 0 {
		t.Errorf("expected empty result, got %v", parsedEmpty)
	}

	// Header-only
	parsedHeaderOnly, err := Parse(strings.NewReader("id,name,age\n"), DefaultParseOptions())
	if err != nil {
		t.Fatalf("header-only parse error: %v", err)
	}
	if len(parsedHeaderOnly.Header) != 3 || len(parsedHeaderOnly.Rows) != 0 {
		t.Errorf("expected 3 headers and 0 rows, got %d headers and %d rows",
			len(parsedHeaderOnly.Header), len(parsedHeaderOnly.Rows))
	}
}

func TestParseUnicodeBurmeseAndEmoji(t *testing.T) {
	input := "id,burmese,emoji\n1,မင်္ဂလာပါ,👨‍👩‍👧‍👦\n2,ကွန်ပျူတာ,🎉\n"
	parsed, err := Parse(strings.NewReader(input), DefaultParseOptions())
	if err != nil {
		t.Fatalf("unicode parse error: %v", err)
	}

	if parsed.Rows[0][1] != "မင်္ဂလာပါ" {
		t.Errorf("burmese text mismatch: %q", parsed.Rows[0][1])
	}
	if parsed.Rows[0][2] != "👨‍👩‍👧‍👦" {
		t.Errorf("emoji sequence mismatch: %q", parsed.Rows[0][2])
	}
}

func TestParseNoHeader(t *testing.T) {
	input := "100,200\n300,400\n"
	opts := DefaultParseOptions()
	opts.HasHeader = false
	parsed, err := Parse(strings.NewReader(input), opts)
	if err != nil {
		t.Fatalf("no header parse error: %v", err)
	}

	if len(parsed.Header) != 2 || parsed.Header[0] != "Column 1" || parsed.Header[1] != "Column 2" {
		t.Errorf("expected synthetic headers Column 1, Column 2, got %v", parsed.Header)
	}
	if len(parsed.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(parsed.Rows))
	}
}

func TestParseMalformedCSV(t *testing.T) {
	// Unclosed quote with LazyQuotes = false
	input := "id,name\n1,\"unclosed quote\n2,normal\n"
	opts := DefaultParseOptions()
	opts.LazyQuotes = false // strictly enforce quoting rules
	_, err := Parse(strings.NewReader(input), opts)
	if err == nil {
		t.Errorf("expected error on malformed CSV with unclosed quote, got nil")
	}
}
