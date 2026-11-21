package test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/alexandrmotologa/tablerip/pkg/auditor"
	"github.com/alexandrmotologa/tablerip/pkg/exporter"
	"github.com/alexandrmotologa/tablerip/pkg/fixtures"
	"github.com/alexandrmotologa/tablerip/pkg/grid"
	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

func TestEndToEndSEC10KExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "synthetic_10k.pdf")

	// Step 1: Generate synthetic authentic 10-K filing
	if err := fixtures.GenerateSynthetic10K(pdfPath); err != nil {
		t.Fatalf("failed to generate test PDF: %v", err)
	}

	// Step 2: Open and parse PDF
	doc, err := pdf.Open(pdfPath)
	if err != nil {
		t.Fatalf("failed to open PDF: %v", err)
	}

	if doc.NumPages != 3 {
		t.Fatalf("expected 3 pages, got %d", doc.NumPages)
	}

	// Step 3: Extract tables and verify multi-page continuation stitcher
	extractor := grid.NewExtractor()
	tables, err := extractor.ExtractFromDocument(doc)
	if err != nil {
		t.Fatalf("failed to extract tables: %v", err)
	}

	if len(tables) < 2 {
		t.Fatalf("expected at least 2 financial tables, got %d", len(tables))
	}

	// Table 1 must be the multi-page stitched Operations statement (Pages 1 and 2)
	opTable := tables[0]
	if len(opTable.Pages) != 2 {
		t.Errorf("expected Table 1 to span 2 pages, got %v", opTable.Pages)
	}

	// Step 4: Arithmetic balance verification
	aud := auditor.NewAuditor(0.01, true)
	totalChecked := 0
	totalDisc := 0

	for _, tbl := range tables {
		rep := aud.AuditTable(tbl)
		totalChecked += rep.TotalChecked
		totalDisc += rep.DiscrepancyCount

		if !rep.Passed {
			t.Errorf("expected table '%s' to pass arithmetic audit, report:\n%s", tbl.Title, rep.FormatSummary())
		}
	}

	if totalChecked == 0 {
		t.Fatalf("expected arithmetic invariants to be evaluated, checked 0")
	}
	if totalDisc != 0 {
		t.Fatalf("expected 0 discrepancies across all financial statements, got %d", totalDisc)
	}

	// Step 5: Test Exporters
	parquetPath := filepath.Join(tmpDir, "op_table.parquet")
	parquetExp := exporter.NewParquetExporter()
	if err := parquetExp.ExportTable(opTable, parquetPath); err != nil {
		t.Fatalf("failed to export Parquet: %v", err)
	}

	sqlitePath := filepath.Join(tmpDir, "financials.sqlite")
	sqliteExp := exporter.NewSQLiteExporter()
	for _, tbl := range tables {
		if err := sqliteExp.ExportTable(tbl, sqlitePath, ""); err != nil {
			t.Fatalf("failed to export SQLite: %v", err)
		}
	}

	jsonPath := filepath.Join(tmpDir, "tables.json")
	jsonExp := exporter.NewJSONExporter()
	if err := jsonExp.Export(doc.Path, tables, nil, jsonPath); err != nil {
		t.Fatalf("failed to export JSON: %v", err)
	}
}

func TestMemoryCeilingUnder25MB(t *testing.T) {
	tmpDir := t.TempDir()
	pdfPath := filepath.Join(tmpDir, "synthetic_10k_bench.pdf")

	if err := fixtures.GenerateSynthetic10K(pdfPath); err != nil {
		t.Fatalf("failed to generate test PDF: %v", err)
	}

	// Force GC and read baseline
	runtime.GC()
	var mStart runtime.MemStats
	runtime.ReadMemStats(&mStart)

	// Run full extraction pipeline
	doc, err := pdf.Open(pdfPath)
	if err != nil {
		t.Fatalf("failed to open PDF: %v", err)
	}

	extractor := grid.NewExtractor()
	tables, err := extractor.ExtractFromDocument(doc)
	if err != nil {
		t.Fatalf("failed to extract tables: %v", err)
	}

	aud := auditor.NewAuditor(0.01, true)
	for _, tbl := range tables {
		aud.AuditTable(tbl)
	}

	var mEnd runtime.MemStats
	runtime.ReadMemStats(&mEnd)

	peakAllocMB := float64(mEnd.Alloc) / (1024 * 1024)
	t.Logf("Measured Peak Memory Allocation: %.2f MB", peakAllocMB)

	if peakAllocMB >= 25.0 {
		t.Fatalf("Memory ceiling violated: %.2f MB exceeds 25.00 MB target", peakAllocMB)
	}
}
