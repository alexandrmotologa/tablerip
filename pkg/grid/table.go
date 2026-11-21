package grid

import (
	"fmt"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

// Cell represents a single table cell in the reconstructed grid.
type Cell struct {
	ColIndex  int      `json:"col_index"`
	RowIndex  int      `json:"row_index"`
	PageNum   int      `json:"page_num"`
	Text      string   `json:"text"`
	RawText   string   `json:"raw_text"`
	Value     float64  `json:"value"`
	IsNumeric bool     `json:"is_numeric"`
	Currency  string   `json:"currency"`
	BBox      pdf.BBox `json:"bbox"`
	Audited   bool     `json:"audited"`
	AuditPass bool     `json:"audit_pass"`
}

// Column represents a detected column with bounding intervals and hierarchical name.
type Column struct {
	Index     int      `json:"index"`
	Name      string   `json:"name"`
	TierNames []string `json:"tier_names,omitempty"`
	X0        float64  `json:"x0"`
	X1        float64  `json:"x1"`
}

// CenterX returns the midpoint of the column.
func (c *Column) CenterX() float64 {
	return (c.X0 + c.X1) / 2.0
}

// Row represents a line in the financial table with hierarchical indentation.
type Row struct {
	Index       int      `json:"index"`
	Level       int      `json:"level"`
	Label       string   `json:"label"`
	Section     string   `json:"section,omitempty"`
	IsHeader    bool     `json:"is_header"`
	IsTotal     bool     `json:"is_total"`
	PageNum     int      `json:"page_num"`
	ParentIndex int      `json:"parent_index"`
	BBox        pdf.BBox `json:"bbox"`
	Cells       []*Cell  `json:"cells"`
}

// GetCell returns the cell for a specific column index, or nil.
func (r *Row) GetCell(colIdx int) *Cell {
	for _, c := range r.Cells {
		if c.ColIndex == colIdx {
			return c
		}
	}
	return nil
}

// Table represents a reconstructed financial table, potentially spanning multiple pages.
type Table struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Columns   []*Column `json:"columns"`
	Rows      []*Row    `json:"rows"`
	Pages     []int     `json:"pages"`
	BBox      pdf.BBox  `json:"bbox"`
	ScaleUnit string    `json:"scale_unit,omitempty"` // e.g. "millions", "thousands"
	ScaleMult float64   `json:"scale_mult,omitempty"` // e.g. 1e6, 1e3
}

// DataRows returns only the non-header rows of the table.
func (t *Table) DataRows() []*Row {
	var rows []*Row
	for _, r := range t.Rows {
		if !r.IsHeader {
			rows = append(rows, r)
		}
	}
	return rows
}

// HeaderRows returns only header rows of the table.
func (t *Table) HeaderRows() []*Row {
	var rows []*Row
	for _, r := range t.Rows {
		if r.IsHeader {
			rows = append(rows, r)
		}
	}
	return rows
}

// String returns a readable plain-text representation for terminal/debugging.
func (t *Table) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== TABLE: %s (Pages: %v) ===\n", t.Title, t.Pages))

	// Headers
	var headerNames []string
	for _, c := range t.Columns {
		headerNames = append(headerNames, fmt.Sprintf("%-25s", c.Name))
	}
	sb.WriteString(strings.Join(headerNames, " | ") + "\n")
	sb.WriteString(strings.Repeat("-", len(headerNames)*28) + "\n")

	// Rows
	for _, r := range t.DataRows() {
		indent := strings.Repeat("  ", r.Level)
		var cellValues []string
		for colIdx := range t.Columns {
			c := r.GetCell(colIdx)
			if c == nil || c.Text == "" {
				cellValues = append(cellValues, fmt.Sprintf("%-25s", ""))
			} else {
				cellValues = append(cellValues, fmt.Sprintf("%-25s", c.Text))
			}
		}
		if len(cellValues) > 0 {
			cellValues[0] = fmt.Sprintf("%-25s", indent+r.Label)
		}
		sb.WriteString(strings.Join(cellValues, " | ") + "\n")
	}

	return sb.String()
}
