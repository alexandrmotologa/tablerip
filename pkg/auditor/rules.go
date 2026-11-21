package auditor

import (
	"math"
	"regexp"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/grid"
)

var (
	deductionRegex    = regexp.MustCompile(`(?i)\b(less|minus|deduction|deductions|accumulated depreciation|allowance)\b`)
	assetRegex        = regexp.MustCompile(`(?i)total\s+assets`)
	liabEqRegex       = regexp.MustCompile(`(?i)total\s+liabilities\s+and\s+(?:(?:shareholders|stockholders)['’]?\s+)?equity`)
	revenueRegex      = regexp.MustCompile(`(?i)total\s+(?:net\s+)?(?:sales|revenue)`)
	cogsRegex         = regexp.MustCompile(`(?i)total\s+cost\s+of\s+(?:sales|goods\s+sold|revenue)|cost\s+of\s+(?:sales|goods\s+sold|revenue)`)
	grossProfRegex    = regexp.MustCompile(`(?i)gross\s+(?:profit|margin)`)
	opExpRegex        = regexp.MustCompile(`(?i)total\s+operating\s+expenses`)
	opIncRegex        = regexp.MustCompile(`(?i)operating\s+(?:income|profit|loss)`)
	incBeforeTaxRegex = regexp.MustCompile(`(?i)income\s+before\s+(?:provision\s+for\s+)?(?:income\s+)?taxes`)
	taxProvRegex      = regexp.MustCompile(`(?i)provision\s+for\s+(?:income\s+)?taxes`)
	netIncRegex       = regexp.MustCompile(`(?i)net\s+(?:income|loss|earnings)`)
)

// InvariantCheck represents a verified or flagged mathematical equation.
type InvariantCheck struct {
	Name            string   `json:"name"`
	ColIndex        int      `json:"col_index"`
	TargetRow       int      `json:"target_row"`
	Formula         string   `json:"formula"`
	Expected        float64  `json:"expected"`
	Actual          float64  `json:"actual"`
	Delta           float64  `json:"delta"`
	Passed          bool     `json:"passed"`
	ParticipantRows []int    `json:"participant_rows"`
}

// EvaluateSumOfParts checks vertical summation for a total row and its components,
// supporting both direct item summation and multi-tier subtotal roll-ups.
func EvaluateSumOfParts(totalRow *grid.Row, componentRows []*grid.Row, precedingSubtotals []*grid.Row, colIdx int, tolerance float64) *InvariantCheck {
	totalCell := totalRow.GetCell(colIdx)
	if totalCell == nil || !totalCell.IsNumeric {
		return nil
	}

	sum := 0.0
	var participantRows []int
	var terms []string

	for _, cr := range componentRows {
		cCell := cr.GetCell(colIdx)
		if cCell == nil || !cCell.IsNumeric {
			continue
		}

		sign := 1.0
		if deductionRegex.MatchString(cr.Label) {
			sign = -1.0
			terms = append(terms, "-("+cr.Label+")")
		} else {
			terms = append(terms, cr.Label)
		}

		sum += sign * cCell.Value
		participantRows = append(participantRows, cr.Index)
	}

	// A valid sum must have at least one component row, or at least two subtotals to sum
	if len(participantRows) == 0 {
		if len(precedingSubtotals) < 2 {
			return nil
		}
		// Try summing subtotals
		subSum := 0.0
		var subParticipants []int
		var subTerms []string
		for _, s := range precedingSubtotals {
			sCell := s.GetCell(colIdx)
			if sCell != nil && sCell.IsNumeric {
				subSum += sCell.Value
				subParticipants = append(subParticipants, s.Index)
				subTerms = append(subTerms, s.Label)
			}
		}
		subDelta := math.Abs(totalCell.Value - subSum)
		if subDelta <= tolerance {
			return &InvariantCheck{
				Name:            "Sum of Subtotals",
				ColIndex:        colIdx,
				TargetRow:       totalRow.Index,
				Formula:         strings.Join(subTerms, " + ") + " = " + totalRow.Label,
				Expected:        subSum,
				Actual:          totalCell.Value,
				Delta:           subDelta,
				Passed:          true,
				ParticipantRows: subParticipants,
			}
		}
		return nil
	}

	delta := math.Abs(totalCell.Value - sum)
	passed := delta <= tolerance

	// If direct component sum doesn't match, check if rolling up with preceding subtotal(s) matches
	if !passed && len(precedingSubtotals) > 0 {
		rollUpSum := sum
		var rollUpParticipants []int
		rollUpParticipants = append(rollUpParticipants, participantRows...)
		var rollUpTerms []string
		rollUpTerms = append(rollUpTerms, terms...)

		for _, sub := range precedingSubtotals {
			subCell := sub.GetCell(colIdx)
			if subCell != nil && subCell.IsNumeric {
				rollUpSum += subCell.Value
				rollUpParticipants = append(rollUpParticipants, sub.Index)
				rollUpTerms = append([]string{sub.Label}, rollUpTerms...)
			}
		}

		rollUpDelta := math.Abs(totalCell.Value - rollUpSum)
		if rollUpDelta <= tolerance {
			return &InvariantCheck{
				Name:            "Sum of Parts (with Subtotals)",
				ColIndex:        colIdx,
				TargetRow:       totalRow.Index,
				Formula:         strings.Join(rollUpTerms, " + ") + " = " + totalRow.Label,
				Expected:        rollUpSum,
				Actual:          totalCell.Value,
				Delta:           rollUpDelta,
				Passed:          true,
				ParticipantRows: rollUpParticipants,
			}
		}
	}

	return &InvariantCheck{
		Name:            "Sum of Parts",
		ColIndex:        colIdx,
		TargetRow:       totalRow.Index,
		Formula:         strings.Join(terms, " + ") + " = " + totalRow.Label,
		Expected:        sum,
		Actual:          totalCell.Value,
		Delta:           delta,
		Passed:          passed,
		ParticipantRows: participantRows,
	}
}

// EvaluateBalanceSheetIdentity checks Total Assets == Total Liabilities & Equity.
func EvaluateBalanceSheetIdentity(table *grid.Table, colIdx int, tolerance float64) *InvariantCheck {
	var assetRow *grid.Row
	var liabEqRow *grid.Row

	for _, r := range table.DataRows() {
		if assetRow == nil && assetRegex.MatchString(r.Label) {
			assetRow = r
		} else if liabEqRow == nil && liabEqRegex.MatchString(r.Label) {
			liabEqRow = r
		}
	}

	if assetRow == nil || liabEqRow == nil {
		return nil
	}

	assetCell := assetRow.GetCell(colIdx)
	liabEqCell := liabEqRow.GetCell(colIdx)

	if assetCell == nil || !assetCell.IsNumeric || liabEqCell == nil || !liabEqCell.IsNumeric {
		return nil
	}

	delta := math.Abs(assetCell.Value - liabEqCell.Value)
	passed := delta <= tolerance

	return &InvariantCheck{
		Name:            "Balance Sheet Identity (Assets = Liabilities + Equity)",
		ColIndex:        colIdx,
		TargetRow:       assetRow.Index,
		Formula:         assetRow.Label + " = " + liabEqRow.Label,
		Expected:        liabEqCell.Value,
		Actual:          assetCell.Value,
		Delta:           delta,
		Passed:          passed,
		ParticipantRows: []int{assetRow.Index, liabEqRow.Index},
	}
}

// EvaluateIncomeStatementInvariants checks Gross Profit, Operating Income, and Net Income identities.
func EvaluateIncomeStatementInvariants(table *grid.Table, colIdx int, tolerance float64) []*InvariantCheck {
	var revRow, cogsRow, grossRow, opExpRow, opIncRow, incBeforeTaxRow, taxProvRow, netIncRow *grid.Row

	for _, r := range table.DataRows() {
		if revRow == nil && revenueRegex.MatchString(r.Label) {
			revRow = r
		} else if cogsRow == nil && cogsRegex.MatchString(r.Label) {
			cogsRow = r
		} else if grossRow == nil && grossProfRegex.MatchString(r.Label) {
			grossRow = r
		} else if opExpRow == nil && opExpRegex.MatchString(r.Label) {
			opExpRow = r
		} else if opIncRow == nil && opIncRegex.MatchString(r.Label) {
			opIncRow = r
		} else if incBeforeTaxRow == nil && incBeforeTaxRegex.MatchString(r.Label) {
			incBeforeTaxRow = r
		} else if taxProvRow == nil && taxProvRegex.MatchString(r.Label) {
			taxProvRow = r
		} else if netIncRow == nil && netIncRegex.MatchString(r.Label) {
			netIncRow = r
		}
	}

	var checks []*InvariantCheck

	// Check 1: Gross Profit = Revenue - COGS
	if revRow != nil && cogsRow != nil && grossRow != nil {
		rCell := revRow.GetCell(colIdx)
		cCell := cogsRow.GetCell(colIdx)
		gCell := grossRow.GetCell(colIdx)

		if rCell != nil && rCell.IsNumeric && cCell != nil && cCell.IsNumeric && gCell != nil && gCell.IsNumeric {
			expected := rCell.Value - math.Abs(cCell.Value)
			delta := math.Abs(gCell.Value - expected)
			checks = append(checks, &InvariantCheck{
				Name:            "Gross Profit (Revenue - Cost of Sales)",
				ColIndex:        colIdx,
				TargetRow:       grossRow.Index,
				Formula:         revRow.Label + " - " + cogsRow.Label + " = " + grossRow.Label,
				Expected:        expected,
				Actual:          gCell.Value,
				Delta:           delta,
				Passed:          delta <= tolerance,
				ParticipantRows: []int{revRow.Index, cogsRow.Index, grossRow.Index},
			})
		}
	}

	// Check 2: Operating Income = Gross Profit - Operating Expenses
	if grossRow != nil && opExpRow != nil && opIncRow != nil {
		gCell := grossRow.GetCell(colIdx)
		eCell := opExpRow.GetCell(colIdx)
		iCell := opIncRow.GetCell(colIdx)

		if gCell != nil && gCell.IsNumeric && eCell != nil && eCell.IsNumeric && iCell != nil && iCell.IsNumeric {
			expected := gCell.Value - math.Abs(eCell.Value)
			delta := math.Abs(iCell.Value - expected)
			checks = append(checks, &InvariantCheck{
				Name:            "Operating Income (Gross Profit - Operating Expenses)",
				ColIndex:        colIdx,
				TargetRow:       opIncRow.Index,
				Formula:         grossRow.Label + " - " + opExpRow.Label + " = " + opIncRow.Label,
				Expected:        expected,
				Actual:          iCell.Value,
				Delta:           delta,
				Passed:          delta <= tolerance,
				ParticipantRows: []int{grossRow.Index, opExpRow.Index, opIncRow.Index},
			})
		}
	}

	// Check 3: Net Income = Income Before Tax - Tax Provision
	if incBeforeTaxRow != nil && taxProvRow != nil && netIncRow != nil {
		bCell := incBeforeTaxRow.GetCell(colIdx)
		tCell := taxProvRow.GetCell(colIdx)
		nCell := netIncRow.GetCell(colIdx)

		if bCell != nil && bCell.IsNumeric && tCell != nil && tCell.IsNumeric && nCell != nil && nCell.IsNumeric {
			expected := bCell.Value - math.Abs(tCell.Value)
			delta := math.Abs(nCell.Value - expected)
			checks = append(checks, &InvariantCheck{
				Name:            "Net Income (Income Before Taxes - Tax Provision)",
				ColIndex:        colIdx,
				TargetRow:       netIncRow.Index,
				Formula:         incBeforeTaxRow.Label + " - " + taxProvRow.Label + " = " + netIncRow.Label,
				Expected:        expected,
				Actual:          nCell.Value,
				Delta:           delta,
				Passed:          delta <= tolerance,
				ParticipantRows: []int{incBeforeTaxRow.Index, taxProvRow.Index, netIncRow.Index},
			})
		}
	}

	return checks
}
