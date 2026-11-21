package auditor

import (
	"testing"

	"github.com/alexandrmotologa/tablerip/pkg/grid"
)

func TestFinancialNumberNormalizer(t *testing.T) {
	tests := []struct {
		input       string
		expectedVal float64
		expectedNum bool
		expectedCur string
	}{
		{"$1,234.50", 1234.50, true, "USD"},
		{"(1,234.50)", -1234.50, true, ""},
		{"$(450)", -450.0, true, "USD"},
		{"€ 9,800.00", 9800.0, true, "EUR"},
		{"1,500-", -1500.0, true, ""},
		{"-", 0.0, true, ""},
		{"—", 0.0, true, ""},
		{"Non-numeric text", 0.0, false, ""},
	}

	for _, tc := range tests {
		val, isNum, cur := ParseFinancialNumber(tc.input)
		if isNum != tc.expectedNum {
			t.Errorf("input '%s': expected isNum %v, got %v", tc.input, tc.expectedNum, isNum)
		}
		if isNum && val != tc.expectedVal {
			t.Errorf("input '%s': expected val %f, got %f", tc.input, tc.expectedVal, val)
		}
		if tc.expectedCur != "" && cur != tc.expectedCur {
			t.Errorf("input '%s': expected cur '%s', got '%s'", tc.input, tc.expectedCur, cur)
		}
	}
}

func TestAuditorSumOfParts(t *testing.T) {
	auditor := NewAuditor(0.01, true)

	// Additive revenue component table:
	// Products: 600
	// Services: 400
	// Total net sales: 1000 (Total)
	table := &grid.Table{
		ID: "tbl_001",
		Columns: []*grid.Column{
			{Index: 0, Name: "Line Item"},
			{Index: 1, Name: "2025"},
		},
		Rows: []*grid.Row{
			{
				Index: 0,
				Label: "Products",
				Cells: []*grid.Cell{{ColIndex: 0, Text: "Products"}, {ColIndex: 1, RawText: "600"}},
			},
			{
				Index: 1,
				Label: "Services",
				Cells: []*grid.Cell{{ColIndex: 0, Text: "Services"}, {ColIndex: 1, RawText: "400"}},
			},
			{
				Index:   2,
				Label:   "Total net sales",
				IsTotal: true,
				Cells:   []*grid.Cell{{ColIndex: 0, Text: "Total net sales"}, {ColIndex: 1, RawText: "1,000"}},
			},
		},
	}

	report := auditor.AuditTable(table)
	if report.DiscrepancyCount != 0 {
		t.Fatalf("expected 0 discrepancies for balanced table, got %d", report.DiscrepancyCount)
	}
	if !report.Passed {
		t.Fatalf("expected audit to pass")
	}

	// Now introduce an arithmetic discrepancy: Total net sales = 900 instead of 1000
	table.Rows[2].Cells[1].RawText = "900"
	reportWithDisc := auditor.AuditTable(table)
	if reportWithDisc.DiscrepancyCount == 0 {
		t.Fatalf("expected discrepancy to be detected")
	}
	if reportWithDisc.Passed {
		t.Fatalf("expected audit to fail on discrepancy")
	}
}

func TestAuditorIncomeStatementInvariants(t *testing.T) {
	auditor := NewAuditor(0.01, true)

	// Revenue: 1000
	// Cost of sales: 400
	// Gross profit: 600
	table := &grid.Table{
		ID: "tbl_is",
		Columns: []*grid.Column{
			{Index: 0, Name: "Line Item"},
			{Index: 1, Name: "2025"},
		},
		Rows: []*grid.Row{
			{
				Index: 0,
				Label: "Total net sales",
				Cells: []*grid.Cell{{ColIndex: 0, Text: "Total net sales"}, {ColIndex: 1, RawText: "1,000"}},
			},
			{
				Index: 1,
				Label: "Cost of sales",
				Cells: []*grid.Cell{{ColIndex: 0, Text: "Cost of sales"}, {ColIndex: 1, RawText: "400"}},
			},
			{
				Index:   2,
				Label:   "Gross profit",
				IsTotal: true,
				Cells:   []*grid.Cell{{ColIndex: 0, Text: "Gross profit"}, {ColIndex: 1, RawText: "600"}},
			},
		},
	}

	report := auditor.AuditTable(table)
	if report.DiscrepancyCount != 0 {
		t.Fatalf("expected 0 discrepancies for gross profit, got %d", report.DiscrepancyCount)
	}
	if !report.Passed {
		t.Fatalf("expected income statement audit to pass")
	}
}

func TestAuditorBalanceSheetIdentity(t *testing.T) {
	auditor := NewAuditor(0.01, true)

	table := &grid.Table{
		ID: "tbl_bs",
		Columns: []*grid.Column{
			{Index: 0, Name: "Line Item"},
			{Index: 1, Name: "2025"},
		},
		Rows: []*grid.Row{
			{
				Index: 0,
				Label: "Total assets",
				Cells: []*grid.Cell{{ColIndex: 0, Text: "Total assets"}, {ColIndex: 1, RawText: "352,583"}},
			},
			{
				Index: 1,
				Label: "Total liabilities and shareholders' equity",
				Cells: []*grid.Cell{{ColIndex: 0, Text: "Total liabilities and shareholders' equity"}, {ColIndex: 1, RawText: "352,583"}},
			},
		},
	}

	report := auditor.AuditTable(table)
	if !report.Passed {
		t.Fatalf("expected balance sheet identity to pass")
	}
	if report.PassedCount != 1 {
		t.Errorf("expected 1 passed check, got %d", report.PassedCount)
	}
}
