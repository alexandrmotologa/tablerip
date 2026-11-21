package grid

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

var pageNumberRegex = regexp.MustCompile(`(?i)^(page\s*\d+|f-\d+|\d+\s*of\s*\d+|\d+)$`)
var filingHeaderRegex = regexp.MustCompile(`(?i)(item\s*8|form\s*10-k|form\s*10-q|part\s*i|notes\s*to\s*consolidated)`)

// Stitcher joins fragmented table slices across page breaks.
type Stitcher struct {
	MinSimilarity float64
	Sigma         float64
}

// NewStitcher creates a Stitcher with default threshold values.
func NewStitcher() *Stitcher {
	return &Stitcher{
		MinSimilarity: 0.75,
		Sigma:         15.0, // Gaussian tolerance in points (~1/5 inch)
	}
}

// IsPageArtifact returns true if the row is a page number, running header, or footer.
func (st *Stitcher) IsPageArtifact(label string) bool {
	cleaned := strings.TrimSpace(label)
	if cleaned == "" {
		return false
	}
	if pageNumberRegex.MatchString(cleaned) {
		return true
	}
	if filingHeaderRegex.MatchString(cleaned) {
		return true
	}
	return false
}

// ColumnSimilarity calculates Gaussian alignment similarity between two sets of columns,
// weighting numerical metric columns heavily as they have fixed coordinate alignment.
func (st *Stitcher) ColumnSimilarity(colsA, colsB []*Column) float64 {
	if len(colsA) == 0 || len(colsB) == 0 {
		return 0.0
	}

	// If column counts differ significantly, not a continuation
	if math.Abs(float64(len(colsA)-len(colsB))) > 1 {
		return 0.0
	}

	twoSigmaSq := 2.0 * st.Sigma * st.Sigma
	totalWeight := 0.0
	weightedScore := 0.0

	for i, cA := range colsA {
		// Numerical columns (index >= 1) get double weight
		weight := 1.0
		if i > 0 {
			weight = 2.0
		}
		totalWeight += weight

		maxColScore := 0.0
		centerA := cA.CenterX()

		for _, cB := range colsB {
			centerB := cB.CenterX()
			diff := centerA - centerB
			score := math.Exp(-(diff * diff) / twoSigmaSq)
			if score > maxColScore {
				maxColScore = score
			}
		}

		weightedScore += weight * maxColScore
	}

	if totalWeight == 0 {
		return 0.0
	}

	return weightedScore / totalWeight
}

// Stitch merges tables across multiple pages that belong to the same logical table.
func (st *Stitcher) Stitch(tables []*Table) []*Table {
	if len(tables) <= 1 {
		return tables
	}

	var consolidated []*Table
	current := tables[0]

	for i := 1; i < len(tables); i++ {
		next := tables[i]

		// Check if consecutive pages
		lastPage := current.Pages[len(current.Pages)-1]
		nextFirstPage := next.Pages[0]
		isConsecutive := (nextFirstPage == lastPage+1)

		// Check if next table introduces a distinctly different financial statement title
		isDifferentStatement := false
		if current.Title != "" && next.Title != "" {
			cTitle := strings.ToLower(current.Title)
			nTitle := strings.ToLower(next.Title)
			if !strings.Contains(nTitle, "continued") && !strings.EqualFold(cTitle, nTitle) {
				// e.g. "Operations" vs "Balance Sheets"
				if (strings.Contains(cTitle, "operation") && strings.Contains(nTitle, "balance")) ||
					(strings.Contains(cTitle, "balance") && strings.Contains(nTitle, "cash")) ||
					(strings.Contains(cTitle, "operation") && strings.Contains(nTitle, "cash")) {
					isDifferentStatement = true
				}
			}
		}

		simScore := st.ColumnSimilarity(current.Columns, next.Columns)

		if isConsecutive && !isDifferentStatement && simScore >= st.MinSimilarity {
			// Merge next into current
			st.mergeTables(current, next)
		} else {
			consolidated = append(consolidated, current)
			current = next
		}
	}
	consolidated = append(consolidated, current)

	return consolidated
}

func (st *Stitcher) mergeTables(dest, src *Table) {
	// Add pages
	for _, p := range src.Pages {
		dest.Pages = append(dest.Pages, p)
	}

	// Expand bounding box
	dest.BBox = dest.BBox.Union(src.BBox)

	// Filter out repeated header rows at the start of src
	srcDataRows := src.DataRows()
	var rowsToAppend []*Row

	for _, r := range srcDataRows {
		// Skip page numbers or headers
		if st.IsPageArtifact(r.Label) {
			continue
		}

		// Check if this row is an accidental duplicate of column headers
		isRepeatHeader := false
		for _, col := range dest.Columns {
			if strings.EqualFold(r.Label, col.Name) {
				isRepeatHeader = true
				break
			}
		}
		if isRepeatHeader {
			continue
		}

		rowsToAppend = append(rowsToAppend, r)
	}

	// Re-index appended rows
	baseIdx := len(dest.Rows)
	for idx, r := range rowsToAppend {
		r.Index = baseIdx + idx
		// Re-map row indices in cells
		for _, c := range r.Cells {
			c.RowIndex = r.Index
		}
		dest.Rows = append(dest.Rows, r)
	}

	if dest.Title == "" && src.Title != "" {
		dest.Title = src.Title
	} else if src.Title != "" && !strings.Contains(dest.Title, src.Title) {
		dest.Title = fmt.Sprintf("%s (Continued)", dest.Title)
	}
}
