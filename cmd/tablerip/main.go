package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/alexandrmotologa/tablerip/pkg/auditor"
	"github.com/alexandrmotologa/tablerip/pkg/batch"
	"github.com/alexandrmotologa/tablerip/pkg/cockpit"
	"github.com/alexandrmotologa/tablerip/pkg/diff"
	"github.com/alexandrmotologa/tablerip/pkg/edgar"
	"github.com/alexandrmotologa/tablerip/pkg/exporter"
	"github.com/alexandrmotologa/tablerip/pkg/fixtures"
	"github.com/alexandrmotologa/tablerip/pkg/grid"
	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "extract":
		runExtract(os.Args[2:])
	case "audit":
		runAudit(os.Args[2:])
	case "export":
		runExport(os.Args[2:])
	case "cockpit":
		runCockpit(os.Args[2:])
	case "benchmark":
		runBenchmark(os.Args[2:])
	case "generate":
		runGenerate(os.Args[2:])
	case "fetch":
		runFetch(os.Args[2:])
	case "diff":
		runDiff(os.Args[2:])
	case "batch":
		runBatch(os.Args[2:])
	case "version":
		fmt.Printf("tablerip v%s (pure Go, zero Cgo)\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command '%s'\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runGenerate(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	out := fs.String("out", "./testdata/sample_10k.pdf", "Output path for generated synthetic PDF")
	fs.Parse(args)

	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Directory creation error: %v\n", err)
		os.Exit(1)
	}

	if err := fixtures.GenerateSynthetic10K(*out); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate sample PDF: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[SUCCESS] Generated authentic synthetic SEC 10-K filing: %s\n", *out)
}

func printUsage() {
	fmt.Println(`tablerip: Pure Go Autonomous Financial Table Extraction & Audit Engine

Usage:
  tablerip <command> [arguments]

Commands:
  extract    Extract tables from PDF and export to Parquet, SQLite, JSON, or CSV
  audit      Run strict arithmetic balance reconciliation on financial statements
  export     Convert extracted tables into a specific target database or file
  fetch      Download SEC EDGAR filings by ticker/form and extract financials
  diff       Reconcile multi-period statements and detect historical restatements
  batch      Process directories of financial PDFs with bounded worker pool
  cockpit    Launch interactive dual-pane visual inspection cockpit on port 8787
  benchmark  Measure execution latency and verify < 25MB RAM memory ceiling
  generate   Create authentic synthetic multi-page SEC 10-K filing PDF
  version    Show version information

Run 'tablerip <command> -h' for command-specific flags.`)
}

func runExtract(args []string) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	input := fs.String("input", "", "Path to input financial PDF file")
	format := fs.String("format", "parquet", "Export format: parquet, sqlite, duckdb, json, csv")
	outDir := fs.String("out-dir", "./data/tables", "Output directory for extracted files")
	runAuditFlag := fs.Bool("audit", true, "Execute arithmetic balance reconciliation")
	launchCockpit := fs.Bool("cockpit", false, "Launch embedded web cockpit after extraction")
	port := fs.Int("port", 8787, "Port for web cockpit")
	fs.Parse(args)

	if *input == "" {
		fmt.Fprintln(os.Stderr, "Error: --input flag is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("[tablerip] Processing document: %s\n", *input)
	start := time.Now()

	doc, tables, reports, err := processDocument(*input, *runAuditFlag, 0.01, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Extraction error: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	fmt.Printf("[tablerip] Extracted %d table(s) across %d page(s) in %v\n", len(tables), doc.NumPages, elapsed)

	// Export files
	if err := exportTables(doc, tables, reports, *format, *outDir); err != nil {
		fmt.Fprintf(os.Stderr, "Export error: %v\n", err)
		os.Exit(1)
	}

	if *launchCockpit {
		startCockpit(doc, tables, reports, *port)
	}
}

func runAudit(args []string) {
	fs := flag.NewFlagSet("audit", flag.ExitOnError)
	input := fs.String("input", "", "Path to input financial PDF file")
	tolerance := fs.Float64("tolerance", 0.01, "Floating-point balance tolerance")
	strict := fs.Bool("strict", true, "Exit with non-zero code on arithmetic discrepancy")
	fs.Parse(args)

	if *input == "" {
		fmt.Fprintln(os.Stderr, "Error: --input flag is required")
		fs.Usage()
		os.Exit(1)
	}

	_, tables, reports, err := processDocument(*input, true, *tolerance, *strict)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Processing error: %v\n", err)
		os.Exit(1)
	}

	totalDiscrepancies := 0
	for i, rep := range reports {
		fmt.Println()
		fmt.Print(rep.FormatSummary())
		totalDiscrepancies += rep.DiscrepancyCount
		if i < len(tables) {
			fmt.Printf("Table: %s (Pages: %v)\n", tables[i].Title, tables[i].Pages)
		}
	}

	if totalDiscrepancies > 0 {
		fmt.Printf("\n[ALERT] Detected %d arithmetic discrepancies!\n", totalDiscrepancies)
		if *strict {
			os.Exit(2)
		}
	} else {
		fmt.Println("\n[SUCCESS] All mathematical invariants verified with zero discrepancies.")
	}
}

func runExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	input := fs.String("input", "", "Path to input financial PDF file")
	format := fs.String("format", "sqlite", "Export format: parquet, sqlite, duckdb, json, csv")
	out := fs.String("out", "", "Output file path")
	tablePrefix := fs.String("table-prefix", "", "Prefix for generated SQL table names")
	fs.Parse(args)

	if *input == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "Error: --input and --out flags are required")
		fs.Usage()
		os.Exit(1)
	}

	doc, tables, reports, err := processDocument(*input, true, 0.01, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Processing error: %v\n", err)
		os.Exit(1)
	}

	switch strings.ToLower(*format) {
	case "sqlite":
		exp := exporter.NewSQLiteExporter()
		for _, t := range tables {
			if err := exp.ExportTable(t, *out, *tablePrefix); err != nil {
				fmt.Fprintf(os.Stderr, "SQLite export error: %v\n", err)
				os.Exit(1)
			}
		}
		fmt.Printf("[SUCCESS] Exported %d table(s) to SQLite database: %s\n", len(tables), *out)

	case "parquet":
		exp := exporter.NewParquetExporter()
		for _, t := range tables {
			dest := *out
			if len(tables) > 1 {
				ext := filepath.Ext(dest)
				base := strings.TrimSuffix(dest, ext)
				dest = fmt.Sprintf("%s_%s%s", base, t.ID, ext)
			}
			if err := exp.ExportTable(t, dest); err != nil {
				fmt.Fprintf(os.Stderr, "Parquet export error: %v\n", err)
				os.Exit(1)
			}
		}
		fmt.Printf("[SUCCESS] Exported %d table(s) to Parquet: %s\n", len(tables), *out)

	case "duckdb":
		exp := exporter.NewDuckDBExporter()
		for _, t := range tables {
			if err := exp.GenerateSQLScript(t, "", *out); err != nil {
				fmt.Fprintf(os.Stderr, "DuckDB script export error: %v\n", err)
				os.Exit(1)
			}
		}
		fmt.Printf("[SUCCESS] Generated DuckDB SQL script: %s\n", *out)

	case "json":
		exp := exporter.NewJSONExporter()
		if err := exp.Export(doc.Path, tables, reports, *out); err != nil {
			fmt.Fprintf(os.Stderr, "JSON export error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[SUCCESS] Exported tables to JSON: %s\n", *out)

	case "csv":
		exp := exporter.NewCSVExporter()
		for _, t := range tables {
			dest := *out
			if len(tables) > 1 {
				ext := filepath.Ext(dest)
				base := strings.TrimSuffix(dest, ext)
				dest = fmt.Sprintf("%s_%s%s", base, t.ID, ext)
			}
			if err := exp.ExportTable(t, dest); err != nil {
				fmt.Fprintf(os.Stderr, "CSV export error: %v\n", err)
				os.Exit(1)
			}
		}
		fmt.Printf("[SUCCESS] Exported CSV: %s\n", *out)

	default:
		fmt.Fprintf(os.Stderr, "Unsupported export format '%s'\n", *format)
		os.Exit(1)
	}
}

func runCockpit(args []string) {
	fs := flag.NewFlagSet("cockpit", flag.ExitOnError)
	input := fs.String("input", "", "Path to input financial PDF file")
	port := fs.Int("port", 8787, "HTTP server port")
	fs.Parse(args)

	if *input == "" {
		fmt.Fprintln(os.Stderr, "Error: --input flag is required")
		fs.Usage()
		os.Exit(1)
	}

	doc, tables, reports, err := processDocument(*input, true, 0.01, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Processing error: %v\n", err)
		os.Exit(1)
	}

	startCockpit(doc, tables, reports, *port)
}

func runBenchmark(args []string) {
	fs := flag.NewFlagSet("benchmark", flag.ExitOnError)
	input := fs.String("input", "", "Path to input financial PDF file")
	iterations := fs.Int("iterations", 5, "Number of benchmark iterations")
	fs.Parse(args)

	if *input == "" {
		fmt.Fprintln(os.Stderr, "Error: --input flag is required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("=== tablerip Performance Benchmark ===\n")
	fmt.Printf("Target File: %s\n", *input)
	fmt.Printf("Iterations: %d\n\n", *iterations)

	var totalDuration time.Duration
	var peakAlloc uint64

	for i := 1; i <= *iterations; i++ {
		runtime.GC()
		var mStart runtime.MemStats
		runtime.ReadMemStats(&mStart)

		start := time.Now()
		doc, tables, reports, err := processDocument(*input, true, 0.01, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Iteration %d failed: %v\n", i, err)
			os.Exit(1)
		}
		dur := time.Since(start)
		totalDuration += dur

		var mEnd runtime.MemStats
		runtime.ReadMemStats(&mEnd)

		allocMB := float64(mEnd.Alloc) / (1024 * 1024)
		sysMB := float64(mEnd.Sys) / (1024 * 1024)

		if mEnd.Alloc > peakAlloc {
			peakAlloc = mEnd.Alloc
		}

		fmt.Printf("Iter %d: %8v | Tables: %d | Pages: %d | Mem Alloc: %.2f MB | Sys: %.2f MB\n",
			i, dur, len(tables), doc.NumPages, allocMB, sysMB)
		_ = reports
	}

	avgDuration := totalDuration / time.Duration(*iterations)
	peakAllocMB := float64(peakAlloc) / (1024 * 1024)

	fmt.Printf("\n--- Benchmark Results ---\n")
	fmt.Printf("Average Latency: %v\n", avgDuration)
	fmt.Printf("Peak Heap Alloc: %.2f MB\n", peakAllocMB)
	if peakAllocMB < 25.0 {
		fmt.Printf("Memory Invariant: PASS (Peak %.2f MB < 25.00 MB target)\n", peakAllocMB)
	} else {
		fmt.Printf("Memory Invariant: WARNING (Peak %.2f MB exceeds 25.00 MB)\n", peakAllocMB)
	}
}

func processDocument(path string, audit bool, tolerance float64, strict bool) (*pdf.Document, []*grid.Table, []*auditor.AuditReport, error) {
	doc, err := pdf.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}

	extractor := grid.NewExtractor()
	tables, err := extractor.ExtractFromDocument(doc)
	if err != nil {
		return doc, nil, nil, err
	}

	var reports []*auditor.AuditReport
	if audit {
		aud := auditor.NewAuditor(tolerance, strict)
		for _, t := range tables {
			rep := aud.AuditTable(t)
			reports = append(reports, rep)
		}
	}

	return doc, tables, reports, nil
}

func exportTables(doc *pdf.Document, tables []*grid.Table, reports []*auditor.AuditReport, format string, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	switch strings.ToLower(format) {
	case "parquet":
		exp := exporter.NewParquetExporter()
		for _, t := range tables {
			p := filepath.Join(outDir, t.ID+".parquet")
			if err := exp.ExportTable(t, p); err != nil {
				return err
			}
			fmt.Printf("  -> Parquet file written: %s\n", p)
		}
	case "sqlite":
		exp := exporter.NewSQLiteExporter()
		dbPath := filepath.Join(outDir, "financials.sqlite")
		for _, t := range tables {
			if err := exp.ExportTable(t, dbPath, ""); err != nil {
				return err
			}
		}
		fmt.Printf("  -> SQLite database written: %s\n", dbPath)
	case "duckdb":
		exp := exporter.NewDuckDBExporter()
		for _, t := range tables {
			sqlPath := filepath.Join(outDir, t.ID+"_duckdb.sql")
			if err := exp.GenerateSQLScript(t, "", sqlPath); err != nil {
				return err
			}
			fmt.Printf("  -> DuckDB SQL script written: %s\n", sqlPath)
		}
	case "json":
		exp := exporter.NewJSONExporter()
		jsonPath := filepath.Join(outDir, "financial_tables.json")
		if err := exp.Export(doc.Path, tables, reports, jsonPath); err != nil {
			return err
		}
		fmt.Printf("  -> JSON export written: %s\n", jsonPath)
	case "csv":
		exp := exporter.NewCSVExporter()
		for _, t := range tables {
			csvPath := filepath.Join(outDir, t.ID+".csv")
			if err := exp.ExportTable(t, csvPath); err != nil {
				return err
			}
			fmt.Printf("  -> CSV file written: %s\n", csvPath)
		}
	}

	return nil
}

func startCockpit(doc *pdf.Document, tables []*grid.Table, reports []*auditor.AuditReport, port int) {
	server, err := cockpit.NewServer(doc, tables, reports, port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize cockpit: %v\n", err)
		os.Exit(1)
	}
	defer server.Close()

	url := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("\n[COCKPIT] Web Cockpit launched at: %s\n", url)
	fmt.Println("[COCKPIT] Press Ctrl+C to terminate.")

	if err := server.Start(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Cockpit error: %v\n", err)
		os.Exit(1)
	}
}

func runFetch(args []string) {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	ticker := fs.String("ticker", "", "Stock ticker symbol (e.g. AAPL, MSFT, NVDA)")
	form := fs.String("form", "10-K", "SEC form type: 10-K, 10-Q, 8-K")
	year := fs.Int("year", 0, "Filing year (default: 0 for most recent)")
	outDir := fs.String("out-dir", "./data/sec", "Directory to save downloaded filing")
	extract := fs.Bool("extract", true, "Extract financial tables after downloading")
	auditFlag := fs.Bool("audit", true, "Run arithmetic balance reconciliation")
	launchCockpit := fs.Bool("cockpit", false, "Launch Cockpit server after download and extraction")
	port := fs.Int("port", 8787, "Port for web cockpit")
	fs.Parse(args)

	if *ticker == "" {
		fmt.Fprintln(os.Stderr, "Error: --ticker flag is required (e.g. -ticker AAPL)")
		fs.Usage()
		os.Exit(1)
	}

	client := edgar.NewClient("")
	fmt.Printf("[EDGAR] Locating SEC filing for %s (Form: %s, Year: %d)...\n", *ticker, *form, *year)
	filing, err := client.FindFiling(*ticker, *form, *year)
	if err != nil {
		fmt.Fprintf(os.Stderr, "EDGAR error: %v\n", err)
		os.Exit(1)
	}

	destFile := filepath.Join(*outDir, fmt.Sprintf("%s_%s_%s.pdf", strings.ToUpper(*ticker), filing.Form, filing.FilingDate))
	fmt.Printf("[EDGAR] Found filing accession %s (Filing Date: %s)\n", filing.AccessionNumber, filing.FilingDate)
	fmt.Printf("[EDGAR] Downloading primary document: %s\n", filing.DocumentURL)

	if err := client.DownloadFiling(filing, destFile); err != nil {
		fmt.Fprintf(os.Stderr, "Download error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[SUCCESS] Downloaded filing to: %s\n", destFile)

	if *extract {
		fmt.Println("[tablerip] Extracting tables from downloaded filing...")
		doc, tables, reports, err := processDocument(destFile, *auditFlag, 0.01, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Extraction error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[SUCCESS] Extracted %d table(s) across %d page(s)\n", len(tables), doc.NumPages)
		if *launchCockpit {
			startCockpit(doc, tables, reports, *port)
		}
	}
}

func runDiff(args []string) {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	base := fs.String("base", "", "Base (prior period) financial PDF file")
	target := fs.String("target", "", "Target (current period) financial PDF file")
	out := fs.String("out", "", "Optional output path to write JSON diff report")
	tolerance := fs.Float64("tolerance", 0.01, "Restatement delta tolerance")
	fs.Parse(args)

	if *base == "" || *target == "" {
		fmt.Fprintln(os.Stderr, "Error: both --base and --target flags are required")
		fs.Usage()
		os.Exit(1)
	}

	fmt.Printf("[tablerip diff] Extracting base filing: %s\n", *base)
	_, baseTables, _, err := processDocument(*base, true, 0.01, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Base extraction error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[tablerip diff] Extracting target filing: %s\n", *target)
	_, targetTables, _, err := processDocument(*target, true, 0.01, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Target extraction error: %v\n", err)
		os.Exit(1)
	}

	engine := diff.NewEngine(*tolerance)
	docDiff := engine.CompareDocuments(baseTables, targetTables, *base, *target)

	fmt.Printf("\n=== Multi-Period Reconciliation Report ===\n")
	fmt.Printf("Base Filing:   %s (%d tables)\n", *base, len(baseTables))
	fmt.Printf("Target Filing: %s (%d tables)\n", *target, len(targetTables))
	fmt.Printf("Compared Line Items: %d\n", docDiff.TotalComparedItems)
	fmt.Printf("Detected Restatements: %d\n\n", docDiff.TotalRestatements)

	for _, td := range docDiff.TableDiffs {
		fmt.Println(td.FormatMarkdown())
	}

	if *out != "" {
		if err := docDiff.ExportJSON(*out); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write JSON diff report: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[SUCCESS] Saved JSON diff report to: %s\n", *out)
	}

	if docDiff.TotalRestatements > 0 {
		fmt.Printf("\n[ALERT] Found %d historical restatements across statements.\n", docDiff.TotalRestatements)
	} else {
		fmt.Println("\n[SUCCESS] All historical prior-period values matched target filing exactly.")
	}
}

func runBatch(args []string) {
	fs := flag.NewFlagSet("batch", flag.ExitOnError)
	inputDir := fs.String("input-dir", "", "Directory containing PDF financial filings")
	outDir := fs.String("out-dir", "./data/batch_out", "Directory for exported tables and audit ledger")
	format := fs.String("format", "all", "Export formats: parquet, sqlite, json, csv, or all")
	concurrency := fs.Int("concurrency", runtime.NumCPU(), "Worker pool concurrency")
	tolerance := fs.Float64("tolerance", 0.01, "Floating-point balance tolerance")
	recursive := fs.Bool("recursive", false, "Recursively scan subdirectories for PDFs")
	fs.Parse(args)

	if *inputDir == "" {
		fmt.Fprintln(os.Stderr, "Error: --input-dir flag is required")
		fs.Usage()
		os.Exit(1)
	}

	var formats []string
	if *format == "all" {
		formats = []string{"parquet", "sqlite", "json", "csv"}
	} else {
		formats = strings.Split(*format, ",")
	}

	cfg := batch.Config{
		Concurrency:          *concurrency,
		ExportFormats:        formats,
		OutputDir:            *outDir,
		Tolerance:            *tolerance,
		CheckAccountingRules: true,
		ConsolidatedSQLite:   true,
		ProgressFn: func(completed, total int, currentFile string, err error) {
			fname := filepath.Base(currentFile)
			if err != nil {
				fmt.Printf("[%d/%d] FAIL: %s (%v)\n", completed, total, fname, err)
			} else {
				fmt.Printf("[%d/%d] PASS: %s\n", completed, total, fname)
			}
		},
	}

	pool := batch.NewPool(cfg)
	fmt.Printf("[BATCH] Starting worker pool (concurrency=%d) on directory: %s\n", *concurrency, *inputDir)
	start := time.Now()

	report, err := pool.ProcessDirectory(*inputDir, *recursive)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Batch processing failed: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	fmt.Printf("\n=== Batch Processing Summary ===\n")
	fmt.Printf("Total Files:        %d\n", report.TotalFiles)
	fmt.Printf("Successful Files:   %d\n", report.SuccessfulFiles)
	fmt.Printf("Failed Files:       %d\n", report.FailedFiles)
	fmt.Printf("Total Tables:       %d\n", report.TotalTables)
	fmt.Printf("Audit Invariants:   %d checked\n", report.TotalAuditChecks)
	fmt.Printf("Discrepancies:      %d\n", report.TotalDiscrepancies)
	fmt.Printf("Elapsed Time:       %v\n", elapsed)
	if report.ConsolidatedLedgerPath != "" {
		fmt.Printf("Audit Ledger:       %s\n", report.ConsolidatedLedgerPath)
	}
	if report.ConsolidatedSQLitePath != "" {
		fmt.Printf("Master SQLite DB:   %s\n", report.ConsolidatedSQLitePath)
	}

	if !report.AllPassed {
		fmt.Println("\n[ALERT] One or more files failed processing or had arithmetic discrepancies.")
	} else {
		fmt.Println("\n[SUCCESS] Batch processing completed with zero discrepancies across all filings.")
	}
}
