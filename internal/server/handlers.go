package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"csv-diff/internal/csvparser"
	"csv-diff/internal/model"
	"csv-diff/internal/session"
)

type APIHandler struct {
	sessions        *session.Manager
	maxUploadSizeMB int64
}

func NewAPIHandler(sessions *session.Manager, maxUploadSizeMB int64) *APIHandler {
	if maxUploadSizeMB <= 0 {
		maxUploadSizeMB = 50 // Default 50 MB
	}
	return &APIHandler{
		sessions:        sessions,
		maxUploadSizeMB: maxUploadSizeMB,
	}
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string, extra any) {
	payload := map[string]any{
		"error": message,
	}
	if extra != nil {
		payload["details"] = extra
	}
	writeJSON(w, status, payload)
}

// HandleUpload accepts original and modified CSV files, validates them, and creates a session.
func (h *APIHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	maxBytes := h.maxUploadSizeMB * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("File upload exceeds maximum limit of %d MB or is invalid", h.maxUploadSizeMB), nil)
		return
	}

	hasHeader := true
	if r.FormValue("has_header") == "false" {
		hasHeader = false
	}

	// 1. Original File
	origFile, origHdr, err := r.FormFile("original_file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Original CSV file is required ('original_file')", nil)
		return
	}
	defer origFile.Close()

	// 2. Modified File
	modFile, modHdr, err := r.FormFile("modified_file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Modified CSV file is required ('modified_file')", nil)
		return
	}
	defer modFile.Close()

	// Parse options
	opts := csvparser.DefaultParseOptions()
	opts.HasHeader = hasHeader

	origParsed, err := csvparser.Parse(origFile, opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse Original CSV: %v", err), nil)
		return
	}

	modParsed, err := csvparser.Parse(modFile, opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse Modified CSV: %v", err), nil)
		return
	}

	// Build metadata
	origMeta := model.FileMetadata{
		FileID:      generateID(),
		FileName:    origHdr.Filename,
		FileSize:    origHdr.Size,
		Header:      origParsed.Header,
		RowCount:    origParsed.TotalRecordCount,
		ColumnCount: len(origParsed.Header),
		HasHeader:   hasHeader,
	}

	modMeta := model.FileMetadata{
		FileID:      generateID(),
		FileName:    modHdr.Filename,
		FileSize:    modHdr.Size,
		Header:      modParsed.Header,
		RowCount:    modParsed.TotalRecordCount,
		ColumnCount: len(modParsed.Header),
		HasHeader:   hasHeader,
	}

	// Calculate common and union columns
	commonCols, allCols := calculateColumnLists(origParsed.Header, modParsed.Header)

	sessionID := generateID()
	h.sessions.CreateSession(sessionID, origParsed, modParsed, origMeta, modMeta)

	resp := model.UploadResponse{
		SessionID:  sessionID,
		Original:   origMeta,
		Modified:   modMeta,
		CommonCols: commonCols,
		AllCols:    allCols,
	}

	writeJSON(w, http.StatusOK, resp)
}

func calculateColumnLists(origCols, modCols []string) ([]string, []string) {
	origMap := make(map[string]bool)
	for _, c := range origCols {
		origMap[c] = true
	}

	var common []string
	seenAll := make(map[string]bool)
	var allCols []string

	for _, c := range origCols {
		if !seenAll[c] {
			allCols = append(allCols, c)
			seenAll[c] = true
		}
	}

	for _, c := range modCols {
		if origMap[c] {
			common = append(common, c)
		}
		if !seenAll[c] {
			allCols = append(allCols, c)
			seenAll[c] = true
		}
	}

	return common, allCols
}

// HandleCompare executes comparison and returns summary + first page of up to 100 differing rows.
func (h *APIHandler) HandleCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	var opts model.CompareOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request body", nil)
		return
	}

	if opts.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required", nil)
		return
	}

	// Validate primary key requirement
	if opts.Mode == model.ModePrimaryKey && strings.TrimSpace(opts.PrimaryKeyCol) == "" {
		writeError(w, http.StatusBadRequest, "primary_key_col is required when comparison mode is primary_key", nil)
		return
	}

	summary, diffRows, err := h.sessions.RunComparison(r.Context(), opts.SessionID, opts)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   err.Error(),
			"summary": summary,
		})
		return
	}

	// Return up to first 100 rows
	initialLimit := 100
	var initialRows []model.RowDiff
	hasMore := false

	if len(diffRows) > initialLimit {
		initialRows = diffRows[:initialLimit]
		hasMore = true
	} else {
		initialRows = diffRows
		hasMore = false
	}

	resp := model.CompareResponse{
		SessionID: opts.SessionID,
		Summary:   *summary,
		Rows:      initialRows,
		Limit:     initialLimit,
		Offset:    0,
		HasMore:   hasMore,
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleResults handles pagination and filtering of cached comparison results.
func (h *APIHandler) HandleResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	var req model.QueryResultsRequest

	if r.Method == http.MethodGet {
		q := r.URL.Query()
		req.SessionID = q.Get("session_id")
		req.Filter = q.Get("filter")
		req.Search = q.Get("search")

		if offStr := q.Get("offset"); offStr != "" {
			if off, err := strconv.Atoi(offStr); err == nil {
				req.Offset = off
			}
		}

		if limStr := q.Get("limit"); limStr != "" {
			if lim, err := strconv.Atoi(limStr); err == nil {
				req.Limit = lim
			}
		} else {
			req.Limit = 100 // default
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid JSON payload", nil)
			return
		}
	}

	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id parameter is required", nil)
		return
	}

	resp, err := h.sessions.QueryResults(req.SessionID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleCancel cancels any active comparison in the session.
func (h *APIHandler) HandleCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	var payload struct {
		SessionID string `json:"session_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)

	if payload.SessionID != "" {
		_ = h.sessions.CancelComparison(payload.SessionID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// HandleHealth provides a simple health check endpoint.
func (h *APIHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
