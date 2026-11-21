package pdf

import (
	"math"
	"sort"
)

// ExtractRulingLines analyzes drawn rectangles and converts thin rectangles into line segments.
func ExtractRulingLines(rects []BBox, maxThickness float64) ([]Line, []Line) {
	var horizontalLines []Line
	var verticalLines []Line

	for _, r := range rects {
		w := r.Width()
		h := r.Height()

		// A thin horizontal rectangle is a horizontal rule
		if h <= maxThickness && w > maxThickness*2 {
			yMid := (r.Y0 + r.Y1) / 2.0
			horizontalLines = append(horizontalLines, Line{
				P0:    Point{X: r.X0, Y: yMid},
				P1:    Point{X: r.X1, Y: yMid},
				Width: h,
			})
		}

		// A thin vertical rectangle is a vertical rule
		if w <= maxThickness && h > maxThickness*2 {
			xMid := (r.X0 + r.X1) / 2.0
			verticalLines = append(verticalLines, Line{
				P0:    Point{X: xMid, Y: r.Y0},
				P1:    Point{X: xMid, Y: r.Y1},
				Width: w,
			})
		}
	}

	return MergeHorizontalLines(horizontalLines, 2.0), MergeVerticalLines(verticalLines, 2.0)
}

// MergeHorizontalLines merges collinear or overlapping horizontal line segments.
func MergeHorizontalLines(lines []Line, tolerance float64) []Line {
	if len(lines) <= 1 {
		return lines
	}

	// Sort by Y, then by X0
	sort.Slice(lines, func(i, j int) bool {
		if math.Abs(lines[i].P0.Y-lines[j].P0.Y) > tolerance {
			return lines[i].P0.Y < lines[j].P0.Y
		}
		return lines[i].P0.X < lines[j].P0.X
	})

	var merged []Line
	current := lines[0]

	for i := 1; i < len(lines); i++ {
		next := lines[i]
		// Check if collinear within Y tolerance
		if math.Abs(current.P0.Y-next.P0.Y) <= tolerance {
			// Check if they overlap or are very close horizontally
			if next.P0.X <= current.P1.X+tolerance {
				// Extend current line
				if next.P1.X > current.P1.X {
					current.P1.X = next.P1.X
				}
				continue
			}
		}
		merged = append(merged, current)
		current = next
	}
	merged = append(merged, current)

	return merged
}

// MergeVerticalLines merges collinear or overlapping vertical line segments.
func MergeVerticalLines(lines []Line, tolerance float64) []Line {
	if len(lines) <= 1 {
		return lines
	}

	// Sort by X, then by Y0
	sort.Slice(lines, func(i, j int) bool {
		if math.Abs(lines[i].P0.X-lines[j].P0.X) > tolerance {
			return lines[i].P0.X < lines[j].P0.X
		}
		return lines[i].P0.Y < lines[j].P0.Y
	})

	var merged []Line
	current := lines[0]

	for i := 1; i < len(lines); i++ {
		next := lines[i]
		// Check if collinear within X tolerance
		if math.Abs(current.P0.X-next.P0.X) <= tolerance {
			// Check if they overlap or are very close vertically
			if next.P0.Y <= current.P1.Y+tolerance {
				// Extend current line
				if next.P1.Y > current.P1.Y {
					current.P1.Y = next.P1.Y
				}
				continue
			}
		}
		merged = append(merged, current)
		current = next
	}
	merged = append(merged, current)

	return merged
}
