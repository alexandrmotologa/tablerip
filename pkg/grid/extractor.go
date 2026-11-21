package grid

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/pdf"
)

var (
	tableTitleRegex   = regexp.MustCompile(`(?i)(consolidated|condensed|statement|balance sheet|operations|cash flows|shareholders' equity|comprehensive income|segment)`)
	scaleRegex        = regexp.MustCompile(`(?i)\(in\s+(millions|thousands|billions)(?:,\s*except[^\)]+)?\)`)
	marginHeaderRegex = regexp.MustCompile(`(?i)(item\s*\d|page\s*\d|f-\d+|\d+\s*of\s*\d+|form\s*10|financial statements)`)
	numericTokenRegex = regexp.MustCompile(`[\d]{1,3}(?:,\d{3})*(?:\.\d+)?|\([\d,]+\.?\d*\)`)
)

// Extractor coordinates topological grid reconstruction from PDF pages.
type Extractor struct {
	segmenter      *Segmenter
	headerResolver *HeaderResolver
	rowClassifier  *RowClassifier
	stitcher       *Stitcher
}

// NewExtractor creates a fully configured Extractor.
func NewExtractor() *Extractor {
	return &Extractor{
		segmenter:      NewSegmenter(),
		headerResolver: NewHeaderResolver(),
		rowClassifier:  NewRowClassifier(),
		stitcher:       NewStitcher(),
	}
}

// ExtractFromDocument extracts all financial tables across all pages in a document,
// stitching multi-page continuation tables seamlessly.
func (e *Extractor) ExtractFromDocument(doc *pdf.Document) ([]*Table, error) {
	if doc == nil || len(doc.Pages) == 0 {
		return nil, fmt.Errorf("document has no pages")
	}

	var allTables []*Table

	for _, page := range doc.Pages {
		pageTables := e.ExtractFromPage(page)
		allTables = append(allTables, pageTables...)
	}

	// Stitch tables that span across consecutive pages
	stitched := e.stitcher.Stitch(allTables)

	// Assign unique IDs
	for i, t := range stitched {
		t.ID = fmt.Sprintf("tbl_%03d", i+1)
		if t.Title == "" {
			t.Title = fmt.Sprintf("Financial Table %d", i+1)
		}
	}

	return stitched, nil
}

// ExtractFromPage extracts tables present on a single PDF page.
func (e *Extractor) ExtractFromPage(page *pdf.PageData) []*Table {
	if page == nil || len(page.Tokens) == 0 {
		return nil
	}

	// Step 1: Detect potential table title and scale annotation
	var title string
	var scaleUnit string
	var scaleMult float64 = 1.0

	// Step 2: Filter out running header and footer artifacts at page margins
	var pageTokens []pdf.Token
	for _, t := range page.Tokens {
		// Filter top/bottom margin running headers
		if t.BBox.Y1 <= 45.0 || t.BBox.Y0 >= page.Height-45.0 {
			if marginHeaderRegex.MatchString(t.Text) || pageNumberRegex.MatchString(t.Text) {
				continue
			}
		}

		// Detect scale
		if m := scaleRegex.FindStringSubmatch(t.Text); len(m) > 1 {
			scaleUnit = strings.ToLower(m[1])
			switch scaleUnit {
			case "millions":
				scaleMult = 1e6
			case "thousands":
				scaleMult = 1e3
			case "billions":
				scaleMult = 1e9
			}
		}

		pageTokens = append(pageTokens, t)
	}

	if len(pageTokens) == 0 {
		return nil
	}

	// Step 3: Find row bands using horizontal projection
	minY := math.MaxFloat64
	maxY := 0.0
	minX := math.MaxFloat64
	maxX := 0.0

	for _, t := range pageTokens {
		if t.BBox.Y0 < minY {
			minY = t.BBox.Y0
		}
		if t.BBox.Y1 > maxY {
			maxY = t.BBox.Y1
		}
		if t.BBox.X0 < minX {
			minX = t.BBox.X0
		}
		if t.BBox.X1 > maxX {
			maxX = t.BBox.X1
		}
	}

	rowBands := e.segmenter.FindRowBands(pageTokens, minY, maxY)
	if len(rowBands) < 3 {
		return nil
	}

	// Step 4: Cluster tokens into rows
	tokenRows := e.segmenter.ClusterTokensIntoRows(pageTokens, rowBands)

	// Step 5: Isolate table rows (skip stand-alone title / annotation rows)
	var candidateTableRows [][]pdf.Token
	var numericTokens []pdf.Token

	for _, tRow := range tokenRows {
		if len(tRow) == 0 {
			continue
		}

		var rowWords []string
		for _, tok := range tRow {
			rowWords = append(rowWords, tok.Text)
		}
		fullRowText := strings.TrimSpace(strings.Join(rowWords, " "))

		// Capture full table title
		if title == "" && tableTitleRegex.MatchString(fullRowText) && tRow[0].FontSize >= 10.0 {
			title = fullRowText
			continue
		}

		// Skip standalone title or scale annotation from table body
		if tableTitleRegex.MatchString(fullRowText) || scaleRegex.MatchString(fullRowText) {
			continue
		}

		candidateTableRows = append(candidateTableRows, tRow)

		// Check if row has numbers
		rowHasNumbers := false
		for _, tok := range tRow {
			if numericTokenRegex.MatchString(tok.Text) {
				rowHasNumbers = true
				break
			}
		}
		if rowHasNumbers {
			numericTokens = append(numericTokens, tRow...)
		}
	}

	if len(candidateTableRows) < 2 {
		return nil
	}

	// Step 6: Detect column channels
	// Prefer numeric rows for gutter detection to prevent spanning headers from bridging columns
	tokensForColumns := numericTokens
	if len(tokensForColumns) < 4 {
		tokensForColumns = pageTokens
	}

	colChannels := e.segmenter.FindColumnChannels(tokensForColumns, minX, maxX, page.VerticalLines)
	if len(colChannels) < 2 {
		return nil
	}

	// Step 7: Separate header rows from data rows
	var headerRows [][]pdf.Token
	var dataRows [][]pdf.Token
	headerPhase := true

	for _, tRow := range candidateTableRows {
		isHeader := false
		if headerPhase {
			hasPeriodOrYear := false
			for _, t := range tRow {
				if e.headerResolver.IsHeaderToken(t.Text) {
					hasPeriodOrYear = true
					break
				}
			}

			if hasPeriodOrYear {
				isHeader = true
			} else if len(headerRows) > 0 {
				headerPhase = false
			}
		}

		if isHeader {
			headerRows = append(headerRows, tRow)
		} else {
			dataRows = append(dataRows, tRow)
		}
	}

	// Resolve composite column headers
	columns := e.headerResolver.ResolveHeaders(headerRows, colChannels)

	// Build structured data rows with indentation hierarchy
	structuredRows := e.rowClassifier.BuildRows(dataRows, columns, page.PageNum)
	if len(structuredRows) == 0 {
		return nil
	}

	tableBBox := pdf.NewBBox(minX, minY, maxX, maxY)

	table := &Table{
		Title:     title,
		Columns:   columns,
		Rows:      structuredRows,
		Pages:     []int{page.PageNum},
		BBox:      tableBBox,
		ScaleUnit: scaleUnit,
		ScaleMult: scaleMult,
	}

	// Extract and link footnotes below data rows
	var footerTokens []pdf.Token
	lastRowY1 := tableBBox.Y1
	if len(structuredRows) > 0 {
		lastRowY1 = structuredRows[len(structuredRows)-1].BBox.Y1
	}

	for _, t := range pageTokens {
		if t.BBox.Y0 >= lastRowY1-2.0 && t.BBox.Y1 > lastRowY1 {
			footerTokens = append(footerTokens, t)
		}
	}

	fnLinker := NewFootnoteLinker()
	footnotes := fnLinker.ExtractFootnotes(footerTokens)
	if len(footnotes) > 0 {
		fnLinker.LinkFootnotesToTable(table, footnotes)
	}

	return []*Table{table}
}
