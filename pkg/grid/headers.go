package grid

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

var (
	yearRegex   = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	periodRegex = regexp.MustCompile(`(?i)(month|year|quarter|ended|period|as of|september|december|march|june|january|february|april|may|july|august|october|november)`)
	auditRegex  = regexp.MustCompile(`(?i)(unaudited|audited|restated|adjusted|pro\s*forma)`)
)

// HeaderResolver reconstructs hierarchical multi-tier column headers.
type HeaderResolver struct{}

// NewHeaderResolver creates a new HeaderResolver.
func NewHeaderResolver() *HeaderResolver {
	return &HeaderResolver{}
}

// IsHeaderToken evaluates if a text token resembles header metadata.
func (hr *HeaderResolver) IsHeaderToken(text string) bool {
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return false
	}
	if yearRegex.MatchString(cleaned) {
		return true
	}
	if periodRegex.MatchString(cleaned) {
		return true
	}
	if auditRegex.MatchString(cleaned) {
		return true
	}
	return false
}

// ResolveHeaders analyzes candidate header token rows and constructs composite column names.
func (hr *HeaderResolver) ResolveHeaders(
	headerTokenRows [][]pdf.Token,
	colIntervals []Interval,
) []*Column {
	numCols := len(colIntervals)
	columns := make([]*Column, numCols)

	for cIdx, interval := range colIntervals {
		columns[cIdx] = &Column{
			Index: cIdx,
			X0:    interval.Start,
			X1:    interval.End,
		}
	}

	if len(headerTokenRows) == 0 {
		// Fallback column names
		for cIdx, col := range columns {
			if cIdx == 0 {
				col.Name = "Line Item"
			} else {
				col.Name = fmt.Sprintf("Column_%d", cIdx)
			}
		}
		return columns
	}

	// For each column, gather header text across all header tiers
	for cIdx, col := range columns {
		var tiers []string

		for _, hRow := range headerTokenRows {
			var rowTokens []string
			for _, t := range hRow {
				// Check if token overlaps with this column
				overlap := mathOverlap(t.BBox.X0, t.BBox.X1, col.X0, col.X1)
				tokenWidth := t.BBox.Width()

				// If token significantly overlaps or spans over this column
				if (tokenWidth > 0 && overlap/tokenWidth >= 0.3) ||
					(t.BBox.CenterX() >= col.X0 && t.BBox.CenterX() <= col.X1) {
					rowTokens = append(rowTokens, strings.TrimSpace(t.Text))
				}
			}

			tierStr := strings.TrimSpace(strings.Join(rowTokens, " "))
			if tierStr != "" {
				tiers = append(tiers, tierStr)
			}
		}

		col.TierNames = tiers

		if cIdx == 0 {
			if len(tiers) > 0 && !yearRegex.MatchString(tiers[0]) {
				col.Name = tiers[len(tiers)-1]
			} else {
				col.Name = "Line Item"
			}
		} else {
			if len(tiers) > 0 {
				col.Name = sanitizeColumnName(strings.Join(tiers, "."))
			} else {
				col.Name = fmt.Sprintf("Column_%d", cIdx)
			}
		}
	}

	return columns
}

func mathOverlap(minA, maxA, minB, maxB float64) float64 {
	overlap := mathMin(maxA, maxB) - mathMax(minA, minB)
	if overlap < 0 {
		return 0
	}
	return overlap
}

func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func mathMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func sanitizeColumnName(name string) string {
	// Clean extra spaces, non-alphanumeric punctuation except periods
	parts := strings.Split(name, ".")
	var cleanParts []string

	for _, p := range parts {
		cleaned := strings.TrimSpace(p)
		// Replace internal multiple spaces with single space
		cleaned = strings.Join(strings.Fields(cleaned), " ")
		if cleaned != "" {
			cleanParts = append(cleanParts, cleaned)
		}
	}

	if len(cleanParts) == 0 {
		return "Unnamed"
	}

	return strings.Join(cleanParts, ".")
}
