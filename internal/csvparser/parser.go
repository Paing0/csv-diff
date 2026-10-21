package csvparser

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ParsedCSV holds the extracted structure and rows of a parsed CSV file.
type ParsedCSV struct {
	Header           []string   // Column names (either from first row or synthetic)
	RawHeader        []string   // Original first row if hasHeader was true
	Rows             [][]string // Data rows (excluding header if hasHeader is true)
	RowStartLines    []int      // 1-based CSV file line number where each data row started
	HasHeader        bool
	MaxColumnCount   int
	TotalRecordCount int
}

// lineTrackingReader wraps an io.Reader and counts physical line numbers.
type lineTrackingReader struct {
	r           io.Reader
	currentLine int
	lastLine    int
	inCR        bool
}

func newLineTrackingReader(r io.Reader) *lineTrackingReader {
	return &lineTrackingReader{
		r:           r,
		currentLine: 1,
		lastLine:    1,
	}
}

func (lr *lineTrackingReader) Read(p []byte) (n int, err error) {
	n, err = lr.r.Read(p)
	for i := 0; i < n; i++ {
		b := p[i]
		if b == '\r' {
			lr.currentLine++
			lr.inCR = true
		} else if b == '\n' {
			if !lr.inCR {
				lr.currentLine++
			}
			lr.inCR = false
		} else {
			lr.inCR = false
		}
	}
	return n, err
}

// ParseOptions controls how CSV parsing behaves.
type ParseOptions struct {
	HasHeader        bool
	LazyQuotes       bool
	TrimLeadingSpace bool
	Comma            rune
}

// DefaultParseOptions provides the standard CSV parsing behavior.
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		HasHeader:        true,
		LazyQuotes:       true,
		TrimLeadingSpace: false,
		Comma:            ',',
	}
}

// Parse parses CSV content from an io.Reader, stripping any UTF-8 BOM.
func Parse(r io.Reader, opts ParseOptions) (*ParsedCSV, error) {
	// Read BOM if present
	br := bufio.NewReader(r)
	bom, err := br.Peek(3)
	if err == nil && len(bom) >= 3 && bom[0] == 0xef && bom[1] == 0xbb && bom[2] == 0xbf {
		_, _ = br.Discard(3)
	}

	csvReader := csv.NewReader(br)
	csvReader.FieldsPerRecord = -1 // Allow variable number of fields
	csvReader.LazyQuotes = opts.LazyQuotes
	csvReader.TrimLeadingSpace = opts.TrimLeadingSpace
	if opts.Comma != 0 {
		csvReader.Comma = opts.Comma
	}

	result := &ParsedCSV{
		HasHeader: opts.HasHeader,
	}

	var allRecords [][]string
	for {
		record, err := csvReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("CSV parse error: %w", err)
		}
		allRecords = append(allRecords, record)
	}

	if len(allRecords) == 0 {
		return result, nil
	}

	maxCols := 0
	for _, rec := range allRecords {
		if len(rec) > maxCols {
			maxCols = len(rec)
		}
	}
	result.MaxColumnCount = maxCols

	if opts.HasHeader && len(allRecords) > 0 {
		rawHdr := allRecords[0]
		result.RawHeader = make([]string, len(rawHdr))
		copy(result.RawHeader, rawHdr)

		hdr := make([]string, len(rawHdr))
		for i, h := range rawHdr {
			name := strings.TrimSpace(h)
			if name == "" {
				name = fmt.Sprintf("Column %d", i+1)
			}
			hdr[i] = name
		}

		// If some rows have more columns than the header, add synthetic headers
		if maxCols > len(hdr) {
			for i := len(hdr); i < maxCols; i++ {
				hdr = append(hdr, fmt.Sprintf("Column %d", i+1))
			}
		}

		result.Header = hdr
		if len(allRecords) > 1 {
			result.Rows = allRecords[1:]
		}
	} else {
		// Generate synthetic header names
		hdr := make([]string, maxCols)
		for i := 0; i < maxCols; i++ {
			hdr[i] = fmt.Sprintf("Column %d", i+1)
		}
		result.Header = hdr
		result.Rows = allRecords
	}

	result.TotalRecordCount = len(result.Rows)
	result.RowStartLines = make([]int, len(result.Rows))
	startOffset := 1
	if opts.HasHeader {
		startOffset = 2
	}
	for i := range result.Rows {
		result.RowStartLines[i] = startOffset + i
	}

	return result, nil
}

// ParseBytes is a convenience helper for byte slices.
func ParseBytes(data []byte, opts ParseOptions) (*ParsedCSV, error) {
	return Parse(bytes.NewReader(data), opts)
}
