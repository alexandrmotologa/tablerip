package fixtures

import (
	"fmt"

	"github.com/jung-kurt/gofpdf"
)

// GenerateSynthetic10K creates an authentic multi-page SEC 10-K PDF filing
// with mathematical invariants, cross-page tables, and hierarchical indentation.
func GenerateSynthetic10K(outputPath string) error {
	pdf := gofpdf.New("P", "pt", "Letter", "") // 612 x 792 pt
	pdf.SetMargins(36, 36, 36)
	pdf.SetAutoPageBreak(false, 0)

	// ==========================================
	// PAGE 1: Statements of Operations (Part 1)
	// ==========================================
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 8)
	pdf.Text(36, 30, "ITEM 8. FINANCIAL STATEMENTS AND SUPPLEMENTARY DATA")
	pdf.Text(540, 30, "Page 67")

	// Table Title
	pdf.SetFont("Helvetica", "B", 12)
	pdf.Text(150, 65, "CONSOLIDATED STATEMENTS OF OPERATIONS")

	pdf.SetFont("Helvetica", "I", 9)
	pdf.Text(210, 80, "(in millions, except per share data)")

	// Table Headers
	pdf.SetFont("Helvetica", "B", 9)
	// Tier 1: Spanning header
	pdf.Text(350, 105, "Three Months Ended September 30,")
	// Ruling line under tier 1
	pdf.Line(320, 110, 560, 110)

	// Tier 2: Sub-columns
	pdf.Text(370, 125, "2025")
	pdf.Text(480, 125, "2024")
	pdf.Line(36, 130, 560, 130)

	// Rows on Page 1
	y := 148.0
	pdf.SetFont("Helvetica", "", 9)

	// Revenue section
	pdf.Text(36, y, "Revenue:")
	y += 18
	pdf.Text(50, y, "Products")
	pdf.Text(370, y, "69,958")
	pdf.Text(480, y, "67,184")
	y += 18
	pdf.Text(50, y, "Services")
	pdf.Text(370, y, "24,972")
	pdf.Text(480, y, "22,314")
	y += 18

	// Total Net Sales
	pdf.SetFont("Helvetica", "B", 9)
	pdf.Line(360, y-10, 410, y-10)
	pdf.Line(470, y-10, 520, y-10)
	pdf.Text(36, y, "Total net sales")
	pdf.Text(370, y, "94,930")
	pdf.Text(480, y, "89,498")
	y += 24

	// Cost of sales
	pdf.SetFont("Helvetica", "", 9)
	pdf.Text(36, y, "Cost of sales:")
	y += 18
	pdf.Text(50, y, "Products cost")
	pdf.Text(370, y, "44,520")
	pdf.Text(480, y, "42,850")
	y += 18
	pdf.Text(50, y, "Services cost")
	pdf.Text(370, y, "7,800")
	pdf.Text(480, y, "6,800")
	y += 18

	pdf.SetFont("Helvetica", "B", 9)
	pdf.Line(360, y-10, 410, y-10)
	pdf.Line(470, y-10, 520, y-10)
	pdf.Text(36, y, "Total cost of sales")
	pdf.Text(370, y, "52,320")
	pdf.Text(480, y, "49,650")
	y += 24

	// Gross Profit
	pdf.Text(36, y, "Gross profit")
	pdf.Text(370, y, "42,610")
	pdf.Text(480, y, "39,848")
	y += 24

	// Operating expenses
	pdf.SetFont("Helvetica", "", 9)
	pdf.Text(36, y, "Operating expenses:")
	y += 18
	pdf.Text(50, y, "Research and development")
	pdf.Text(370, y, "7,764")
	pdf.Text(480, y, "7,307")
	y += 18
	pdf.Text(50, y, "Selling, general and administrative")
	pdf.Text(370, y, "6,520")
	pdf.Text(480, y, "6,151")
	y += 18

	pdf.SetFont("Helvetica", "B", 9)
	pdf.Line(360, y-10, 410, y-10)
	pdf.Line(470, y-10, 520, y-10)
	pdf.Text(36, y, "Total operating expenses")
	pdf.Text(370, y, "14,284")
	pdf.Text(480, y, "13,458")
	y += 24

	// Operating income
	pdf.Text(36, y, "Operating income")
	pdf.Text(370, y, "28,326")
	pdf.Text(480, y, "26,390")

	// ==========================================
	// PAGE 2: Statements of Operations (Part 2 - Continuation)
	// ==========================================
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 8)
	pdf.Text(36, 30, "ITEM 8. FINANCIAL STATEMENTS AND SUPPLEMENTARY DATA")
	pdf.Text(540, 30, "Page 68")

	pdf.SetFont("Helvetica", "B", 12)
	pdf.Text(150, 65, "CONSOLIDATED STATEMENTS OF OPERATIONS (Continued)")

	// Re-aligned headers on continuation page
	pdf.SetFont("Helvetica", "B", 9)
	pdf.Text(350, 105, "Three Months Ended September 30,")
	pdf.Line(320, 110, 560, 110)
	pdf.Text(370, 125, "2025")
	pdf.Text(480, 125, "2024")
	pdf.Line(36, 130, 560, 130)

	y = 150.0
	pdf.SetFont("Helvetica", "", 9)
	pdf.Text(36, y, "Other income, net")
	pdf.Text(370, y, "192")
	pdf.Text(480, y, "231")
	y += 22

	pdf.SetFont("Helvetica", "B", 9)
	pdf.Text(36, y, "Income before provision for income taxes")
	pdf.Text(370, y, "28,518")
	pdf.Text(480, y, "26,621")
	y += 22

	pdf.SetFont("Helvetica", "", 9)
	pdf.Text(36, y, "Provision for income taxes")
	pdf.Text(370, y, "4,284")
	pdf.Text(480, y, "4,050")
	y += 24

	pdf.SetFont("Helvetica", "B", 9)
	pdf.Line(360, y-10, 410, y-10)
	pdf.Line(470, y-10, 520, y-10)
	pdf.Text(36, y, "Net income")
	pdf.Text(370, y, "24,234")
	pdf.Text(480, y, "22,571")
	pdf.Line(360, y+4, 410, y+4)
	pdf.Line(360, y+6, 410, y+6)
	pdf.Line(470, y+4, 520, y+4)
	pdf.Line(470, y+6, 520, y+6)

	// ==========================================
	// PAGE 3: Balance Sheet
	// ==========================================
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 8)
	pdf.Text(36, 30, "ITEM 8. FINANCIAL STATEMENTS AND SUPPLEMENTARY DATA")
	pdf.Text(540, 30, "Page 69")

	pdf.SetFont("Helvetica", "B", 12)
	pdf.Text(190, 65, "CONSOLIDATED BALANCE SHEETS")

	pdf.SetFont("Helvetica", "I", 9)
	pdf.Text(210, 80, "(in millions, except number of shares)")

	// Headers
	pdf.SetFont("Helvetica", "B", 9)
	pdf.Text(360, 105, "September 30, 2025")
	pdf.Text(480, 105, "September 30, 2024")
	pdf.Line(36, 115, 560, 115)

	y = 135.0
	// Assets
	pdf.SetFont("Helvetica", "B", 9)
	pdf.Text(36, y, "ASSETS:")
	y += 18
	pdf.SetFont("Helvetica", "", 9)
	pdf.Text(50, y, "Cash and cash equivalents")
	pdf.Text(370, y, "29,943")
	pdf.Text(480, y, "29,965")
	y += 18
	pdf.Text(50, y, "Marketable securities")
	pdf.Text(370, y, "35,274")
	pdf.Text(480, y, "31,590")
	y += 18
	pdf.Text(50, y, "Accounts receivable, net")
	pdf.Text(370, y, "69,968")
	pdf.Text(480, y, "60,985")
	y += 18
	pdf.SetFont("Helvetica", "B", 9)
	pdf.Line(360, y-10, 410, y-10)
	pdf.Line(470, y-10, 520, y-10)
	pdf.Text(36, y, "Total current assets")
	pdf.Text(370, y, "135,185")
	pdf.Text(480, y, "122,540")
	y += 24

	pdf.SetFont("Helvetica", "", 9)
	pdf.Text(50, y, "Property, plant and equipment, net")
	pdf.Text(370, y, "45,248")
	pdf.Text(480, y, "43,715")
	y += 18
	pdf.Text(50, y, "Other non-current assets")
	pdf.Text(370, y, "172,150")
	pdf.Text(480, y, "186,328")
	y += 24

	pdf.SetFont("Helvetica", "B", 9)
	pdf.Line(360, y-10, 410, y-10)
	pdf.Line(470, y-10, 520, y-10)
	pdf.Text(36, y, "Total assets")
	pdf.Text(370, y, "352,583")
	pdf.Text(480, y, "352,583")
	pdf.Line(360, y+4, 410, y+4)
	pdf.Line(360, y+6, 410, y+6)
	pdf.Line(470, y+4, 520, y+4)
	pdf.Line(470, y+6, 520, y+6)
	y += 35

	// Liabilities and Shareholders' Equity
	pdf.Text(36, y, "LIABILITIES AND SHAREHOLDERS' EQUITY:")
	y += 18
	pdf.SetFont("Helvetica", "", 9)
	pdf.Text(50, y, "Total liabilities")
	pdf.Text(370, y, "285,683")
	pdf.Text(480, y, "285,683")
	y += 18
	pdf.Text(50, y, "Total shareholders' equity")
	pdf.Text(370, y, "66,900")
	pdf.Text(480, y, "66,900")
	y += 24

	pdf.SetFont("Helvetica", "B", 9)
	pdf.Line(360, y-10, 410, y-10)
	pdf.Line(470, y-10, 520, y-10)
	pdf.Text(36, y, "Total liabilities and shareholders' equity")
	pdf.Text(370, y, "352,583")
	pdf.Text(480, y, "352,583")
	pdf.Line(360, y+4, 410, y+4)
	pdf.Line(360, y+6, 410, y+6)
	pdf.Line(470, y+4, 520, y+4)
	pdf.Line(470, y+6, 520, y+6)

	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return fmt.Errorf("failed to write synthetic 10-K PDF: %w", err)
	}

	return nil
}
