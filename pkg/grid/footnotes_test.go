package grid

import (
	"testing"

	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

func TestFootnoteLinker(t *testing.T) {
	linker := NewFootnoteLinker()

	footerTokens := []pdf.Token{
		{Text: "(1)", BBox: pdf.NewBBox(36, 500, 50, 510)},
		{Text: "Includes", BBox: pdf.NewBBox(55, 500, 100, 510)},
		{Text: "stock-based", BBox: pdf.NewBBox(105, 500, 170, 510)},
		{Text: "compensation", BBox: pdf.NewBBox(175, 500, 250, 510)},
		{Text: "of", BBox: pdf.NewBBox(255, 500, 270, 510)},
		{Text: "$452M", BBox: pdf.NewBBox(275, 500, 310, 510)},
	}

	footnotes := linker.ExtractFootnotes(footerTokens)
	if len(footnotes) != 1 {
		t.Fatalf("expected 1 footnote, got %d", len(footnotes))
	}
	if footnotes[0].Marker != "(1)" {
		t.Errorf("expected marker '(1)', got '%s'", footnotes[0].Marker)
	}
	if footnotes[0].Text != "Includes stock-based compensation of $452M" {
		t.Errorf("unexpected footnote text: '%s'", footnotes[0].Text)
	}

	// Test Linking to cells
	table := &Table{
		Rows: []*Row{
			{
				Label: "Research and development (1)",
				Cells: []*Cell{
					{ColIndex: 0, RawText: "Research and development (1)"},
					{ColIndex: 1, RawText: "7,764 (1)"},
				},
			},
		},
	}

	linker.LinkFootnotesToTable(table, footnotes)

	if table.Rows[0].FootnoteMarker != "(1)" {
		t.Errorf("expected row marker '(1)', got '%s'", table.Rows[0].FootnoteMarker)
	}
	if table.Rows[0].FootnoteText != "Includes stock-based compensation of $452M" {
		t.Errorf("expected footnote text on row, got '%s'", table.Rows[0].FootnoteText)
	}
	if table.Rows[0].Cells[1].FootnoteMarker != "(1)" {
		t.Errorf("expected cell marker '(1)', got '%s'", table.Rows[0].Cells[1].FootnoteMarker)
	}
}
