package auditor

import (
	"fmt"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/grid"
)

// AuditReport details the mathematical balance verification of a table.
type AuditReport struct {
	TableID          string            `json:"table_id"`
	TotalChecked     int               `json:"total_checked"`
	PassedCount      int               `json:"passed_count"`
	DiscrepancyCount int               `json:"discrepancy_count"`
	ConfidenceScore  float64           `json:"confidence_score"`
	Passed           bool              `json:"passed"`
	Checks           []*InvariantCheck `json:"checks"`
}

// Auditor executes arithmetic verification over extracted tables.
type Auditor struct {
	Tolerance float64
	Strict    bool
}

// NewAuditor creates an Auditor with standard tolerance.
func NewAuditor(tolerance float64, strict bool) *Auditor {
	if tolerance <= 0 {
		tolerance = 0.01 // Standard financial penny tolerance
	}
	return &Auditor{
		Tolerance: tolerance,
		Strict:    strict,
	}
}

// AuditTable normalizes numbers and executes all mathematical reconciliation rules.
func (a *Auditor) AuditTable(table *grid.Table) *AuditReport {
	if table == nil {
		return nil
	}

	// Step 1: Normalize all cell numbers
	a.normalizeTableCells(table)

	report := &AuditReport{
		TableID: table.ID,
		Checks:  make([]*InvariantCheck, 0),
	}

	dataRows := table.DataRows()
	if len(dataRows) == 0 || len(table.Columns) < 2 {
		report.Passed = true
		report.ConfidenceScore = 1.0
		return report
	}

	// Step 2: Vertical summations for total rows
	var currentBlock []*grid.Row
	var precedingSubtotals []*grid.Row
	var currentSection string

	for _, row := range dataRows {
		if row.Section != "" && row.Section != currentSection {
			currentSection = row.Section
			precedingSubtotals = nil
			currentBlock = nil
		}

		if row.IsTotal {
			// If row is a known multi-term identity, let specialized invariant check handle it
			isDedicatedIdentity := grossProfRegex.MatchString(row.Label) || opIncRegex.MatchString(row.Label) || netIncRegex.MatchString(row.Label)

			if !isDedicatedIdentity {
				for colIdx := 1; colIdx < len(table.Columns); colIdx++ {
					check := EvaluateSumOfParts(row, currentBlock, precedingSubtotals, colIdx, a.Tolerance)
					if check != nil {
						report.Checks = append(report.Checks, check)
					}
				}
			}
			// Save as subtotal for subsequent grand totals, and reset current block
			precedingSubtotals = append(precedingSubtotals, row)
			currentBlock = nil
		} else {
			// Add non-empty line item to current component block
			if strings.TrimSpace(row.Label) != "" {
				currentBlock = append(currentBlock, row)
			}
		}
	}

	// Step 3: Check table-wide identities across all value columns
	for colIdx := 1; colIdx < len(table.Columns); colIdx++ {
		// Balance sheet identity
		if bsCheck := EvaluateBalanceSheetIdentity(table, colIdx, a.Tolerance); bsCheck != nil {
			report.Checks = append(report.Checks, bsCheck)
		}

		// Income statement identities
		isChecks := EvaluateIncomeStatementInvariants(table, colIdx, a.Tolerance)
		report.Checks = append(report.Checks, isChecks...)
	}

	// Step 4: Aggregate statistics and mark cells
	passedCount := 0
	discCount := 0

	for _, check := range report.Checks {
		report.TotalChecked++
		if check.Passed {
			passedCount++
		} else {
			discCount++
		}

		// Update cell status
		if check.TargetRow >= 0 && check.TargetRow < len(table.Rows) {
			targetRow := table.Rows[check.TargetRow]
			if targetCell := targetRow.GetCell(check.ColIndex); targetCell != nil {
				targetCell.Audited = true
				if !check.Passed {
					targetCell.AuditPass = false
				} else if !targetCell.Audited || targetCell.AuditPass {
					targetCell.AuditPass = true
				}
			}
		}

		for _, pIdx := range check.ParticipantRows {
			if pIdx >= 0 && pIdx < len(table.Rows) {
				pRow := table.Rows[pIdx]
				if pCell := pRow.GetCell(check.ColIndex); pCell != nil {
					pCell.Audited = true
					if !check.Passed {
						pCell.AuditPass = false
					} else if !pCell.AuditPass && pCell.Audited {
						// Keep existing failure if any
					} else {
						pCell.AuditPass = true
					}
				}
			}
		}
	}

	report.PassedCount = passedCount
	report.DiscrepancyCount = discCount

	if report.TotalChecked > 0 {
		report.ConfidenceScore = float64(passedCount) / float64(report.TotalChecked)
		report.Passed = (discCount == 0)
	} else {
		report.ConfidenceScore = 1.0
		report.Passed = true
	}

	return report
}

func (a *Auditor) normalizeTableCells(table *grid.Table) {
	for _, row := range table.Rows {
		tag := MapLabelToGAAP(row.Label, row.Section)
		row.GAAPTag = tag

		for _, cell := range row.Cells {
			val, isNum, cur := ParseFinancialNumber(cell.RawText)
			cell.Value = val
			cell.IsNumeric = isNum
			cell.Currency = cur
			cell.GAAPTag = tag
			cell.Audited = false
			cell.AuditPass = true
		}
	}
}

// FormatSummary generates a readable text summary of the audit report.
func (rep *AuditReport) FormatSummary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== ARITHMETIC AUDIT REPORT: %s ===\n", rep.TableID))
	sb.WriteString(fmt.Sprintf("Status: %s (Confidence: %.1f%%)\n", map[bool]string{true: "PASSED", false: "FAILED"}[rep.Passed], rep.ConfidenceScore*100))
	sb.WriteString(fmt.Sprintf("Checked Invariants: %d | Passed: %d | Discrepancies: %d\n", rep.TotalChecked, rep.PassedCount, rep.DiscrepancyCount))

	if len(rep.Checks) > 0 {
		sb.WriteString("\nInvariant Evaluations:\n")
		for _, c := range rep.Checks {
			symbol := "[PASS]"
			if !c.Passed {
				symbol = "[FAIL]"
			}
			sb.WriteString(fmt.Sprintf("  %s %s (Col %d): Expected %.2f vs Actual %.2f (Delta: %.4f)\n",
				symbol, c.Name, c.ColIndex, c.Expected, c.Actual, c.Delta))
		}
	}

	return sb.String()
}
