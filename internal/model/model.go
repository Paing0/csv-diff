package model

// DiffChunkType defines whether a chunk of text is unchanged, inserted, or deleted.
type DiffChunkType string

const (
	ChunkEqual  DiffChunkType = "equal"
	ChunkInsert DiffChunkType = "insert"
	ChunkDelete DiffChunkType = "delete"
)

// DiffChunk represents a segment of text in fine-grained character/grapheme diffing.
type DiffChunk struct {
	Type DiffChunkType `json:"type"`
	Text string        `json:"text"`
}

// CellStatus represents the difference state of a single cell.
type CellStatus string

const (
	CellUnchanged CellStatus = "unchanged"
	CellModified  CellStatus = "modified"
	CellAdded     CellStatus = "added"
	CellRemoved   CellStatus = "removed"
	CellMissing   CellStatus = "missing"
)

// CellDiff represents the comparison result of a single cell in a row.
type CellDiff struct {
	ColName        string      `json:"col_name"`
	ColIndex       int         `json:"col_index"`
	OriginalValue  *string     `json:"original_value"` // nil if cell is missing
	ModifiedValue  *string     `json:"modified_value"` // nil if cell is missing
	Status         CellStatus  `json:"status"`
	ChunksOriginal []DiffChunk `json:"chunks_original,omitempty"`
	ChunksModified []DiffChunk `json:"chunks_modified,omitempty"`
	IsKey          bool        `json:"is_key,omitempty"`
	IsIgnored      bool        `json:"is_ignored,omitempty"`
}

// RowStatus represents whether a row was modified, added, or removed.
type RowStatus string

const (
	RowModified RowStatus = "modified"
	RowAdded    RowStatus = "added"
	RowRemoved  RowStatus = "removed"
)

// RowDiff represents a row that has differences.
type RowDiff struct {
	DiffIndex        int        `json:"diff_index"`                 // 1-based index among differing rows
	OriginalRowNum   int        `json:"original_row_num,omitempty"` // 1-based CSV line number (0 if added)
	ModifiedRowNum   int        `json:"modified_row_num,omitempty"` // 1-based CSV line number (0 if removed)
	Status           RowStatus  `json:"status"`
	Key              string     `json:"key,omitempty"` // Primary key value if in PK mode
	Cells            []CellDiff `json:"cells"`
	ChangedCellCount int        `json:"changed_cell_count"`
}

// ComparisonMode specifies whether comparison is positional or by primary key.
type ComparisonMode string

const (
	ModePositional ComparisonMode = "positional"
	ModePrimaryKey ComparisonMode = "primary_key"
)

// ColumnFilterMode specifies how columns are filtered.
type ColumnFilterMode string

const (
	FilterAll     ColumnFilterMode = "all"
	FilterInclude ColumnFilterMode = "include"
	FilterExclude ColumnFilterMode = "exclude"
)

// ColumnFilter holds user configuration for included/excluded columns.
type ColumnFilter struct {
	Mode    ColumnFilterMode `json:"mode"`
	Columns []string         `json:"columns"`
}

// CompareOptions contains all user-configurable comparison options.
type CompareOptions struct {
	SessionID     string         `json:"session_id"`
	Mode          ComparisonMode `json:"mode"`
	PrimaryKeyCol string         `json:"primary_key_col,omitempty"`
	ColumnFilter  ColumnFilter   `json:"column_filter"`
	HasHeader     bool           `json:"has_header"`
}

// DuplicateKeyInfo describes duplicate primary keys found during validation.
type DuplicateKeyInfo struct {
	Key        string `json:"key"`
	FileSource string `json:"file_source"` // "original" or "modified"
	Rows       []int  `json:"rows"`        // 1-based row numbers
}

// EmptyKeyInfo describes rows with empty primary key values.
type EmptyKeyInfo struct {
	FileSource string `json:"file_source"`
	Rows       []int  `json:"rows"`
}

// CompareSummary provides high-level statistics for the comparison.
type CompareSummary struct {
	TotalOriginalRows  int                `json:"total_original_rows"`
	TotalModifiedRows  int                `json:"total_modified_rows"`
	TotalDifferingRows int                `json:"total_differing_rows"`
	ModifiedRows       int                `json:"modified_rows"`
	AddedRows          int                `json:"added_rows"`
	RemovedRows        int                `json:"removed_rows"`
	UnchangedRows      int                `json:"unchanged_rows"`
	TotalChangedCells  int                `json:"total_changed_cells"`
	Columns            []string           `json:"columns"`
	KeyColumn          string             `json:"key_column,omitempty"`
	DuplicateKeys      []DuplicateKeyInfo `json:"duplicate_keys,omitempty"`
	EmptyKeys          []EmptyKeyInfo     `json:"empty_keys,omitempty"`
	Warnings           []string           `json:"warnings,omitempty"`
}

// FileMetadata stores inspected details of an uploaded CSV file.
type FileMetadata struct {
	FileID      string     `json:"file_id"`
	FileName    string     `json:"file_name"`
	FileSize    int64      `json:"file_size"`
	Header      []string   `json:"header"`
	RowCount    int        `json:"row_count"`
	ColumnCount int        `json:"column_count"`
	HasHeader   bool       `json:"has_header"`
	SampleRows  [][]string `json:"sample_rows,omitempty"`
}

// UploadResponse is returned after uploading and inspecting both CSV files.
type UploadResponse struct {
	SessionID  string       `json:"session_id"`
	Original   FileMetadata `json:"original"`
	Modified   FileMetadata `json:"modified"`
	CommonCols []string     `json:"common_columns"`
	AllCols    []string     `json:"all_columns"`
}

// CompareResponse is returned upon initiating or finishing comparison.
type CompareResponse struct {
	SessionID string         `json:"session_id"`
	Summary   CompareSummary `json:"summary"`
	Rows      []RowDiff      `json:"rows"` // First page (up to 100 differing rows)
	Limit     int            `json:"limit"`
	Offset    int            `json:"offset"`
	HasMore   bool           `json:"has_more"`
}

// QueryResultsRequest is used to paginate or filter existing comparison results.
type QueryResultsRequest struct {
	SessionID string `json:"session_id"`
	Offset    int    `json:"offset"`
	Limit     int    `json:"limit"`  // 0 or negative means all
	Filter    string `json:"filter"` // "all", "modified", "added", "removed"
	Search    string `json:"search"` // substring search across cell values
}

// QueryResultsResponse returns paginated results from a stored comparison.
type QueryResultsResponse struct {
	SessionID     string    `json:"session_id"`
	TotalFiltered int       `json:"total_filtered"`
	Offset        int       `json:"offset"`
	Limit         int       `json:"limit"`
	HasMore       bool      `json:"has_more"`
	Rows          []RowDiff `json:"rows"`
}
