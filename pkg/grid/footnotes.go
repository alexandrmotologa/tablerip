package grid

import (
	"regexp"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

// Footnote represents an explanatory footnote parsed from table margins or notes.
type Footnote struct {
	Marker string   `json:"marker"`
	Text   string   `json:"text"`
	BBox   pdf.BBox `json:"bbox"`
}

var (
	// Matches footnote definitions e.g. "(1) Includes stock compensation..." or "* Unaudited..."
	footnoteDefRegex = regexp.MustCompile(`^(\(\d+\)|\([a-z]\)|\*|†)\s*(.+)`)
	// Matches footnote markers attached at the end of cell text e.g. "94,930 (1)" or "Revenue *"
	footnoteMarkerRegex = regexp.MustCompile(`[\s\(]*(\(\d+\)|\([a-z]\)|\*|†)\)?$`)
)

// FootnoteLinker extracts footnotes below table structures and links them to corresponding cells.
type FootnoteLinker struct{}

// NewFootnoteLinker creates a FootnoteLinker.
func NewFootnoteLinker() *FootnoteLinker {
	return &FootnoteLinker{}
}

// ExtractFootnotes finds footnote definition lines below a table's data rows.
func (fl *FootnoteLinker) ExtractFootnotes(footerTokens []pdf.Token) []*Footnote {
	if len(footerTokens) == 0 {
		return nil
	}

	// Cluster tokens on the same horizontal baseline into lines
	var lines [][]pdf.Token
	var currentLine []pdf.Token

	for _, t := range footerTokens {
		if len(currentLine) == 0 {
			currentLine = append(currentLine, t)
			continue
		}

		prev := currentLine[len(currentLine)-1]
		if mathAbs(t.BBox.Y0-prev.BBox.Y0) <= 3.5 {
			currentLine = append(currentLine, t)
		} else {
			lines = append(lines, currentLine)
			currentLine = []pdf.Token{t}
		}
	}
	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}

	var footnotes []*Footnote
	for _, l := range lines {
		var words []string
		lineBBox := l[0].BBox
		for _, t := range l {
			words = append(words, t.Text)
			lineBBox = lineBBox.Union(t.BBox)
		}
		fullText := strings.TrimSpace(strings.Join(words, " "))

		if m := footnoteDefRegex.FindStringSubmatch(fullText); len(m) > 2 {
			footnotes = append(footnotes, &Footnote{
				Marker: m[1],
				Text:   strings.TrimSpace(m[2]),
				BBox:   lineBBox,
			})
		}
	}

	return footnotes
}

// LinkFootnotesToTable inspects rows and cells, matching any footnote marker to the table's footnotes.
func (fl *FootnoteLinker) LinkFootnotesToTable(table *Table, footnotes []*Footnote) {
	if table == nil || len(footnotes) == 0 {
		return
	}

	table.Footnotes = footnotes
	footnoteMap := make(map[string]string)
	for _, fn := range footnotes {
		footnoteMap[fn.Marker] = fn.Text
	}

	for _, r := range table.Rows {
		// Check row label
		if m := footnoteMarkerRegex.FindStringSubmatch(r.Label); len(m) > 1 {
			marker := m[1]
			if txt, ok := footnoteMap[marker]; ok {
				r.FootnoteMarker = marker
				r.FootnoteText = txt
			}
		}

		// Check cells
		for _, c := range r.Cells {
			if m := footnoteMarkerRegex.FindStringSubmatch(c.RawText); len(m) > 1 {
				marker := m[1]
				if txt, ok := footnoteMap[marker]; ok {
					c.FootnoteMarker = marker
					c.FootnoteText = txt
				}
			}
		}
	}
}

func mathAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
