package grid

import (
	"math"
	"regexp"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

var totalKeywords = regexp.MustCompile(`(?i)\b(total|subtotal|net income|net loss|operating income|operating loss|gross profit|balance at end|balance at close)\b`)
var sectionHeaderKeywords = regexp.MustCompile(`(?i)(assets:|liabilities:|equity:|operating activities|investing activities|financing activities|revenue:|expenses:|costs:)`)

// RowClassifier analyzes token groups and extracts structured table rows with hierarchy.
type RowClassifier struct {
	IndentStep float64
}

// NewRowClassifier creates a RowClassifier.
func NewRowClassifier() *RowClassifier {
	return &RowClassifier{
		IndentStep: 12.0, // Standard 12-point typographic indent
	}
}

// BuildRows converts token rows into structured Row objects with cell values.
func (rc *RowClassifier) BuildRows(
	tokenRows [][]pdf.Token,
	columns []*Column,
	pageNum int,
) []*Row {
	if len(tokenRows) == 0 || len(columns) == 0 {
		return nil
	}

	// Step 1: Find baseline left margin for Column 0 (the row label column)
	minX0 := math.MaxFloat64
	for _, tRow := range tokenRows {
		for _, t := range tRow {
			if t.BBox.CenterX() <= columns[0].X1 {
				if t.BBox.X0 < minX0 {
					minX0 = t.BBox.X0
				}
				break
			}
		}
	}
	if minX0 == math.MaxFloat64 {
		minX0 = columns[0].X0
	}

	var rows []*Row
	var activeSection string
	parentStack := make([]int, 0) // Stack of row indices for parent tracking

	for rIdx, tRow := range tokenRows {
		if len(tRow) == 0 {
			continue
		}

		// Calculate row bounding box
		rowBBox := tRow[0].BBox
		for _, t := range tRow[1:] {
			rowBBox = rowBBox.Union(t.BBox)
		}

		// Distribute tokens into columns
		cellsByCol := make([][]pdf.Token, len(columns))
		for _, t := range tRow {
			bestCol := -1
			maxOverlap := 0.0

			for cIdx, col := range columns {
				overlap := mathOverlap(t.BBox.X0, t.BBox.X1, col.X0, col.X1)
				if overlap > maxOverlap {
					maxOverlap = overlap
					bestCol = cIdx
				}
			}

			// If no strict overlap, find closest column center
			if bestCol == -1 {
				minDist := math.MaxFloat64
				for cIdx, col := range columns {
					dist := math.Abs(t.BBox.CenterX() - col.CenterX())
					if dist < minDist {
						minDist = dist
						bestCol = cIdx
					}
				}
			}

			if bestCol >= 0 {
				cellsByCol[bestCol] = append(cellsByCol[bestCol], t)
			}
		}

		// Extract label from Column 0 tokens
		var labelParts []string
		firstTokenX0 := minX0
		if len(cellsByCol[0]) > 0 {
			firstTokenX0 = cellsByCol[0][0].BBox.X0
			for _, t := range cellsByCol[0] {
				labelParts = append(labelParts, t.Text)
			}
		}
		label := strings.TrimSpace(strings.Join(labelParts, " "))

		// Compute indentation level
		indentOffset := math.Max(0, firstTokenX0-minX0)
		level := int(math.Round(indentOffset / rc.IndentStep))
		if level < 0 {
			level = 0
		}
		if level > 5 {
			level = 5 // Cap nesting depth
		}

		// Check if section header or total
		isTotal := totalKeywords.MatchString(label)
		isSection := sectionHeaderKeywords.MatchString(label) || (len(label) > 0 && label[len(label)-1] == ':')

		// Build Cell objects
		var cells []*Cell
		numMetricCells := 0

		for cIdx := 0; cIdx < len(columns); cIdx++ {
			cTokens := cellsByCol[cIdx]
			var cellText string
			var cellBBox pdf.BBox

			if len(cTokens) > 0 {
				var words []string
				cellBBox = cTokens[0].BBox
				for _, ct := range cTokens {
					words = append(words, ct.Text)
					cellBBox = cellBBox.Union(ct.BBox)
				}
				cellText = strings.TrimSpace(strings.Join(words, " "))
			}

			if cIdx > 0 && cellText != "" && cellText != "-" && cellText != "—" {
				numMetricCells++
			}

			cells = append(cells, &Cell{
				ColIndex:  cIdx,
				RowIndex:  rIdx,
				PageNum:   pageNum,
				Text:      cellText,
				RawText:   cellText,
				BBox:      cellBBox,
				IsNumeric: false, // Will be parsed by auditor/normalizer
			})
		}

		// A row with label but zero numerical values and section keyword is a section header
		if numMetricCells == 0 && (isSection || level == 0 && !isTotal) && len(label) > 0 {
			activeSection = label
		}

		// Manage parent-child hierarchy stack
		for len(parentStack) > level {
			parentStack = parentStack[:len(parentStack)-1]
		}
		parentIdx := -1
		if len(parentStack) > 0 {
			parentIdx = parentStack[len(parentStack)-1]
		}
		parentStack = append(parentStack, rIdx)

		rows = append(rows, &Row{
			Index:       rIdx,
			Level:       level,
			Label:       label,
			Section:     activeSection,
			IsHeader:    false,
			IsTotal:     isTotal,
			PageNum:     pageNum,
			ParentIndex: parentIdx,
			BBox:        rowBBox,
			Cells:       cells,
		})
	}

	return rows
}
