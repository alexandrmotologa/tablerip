package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexandrmotologa/tablerip/pkg/auditor"
	"github.com/alexandrmotologa/tablerip/pkg/grid"
)

// DocumentExport represents the full JSON export structure for a processed PDF.
type DocumentExport struct {
	DocumentPath string          `json:"document_path"`
	Tables       []TableExport   `json:"tables"`
	Summary      DocumentSummary `json:"summary"`
}

// TableExport represents a single exported table with audit metadata.
type TableExport struct {
	Table       *grid.Table          `json:"table"`
	AuditReport *auditor.AuditReport `json:"audit_report,omitempty"`
}

// DocumentSummary aggregates high-level extraction metrics.
type DocumentSummary struct {
	TotalTables     int     `json:"total_tables"`
	TotalRows       int     `json:"total_rows"`
	TotalCells      int     `json:"total_cells"`
	AuditConfidence float64 `json:"audit_confidence"`
	AuditPassed     bool    `json:"audit_passed"`
}

// JSONExporter serializes tables to formatted JSON.
type JSONExporter struct{}

// NewJSONExporter creates a JSONExporter.
func NewJSONExporter() *JSONExporter {
	return &JSONExporter{}
}

// Export writes the extracted tables and audit reports to a JSON file.
func (je *JSONExporter) Export(docPath string, tables []*grid.Table, reports []*auditor.AuditReport, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	reportMap := make(map[string]*auditor.AuditReport)
	for _, r := range reports {
		if r != nil {
			reportMap[r.TableID] = r
		}
	}

	docExport := DocumentExport{
		DocumentPath: docPath,
		Tables:       make([]TableExport, 0, len(tables)),
	}

	totalRows := 0
	totalCells := 0
	sumConfidence := 0.0
	allPassed := true

	for _, t := range tables {
		rep := reportMap[t.ID]
		docExport.Tables = append(docExport.Tables, TableExport{
			Table:       t,
			AuditReport: rep,
		})

		totalRows += len(t.DataRows())
		for _, r := range t.DataRows() {
			totalCells += len(r.Cells)
		}

		if rep != nil {
			sumConfidence += rep.ConfidenceScore
			if !rep.Passed {
				allPassed = false
			}
		} else {
			sumConfidence += 1.0
		}
	}

	avgConfidence := 1.0
	if len(tables) > 0 {
		avgConfidence = sumConfidence / float64(len(tables))
	}

	docExport.Summary = DocumentSummary{
		TotalTables:     len(tables),
		TotalRows:       totalRows,
		TotalCells:      totalCells,
		AuditConfidence: avgConfidence,
		AuditPassed:     allPassed,
	}

	data, err := json.MarshalIndent(docExport, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return os.WriteFile(outputPath, data, 0644)
}
