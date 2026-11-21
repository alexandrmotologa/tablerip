package auditor

import (
	"testing"
)

func TestMapLabelToGAAP(t *testing.T) {
	tests := []struct {
		label    string
		section  string
		expected string
	}{
		{"Total net sales", "", "us-gaap:Revenues"},
		{"Cost of sales", "", "us-gaap:CostOfGoodsAndServicesSold"},
		{"Gross profit", "", "us-gaap:GrossProfit"},
		{"Research and development", "", "us-gaap:ResearchAndDevelopmentExpense"},
		{"Operating income", "", "us-gaap:OperatingIncomeLoss"},
		{"Net income", "", "us-gaap:NetIncomeLoss"},
		{"Cash and cash equivalents", "ASSETS:", "us-gaap:CashAndCashEquivalentsAtCarryingValue"},
		{"Total current assets", "ASSETS:", "us-gaap:AssetsCurrent"},
		{"Total assets", "ASSETS:", "us-gaap:Assets"},
		{"Total liabilities", "LIABILITIES:", "us-gaap:Liabilities"},
		{"Total shareholders' equity", "EQUITY:", "us-gaap:StockholdersEquity"},
		{"Total liabilities and shareholders' equity", "", "us-gaap:LiabilitiesAndStockholdersEquity"},
		{"Random custom description", "", ""},
	}

	for _, tc := range tests {
		tag := MapLabelToGAAP(tc.label, tc.section)
		if tag != tc.expected {
			t.Errorf("label '%s' (section '%s'): expected GAAP tag '%s', got '%s'",
				tc.label, tc.section, tc.expected, tag)
		}
	}
}
