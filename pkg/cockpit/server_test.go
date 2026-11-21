package cockpit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexandrmotologa/tablerip/pkg/grid"
	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

func TestCockpitServerEndpoints(t *testing.T) {
	doc := &pdf.Document{
		Path:     "test.pdf",
		NumPages: 1,
		Pages: []*pdf.PageData{
			{PageNum: 1, Width: 612, Height: 792},
		},
	}

	table := &grid.Table{
		ID:    "tbl_001",
		Title: "Test Operations",
		Columns: []*grid.Column{
			{Index: 0, Name: "Line Item"},
			{Index: 1, Name: "2025"},
		},
		Rows: []*grid.Row{
			{
				Index: 0,
				Level: 0,
				Label: "Revenue",
				Cells: []*grid.Cell{
					{ColIndex: 0, Text: "Revenue"},
					{ColIndex: 1, RawText: "5000", Value: 5000.0, IsNumeric: true},
				},
			},
		},
		Pages: []int{1},
	}

	server, err := NewServer(doc, []*grid.Table{table}, nil, 8787)
	if err != nil {
		t.Fatalf("failed to create cockpit server: %v", err)
	}
	defer server.Close()

	// Test 1: GET /api/document
	reqDoc := httptest.NewRequest("GET", "/api/document", nil)
	wDoc := httptest.NewRecorder()
	server.handleDocument(wDoc, reqDoc)
	if wDoc.Code != http.StatusOK {
		t.Errorf("expected 200 OK from /api/document, got %d", wDoc.Code)
	}

	// Test 2: GET /api/tables
	reqTbl := httptest.NewRequest("GET", "/api/tables", nil)
	wTbl := httptest.NewRecorder()
	server.handleTables(wTbl, reqTbl)
	if wTbl.Code != http.StatusOK {
		t.Errorf("expected 200 OK from /api/tables, got %d", wTbl.Code)
	}

	// Test 3: POST /api/query
	queryBody, _ := json.Marshal(QueryRequest{SQL: "SELECT row_label, col_2025 FROM tbl_001;"})
	reqSQL := httptest.NewRequest("POST", "/api/query", bytes.NewReader(queryBody))
	wSQL := httptest.NewRecorder()
	server.handleSQLQuery(wSQL, reqSQL)
	if wSQL.Code != http.StatusOK {
		t.Errorf("expected 200 OK from /api/query, got %d", wSQL.Code)
	}

	var resp QueryResponse
	if err := json.NewDecoder(wSQL.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode query response: %v", err)
	}
	if len(resp.Rows) != 1 {
		t.Fatalf("expected 1 row from SQL query, got %d", len(resp.Rows))
	}
	if resp.Rows[0][0] != "Revenue" {
		t.Errorf("expected row_label 'Revenue', got '%v'", resp.Rows[0][0])
	}
}
