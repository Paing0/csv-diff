package diff

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"csv-diff/internal/csvparser"
	"csv-diff/internal/model"
)

func parseHelper(t *testing.T, content string) *csvparser.ParsedCSV {
	t.Helper()
	parsed, err := csvparser.Parse(strings.NewReader(content), csvparser.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse helper error: %v", err)
	}
	return parsed
}

func TestIdenticalFiles(t *testing.T) {
	csv := "id,name,role\n1,Alice,Dev\n2,Bob,Ops\n"
	orig := parseHelper(t, csv)
	mod := parseHelper(t, csv)

	engine := NewEngine()
	opts := model.CompareOptions{Mode: model.ModePositional}
	summary, diffs, err := engine.Compare(context.Background(), orig, mod, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.TotalDifferingRows != 0 {
		t.Errorf("expected 0 differing rows, got %d", summary.TotalDifferingRows)
	}
	if summary.UnchangedRows != 2 {
		t.Errorf("expected 2 unchanged rows, got %d", summary.UnchangedRows)
	}
	if len(diffs) != 0 {
		t.Errorf("expected empty diff slice, got %d", len(diffs))
	}
}

func TestPositionalModifiedAndMultipleCells(t *testing.T) {
	origCSV := "id,name,role,salary\n1,Alice,Dev,100\n2,Bob,Ops,200\n"
	modCSV := "id,name,role,salary\n1,Alice,Lead,150\n2,Bob,Ops,200\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{Mode: model.ModePositional}
	summary, diffs, err := engine.Compare(context.Background(), orig, mod, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.TotalDifferingRows != 1 {
		t.Fatalf("expected 1 differing row, got %d", summary.TotalDifferingRows)
	}
	if summary.ModifiedRows != 1 || summary.TotalChangedCells != 2 {
		t.Errorf("expected 1 modified row with 2 changed cells, got %d rows and %d cells",
			summary.ModifiedRows, summary.TotalChangedCells)
	}

	diffRow := diffs[0]
	if diffRow.Status != model.RowModified {
		t.Errorf("expected row status modified, got %s", diffRow.Status)
	}
	if diffRow.OriginalRowNum != 2 || diffRow.ModifiedRowNum != 2 {
		t.Errorf("expected row nums 2 and 2, got %d and %d", diffRow.OriginalRowNum, diffRow.ModifiedRowNum)
	}
	if diffRow.Cells[2].Status != model.CellModified || diffRow.Cells[3].Status != model.CellModified {
		t.Errorf("expected cells 2 and 3 modified, got %s and %s", diffRow.Cells[2].Status, diffRow.Cells[3].Status)
	}
}

func TestPositionalAddedAndRemovedRows(t *testing.T) {
	origCSV := "id,val\n1,A\n2,B\n3,C\n"
	modCSV := "id,val\n1,A\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{Mode: model.ModePositional}
	summary, diffs, err := engine.Compare(context.Background(), orig, mod, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.RemovedRows != 2 || summary.AddedRows != 0 {
		t.Errorf("expected 2 removed rows, got removed=%d added=%d", summary.RemovedRows, summary.AddedRows)
	}
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diff rows, got %d", len(diffs))
	}
	if diffs[0].Status != model.RowRemoved || diffs[1].Status != model.RowRemoved {
		t.Errorf("expected RowRemoved, got %s, %s", diffs[0].Status, diffs[1].Status)
	}

	// Reverse for added
	summary2, diffs2, err := engine.Compare(context.Background(), mod, orig, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary2.AddedRows != 2 || summary2.RemovedRows != 0 {
		t.Errorf("expected 2 added rows, got added=%d removed=%d", summary2.AddedRows, summary2.RemovedRows)
	}
	if diffs2[0].Status != model.RowAdded {
		t.Errorf("expected RowAdded, got %s", diffs2[0].Status)
	}
}

func TestColumnFilteringIncludeAndExclude(t *testing.T) {
	origCSV := "id,name,role\n1,Alice,Dev\n2,Bob,Ops\n"
	modCSV := "id,name,role\n1,Alice,Manager\n2,Bobby,Ops\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()

	// Test Exclude 'role': Row 1 only differs in 'role', so it should be UNCHANGED. Row 2 differs in 'name', so it should be MODIFIED.
	optsExclude := model.CompareOptions{
		Mode: model.ModePositional,
		ColumnFilter: model.ColumnFilter{
			Mode:    model.FilterExclude,
			Columns: []string{"role"},
		},
	}
	summaryEx, diffsEx, err := engine.Compare(context.Background(), orig, mod, optsExclude)
	if err != nil {
		t.Fatalf("exclude filter error: %v", err)
	}
	if summaryEx.TotalDifferingRows != 1 {
		t.Fatalf("expected 1 differing row when excluding role, got %d", summaryEx.TotalDifferingRows)
	}
	if diffsEx[0].OriginalRowNum != 3 { // Row 2 in CSV is line 3
		t.Errorf("expected line 3 to differ, got %d", diffsEx[0].OriginalRowNum)
	}

	// Test Include 'name': Row 1 does not differ in name. Row 2 does.
	optsInclude := model.CompareOptions{
		Mode: model.ModePositional,
		ColumnFilter: model.ColumnFilter{
			Mode:    model.FilterInclude,
			Columns: []string{"name"},
		},
	}
	summaryIn, diffsIn, err := engine.Compare(context.Background(), orig, mod, optsInclude)
	if err != nil {
		t.Fatalf("include filter error: %v", err)
	}
	if summaryIn.TotalDifferingRows != 1 || diffsIn[0].OriginalRowNum != 3 {
		t.Fatalf("expected 1 differing row (line 3) for include 'name', got %d", summaryIn.TotalDifferingRows)
	}
}

func TestMissingAndEmptyCells(t *testing.T) {
	// Empty cell "" vs Missing cell nil (different column count)
	origCSV := "c1,c2\na,\n"
	modCSV := "c1,c2\na\n" // row has only 1 field, c2 is missing
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{Mode: model.ModePositional}
	summary, diffs, err := engine.Compare(context.Background(), orig, mod, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty cell vs Missing cell MUST be treated as a difference according to requirements
	if summary.TotalDifferingRows != 1 {
		t.Fatalf("expected 1 differing row for empty vs missing cell, got %d", summary.TotalDifferingRows)
	}
	cell := diffs[0].Cells[1]
	if cell.OriginalValue == nil || *cell.OriginalValue != "" {
		t.Errorf("expected original empty string, got %v", cell.OriginalValue)
	}
	if cell.ModifiedValue != nil {
		t.Errorf("expected modified nil (missing), got %v", cell.ModifiedValue)
	}
}

func TestUnicodeCodePointDifferenceNFCvsNFD(t *testing.T) {
	// Visually identical: NFC "é" (\u00e9) vs NFD "e\u0301" (\u0065\u0301)
	// Requirement: "Visually identical Unicode strings may have different underlying code-point sequences. Detect those differences by default."
	origCSV := "id,text\n1,\u00e9\n"
	modCSV := "id,text\n1,\u0065\u0301\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{Mode: model.ModePositional}
	summary, diffs, err := engine.Compare(context.Background(), orig, mod, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.TotalDifferingRows != 1 {
		t.Errorf("expected diff between NFC and NFD strings, got %d diffs", summary.TotalDifferingRows)
	}
	if len(diffs) > 0 && diffs[0].Cells[1].Status != model.CellModified {
		t.Errorf("expected cell 1 modified for NFC/NFD diff")
	}
}

func TestPrimaryKeyMatchingAndReordering(t *testing.T) {
	// Records are in different physical row positions and column order
	origCSV := "emp_id,name,dept\n101,Alice,Engineering\n102,Bob,Marketing\n103,Charlie,Finance\n"
	modCSV := "dept,emp_id,name\nAccounting,103,Charlie\nMarketing,102,Bob\nEngineering,101,Alice\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{
		Mode:          model.ModePrimaryKey,
		PrimaryKeyCol: "emp_id",
	}

	summary, diffs, err := engine.Compare(context.Background(), orig, mod, opts)
	if err != nil {
		t.Fatalf("primary key comparison error: %v", err)
	}

	// 101: unchanged (Alice, Engineering)
	// 102: unchanged (Bob, Marketing)
	// 103: modified (dept changed from Finance to Accounting)
	// Row position difference and column order difference alone should NOT cause differences!
	if summary.TotalDifferingRows != 1 {
		t.Fatalf("expected 1 differing row, got %d", summary.TotalDifferingRows)
	}
	if summary.UnchangedRows != 2 {
		t.Errorf("expected 2 unchanged rows, got %d", summary.UnchangedRows)
	}
	if diffs[0].Key != "103" {
		t.Errorf("expected diff key '103', got %q", diffs[0].Key)
	}
}

func TestPrimaryKeyAddedAndRemoved(t *testing.T) {
	origCSV := "id,name\n1,Alice\n2,Bob\n"
	modCSV := "id,name\n2,Bob\n3,Charlie\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{
		Mode:          model.ModePrimaryKey,
		PrimaryKeyCol: "id",
	}

	summary, diffs, err := engine.Compare(context.Background(), orig, mod, opts)
	if err != nil {
		t.Fatalf("primary key error: %v", err)
	}

	if summary.RemovedRows != 1 || summary.AddedRows != 1 || summary.UnchangedRows != 1 {
		t.Errorf("summary mismatch: removed=%d, added=%d, unchanged=%d",
			summary.RemovedRows, summary.AddedRows, summary.UnchangedRows)
	}
	if diffs[0].Status != model.RowRemoved || diffs[0].Key != "1" {
		t.Errorf("expected first diff removed key 1, got %v", diffs[0])
	}
	if diffs[1].Status != model.RowAdded || diffs[1].Key != "3" {
		t.Errorf("expected second diff added key 3, got %v", diffs[1])
	}
}

func TestPrimaryKeyDuplicateKeysDetection(t *testing.T) {
	origCSV := "id,name\n1,Alice\n1,Bob\n" // Duplicate key '1'
	modCSV := "id,name\n1,Alice\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{
		Mode:          model.ModePrimaryKey,
		PrimaryKeyCol: "id",
	}

	summary, _, err := engine.Compare(context.Background(), orig, mod, opts)
	if err == nil {
		t.Fatalf("expected duplicate key error, got nil")
	}
	if len(summary.DuplicateKeys) == 0 {
		t.Errorf("expected duplicate keys reported in summary")
	}
	if summary.DuplicateKeys[0].Key != "1" || len(summary.DuplicateKeys[0].Rows) != 2 {
		t.Errorf("duplicate key details mismatch: %v", summary.DuplicateKeys)
	}
}

func TestPrimaryKeyEmptyKeysDetection(t *testing.T) {
	origCSV := "id,name\n1,Alice\n,Bob\n" // Empty key
	modCSV := "id,name\n1,Alice\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{
		Mode:          model.ModePrimaryKey,
		PrimaryKeyCol: "id",
	}

	summary, _, err := engine.Compare(context.Background(), orig, mod, opts)
	if err == nil {
		t.Fatalf("expected empty key error, got nil")
	}
	if len(summary.EmptyKeys) == 0 {
		t.Errorf("expected empty keys reported in summary")
	}
}

func TestPrimaryKeyExcludedFromValueComparison(t *testing.T) {
	// PK excluded from value comparison: matching still works
	origCSV := "id,name\n1,Alice\n"
	modCSV := "id,name\n1,Alice\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{
		Mode:          model.ModePrimaryKey,
		PrimaryKeyCol: "id",
		ColumnFilter: model.ColumnFilter{
			Mode:    model.FilterExclude,
			Columns: []string{"id"},
		},
	}

	summary, diffs, err := engine.Compare(context.Background(), orig, mod, opts)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if summary.TotalDifferingRows != 0 {
		t.Errorf("expected 0 differing rows, got %d", summary.TotalDifferingRows)
	}
	if len(diffs) != 0 {
		t.Errorf("expected no diffs, got %d", len(diffs))
	}
}

func TestContextCancellation(t *testing.T) {
	origCSV := "id,name\n1,A\n2,B\n"
	modCSV := "id,name\n1,A\n2,C\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	engine := NewEngine()
	opts := model.CompareOptions{Mode: model.ModePositional}
	_, _, err := engine.Compare(ctx, orig, mod, opts)
	if err == nil {
		t.Fatalf("expected error on cancelled context, got nil")
	}
}

func TestDiffMultilineAndQuotes(t *testing.T) {
	origCSV := "id,note\n1,\"Line 1\nLine 2\"\n2,\"He said \"\"Hello\"\"\"\n3,\"Comma, in, field\"\n"
	modCSV := "id,note\n1,\"Line 1\nLine Modified\"\n2,\"He said \"\"Hi\"\"\"\n3,\"Comma, in, field\"\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{Mode: model.ModePositional}
	summary, diffs, err := engine.Compare(context.Background(), orig, mod, opts)
	if err != nil {
		t.Fatalf("multiline diff error: %v", err)
	}

	if summary.TotalDifferingRows != 2 {
		t.Errorf("expected 2 differing rows, got %d", summary.TotalDifferingRows)
	}
	if len(diffs) != 2 {
		t.Errorf("expected 2 diff items, got %d", len(diffs))
	}
	if summary.UnchangedRows != 1 {
		t.Errorf("expected 1 unchanged row, got %d", summary.UnchangedRows)
	}
}

func TestPrimaryKeyMissingColumnsAcrossFiles(t *testing.T) {
	// Original has [id, name], Modified has [id, name, age] -> Must return error
	origCSV := "id,name\n1,Alice\n2,Bob\n"
	modCSV := "id,name,age\n1,Alice,30\n2,Bob,25\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{
		Mode:          model.ModePrimaryKey,
		PrimaryKeyCol: "id",
	}

	_, _, err := engine.Compare(context.Background(), orig, mod, opts)
	if err == nil {
		t.Fatalf("expected error when files have unequal header columns, got nil")
	}
	if !strings.Contains(err.Error(), "equal header column names") {
		t.Errorf("expected error message to mention equal header column names, got: %v", err)
	}
}

func TestPositionalHeaderMismatchError(t *testing.T) {
	origCSV := "id,name\n1,Alice\n"
	modCSV := "id,title\n1,Alice\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{Mode: model.ModePositional}
	_, _, err := engine.Compare(context.Background(), orig, mod, opts)
	if err == nil {
		t.Fatalf("expected error when positional headers do not match, got nil")
	}
	if !strings.Contains(err.Error(), "equal header column names") {
		t.Errorf("expected error message to mention equal header column names, got: %v", err)
	}
}

func TestAllColumnsExcludedValidationError(t *testing.T) {
	origCSV := "id,name\n1,Alice\n"
	modCSV := "id,name\n1,Bob\n"
	orig := parseHelper(t, origCSV)
	mod := parseHelper(t, modCSV)

	engine := NewEngine()
	opts := model.CompareOptions{
		Mode: model.ModePositional,
		ColumnFilter: model.ColumnFilter{
			Mode:    model.FilterExclude,
			Columns: []string{"id", "name"},
		},
	}

	_, _, err := engine.Compare(context.Background(), orig, mod, opts)
	if err == nil {
		t.Fatalf("expected validation error when all columns are excluded, got nil")
	}
}

func TestLargeCSVInputs(t *testing.T) {
	var origBuf, modBuf strings.Builder
	origBuf.WriteString("id,name,value\n")
	modBuf.WriteString("id,name,value\n")

	// 5,000 rows
	for i := 1; i <= 5000; i++ {
		origBuf.WriteString(fmt.Sprintf("%d,Name%d,Val%d\n", i, i, i))
		if i%10 == 0 {
			modBuf.WriteString(fmt.Sprintf("%d,Name%d,Changed%d\n", i, i, i))
		} else {
			modBuf.WriteString(fmt.Sprintf("%d,Name%d,Val%d\n", i, i, i))
		}
	}

	orig := parseHelper(t, origBuf.String())
	mod := parseHelper(t, modBuf.String())

	engine := NewEngine()
	opts := model.CompareOptions{Mode: model.ModePositional}
	summary, diffs, err := engine.Compare(context.Background(), orig, mod, opts)
	if err != nil {
		t.Fatalf("large input error: %v", err)
	}

	// Exactly 500 rows should differ (5000 / 10 = 500)
	if summary.TotalDifferingRows != 500 {
		t.Errorf("expected 500 differing rows, got %d", summary.TotalDifferingRows)
	}
	if summary.UnchangedRows != 4500 {
		t.Errorf("expected 4500 unchanged rows, got %d", summary.UnchangedRows)
	}
	if len(diffs) != 500 {
		t.Errorf("expected 500 diff rows in results, got %d", len(diffs))
	}
}
