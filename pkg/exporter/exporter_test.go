package exporter

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandrmotologa/tablerip/pkg/grid"
	"github.com/alexandrmotologa/tablerip/pkg/pdf"
	"github.com/parquet-go/parquet-go"
	_ "modernc.org/sqlite"
)

func createSampleTable() *grid.Table {
	cols := []*grid.Column{
		{Index: 0, Name: "Line Item", X0: 10, X1: 150},
		{Index: 1, Name: "2025", X0: 150, X1: 250},
		{Index: 2, Name: "2024", X0: 250, X1: 350},
	}

	rows := []*grid.Row{
		{
			Index: 0,
			Level: 0,
			Label: "Total net sales",
			Cells: []*grid.Cell{
				{ColIndex: 0, Text: "Total net sales", RawText: "Total net sales"},
				{ColIndex: 1, Text: "94930", RawText: "94,930", Value: 94930.0, IsNumeric: true, BBox: pdf.NewBBox(160, 50, 200, 60)},
				{ColIndex: 2, Text: "89498", RawText: "89,498", Value: 89498.0, IsNumeric: true, BBox: pdf.NewBBox(260, 50, 300, 60)},
			},
		},
		{
			Index: 1,
			Level: 1,
			Label: "Products",
			Cells: []*grid.Cell{
				{ColIndex: 0, Text: "Products", RawText: "Products"},
				{ColIndex: 1, Text: "69958", RawText: "69,958", Value: 69958.0, IsNumeric: true, BBox: pdf.NewBBox(160, 70, 200, 80)},
				{ColIndex: 2, Text: "67184", RawText: "67,184", Value: 67184.0, IsNumeric: true, BBox: pdf.NewBBox(260, 70, 300, 80)},
			},
		},
	}

	return &grid.Table{
		ID:      "tbl_001",
		Title:   "Statements of Operations",
		Columns: cols,
		Rows:    rows,
		Pages:   []int{1},
		BBox:    pdf.NewBBox(10, 40, 350, 100),
	}
}

func TestParquetExport(t *testing.T) {
	tmpDir := t.TempDir()
	parquetPath := filepath.Join(tmpDir, "test.parquet")

	table := createSampleTable()
	exp := NewParquetExporter()

	err := exp.ExportTable(table, parquetPath)
	if err != nil {
		t.Fatalf("failed to export parquet: %v", err)
	}

	// Verify file exists
	info, err := os.Stat(parquetPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("expected parquet file to exist with size > 0, got size %d", info.Size())
	}

	// Read back and verify records
	rows, err := parquet.ReadFile[ParquetCellRecord](parquetPath)
	if err != nil {
		t.Fatalf("failed to read back parquet: %v", err)
	}

	if len(rows) != 4 { // 2 data rows * 2 metric columns = 4 records
		t.Fatalf("expected 4 parquet records, got %d", len(rows))
	}

	if rows[0].Value != 94930.0 {
		t.Errorf("expected first record value 94930, got %f", rows[0].Value)
	}
	if rows[0].RowLabel != "Total net sales" {
		t.Errorf("expected row label 'Total net sales', got '%s'", rows[0].RowLabel)
	}
}

func TestSQLiteExport(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "financials.sqlite")

	table := createSampleTable()
	exp := NewSQLiteExporter()

	err := exp.ExportTable(table, dbPath, "")
	if err != nil {
		t.Fatalf("failed to export sqlite: %v", err)
	}

	// Read back using database/sql
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open exported sqlite db: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM tbl_001;").Scan(&count)
	if err != nil {
		t.Fatalf("query failed on data table: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows in data table, got %d", count)
	}

	var citCount int
	err = db.QueryRow("SELECT COUNT(*) FROM tbl_001_citations;").Scan(&citCount)
	if err != nil {
		t.Fatalf("query failed on citations table: %v", err)
	}
	if citCount != 4 {
		t.Fatalf("expected 4 citations, got %d", citCount)
	}
}

func TestDuckDBExport(t *testing.T) {
	tmpDir := t.TempDir()
	sqlPath := filepath.Join(tmpDir, "load.sql")

	table := createSampleTable()
	exp := NewDuckDBExporter()

	err := exp.GenerateSQLScript(table, "test.parquet", sqlPath)
	if err != nil {
		t.Fatalf("failed to generate duckdb script: %v", err)
	}

	content, err := os.ReadFile(sqlPath)
	if err != nil || len(content) == 0 {
		t.Fatalf("expected duckdb script to have content")
	}
}

func TestJSONAndCSVExport(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "out.json")
	csvPath := filepath.Join(tmpDir, "out.csv")

	table := createSampleTable()

	jsonExp := NewJSONExporter()
	if err := jsonExp.Export("doc.pdf", []*grid.Table{table}, nil, jsonPath); err != nil {
		t.Fatalf("failed to export json: %v", err)
	}

	csvExp := NewCSVExporter()
	if err := csvExp.ExportTable(table, csvPath); err != nil {
		t.Fatalf("failed to export csv: %v", err)
	}

	jInfo, _ := os.Stat(jsonPath)
	cInfo, _ := os.Stat(csvPath)
	if jInfo.Size() == 0 || cInfo.Size() == 0 {
		t.Fatalf("expected non-empty output files")
	}
}
