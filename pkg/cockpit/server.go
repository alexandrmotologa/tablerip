package cockpit

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/auditor"
	"github.com/alexandrmotologa/tablerip/pkg/exporter"
	"github.com/alexandrmotologa/tablerip/pkg/grid"
	"github.com/alexandrmotologa/tablerip/pkg/pdf"
	_ "modernc.org/sqlite"
)

//go:embed web/*
var webFS embed.FS

// Server hosts the embedded Cockpit web interface and interactive query API.
type Server struct {
	doc     *pdf.Document
	tables  []*grid.Table
	reports []*auditor.AuditReport
	db      *sql.DB
	port    int
	server  *http.Server
}

// NewServer initializes the Cockpit server with extracted tables and in-memory database.
func NewServer(doc *pdf.Document, tables []*grid.Table, reports []*auditor.AuditReport, port int) (*Server, error) {
	if port <= 0 {
		port = 8787
	}

	// Initialize in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize in-memory SQLite db: %w", err)
	}

	sqliteExp := exporter.NewSQLiteExporter()
	for _, t := range tables {
		if err := sqliteExp.ExportTableToDB(t, db, ""); err != nil {
			return nil, fmt.Errorf("failed to populate in-memory table '%s': %w", t.ID, err)
		}
	}

	return &Server{
		doc:     doc,
		tables:  tables,
		reports: reports,
		db:      db,
		port:    port,
	}, nil
}

// Start launches the HTTP service.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Static web assets from embed.FS
	subFS, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("failed to sub embed fs: %w", err)
	}
	fileServer := http.FileServer(http.FS(subFS))
	mux.Handle("/", fileServer)

	// API endpoints
	mux.HandleFunc("/api/document", s.handleDocument)
	mux.HandleFunc("/api/tables", s.handleTables)
	mux.HandleFunc("/api/query", s.handleSQLQuery)
	mux.HandleFunc("/api/export", s.handleExport)
	mux.HandleFunc("/api/cell/update", s.handleCellUpdate)

	addr := fmt.Sprintf(":%d", s.port)
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s.server.ListenAndServe()
}

// Close terminates the HTTP server and database.
func (s *Server) Close() error {
	if s.server != nil {
		s.server.Close()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Server) handleDocument(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.doc == nil {
		http.Error(w, `{"error": "no document loaded"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(s.doc)
}

func (s *Server) handleTables(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	reportMap := make(map[string]*auditor.AuditReport)
	for _, rep := range s.reports {
		if rep != nil {
			reportMap[rep.TableID] = rep
		}
	}

	type TableExportItem struct {
		Table       *grid.Table          `json:"table"`
		AuditReport *auditor.AuditReport `json:"audit_report,omitempty"`
	}

	var items []TableExportItem
	for _, t := range s.tables {
		items = append(items, TableExportItem{
			Table:       t,
			AuditReport: reportMap[t.ID],
		})
	}

	json.NewEncoder(w).Encode(items)
}

type QueryRequest struct {
	SQL string `json:"sql"`
}

type QueryResponse struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
	Error   string   `json:"error,omitempty"`
}

func (s *Server) handleSQLQuery(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(QueryResponse{Error: "invalid JSON body"})
		return
	}

	cleanedSQL := strings.TrimSpace(req.SQL)
	if cleanedSQL == "" {
		json.NewEncoder(w).Encode(QueryResponse{Error: "empty SQL query"})
		return
	}

	rows, err := s.db.Query(cleanedSQL)
	if err != nil {
		json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
		return
	}

	var resultRows [][]any
	for rows.Next() {
		colValues := make([]any, len(cols))
		colPointers := make([]any, len(cols))
		for i := range colValues {
			colPointers[i] = &colValues[i]
		}

		if err := rows.Scan(colPointers...); err != nil {
			json.NewEncoder(w).Encode(QueryResponse{Error: err.Error()})
			return
		}

		// Convert bytes to string for clean JSON display
		rowCopy := make([]any, len(cols))
		for i, val := range colValues {
			if b, ok := val.([]byte); ok {
				rowCopy[i] = string(b)
			} else {
				rowCopy[i] = val
			}
		}

		resultRows = append(resultRows, rowCopy)
	}

	json.NewEncoder(w).Encode(QueryResponse{
		Columns: cols,
		Rows:    resultRows,
	})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	tableID := r.URL.Query().Get("table")
	format := strings.ToLower(r.URL.Query().Get("format"))

	var targetTable *grid.Table
	for _, t := range s.tables {
		if t.ID == tableID {
			targetTable = t
			break
		}
	}

	if targetTable == nil && len(s.tables) > 0 {
		targetTable = s.tables[0]
	}

	if targetTable == nil {
		http.Error(w, "no table available to export", http.StatusNotFound)
		return
	}

	tmpDir := os.TempDir()

	switch format {
	case "parquet":
		outPath := filepath.Join(tmpDir, targetTable.ID+".parquet")
		exp := exporter.NewParquetExporter()
		if err := exp.ExportTable(targetTable, outPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.parquet\"", targetTable.ID))
		w.Header().Set("Content-Type", "application/octet-stream")
		serveFileAndDelete(w, outPath)

	case "sqlite":
		outPath := filepath.Join(tmpDir, targetTable.ID+".sqlite")
		exp := exporter.NewSQLiteExporter()
		if err := exp.ExportTable(targetTable, outPath, ""); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.sqlite\"", targetTable.ID))
		w.Header().Set("Content-Type", "application/x-sqlite3")
		serveFileAndDelete(w, outPath)

	case "json":
		outPath := filepath.Join(tmpDir, targetTable.ID+".json")
		exp := exporter.NewJSONExporter()
		if err := exp.Export(s.doc.Path, []*grid.Table{targetTable}, s.reports, outPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.json\"", targetTable.ID))
		w.Header().Set("Content-Type", "application/json")
		serveFileAndDelete(w, outPath)

	case "csv":
		outPath := filepath.Join(tmpDir, targetTable.ID+".csv")
		exp := exporter.NewCSVExporter()
		if err := exp.ExportTable(targetTable, outPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.csv\"", targetTable.ID))
		w.Header().Set("Content-Type", "text/csv")
		serveFileAndDelete(w, outPath)

	default:
		http.Error(w, "unsupported export format (use parquet, sqlite, json, or csv)", http.StatusBadRequest)
	}
}

func serveFileAndDelete(w http.ResponseWriter, filePath string) {
	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	defer os.Remove(filePath)

	io.Copy(w, f)
}

// CellUpdateRequest represents payload for live in-browser cell edits.
type CellUpdateRequest struct {
	TableID  string `json:"table_id"`
	RowIndex int    `json:"row_index"`
	ColIndex int    `json:"col_index"`
	RawText  string `json:"raw_text"`
}

// CellUpdateResponse represents the response containing re-audited table state.
type CellUpdateResponse struct {
	Success     bool                 `json:"success"`
	Error       string               `json:"error,omitempty"`
	Table       *grid.Table          `json:"table,omitempty"`
	AuditReport *auditor.AuditReport `json:"audit_report,omitempty"`
}

func (s *Server) handleCellUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req CellUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(CellUpdateResponse{Success: false, Error: "invalid JSON body"})
		return
	}

	var targetTable *grid.Table
	for _, t := range s.tables {
		if t.ID == req.TableID {
			targetTable = t
			break
		}
	}
	if targetTable == nil {
		json.NewEncoder(w).Encode(CellUpdateResponse{Success: false, Error: "table not found"})
		return
	}

	// Update cell in table data structure
	for _, row := range targetTable.Rows {
		if row.Index == req.RowIndex {
			if req.ColIndex == 0 {
				row.Label = req.RawText
			}
			for _, c := range row.Cells {
				if c.ColIndex == req.ColIndex {
					c.RawText = req.RawText
					if numVal, isNum, _ := auditor.ParseFinancialNumber(req.RawText); isNum {
						c.Value = numVal
						c.IsNumeric = true
					} else {
						c.Value = 0
						c.IsNumeric = false
					}
					break
				}
			}
			break
		}
	}

	// Re-run arithmetic audit
	aud := auditor.NewAuditor(0.01, true)
	newReport := aud.AuditTable(targetTable)

	// Replace report in memory
	foundReport := false
	for i, rep := range s.reports {
		if rep != nil && rep.TableID == targetTable.ID {
			s.reports[i] = newReport
			foundReport = true
			break
		}
	}
	if !foundReport {
		s.reports = append(s.reports, newReport)
	}

	// Re-sync SQLite in-memory table
	if s.db != nil {
		tblName := targetTable.ID
		_, _ = s.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s;", tblName))
		_, _ = s.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s_citations;", tblName))
		exp := exporter.NewSQLiteExporter()
		_ = exp.ExportTableToDB(targetTable, s.db, "")
	}

	json.NewEncoder(w).Encode(CellUpdateResponse{
		Success:     true,
		Table:       targetTable,
		AuditReport: newReport,
	})
}
