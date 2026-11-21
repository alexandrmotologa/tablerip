package exporter

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/grid"
)

// CSVExporter writes structured CSV files with indentation preservation.
type CSVExporter struct{}

// NewCSVExporter creates a CSVExporter.
func NewCSVExporter() *CSVExporter {
	return &CSVExporter{}
}

// ExportTable writes a table to a CSV file.
func (ce *CSVExporter) ExportTable(table *grid.Table, outputPath string) error {
	if table == nil {
		return fmt.Errorf("table is nil")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header row
	var headerRow []string
	headerRow = append(headerRow, "Hierarchy_Level")
	for _, col := range table.Columns {
		headerRow = append(headerRow, col.Name)
	}
	if err := writer.Write(headerRow); err != nil {
		return fmt.Errorf("failed to write header row: %w", err)
	}

	// Write data rows
	for _, r := range table.DataRows() {
		var record []string
		record = append(record, fmt.Sprintf("%d", r.Level))

		// Column 0 with indentation spaces
		indent := strings.Repeat("  ", r.Level)
		record = append(record, indent+r.Label)

		for colIdx := 1; colIdx < len(table.Columns); colIdx++ {
			cell := r.GetCell(colIdx)
			if cell != nil && cell.IsNumeric {
				record = append(record, fmt.Sprintf("%.2f", cell.Value))
			} else if cell != nil {
				record = append(record, cell.RawText)
			} else {
				record = append(record, "")
			}
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write data row: %w", err)
		}
	}

	return nil
}
