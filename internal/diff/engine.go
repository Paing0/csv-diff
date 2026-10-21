package diff

import (
	"context"
	"fmt"
	"sort"

	"csv-diff/internal/csvparser"
	"csv-diff/internal/model"
)

// Engine performs CSV comparison according to configuration.
type Engine struct{}

// NewEngine creates a new comparison engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Compare executes comparison of original and modified CSVs.
func (e *Engine) Compare(ctx context.Context, orig, mod *csvparser.ParsedCSV, opts model.CompareOptions) (*model.CompareSummary, []model.RowDiff, error) {
	if orig == nil || mod == nil {
		return nil, nil, fmt.Errorf("both original and modified CSV files must be provided")
	}

	if opts.Mode == model.ModePrimaryKey {
		return e.comparePrimaryKey(ctx, orig, mod, opts)
	}

	return e.comparePositional(ctx, orig, mod, opts)
}

// isColumnEligible determines whether a column should be compared based on filter options.
func isColumnEligible(colName string, isKey bool, opts model.CompareOptions) bool {
	// If it's a key column and PK mode, check if explicitly excluded from value comparison
	filter := opts.ColumnFilter
	switch filter.Mode {
	case model.FilterInclude:
		for _, c := range filter.Columns {
			if c == colName {
				return true
			}
		}
		return false
	case model.FilterExclude:
		for _, c := range filter.Columns {
			if c == colName {
				return false
			}
		}
		return true
	case model.FilterAll:
		fallthrough
	default:
		return true
	}
}

// comparePositional compares CSV files by row position.
func (e *Engine) comparePositional(ctx context.Context, orig, mod *csvparser.ParsedCSV, opts model.CompareOptions) (*model.CompareSummary, []model.RowDiff, error) {
	// Determine columns for display and comparison
	unifiedCols := buildUnifiedColumnsPositional(orig, mod)
	if len(unifiedCols) == 0 {
		return nil, nil, fmt.Errorf("no columns available for comparison")
	}

	// Validate that at least one column is eligible
	hasEligibleCol := false
	for _, col := range unifiedCols {
		if isColumnEligible(col, false, opts) {
			hasEligibleCol = true
			break
		}
	}
	if !hasEligibleCol {
		return nil, nil, fmt.Errorf("no columns selected for comparison. At least one column must be compared")
	}

	summary := &model.CompareSummary{
		TotalOriginalRows: len(orig.Rows),
		TotalModifiedRows: len(mod.Rows),
		Columns:           unifiedCols,
	}

	var diffRows []model.RowDiff
	diffIndex := 1

	origCount := len(orig.Rows)
	modCount := len(mod.Rows)
	maxCount := origCount
	if modCount > maxCount {
		maxCount = modCount
	}

	for i := 0; i < maxCount; i++ {
		// Check for context cancellation
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			default:
			}
		}

		if i < origCount && i < modCount {
			// Row exists in both -> potential modification
			origRow := orig.Rows[i]
			modRow := mod.Rows[i]
			origRowNum := orig.RowStartLines[i]
			modRowNum := mod.RowStartLines[i]

			rowDiff, isDiff := compareRowCells(origRow, modRow, unifiedCols, opts, diffIndex, origRowNum, modRowNum, "")
			if isDiff {
				diffRows = append(diffRows, rowDiff)
				diffIndex++
				summary.ModifiedRows++
				summary.TotalChangedCells += rowDiff.ChangedCellCount
			} else {
				summary.UnchangedRows++
			}
		} else if i < origCount {
			// Row exists only in original -> removed
			origRow := orig.Rows[i]
			origRowNum := orig.RowStartLines[i]

			rowDiff := buildRemovedRow(origRow, unifiedCols, diffIndex, origRowNum, "")
			diffRows = append(diffRows, rowDiff)
			diffIndex++
			summary.RemovedRows++
		} else {
			// Row exists only in modified -> added
			modRow := mod.Rows[i]
			modRowNum := mod.RowStartLines[i]

			rowDiff := buildAddedRow(modRow, unifiedCols, diffIndex, modRowNum, "")
			diffRows = append(diffRows, rowDiff)
			diffIndex++
			summary.AddedRows++
		}
	}

	summary.TotalDifferingRows = len(diffRows)
	return summary, diffRows, nil
}

// comparePrimaryKey compares CSV files using a selected primary key column.
func (e *Engine) comparePrimaryKey(ctx context.Context, orig, mod *csvparser.ParsedCSV, opts model.CompareOptions) (*model.CompareSummary, []model.RowDiff, error) {
	if opts.PrimaryKeyCol == "" {
		return nil, nil, fmt.Errorf("primary key column must be specified")
	}

	origKeyIdx := findColumnIndex(orig.Header, opts.PrimaryKeyCol)
	if origKeyIdx == -1 {
		return nil, nil, fmt.Errorf("primary key column %q not found in original file", opts.PrimaryKeyCol)
	}

	modKeyIdx := findColumnIndex(mod.Header, opts.PrimaryKeyCol)
	if modKeyIdx == -1 {
		return nil, nil, fmt.Errorf("primary key column %q not found in modified file", opts.PrimaryKeyCol)
	}

	// Validate duplicate and empty primary keys
	origDupes, origEmpty := findKeyIssues(orig.Rows, orig.RowStartLines, origKeyIdx, "original")
	modDupes, modEmpty := findKeyIssues(mod.Rows, mod.RowStartLines, modKeyIdx, "modified")

	allDupes := append(origDupes, modDupes...)
	allEmpty := append(origEmpty, modEmpty...)

	summary := &model.CompareSummary{
		TotalOriginalRows: len(orig.Rows),
		TotalModifiedRows: len(mod.Rows),
		KeyColumn:         opts.PrimaryKeyCol,
		DuplicateKeys:     allDupes,
		EmptyKeys:         allEmpty,
	}

	if len(allDupes) > 0 || len(allEmpty) > 0 {
		return summary, nil, fmt.Errorf("primary key validation failed: duplicate or empty keys detected")
	}

	// Unified columns: PK column first, then remaining original columns, then modified-only columns
	unifiedCols := buildUnifiedColumnsPK(orig.Header, mod.Header, opts.PrimaryKeyCol)
	summary.Columns = unifiedCols

	// Validate that at least one column is eligible for comparison
	hasEligibleCol := false
	for _, col := range unifiedCols {
		isKey := col == opts.PrimaryKeyCol
		if isColumnEligible(col, isKey, opts) {
			hasEligibleCol = true
			break
		}
	}
	if !hasEligibleCol {
		return nil, nil, fmt.Errorf("no columns selected for comparison. At least one column must be compared")
	}

	// Index rows by primary key
	origKeyMap := make(map[string]int, len(orig.Rows))
	for i, row := range orig.Rows {
		if origKeyIdx < len(row) {
			origKeyMap[row[origKeyIdx]] = i
		}
	}

	modKeyMap := make(map[string]int, len(mod.Rows))
	for i, row := range mod.Rows {
		if modKeyIdx < len(row) {
			modKeyMap[row[modKeyIdx]] = i
		}
	}

	// Build column index maps for fast header-based cell lookup
	origColMap := make(map[string]int, len(orig.Header))
	for i, h := range orig.Header {
		origColMap[h] = i
	}
	modColMap := make(map[string]int, len(mod.Header))
	for i, h := range mod.Header {
		modColMap[h] = i
	}

	var diffRows []model.RowDiff
	diffIndex := 1

	// Track which modified keys were matched
	matchedModKeys := make(map[string]bool, len(mod.Rows))

	// 1. Process all original records in original order
	for i, origRow := range orig.Rows {
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			default:
			}
		}

		keyVal := ""
		if origKeyIdx < len(origRow) {
			keyVal = origRow[origKeyIdx]
		}
		origRowNum := orig.RowStartLines[i]

		modRowIdx, foundInMod := modKeyMap[keyVal]
		if foundInMod {
			matchedModKeys[keyVal] = true
			modRow := mod.Rows[modRowIdx]
			modRowNum := mod.RowStartLines[modRowIdx]

			rowDiff, isDiff := compareRowCellsPK(origRow, modRow, origColMap, modColMap, unifiedCols, opts, diffIndex, origRowNum, modRowNum, keyVal)
			if isDiff {
				diffRows = append(diffRows, rowDiff)
				diffIndex++
				summary.ModifiedRows++
				summary.TotalChangedCells += rowDiff.ChangedCellCount
			} else {
				summary.UnchangedRows++
			}
		} else {
			// Key exists only in original -> removed
			rowDiff := buildRemovedRowPK(origRow, origColMap, unifiedCols, diffIndex, origRowNum, keyVal, opts.PrimaryKeyCol)
			diffRows = append(diffRows, rowDiff)
			diffIndex++
			summary.RemovedRows++
		}
	}

	// 2. Process remaining modified records (added) in modified order
	for j, modRow := range mod.Rows {
		keyVal := ""
		if modKeyIdx < len(modRow) {
			keyVal = modRow[modKeyIdx]
		}

		if !matchedModKeys[keyVal] {
			modRowNum := mod.RowStartLines[j]
			rowDiff := buildAddedRowPK(modRow, modColMap, unifiedCols, diffIndex, modRowNum, keyVal, opts.PrimaryKeyCol)
			diffRows = append(diffRows, rowDiff)
			diffIndex++
			summary.AddedRows++
		}
	}

	summary.TotalDifferingRows = len(diffRows)
	return summary, diffRows, nil
}

func compareRowCells(origRow, modRow []string, columns []string, opts model.CompareOptions, diffIdx, origRowNum, modRowNum int, keyVal string) (model.RowDiff, bool) {
	rowDiff := model.RowDiff{
		DiffIndex:      diffIdx,
		OriginalRowNum: origRowNum,
		ModifiedRowNum: modRowNum,
		Status:         model.RowModified,
		Key:            keyVal,
		Cells:          make([]model.CellDiff, len(columns)),
	}

	hasAnyEligibleDiff := false
	changedCells := 0

	for colIdx, colName := range columns {
		var origValPtr *string
		if colIdx < len(origRow) {
			val := origRow[colIdx]
			origValPtr = &val
		}

		var modValPtr *string
		if colIdx < len(modRow) {
			val := modRow[colIdx]
			modValPtr = &val
		}

		isEligible := isColumnEligible(colName, false, opts)

		cellDiff := buildCellDiff(colName, colIdx, origValPtr, modValPtr, isEligible, false)
		rowDiff.Cells[colIdx] = cellDiff

		if cellDiff.Status == model.CellModified {
			if isEligible {
				hasAnyEligibleDiff = true
				changedCells++
			}
		}
	}

	rowDiff.ChangedCellCount = changedCells
	return rowDiff, hasAnyEligibleDiff
}

func compareRowCellsPK(origRow, modRow []string, origColMap, modColMap map[string]int, columns []string, opts model.CompareOptions, diffIdx, origRowNum, modRowNum int, keyVal string) (model.RowDiff, bool) {
	rowDiff := model.RowDiff{
		DiffIndex:      diffIdx,
		OriginalRowNum: origRowNum,
		ModifiedRowNum: modRowNum,
		Status:         model.RowModified,
		Key:            keyVal,
		Cells:          make([]model.CellDiff, len(columns)),
	}

	hasAnyEligibleDiff := false
	changedCells := 0

	for colIdx, colName := range columns {
		isKey := (colName == opts.PrimaryKeyCol)
		var origValPtr *string
		if idx, ok := origColMap[colName]; ok && idx < len(origRow) {
			val := origRow[idx]
			origValPtr = &val
		}

		var modValPtr *string
		if idx, ok := modColMap[colName]; ok && idx < len(modRow) {
			val := modRow[idx]
			modValPtr = &val
		}

		isEligible := isColumnEligible(colName, isKey, opts)
		// If key column is excluded from value comparison:
		if isKey && !isEligible {
			isEligible = false
		}

		cellDiff := buildCellDiff(colName, colIdx, origValPtr, modValPtr, isEligible, isKey)
		rowDiff.Cells[colIdx] = cellDiff

		if cellDiff.Status == model.CellModified {
			if isEligible {
				hasAnyEligibleDiff = true
				changedCells++
			}
		}
	}

	rowDiff.ChangedCellCount = changedCells
	return rowDiff, hasAnyEligibleDiff
}

func buildCellDiff(colName string, colIdx int, origVal, modVal *string, isEligible, isKey bool) model.CellDiff {
	cell := model.CellDiff{
		ColName:       colName,
		ColIndex:      colIdx,
		OriginalValue: origVal,
		ModifiedValue: modVal,
		IsKey:         isKey,
		IsIgnored:     !isEligible,
	}

	// Check missing vs present
	if origVal == nil && modVal == nil {
		cell.Status = model.CellUnchanged
		return cell
	}

	if origVal == nil {
		// Missing in original, present in modified
		if !isEligible {
			cell.Status = model.CellUnchanged
		} else {
			cell.Status = model.CellModified
			cell.ChunksModified = []model.DiffChunk{{Type: model.ChunkInsert, Text: *modVal}}
		}
		return cell
	}

	if modVal == nil {
		// Present in original, missing in modified
		if !isEligible {
			cell.Status = model.CellUnchanged
		} else {
			cell.Status = model.CellModified
			cell.ChunksOriginal = []model.DiffChunk{{Type: model.ChunkDelete, Text: *origVal}}
		}
		return cell
	}

	// Both present: exact comparison
	if *origVal == *modVal {
		cell.Status = model.CellUnchanged
		chunk := model.DiffChunk{Type: model.ChunkEqual, Text: *origVal}
		cell.ChunksOriginal = []model.DiffChunk{chunk}
		cell.ChunksModified = []model.DiffChunk{chunk}
		return cell
	}

	// Values differ
	if !isEligible {
		cell.Status = model.CellUnchanged
		cell.ChunksOriginal = []model.DiffChunk{{Type: model.ChunkEqual, Text: *origVal}}
		cell.ChunksModified = []model.DiffChunk{{Type: model.ChunkEqual, Text: *modVal}}
		return cell
	}

	cell.Status = model.CellModified
	origChunks, modChunks := DiffStrings(*origVal, *modVal)
	cell.ChunksOriginal = origChunks
	cell.ChunksModified = modChunks
	return cell
}

func buildRemovedRow(origRow []string, columns []string, diffIdx, origRowNum int, keyVal string) model.RowDiff {
	rowDiff := model.RowDiff{
		DiffIndex:      diffIdx,
		OriginalRowNum: origRowNum,
		Status:         model.RowRemoved,
		Key:            keyVal,
		Cells:          make([]model.CellDiff, len(columns)),
	}

	for colIdx, colName := range columns {
		var origValPtr *string
		if colIdx < len(origRow) {
			val := origRow[colIdx]
			origValPtr = &val
		}

		cell := model.CellDiff{
			ColName:       colName,
			ColIndex:      colIdx,
			OriginalValue: origValPtr,
			ModifiedValue: nil,
			Status:        model.CellRemoved,
		}
		if origValPtr != nil {
			cell.ChunksOriginal = []model.DiffChunk{{Type: model.ChunkDelete, Text: *origValPtr}}
		}
		rowDiff.Cells[colIdx] = cell
	}
	return rowDiff
}

func buildAddedRow(modRow []string, columns []string, diffIdx, modRowNum int, keyVal string) model.RowDiff {
	rowDiff := model.RowDiff{
		DiffIndex:      diffIdx,
		ModifiedRowNum: modRowNum,
		Status:         model.RowAdded,
		Key:            keyVal,
		Cells:          make([]model.CellDiff, len(columns)),
	}

	for colIdx, colName := range columns {
		var modValPtr *string
		if colIdx < len(modRow) {
			val := modRow[colIdx]
			modValPtr = &val
		}

		cell := model.CellDiff{
			ColName:       colName,
			ColIndex:      colIdx,
			OriginalValue: nil,
			ModifiedValue: modValPtr,
			Status:        model.CellAdded,
		}
		if modValPtr != nil {
			cell.ChunksModified = []model.DiffChunk{{Type: model.ChunkInsert, Text: *modValPtr}}
		}
		rowDiff.Cells[colIdx] = cell
	}
	return rowDiff
}

func buildRemovedRowPK(origRow []string, origColMap map[string]int, columns []string, diffIdx, origRowNum int, keyVal, keyCol string) model.RowDiff {
	rowDiff := model.RowDiff{
		DiffIndex:      diffIdx,
		OriginalRowNum: origRowNum,
		Status:         model.RowRemoved,
		Key:            keyVal,
		Cells:          make([]model.CellDiff, len(columns)),
	}

	for colIdx, colName := range columns {
		var origValPtr *string
		if idx, ok := origColMap[colName]; ok && idx < len(origRow) {
			val := origRow[idx]
			origValPtr = &val
		}

		cell := model.CellDiff{
			ColName:       colName,
			ColIndex:      colIdx,
			OriginalValue: origValPtr,
			ModifiedValue: nil,
			Status:        model.CellRemoved,
			IsKey:         colName == keyCol,
		}
		if origValPtr != nil {
			cell.ChunksOriginal = []model.DiffChunk{{Type: model.ChunkDelete, Text: *origValPtr}}
		}
		rowDiff.Cells[colIdx] = cell
	}
	return rowDiff
}

func buildAddedRowPK(modRow []string, modColMap map[string]int, columns []string, diffIdx, modRowNum int, keyVal, keyCol string) model.RowDiff {
	rowDiff := model.RowDiff{
		DiffIndex:      diffIdx,
		ModifiedRowNum: modRowNum,
		Status:         model.RowAdded,
		Key:            keyVal,
		Cells:          make([]model.CellDiff, len(columns)),
	}

	for colIdx, colName := range columns {
		var modValPtr *string
		if idx, ok := modColMap[colName]; ok && idx < len(modRow) {
			val := modRow[idx]
			modValPtr = &val
		}

		cell := model.CellDiff{
			ColName:       colName,
			ColIndex:      colIdx,
			OriginalValue: nil,
			ModifiedValue: modValPtr,
			Status:        model.CellAdded,
			IsKey:         colName == keyCol,
		}
		if modValPtr != nil {
			cell.ChunksModified = []model.DiffChunk{{Type: model.ChunkInsert, Text: *modValPtr}}
		}
		rowDiff.Cells[colIdx] = cell
	}
	return rowDiff
}

func buildUnifiedColumnsPositional(orig, mod *csvparser.ParsedCSV) []string {
	var cols []string
	origLen := len(orig.Header)
	modLen := len(mod.Header)

	maxLen := origLen
	if modLen > maxLen {
		maxLen = modLen
	}

	for i := 0; i < maxLen; i++ {
		if i < origLen && orig.Header[i] != "" {
			cols = append(cols, orig.Header[i])
		} else if i < modLen && mod.Header[i] != "" {
			cols = append(cols, mod.Header[i])
		} else {
			cols = append(cols, fmt.Sprintf("Column %d", i+1))
		}
	}
	return cols
}

func buildUnifiedColumnsPK(origHeader, modHeader []string, pkCol string) []string {
	var cols []string
	seen := make(map[string]bool)

	// PK column first
	cols = append(cols, pkCol)
	seen[pkCol] = true

	// Original columns
	for _, h := range origHeader {
		if !seen[h] {
			cols = append(cols, h)
			seen[h] = true
		}
	}

	// Modified columns not in original
	for _, h := range modHeader {
		if !seen[h] {
			cols = append(cols, h)
			seen[h] = true
		}
	}

	return cols
}

func findColumnIndex(headers []string, colName string) int {
	for i, h := range headers {
		if h == colName {
			return i
		}
	}
	return -1
}

func findKeyIssues(rows [][]string, lines []int, keyIdx int, source string) ([]model.DuplicateKeyInfo, []model.EmptyKeyInfo) {
	keyRowMap := make(map[string][]int)
	var emptyRows []int

	for i, row := range rows {
		lineNum := lines[i]
		if keyIdx >= len(row) || row[keyIdx] == "" {
			emptyRows = append(emptyRows, lineNum)
			continue
		}
		k := row[keyIdx]
		keyRowMap[k] = append(keyRowMap[k], lineNum)
	}

	var duplicates []model.DuplicateKeyInfo
	for k, rowNums := range keyRowMap {
		if len(rowNums) > 1 {
			duplicates = append(duplicates, model.DuplicateKeyInfo{
				Key:        k,
				FileSource: source,
				Rows:       rowNums,
			})
		}
	}

	// Sort duplicates for deterministic error output
	sort.Slice(duplicates, func(i, j int) bool {
		return duplicates[i].Key < duplicates[j].Key
	})

	var emptyKeys []model.EmptyKeyInfo
	if len(emptyRows) > 0 {
		emptyKeys = append(emptyKeys, model.EmptyKeyInfo{
			FileSource: source,
			Rows:       emptyRows,
		})
	}

	return duplicates, emptyKeys
}
