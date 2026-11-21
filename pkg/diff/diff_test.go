package diff

import (
	"strings"
	"testing"

	"github.com/alexandrmotologa/tablerip/pkg/grid"
)

func TestTableDiffAndRestatement(t *testing.T) {
	// Base filing (e.g. 2024 10-K):
	// Column 1: 2024
	baseTable := &grid.Table{
		ID:    "tbl_base",
		Title: "Consolidated Statements of Operations",
		Columns: []*grid.Column{
			{Index: 0, Name: "Line Item"},
			{Index: 1, Name: "2024"},
		},
		Rows: []*grid.Row{
			{
				Index:   0,
				Label:   "Total net sales",
				GAAPTag: "us-gaap:Revenues",
				Cells: []*grid.Cell{
					{ColIndex: 0, RawText: "Total net sales"},
					{ColIndex: 1, Value: 1000.0, IsNumeric: true},
				},
			},
			{
				Index:   1,
				Label:   "Net income",
				GAAPTag: "us-gaap:NetIncomeLoss",
				Cells: []*grid.Cell{
					{ColIndex: 0, RawText: "Net income"},
					{ColIndex: 1, Value: 200.0, IsNumeric: true},
				},
			},
		},
	}

	// Target filing (e.g. 2025 10-K):
	// Column 1: 2025, Column 2: 2024 (where Net income for 2024 is restated to 180.0!)
	targetTable := &grid.Table{
		ID:    "tbl_target",
		Title: "Consolidated Statements of Operations",
		Columns: []*grid.Column{
			{Index: 0, Name: "Line Item"},
			{Index: 1, Name: "2025"},
			{Index: 2, Name: "2024"},
		},
		Rows: []*grid.Row{
			{
				Index:   0,
				Label:   "Total net sales",
				GAAPTag: "us-gaap:Revenues",
				Cells: []*grid.Cell{
					{ColIndex: 0, RawText: "Total net sales"},
					{ColIndex: 1, Value: 1100.0, IsNumeric: true},
					{ColIndex: 2, Value: 1000.0, IsNumeric: true}, // Exact match
				},
			},
			{
				Index:   1,
				Label:   "Net income",
				GAAPTag: "us-gaap:NetIncomeLoss",
				Cells: []*grid.Cell{
					{ColIndex: 0, RawText: "Net income"},
					{ColIndex: 1, Value: 250.0, IsNumeric: true},
					{ColIndex: 2, Value: 180.0, IsNumeric: true}, // Restated from 200 to 180!
				},
			},
		},
	}

	engine := NewEngine(0.01)
	tDiff := engine.CompareTables(baseTable, targetTable)

	if tDiff.TotalComparedItems != 2 {
		t.Fatalf("expected 2 compared items, got %d", tDiff.TotalComparedItems)
	}

	if tDiff.RestatementCount != 1 {
		t.Fatalf("expected 1 restatement, got %d", tDiff.RestatementCount)
	}

	restatementFound := false
	for _, r := range tDiff.RowDiffs {
		if r.GAAPTag == "us-gaap:NetIncomeLoss" {
			if !r.IsRestatement {
				t.Errorf("expected Net income to be flagged as restatement")
			}
			if r.Delta != -20.0 {
				t.Errorf("expected delta -20.0, got %f", r.Delta)
			}
			if r.DeltaPct != -10.0 {
				t.Errorf("expected delta pct -10.0%%, got %f%%", r.DeltaPct)
			}
			restatementFound = true
		}
	}

	if !restatementFound {
		t.Fatalf("Net income restatement record not found")
	}

	md := tDiff.FormatMarkdown()
	if !strings.Contains(md, "RESTATEMENT") {
		t.Errorf("expected markdown report to contain 'RESTATEMENT'")
	}
}
