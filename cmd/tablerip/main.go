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
	"github.com/alexandrmotologa/tablerip/pkg/cockpit"
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
