package grid

import (
	"testing"

	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

func TestSpatialSegmenter(t *testing.T) {
	segmenter := NewSegmenter()

	// Simulate tokens for 3 columns and 3 rows
	tokens := []pdf.Token{
		// Row 1 (Y: 50-60)
		{Text: "Revenue", BBox: pdf.NewBBox(10, 50, 80, 60)},
		{Text: "100", BBox: pdf.NewBBox(150, 50, 180, 60)},
		{Text: "200", BBox: pdf.NewBBox(250, 50, 280, 60)},
		// Row 2 (Y: 70-80)
		{Text: "Cost of sales", BBox: pdf.NewBBox(10, 70, 90, 80)},
		{Text: "40", BBox: pdf.NewBBox(150, 70, 170, 80)},
		{Text: "80", BBox: pdf.NewBBox(250, 70, 270, 80)},
		// Row 3 (Y: 90-100)
		{Text: "Gross profit", BBox: pdf.NewBBox(10, 90, 85, 100)},
		{Text: "60", BBox: pdf.NewBBox(150, 90, 170, 100)},
		{Text: "120", BBox: pdf.NewBBox(250, 90, 280, 100)},
	}

	bands := segmenter.FindRowBands(tokens, 0, 150)
	if len(bands) != 3 {
		t.Fatalf("expected 3 row bands, got %d", len(bands))
	}

	cols := segmenter.FindColumnChannels(tokens, 0, 300, nil)
	if len(cols) != 3 {
		t.Fatalf("expected 3 column channels, got %d", len(cols))
	}
}

func TestHeaderResolver(t *testing.T) {
	resolver := NewHeaderResolver()

	colIntervals := []Interval{
		{Start: 0, End: 100},
		{Start: 100, End: 200},
		{Start: 200, End: 300},
	}

	// Two tiers of headers:
	// Tier 1: "Three Months Ended September 30," spanning cols 1 and 2
	// Tier 2: "2025" in col 1, "2024" in col 2
	headerRows := [][]pdf.Token{
		{
			{Text: "Three Months Ended September 30,", BBox: pdf.NewBBox(100, 20, 300, 32)},
		},
		{
			{Text: "2025", BBox: pdf.NewBBox(120, 35, 160, 47)},
			{Text: "2024", BBox: pdf.NewBBox(220, 35, 260, 47)},
		},
	}

	columns := resolver.ResolveHeaders(headerRows, colIntervals)
	if len(columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(columns))
	}

	if columns[0].Name != "Line Item" {
		t.Errorf("expected col 0 name 'Line Item', got '%s'", columns[0].Name)
	}

	expectedCol1 := "Three Months Ended September 30,.2025"
	if columns[1].Name != expectedCol1 {
		t.Errorf("expected col 1 name '%s', got '%s'", expectedCol1, columns[1].Name)
	}

	expectedCol2 := "Three Months Ended September 30,.2024"
	if columns[2].Name != expectedCol2 {
		t.Errorf("expected col 2 name '%s', got '%s'", expectedCol2, columns[2].Name)
	}
}

func TestRowIndentationHierarchy(t *testing.T) {
	classifier := NewRowClassifier()

	columns := []*Column{
		{Index: 0, Name: "Line Item", X0: 10, X1: 150},
		{Index: 1, Name: "2025", X0: 150, X1: 250},
	}

	// Row 0: Operating expenses (Base, X0 = 20) -> Level 0
	// Row 1: Research and development (Indented, X0 = 32) -> Level 1
	// Row 2: Total operating expenses (Base, X0 = 20) -> Level 0, IsTotal = true
	tokenRows := [][]pdf.Token{
		{
			{Text: "Operating expenses:", BBox: pdf.NewBBox(20, 50, 130, 60)},
		},
		{
			{Text: "Research and development", BBox: pdf.NewBBox(32, 70, 145, 80)},
			{Text: "1500", BBox: pdf.NewBBox(180, 70, 210, 80)},
		},
		{
			{Text: "Total operating expenses", BBox: pdf.NewBBox(20, 90, 140, 100)},
			{Text: "1500", BBox: pdf.NewBBox(180, 90, 210, 100)},
		},
	}

	rows := classifier.BuildRows(tokenRows, columns, 1)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	if rows[0].Level != 0 {
		t.Errorf("expected row 0 level 0, got %d", rows[0].Level)
	}
	if rows[1].Level != 1 {
		t.Errorf("expected row 1 level 1, got %d", rows[1].Level)
	}
	if rows[1].ParentIndex != 0 {
		t.Errorf("expected row 1 parent index 0, got %d", rows[1].ParentIndex)
	}
	if !rows[2].IsTotal {
		t.Errorf("expected row 2 to be marked as Total")
	}
}

func TestMultiPageStitcher(t *testing.T) {
	stitcher := NewStitcher()

	colsA := []*Column{
		{Index: 0, Name: "Line Item", X0: 10, X1: 100},
		{Index: 1, Name: "2025", X0: 100, X1: 200},
	}
	colsB := []*Column{
		{Index: 0, Name: "Line Item", X0: 11, X1: 99},
		{Index: 1, Name: "2025", X0: 102, X1: 198},
	}

	score := stitcher.ColumnSimilarity(colsA, colsB)
	if score < 0.95 {
		t.Fatalf("expected high similarity > 0.95, got %f", score)
	}

	tblA := &Table{
		ID:      "tbl_001",
		Title:   "Statement of Operations",
		Columns: colsA,
		Pages:   []int{1},
		Rows: []*Row{
			{Index: 0, Label: "Revenue", Cells: []*Cell{{ColIndex: 0, Text: "Revenue"}, {ColIndex: 1, Text: "100"}}},
		},
	}

	tblB := &Table{
		ID:      "tbl_002",
		Title:   "Statement of Operations",
		Columns: colsB,
		Pages:   []int{2},
		Rows: []*Row{
			{Index: 0, Label: "Operating Income", Cells: []*Cell{{ColIndex: 0, Text: "Operating Income"}, {ColIndex: 1, Text: "40"}}},
		},
	}

	stitched := stitcher.Stitch([]*Table{tblA, tblB})
	if len(stitched) != 1 {
		t.Fatalf("expected 1 merged table, got %d", len(stitched))
	}
	if len(stitched[0].Pages) != 2 {
		t.Errorf("expected 2 pages in merged table, got %v", stitched[0].Pages)
	}
	if len(stitched[0].Rows) != 2 {
		t.Errorf("expected 2 rows in merged table, got %d", len(stitched[0].Rows))
	}
}
