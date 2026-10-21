package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"csv-diff/internal/model"
	"csv-diff/internal/session"
)

func createTestAPI(maxMB int64) *APIHandler {
	sessions := session.NewManager(1 * time.Hour)
	return NewAPIHandler(sessions, maxMB)
}

func createMultipartRequest(t *testing.T, origCSV, modCSV string, hasHeader bool) (*http.Request, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Original
	if origCSV != "" {
		part, err := writer.CreateFormFile("original_file", "original.csv")
		if err != nil {
			t.Fatalf("failed to create original part: %v", err)
		}
		if _, err := io.WriteString(part, origCSV); err != nil {
			t.Fatalf("failed to write original: %v", err)
		}
	}

	// Modified
	if modCSV != "" {
		part, err := writer.CreateFormFile("modified_file", "modified.csv")
		if err != nil {
			t.Fatalf("failed to create modified part: %v", err)
		}
		if _, err := io.WriteString(part, modCSV); err != nil {
			t.Fatalf("failed to write modified: %v", err)
		}
	}

	if !hasHeader {
		_ = writer.WriteField("has_header", "false")
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, writer.FormDataContentType()
}

func TestUploadValidFiles(t *testing.T) {
	api := createTestAPI(10)
	origCSV := "id,name\n1,Alice\n2,Bob\n"
	modCSV := "id,name\n1,Alice\n2,Robert\n"

	req, _ := createMultipartRequest(t, origCSV, modCSV, true)
	w := httptest.NewRecorder()
	api.HandleUpload(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var data model.UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}

	if data.SessionID == "" {
		t.Errorf("expected non-empty session ID")
	}
	if data.Original.RowCount != 2 || data.Modified.RowCount != 2 {
		t.Errorf("row counts mismatch: orig=%d, mod=%d", data.Original.RowCount, data.Modified.RowCount)
	}
	if len(data.CommonCols) != 2 || data.CommonCols[0] != "id" || data.CommonCols[1] != "name" {
		t.Errorf("common columns mismatch: %v", data.CommonCols)
	}
}

func TestUploadMissingFile(t *testing.T) {
	api := createTestAPI(10)
	origCSV := "id,name\n1,Alice\n"

	// Modified file missing
	req, _ := createMultipartRequest(t, origCSV, "", true)
	w := httptest.NewRecorder()
	api.HandleUpload(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400 for missing file, got %d", resp.StatusCode)
	}
}

func TestUploadOversizedFile(t *testing.T) {
	// 1 MB limit
	api := createTestAPI(1)

	// Create 2MB content
	largeContent := strings.Repeat("1,Alice,Engineer,Software,Department,Team\n", 50000)
	req, _ := createMultipartRequest(t, largeContent, largeContent, true)
	w := httptest.NewRecorder()
	api.HandleUpload(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 bad request on oversized upload, got %d", resp.StatusCode)
	}
}

func TestComparisonAndPaginationFlow(t *testing.T) {
	api := createTestAPI(10)

	// Create 150 rows in original and modified with differences
	var origBuf, modBuf strings.Builder
	origBuf.WriteString("id,value\n")
	modBuf.WriteString("id,value\n")

	for i := 1; i <= 150; i++ {
		origBuf.WriteString(fmt.Sprintf("%d,valA_%d\n", i, i))
		modBuf.WriteString(fmt.Sprintf("%d,valB_%d\n", i, i)) // all 150 modified
	}

	upReq, _ := createMultipartRequest(t, origBuf.String(), modBuf.String(), true)
	upRec := httptest.NewRecorder()
	api.HandleUpload(upRec, upReq)

	var upData model.UploadResponse
	_ = json.NewDecoder(upRec.Body).Decode(&upData)

	sessID := upData.SessionID

	// Compare
	cmpPayload := model.CompareOptions{
		SessionID: sessID,
		Mode:      model.ModePositional,
	}
	body, _ := json.Marshal(cmpPayload)
	cmpReq := httptest.NewRequest(http.MethodPost, "/api/compare", bytes.NewReader(body))
	cmpRec := httptest.NewRecorder()
	api.HandleCompare(cmpRec, cmpReq)

	if cmpRec.Code != http.StatusOK {
		t.Fatalf("compare failed: %s", cmpRec.Body.String())
	}

	var cmpData model.CompareResponse
	_ = json.NewDecoder(cmpRec.Body).Decode(&cmpData)

	if cmpData.Summary.TotalDifferingRows != 150 {
		t.Errorf("expected 150 differing rows, got %d", cmpData.Summary.TotalDifferingRows)
	}
	// Verify initial 100 row limit
	if len(cmpData.Rows) != 100 {
		t.Errorf("expected initial 100 rows, got %d", len(cmpData.Rows))
	}
	if !cmpData.HasMore {
		t.Errorf("expected HasMore to be true")
	}

	// Fetch remaining page via GET /api/results
	resReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/results?session_id=%s&offset=100&limit=100", sessID), nil)
	resRec := httptest.NewRecorder()
	api.HandleResults(resRec, resReq)

	if resRec.Code != http.StatusOK {
		t.Fatalf("results fetch failed: %s", resRec.Body.String())
	}

	var resData model.QueryResultsResponse
	_ = json.NewDecoder(resRec.Body).Decode(&resData)

	if len(resData.Rows) != 50 {
		t.Errorf("expected 50 remaining rows, got %d", len(resData.Rows))
	}
	if resData.HasMore {
		t.Errorf("expected HasMore to be false on last page")
	}
}

func TestInvalidPrimaryKeySelection(t *testing.T) {
	api := createTestAPI(10)
	origCSV := "id,name\n1,Alice\n"
	modCSV := "id,name\n1,Bob\n"

	upReq, _ := createMultipartRequest(t, origCSV, modCSV, true)
	upRec := httptest.NewRecorder()
	api.HandleUpload(upRec, upReq)

	var upData model.UploadResponse
	_ = json.NewDecoder(upRec.Body).Decode(&upData)

	// PK column 'non_existent'
	cmpPayload := model.CompareOptions{
		SessionID:     upData.SessionID,
		Mode:          model.ModePrimaryKey,
		PrimaryKeyCol: "non_existent",
	}
	body, _ := json.Marshal(cmpPayload)
	cmpReq := httptest.NewRequest(http.MethodPost, "/api/compare", bytes.NewReader(body))
	cmpRec := httptest.NewRecorder()
	api.HandleCompare(cmpRec, cmpReq)

	if cmpRec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on invalid PK column, got %d", cmpRec.Code)
	}
}

func TestCancelAndHealthEndpoints(t *testing.T) {
	api := createTestAPI(10)

	// Health
	hReq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	hRec := httptest.NewRecorder()
	api.HandleHealth(hRec, hReq)
	if hRec.Code != http.StatusOK {
		t.Errorf("health returned status %d", hRec.Code)
	}

	// Cancel
	cReq := httptest.NewRequest(http.MethodPost, "/api/cancel", strings.NewReader(`{"session_id":"test"}`))
	cRec := httptest.NewRecorder()
	api.HandleCancel(cRec, cReq)
	if cRec.Code != http.StatusOK {
		t.Errorf("cancel returned status %d", cRec.Code)
	}
}
