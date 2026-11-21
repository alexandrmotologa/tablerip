package exporter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexandrmotologa/tablerip/pkg/grid"
	"github.com/parquet-go/parquet-go"
)

// ParquetCellRecord represents a citation-grounded financial metric in Parquet.
type ParquetCellRecord struct {
	TableID        string  `parquet:"table_id"`
	RowIndex       int64   `parquet:"row_index"`
	HierarchyLevel int32   `parquet:"hierarchy_level"`
	RowLabel       string  `parquet:"row_label"`
	Section        string  `parquet:"section"`
	GAAPTag        string  `parquet:"gaap_tag"`
	FootnoteMarker string  `parquet:"footnote_marker"`
	FootnoteText   string  `parquet:"footnote_text"`
	IsTotal        bool    `parquet:"is_total"`
	PageNumber     int32   `parquet:"page_number"`
	ColIndex       int32   `parquet:"col_index"`
	ColName        string  `parquet:"col_name"`
	RawText        string  `parquet:"raw_text"`
	Value          float64 `parquet:"value"`
	IsNumeric      bool    `parquet:"is_numeric"`
	Currency       string  `parquet:"currency"`
	BBoxX0         float64 `parquet:"bbox_x0"`
	BBoxY0         float64 `parquet:"bbox_y0"`
	BBoxX1         float64 `parquet:"bbox_x1"`
	BBoxY1         float64 `parquet:"bbox_y1"`
	Audited        bool    `parquet:"audited"`
	AuditPass      bool    `parquet:"audit_pass"`
}

// ParquetExporter handles serializing tables into Apache Parquet files.
type ParquetExporter struct{}

// NewParquetExporter creates a ParquetExporter.
func NewParquetExporter() *ParquetExporter {
	return &ParquetExporter{}
}

// ExportTable writes a single Table to a Parquet file.
func (pe *ParquetExporter) ExportTable(table *grid.Table, outputPath string) error {
	if table == nil {
		return fmt.Errorf("table is nil")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	var records []ParquetCellRecord

	for _, row := range table.DataRows() {
		for colIdx := 1; colIdx < len(table.Columns); colIdx++ {
			col := table.Columns[colIdx]
			cell := row.GetCell(colIdx)

			rec := ParquetCellRecord{
				TableID:        table.ID,
				RowIndex:       int64(row.Index),
				HierarchyLevel: int32(row.Level),
				RowLabel:       row.Label,
				Section:        row.Section,
				GAAPTag:        row.GAAPTag,
				FootnoteMarker: row.FootnoteMarker,
				FootnoteText:   row.FootnoteText,
				IsTotal:        row.IsTotal,
				PageNumber:     int32(row.PageNum),
				ColIndex:       int32(colIdx),
				ColName:        col.Name,
				Audited:        false,
				AuditPass:      true,
			}

			if cell != nil {
				rec.RawText = cell.RawText
				rec.Value = cell.Value
				rec.IsNumeric = cell.IsNumeric
				rec.Currency = cell.Currency
				if cell.GAAPTag != "" {
					rec.GAAPTag = cell.GAAPTag
				}
				if cell.FootnoteMarker != "" {
					rec.FootnoteMarker = cell.FootnoteMarker
					rec.FootnoteText = cell.FootnoteText
				}
				rec.BBoxX0 = cell.BBox.X0
				rec.BBoxY0 = cell.BBox.Y0
				rec.BBoxX1 = cell.BBox.X1
				rec.BBoxY1 = cell.BBox.Y1
				rec.Audited = cell.Audited
				rec.AuditPass = cell.AuditPass
			}

			records = append(records, rec)
		}
	}

	return parquet.WriteFile(outputPath, records, parquet.Compression(&parquet.Snappy))
}
