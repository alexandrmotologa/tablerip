package grid

import (
	"math"
	"sort"

	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

// Interval represents a 1D continuous range [Start, End].
type Interval struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Length returns the length of the interval.
func (in Interval) Length() float64 {
	return math.Max(0, in.End-in.Start)
}

// Center returns the midpoint of the interval.
func (in Interval) Center() float64 {
	return (in.Start + in.End) / 2.0
}

// Segmenter executes spatial projection profiling (X-Cut and Y-Cut).
type Segmenter struct {
	BinSize       float64
	MinGutter     float64
	RowSeparation float64
}

// NewSegmenter creates a Segmenter with sensible default geometric thresholds.
func NewSegmenter() *Segmenter {
	return &Segmenter{
		BinSize:       1.0,  // 1-point discretization
		MinGutter:     8.0,  // Minimum gutter width between columns in points
		RowSeparation: 2.0,  // Minimum vertical separation between rows in points
	}
}

// HorizontalProjection computes H(y) histogram across the specified bounding box.
func (s *Segmenter) HorizontalProjection(tokens []pdf.Token, yMin, yMax float64) []int {
	if yMax <= yMin {
		return nil
	}
	numBins := int(math.Ceil((yMax - yMin) / s.BinSize))
	hist := make([]int, numBins)

	for _, t := range tokens {
		tY0 := math.Max(t.BBox.Y0, yMin)
		tY1 := math.Min(t.BBox.Y1, yMax)
		if tY1 <= tY0 {
			continue
		}

		binStart := int((tY0 - yMin) / s.BinSize)
		binEnd := int((tY1 - yMin) / s.BinSize)

		if binStart < 0 {
			binStart = 0
		}
		if binEnd >= numBins {
			binEnd = numBins - 1
		}

		for b := binStart; b <= binEnd; b++ {
			hist[b]++
		}
	}

	return hist
}

// VerticalProjection computes V(x) histogram across the specified bounding box.
func (s *Segmenter) VerticalProjection(tokens []pdf.Token, xMin, xMax float64) []int {
	if xMax <= xMin {
		return nil
	}
	numBins := int(math.Ceil((xMax - xMin) / s.BinSize))
	hist := make([]int, numBins)

	for _, t := range tokens {
		tX0 := math.Max(t.BBox.X0, xMin)
		tX1 := math.Min(t.BBox.X1, xMax)
		if tX1 <= tX0 {
			continue
		}

		binStart := int((tX0 - xMin) / s.BinSize)
		binEnd := int((tX1 - xMin) / s.BinSize)

		if binStart < 0 {
			binStart = 0
		}
		if binEnd >= numBins {
			binEnd = numBins - 1
		}

		for b := binStart; b <= binEnd; b++ {
			hist[b]++
		}
	}

	return hist
}

// FindRowBands extracts vertical row intervals from horizontal projection histogram valleys.
func (s *Segmenter) FindRowBands(tokens []pdf.Token, yMin, yMax float64) []Interval {
	hist := s.HorizontalProjection(tokens, yMin, yMax)
	if len(hist) == 0 {
		return nil
	}

	var bands []Interval
	inBand := false
	var bandStart float64

	for i, count := range hist {
		yPos := yMin + float64(i)*s.BinSize

		if count > 0 && !inBand {
			inBand = true
			bandStart = yPos
		} else if count == 0 && inBand {
			inBand = false
			bandEnd := yPos
			if bandEnd-bandStart >= 3.0 { // Minimum row height
				bands = append(bands, Interval{Start: bandStart, End: bandEnd})
			}
		}
	}

	if inBand {
		bands = append(bands, Interval{Start: bandStart, End: yMax})
	}

	return bands
}

// FindColumnChannels extracts column horizontal intervals using adaptive valley thresholding.
func (s *Segmenter) FindColumnChannels(tokens []pdf.Token, xMin, xMax float64, vLines []pdf.Line) []Interval {
	hist := s.VerticalProjection(tokens, xMin, xMax)
	if len(hist) == 0 {
		return nil
	}

	maxCount := 0
	for _, c := range hist {
		if c > maxCount {
			maxCount = c
		}
	}

	// Adaptive threshold: a gutter valley is where density drops below 20% of peak density
	// or 0 when maxCount is low.
	valleyThreshold := 1
	if maxCount >= 4 {
		valleyThreshold = int(math.Ceil(float64(maxCount) * 0.18))
	}

	// Step 1: Detect column content blocks (where hist >= valleyThreshold)
	var contentBlocks []Interval
	inBlock := false
	var blockStart float64

	for i, count := range hist {
		xPos := xMin + float64(i)*s.BinSize

		if count >= valleyThreshold && !inBlock {
			inBlock = true
			blockStart = xPos
		} else if count < valleyThreshold && inBlock {
			inBlock = false
			blockEnd := xPos
			contentBlocks = append(contentBlocks, Interval{Start: blockStart, End: blockEnd})
		}
	}
	if inBlock {
		contentBlocks = append(contentBlocks, Interval{Start: blockStart, End: xMax})
	}

	if len(contentBlocks) == 0 {
		return nil
	}

	// Step 2: Merge adjacent blocks if the gutter between them is smaller than MinGutter
	var merged []Interval
	curr := contentBlocks[0]

	for i := 1; i < len(contentBlocks); i++ {
		next := contentBlocks[i]
		gutter := next.Start - curr.End

		if gutter < s.MinGutter {
			curr.End = next.End
		} else {
			merged = append(merged, curr)
			curr = next
		}
	}
	merged = append(merged, curr)

	// Step 3: Expand intervals to meet at the midpoints of gutters so every token is covered
	var columns []Interval
	for i := 0; i < len(merged); i++ {
		col := merged[i]
		leftBound := col.Start
		rightBound := col.End

		if i > 0 {
			prevEnd := merged[i-1].End
			leftBound = (prevEnd + col.Start) / 2.0
		} else {
			leftBound = math.Max(0, col.Start-s.MinGutter/2.0)
		}

		if i < len(merged)-1 {
			nextStart := merged[i+1].Start
			rightBound = (col.End + nextStart) / 2.0
		} else {
			rightBound = col.End + s.MinGutter/2.0
		}

		columns = append(columns, Interval{Start: leftBound, End: rightBound})
	}

	return columns
}

// ClusterTokensIntoRows groups tokens into row slices based on detected vertical bands.
func (s *Segmenter) ClusterTokensIntoRows(tokens []pdf.Token, rowBands []Interval) [][]pdf.Token {
	rows := make([][]pdf.Token, len(rowBands))

	for _, t := range tokens {
		tCenterY := t.BBox.CenterY()
		bestRow := -1
		minDist := math.MaxFloat64

		for rIdx, band := range rowBands {
			dist := math.Abs(tCenterY - band.Center())
			if tCenterY >= band.Start && tCenterY <= band.End {
				bestRow = rIdx
				minDist = dist
				break
			}
			if dist < minDist {
				minDist = dist
				bestRow = rIdx
			}
		}

		if bestRow >= 0 && minDist < 15.0 {
			rows[bestRow] = append(rows[bestRow], t)
		}
	}

	// Sort tokens in each row from left to right
	for rIdx := range rows {
		sort.Slice(rows[rIdx], func(i, j int) bool {
			return rows[rIdx][i].BBox.X0 < rows[rIdx][j].BBox.X0
		})
	}

	return rows
}
