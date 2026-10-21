package session

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"csv-diff/internal/csvparser"
	"csv-diff/internal/model"
)

func buildSampleSession(t *testing.T, count int) (*Manager, string) {
	t.Helper()
	var origBuf, modBuf strings.Builder
	origBuf.WriteString("id,name,value\n")
	modBuf.WriteString("id,name,value\n")

	for i := 1; i <= count; i++ {
		origBuf.WriteString(fmt.Sprintf("%d,Item%d,orig%d\n", i, i, i))
		if i%2 == 0 {
			modBuf.WriteString(fmt.Sprintf("%d,Item%d,mod%d\n", i, i, i)) // modified
		} else {
			modBuf.WriteString(fmt.Sprintf("%d,Item%d,orig%d\n", i, i, i)) // identical
		}
	}
	// Add an extra row in mod
	modBuf.WriteString(fmt.Sprintf("%d,ItemExtra,modExtra\n", count+1))

	origParsed, err := csvparser.Parse(strings.NewReader(origBuf.String()), csvparser.DefaultParseOptions())
	if err != nil {
		t.Fatalf("orig parse error: %v", err)
	}
	modParsed, err := csvparser.Parse(strings.NewReader(modBuf.String()), csvparser.DefaultParseOptions())
	if err != nil {
		t.Fatalf("mod parse error: %v", err)
	}

	m := NewManager(1 * time.Hour)
	sessID := "test-session"
	m.CreateSession(sessID, origParsed, modParsed, model.FileMetadata{FileName: "orig.csv"}, model.FileMetadata{FileName: "mod.csv"})

	return m, sessID
}

func TestSessionPaginationAnd100Limit(t *testing.T) {
	// 250 rows -> 125 modified + 1 added = 126 differing rows
	m, sessID := buildSampleSession(t, 250)

	opts := model.CompareOptions{Mode: model.ModePositional}
	summary, diffs, err := m.RunComparison(context.Background(), sessID, opts)
	if err != nil {
		t.Fatalf("comparison failed: %v", err)
	}

	if summary.TotalDifferingRows != 126 {
		t.Fatalf("expected 126 differing rows, got %d", summary.TotalDifferingRows)
	}
	if len(diffs) != 126 {
		t.Fatalf("expected 126 diffs in cache, got %d", len(diffs))
	}

	// 1. Initial page: limit 100
	resp1, err := m.QueryResults(sessID, model.QueryResultsRequest{
		SessionID: sessID,
		Offset:    0,
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("query page 1 error: %v", err)
	}
	if len(resp1.Rows) != 100 {
		t.Errorf("expected 100 rows in first page, got %d", len(resp1.Rows))
	}
	if !resp1.HasMore {
		t.Errorf("expected HasMore = true for page 1")
	}

	// 2. Next page or load remaining: offset 100, limit 0 (all remaining)
	resp2, err := m.QueryResults(sessID, model.QueryResultsRequest{
		SessionID: sessID,
		Offset:    100,
		Limit:     0, // all remaining
	})
	if err != nil {
		t.Fatalf("query remaining error: %v", err)
	}
	if len(resp2.Rows) != 26 {
		t.Errorf("expected 26 rows in remaining chunk, got %d", len(resp2.Rows))
	}
	if resp2.HasMore {
		t.Errorf("expected HasMore = false after loading all remaining")
	}
}

func TestSessionFilteringAndSearch(t *testing.T) {
	m, sessID := buildSampleSession(t, 20)
	opts := model.CompareOptions{Mode: model.ModePositional}
	_, _, err := m.RunComparison(context.Background(), sessID, opts)
	if err != nil {
		t.Fatalf("comparison failed: %v", err)
	}

	// Filter by "added"
	addedResp, err := m.QueryResults(sessID, model.QueryResultsRequest{
		SessionID: sessID,
		Filter:    "added",
	})
	if err != nil {
		t.Fatalf("filter added error: %v", err)
	}
	if addedResp.TotalFiltered != 1 {
		t.Errorf("expected 1 added row, got %d", addedResp.TotalFiltered)
	}

	// Filter by search query "mod10"
	searchResp, err := m.QueryResults(sessID, model.QueryResultsRequest{
		SessionID: sessID,
		Search:    "mod10",
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if searchResp.TotalFiltered != 1 {
		t.Errorf("expected 1 row matching mod10, got %d", searchResp.TotalFiltered)
	}
}

func TestSessionCleanupExpired(t *testing.T) {
	m := NewManager(10 * time.Millisecond)
	sessID := "expired-test"
	m.CreateSession(sessID, nil, nil, model.FileMetadata{}, model.FileMetadata{})

	// Wait for TTL to pass
	time.Sleep(20 * time.Millisecond)
	m.CleanupExpired()

	_, err := m.GetSession(sessID)
	if err == nil {
		t.Errorf("expected error getting expired session, got nil")
	}
}
