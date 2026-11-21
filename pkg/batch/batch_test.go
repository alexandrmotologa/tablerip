package batch

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/alexandrmotologa/tablerip/pkg/fixtures"
)

func TestBatchProcessing(t *testing.T) {
	tmpDir := t.TempDir()
	inDir := filepath.Join(tmpDir, "inputs")
	outDir := filepath.Join(tmpDir, "outputs")
	if err := os.MkdirAll(inDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	// Generate 3 valid synthetic 10-K PDFs
	files := make([]string, 3)
	for i := 0; i < 3; i++ {
		p := filepath.Join(inDir, filepath.Base(filepath.Join(inDir, "filing_"+string(rune('A'+i))+".pdf")))
		if err := fixtures.GenerateSynthetic10K(p); err != nil {
			t.Fatalf("failed to generate test PDF %d: %v", i, err)
		}
		files[i] = p
	}

	// Add 1 corrupted dummy file to verify error resilience
	corrupted := filepath.Join(inDir, "corrupted.pdf")
	if err := os.WriteFile(corrupted, []byte("%PDF-corrupted-bytes-only"), 0644); err != nil {
		t.Fatalf("failed to write corrupted pdf: %v", err)
	}
	files = append(files, corrupted)

	var progressCalls int32
	cfg := Config{
		Concurrency:          2,
		ExportFormats:        []string{"parquet", "sqlite", "json", "csv"},
		OutputDir:            outDir,
		Tolerance:            0.01,
		CheckAccountingRules: true,
		ConsolidatedSQLite:   true,
		ProgressFn: func(completed, total int, currentFile string, err error) {
			atomic.AddInt32(&progressCalls, 1)
		},
	}

	pool := NewPool(cfg)
	report, err := pool.ProcessFiles(files)
	if err != nil {
		t.Fatalf("unexpected pool failure: %v", err)
	}

	if report.TotalFiles != 4 {
		t.Errorf("expected 4 total files, got %d", report.TotalFiles)
	}
	if report.SuccessfulFiles != 3 {
		t.Errorf("expected 3 successful files, got %d", report.SuccessfulFiles)
	}
	if report.FailedFiles != 1 {
		t.Errorf("expected 1 failed file (corrupted), got %d", report.FailedFiles)
	}
	if report.TotalTables < 6 { // 3 files * at least 2 tables each
		t.Errorf("expected at least 6 total tables, got %d", report.TotalTables)
	}
	if report.TotalAuditChecks == 0 {
		t.Errorf("expected >0 audit checks across batch")
	}
	if report.TotalDiscrepancies != 0 {
		t.Errorf("expected 0 discrepancies in synthetic valid files, got %d", report.TotalDiscrepancies)
	}
	if int(atomic.LoadInt32(&progressCalls)) != 4 {
		t.Errorf("expected 4 progress callbacks, got %d", progressCalls)
	}

	// Check consolidated ledger file
	if _, err := os.Stat(report.ConsolidatedLedgerPath); err != nil {
		t.Errorf("expected consolidated ledger file to exist at %s: %v", report.ConsolidatedLedgerPath, err)
	}

	// Check consolidated SQLite file
	if _, err := os.Stat(report.ConsolidatedSQLitePath); err != nil {
		t.Errorf("expected consolidated SQLite file to exist at %s: %v", report.ConsolidatedSQLitePath, err)
	}
}

func TestProcessDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	inDir := filepath.Join(tmpDir, "inputs_dir")
	outDir := filepath.Join(tmpDir, "outputs_dir")
	if err := os.MkdirAll(inDir, 0755); err != nil {
		t.Fatalf("failed to create input dir: %v", err)
	}

	p1 := filepath.Join(inDir, "doc1.pdf")
	if err := fixtures.GenerateSynthetic10K(p1); err != nil {
		t.Fatalf("failed to generate doc1: %v", err)
	}

	cfg := Config{
		Concurrency: 1,
		OutputDir:   outDir,
	}
	pool := NewPool(cfg)
	report, err := pool.ProcessDirectory(inDir, false)
	if err != nil {
		t.Fatalf("ProcessDirectory failed: %v", err)
	}

	if report.TotalFiles != 1 {
		t.Errorf("expected 1 file found, got %d", report.TotalFiles)
	}
	if report.SuccessfulFiles != 1 {
		t.Errorf("expected 1 successful file, got %d", report.SuccessfulFiles)
	}
}
