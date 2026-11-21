package batch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/tablerip/pkg/auditor"
	"github.com/alexandrmotologa/tablerip/pkg/exporter"
	"github.com/alexandrmotologa/tablerip/pkg/grid"
	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

// Config defines execution parameters for batch processing.
type Config struct {
	Concurrency           int
	ExportFormats         []string // "parquet", "sqlite", "json", "csv", or "all"
	OutputDir             string
	Tolerance             float64
	CheckAccountingRules  bool
	StopOnError           bool
	ConsolidatedSQLite    bool
	ProgressFn            func(completed, total int, currentFile string, err error)
}

// FileResult holds extraction and audit metrics for an individual PDF file.
type FileResult struct {
	FilePath           string                  `json:"file_path"`
	FileName           string                  `json:"file_name"`
	NumPages           int                     `json:"num_pages"`
	TableCount         int                     `json:"table_count"`
	Tables             []*grid.Table           `json:"-"`
	AuditReports       []*auditor.AuditReport  `json:"audit_reports,omitempty"`
	TotalChecks        int                     `json:"total_checks"`
	DiscrepancyCount   int                     `json:"discrepancy_count"`
	Passed             bool                    `json:"passed"`
	Error              string                  `json:"error,omitempty"`
	DurationMs         int64                   `json:"duration_ms"`
	ExportedFiles      []string                `json:"exported_files,omitempty"`
}

// BatchReport summarizes results across all files in the batch run.
type BatchReport struct {
	TotalFiles             int           `json:"total_files"`
	SuccessfulFiles        int           `json:"successful_files"`
	FailedFiles            int           `json:"failed_files"`
	TotalTables            int           `json:"total_tables"`
	TotalAuditChecks       int           `json:"total_audit_checks"`
	TotalDiscrepancies     int           `json:"total_discrepancies"`
	AllPassed              bool          `json:"all_passed"`
	StartTime              time.Time     `json:"start_time"`
	EndTime                time.Time     `json:"end_time"`
	DurationMs             int64         `json:"duration_ms"`
	ConsolidatedLedgerPath string        `json:"consolidated_ledger_path,omitempty"`
	ConsolidatedSQLitePath string        `json:"consolidated_sqlite_path,omitempty"`
	Results                []*FileResult `json:"results"`
}

// Pool orchestrates parallel extraction and audit over a file set.
type Pool struct {
	cfg Config
}

// NewPool creates a new worker pool with validated configuration.
func NewPool(cfg Config) *Pool {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = runtime.NumCPU()
	}
	if cfg.Concurrency > 128 {
		cfg.Concurrency = 128
	}
	if cfg.Tolerance <= 0 {
		cfg.Tolerance = 0.01
	}
	return &Pool{cfg: cfg}
}

// ProcessFiles processes a list of PDF file paths concurrently.
func (p *Pool) ProcessFiles(files []string) (*BatchReport, error) {
	startTime := time.Now()
	total := len(files)
	report := &BatchReport{
		TotalFiles: total,
		StartTime:  startTime,
		AllPassed:  true,
		Results:    make([]*FileResult, 0, total),
	}

	if total == 0 {
		report.EndTime = time.Now()
		report.DurationMs = report.EndTime.Sub(startTime).Milliseconds()
		return report, nil
	}

	if p.cfg.OutputDir != "" {
		if err := os.MkdirAll(p.cfg.OutputDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	jobs := make(chan string, total)
	results := make(chan *FileResult, total)

	var wg sync.WaitGroup
	workers := p.cfg.Concurrency
	if workers > total {
		workers = total
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				res := p.processSingleFile(file)
				results <- res
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	// Wait in background and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	var sqliteExp *exporter.SQLiteExporter
	var consolidatedDBPath string
	if p.cfg.ConsolidatedSQLite && p.cfg.OutputDir != "" {
		consolidatedDBPath = filepath.Join(p.cfg.OutputDir, "financials_master.sqlite")
		sqliteExp = exporter.NewSQLiteExporter()
		report.ConsolidatedSQLitePath = consolidatedDBPath
	}

	completed := 0
	var mu sync.Mutex

	for res := range results {
		mu.Lock()
		completed++
		report.Results = append(report.Results, res)

		if res.Error != "" {
			report.FailedFiles++
			report.AllPassed = false
		} else {
			report.SuccessfulFiles++
			report.TotalTables += res.TableCount
			report.TotalAuditChecks += res.TotalChecks
			report.TotalDiscrepancies += res.DiscrepancyCount
			if !res.Passed {
				report.AllPassed = false
			}

			// Add to consolidated SQLite if enabled
			if sqliteExp != nil && len(res.Tables) > 0 {
				prefix := sanitizeIdentifier(strings.TrimSuffix(res.FileName, filepath.Ext(res.FileName)))
				for _, tbl := range res.Tables {
					_ = sqliteExp.ExportTable(tbl, consolidatedDBPath, prefix)
				}
			}
		}

		if p.cfg.ProgressFn != nil {
			var err error
			if res.Error != "" {
				err = fmt.Errorf("%s", res.Error)
			}
			p.cfg.ProgressFn(completed, total, res.FilePath, err)
		}
		mu.Unlock()

		if p.cfg.StopOnError && res.Error != "" {
			break
		}
	}

	report.EndTime = time.Now()
	report.DurationMs = report.EndTime.Sub(startTime).Milliseconds()

	// Write consolidated audit ledger JSON if output directory is defined
	if p.cfg.OutputDir != "" {
		ledgerPath := filepath.Join(p.cfg.OutputDir, "audit_ledger.json")
		data, err := json.MarshalIndent(report, "", "  ")
		if err == nil {
			if writeErr := os.WriteFile(ledgerPath, data, 0644); writeErr == nil {
				report.ConsolidatedLedgerPath = ledgerPath
			}
		}
	}

	return report, nil
}

// ProcessDirectory searches the given directory for PDF files and processes them.
func (p *Pool) ProcessDirectory(inputDir string, recursive bool) (*BatchReport, error) {
	var files []string
	if recursive {
		err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".pdf") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		entries, err := os.ReadDir(inputDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".pdf") {
				files = append(files, filepath.Join(inputDir, e.Name()))
			}
		}
	}

	return p.ProcessFiles(files)
}

func (p *Pool) processSingleFile(filePath string) *FileResult {
	start := time.Now()
	baseName := filepath.Base(filePath)
	result := &FileResult{
		FilePath:      filePath,
		FileName:      baseName,
		Passed:        true,
		ExportedFiles: make([]string, 0),
	}

	doc, err := pdf.Open(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("pdf open error: %v", err)
		result.Passed = false
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}
	result.NumPages = doc.NumPages

	extractor := grid.NewExtractor()
	tables, err := extractor.ExtractFromDocument(doc)
	if err != nil {
		result.Error = fmt.Sprintf("extraction error: %v", err)
		result.Passed = false
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	result.Tables = tables
	result.TableCount = len(tables)

	// Run arithmetic auditor
	aud := auditor.NewAuditor(p.cfg.Tolerance, p.cfg.CheckAccountingRules)
	for _, tbl := range tables {
		rep := aud.AuditTable(tbl)
		result.AuditReports = append(result.AuditReports, rep)
		result.TotalChecks += rep.TotalChecked
		result.DiscrepancyCount += rep.DiscrepancyCount
		if !rep.Passed {
			result.Passed = false
		}
	}

	// Exports
	if p.cfg.OutputDir != "" && len(p.cfg.ExportFormats) > 0 && len(tables) > 0 {
		fileStem := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		subDir := filepath.Join(p.cfg.OutputDir, fileStem)
		_ = os.MkdirAll(subDir, 0755)

		formats := p.cfg.ExportFormats
		exportAll := false
		for _, f := range formats {
			if strings.EqualFold(f, "all") {
				exportAll = true
				break
			}
		}

		if exportAll || containsFormat(formats, "json") {
			jsonPath := filepath.Join(subDir, "tables.json")
			jsonExp := exporter.NewJSONExporter()
			if err := jsonExp.Export(filePath, tables, result.AuditReports, jsonPath); err == nil {
				result.ExportedFiles = append(result.ExportedFiles, jsonPath)
			}
		}

		if exportAll || containsFormat(formats, "sqlite") {
			sqlitePath := filepath.Join(subDir, "tables.sqlite")
			sqliteExp := exporter.NewSQLiteExporter()
			exported := false
			for _, tbl := range tables {
				if err := sqliteExp.ExportTable(tbl, sqlitePath, ""); err == nil {
					exported = true
				}
			}
			if exported {
				result.ExportedFiles = append(result.ExportedFiles, sqlitePath)
			}
		}

		if exportAll || containsFormat(formats, "parquet") {
			parquetExp := exporter.NewParquetExporter()
			for i, tbl := range tables {
				tblName := sanitizeIdentifier(tbl.Title)
				if tblName == "" {
					tblName = fmt.Sprintf("table_%d", i+1)
				}
				pPath := filepath.Join(subDir, fmt.Sprintf("%s.parquet", tblName))
				if err := parquetExp.ExportTable(tbl, pPath); err == nil {
					result.ExportedFiles = append(result.ExportedFiles, pPath)
				}
			}
		}

		if exportAll || containsFormat(formats, "csv") {
			csvExp := exporter.NewCSVExporter()
			for i, tbl := range tables {
				tblName := sanitizeIdentifier(tbl.Title)
				if tblName == "" {
					tblName = fmt.Sprintf("table_%d", i+1)
				}
				cPath := filepath.Join(subDir, fmt.Sprintf("%s.csv", tblName))
				if err := csvExp.ExportTable(tbl, cPath); err == nil {
					result.ExportedFiles = append(result.ExportedFiles, cPath)
				}
			}
		}
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

func containsFormat(formats []string, target string) bool {
	for _, f := range formats {
		if strings.EqualFold(f, target) {
			return true
		}
	}
	return false
}

func sanitizeIdentifier(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('_')
		}
	}
	res := strings.Trim(b.String(), "_")
	if len(res) > 40 {
		res = res[:40]
	}
	return res
}
