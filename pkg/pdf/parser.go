package pdf

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"rsc.io/pdf"
)

type matrix [3][3]float64

var identMatrix = matrix{
	{1, 0, 0},
	{0, 1, 0},
	{0, 0, 1},
}

func (m matrix) mul(n matrix) matrix {
	var r matrix
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			r[i][j] = m[i][0]*n[0][j] + m[i][1]*n[1][j] + m[i][2]*n[2][j]
		}
	}
	return r
}

func (m matrix) transformPoint(p Point) Point {
	return Point{
		X: p.X*m[0][0] + p.Y*m[1][0] + m[2][0],
		Y: p.X*m[0][1] + p.Y*m[1][1] + m[2][1],
	}
}

type gstate struct {
	Tc    float64
	Tw    float64
	Th    float64
	Tl    float64
	Tf    pdf.Font
	Tfs   float64
	Tmode int
	Trise float64
	Tm    matrix
	Tlm   matrix
	CTM   matrix
}

// Token represents an atomic or clustered text element on a PDF page.
type Token struct {
	Text     string  `json:"text"`
	BBox     BBox    `json:"bbox"`
	Font     string  `json:"font"`
	FontSize float64 `json:"font_size"`
	PageNum  int     `json:"page_num"`
}

// PageData contains the extracted vector tokens, ruling lines, and dimensions of a page.
type PageData struct {
	PageNum         int     `json:"page_num"`
	Width           float64 `json:"width"`
	Height          float64 `json:"height"`
	Tokens          []Token `json:"tokens"`
	HorizontalLines []Line  `json:"horizontal_lines"`
	VerticalLines   []Line  `json:"vertical_lines"`
	Rects           []BBox  `json:"rects"`
}

// Document holds the parsed pages and metadata of a PDF file.
type Document struct {
	Path     string      `json:"path"`
	NumPages int         `json:"num_pages"`
	Pages    []*PageData `json:"pages"`
}

// Open reads a PDF file from disk and parses each page's geometric tokens and lines.
func Open(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF file '%s': %w", path, err)
	}

	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse PDF '%s': %w", path, err)
	}

	numPages := reader.NumPage()
	doc := &Document{
		Path:     path,
		NumPages: numPages,
		Pages:    make([]*PageData, 0, numPages),
	}

	for pageIdx := 1; pageIdx <= numPages; pageIdx++ {
		page := reader.Page(pageIdx)
		pageData := extractPage(page, pageIdx)
		doc.Pages = append(doc.Pages, pageData)
	}

	return doc, nil
}

func extractPage(page pdf.Page, pageNum int) *PageData {
	width, height := getPageDimensions(page)
	strm := page.V.Key("Contents")

	var g = gstate{
		Th:  1,
		CTM: identMatrix,
		Tm:  identMatrix,
		Tlm: identMatrix,
		Tfs: 10,
	}

	var rawTokens []Token
	var rects []BBox
	var lines []Line
	var gstack []gstate
	var currentPath []Point

	var enc pdf.TextEncoding

	showText := func(s string) {
		var decoded string
		if enc != nil {
			decoded = enc.Decode(s)
		} else {
			decoded = s
		}

		baseFont := g.Tf.BaseFont()
		if idx := strings.Index(baseFont, "+"); idx >= 0 {
			baseFont = baseFont[idx+1:]
		}

		// Buffer words directly on the current line
		var wordRunes []rune
		var wordX0, wordX1, wordY0, wordY1 float64
		wordStarted := false

		flushWord := func() {
			if len(wordRunes) == 0 {
				return
			}
			textStr := strings.TrimSpace(string(wordRunes))
			if textStr != "" {
				// Convert to top-left origin coordinates
				layoutY0 := height - wordY1
				layoutY1 := height - wordY0

				rawTokens = append(rawTokens, Token{
					Text:     textStr,
					BBox:     NewBBox(wordX0, layoutY0, wordX1, layoutY1),
					Font:     baseFont,
					FontSize: g.Tfs,
					PageNum:  pageNum,
				})
			}
			wordRunes = nil
			wordStarted = false
		}

		n := 0
		for _, ch := range decoded {
			trm := matrix{
				{g.Tfs * g.Th, 0, 0},
				{0, g.Tfs, 0},
				{0, g.Trise, 1},
			}.mul(g.Tm).mul(g.CTM)

			var code int
			if n < len(s) {
				code = int(s[n])
				n++
			} else {
				code = int(ch)
			}

			w0 := g.Tf.Width(code)
			if w0 <= 0 {
				w0 = standardCharWidth(ch, baseFont)
			}

			glyphX := trm[2][0]
			glyphY := trm[2][1]
			glyphW := w0 / 1000.0 * trm[0][0]
			glyphH := g.Tfs

			if ch == ' ' {
				flushWord()
			} else {
				if !wordStarted {
					wordStarted = true
					wordX0 = glyphX
					wordX1 = glyphX + glyphW
					wordY0 = glyphY
					wordY1 = glyphY + glyphH
				} else {
					if glyphX+glyphW > wordX1 {
						wordX1 = glyphX + glyphW
					}
					if glyphY+glyphH > wordY1 {
						wordY1 = glyphY + glyphH
					}
					if glyphY < wordY0 {
						wordY0 = glyphY
					}
				}
				wordRunes = append(wordRunes, ch)
			}

			tx := (w0/1000.0*g.Tfs + g.Tc)
			if ch == ' ' {
				tx += g.Tw
			}
			tx *= g.Th
			g.Tm = matrix{{1, 0, 0}, {0, 1, 0}, {tx, 0, 1}}.mul(g.Tm)
		}

		flushWord()
	}

	pdf.Interpret(strm, func(stk *pdf.Stack, op string) {
		n := stk.Len()
		args := make([]pdf.Value, n)
		for i := n - 1; i >= 0; i-- {
			args[i] = stk.Pop()
		}

		switch op {
		case "cm": // Concat CTM
			if len(args) == 6 {
				var m matrix
				for i := 0; i < 6; i++ {
					m[i/2][i%2] = args[i].Float64()
				}
				m[2][2] = 1
				g.CTM = m.mul(g.CTM)
			}

		case "q":
			gstack = append(gstack, g)
		case "Q":
			if len(gstack) > 0 {
				g = gstack[len(gstack)-1]
				gstack = gstack[:len(gstack)-1]
			}

		case "BT":
			g.Tm = identMatrix
			g.Tlm = identMatrix

		case "ET":
			// End text

		case "T*":
			x := matrix{{1, 0, 0}, {0, 1, 0}, {0, -g.Tl, 1}}
			g.Tlm = x.mul(g.Tlm)
			g.Tm = g.Tlm

		case "Tc":
			if len(args) >= 1 {
				g.Tc = args[0].Float64()
			}

		case "Tw":
			if len(args) >= 1 {
				g.Tw = args[0].Float64()
			}

		case "Td", "TD":
			if len(args) >= 2 {
				tx := args[0].Float64()
				ty := args[1].Float64()
				if op == "TD" {
					g.Tl = -ty
				}
				x := matrix{{1, 0, 0}, {0, 1, 0}, {tx, ty, 1}}
				g.Tlm = x.mul(g.Tlm)
				g.Tm = g.Tlm
			}

		case "Tf":
			if len(args) >= 2 {
				fontName := args[0].Name()
				g.Tf = page.Font(fontName)
				enc = g.Tf.Encoder()
				g.Tfs = args[1].Float64()
			}

		case "TL":
			if len(args) >= 1 {
				g.Tl = args[0].Float64()
			}

		case "Tm":
			if len(args) == 6 {
				var m matrix
				for i := 0; i < 6; i++ {
					m[i/2][i%2] = args[i].Float64()
				}
				m[2][2] = 1
				g.Tm = m
				g.Tlm = m
			}

		case "Tj":
			if len(args) >= 1 {
				showText(args[0].RawString())
			}

		case "TJ":
			if len(args) >= 1 {
				v := args[0]
				for i := 0; i < v.Len(); i++ {
					item := v.Index(i)
					if item.Kind() == pdf.String {
						showText(item.RawString())
					} else {
						tx := -item.Float64() / 1000.0 * g.Tfs * g.Th
						g.Tm = matrix{{1, 0, 0}, {0, 1, 0}, {tx, 0, 1}}.mul(g.Tm)
					}
				}
			}

		case "re":
			if len(args) == 4 {
				x, y, w, h := args[0].Float64(), args[1].Float64(), args[2].Float64(), args[3].Float64()
				p0 := g.CTM.transformPoint(Point{X: x, Y: y + h})
				p1 := g.CTM.transformPoint(Point{X: x + w, Y: y})
				rects = append(rects, NewBBox(p0.X, height-p0.Y, p1.X, height-p1.Y))
			}

		case "m": // Move to
			if len(args) >= 2 {
				p := g.CTM.transformPoint(Point{X: args[0].Float64(), Y: args[1].Float64()})
				currentPath = []Point{p}
			}

		case "l": // Line to
			if len(args) >= 2 && len(currentPath) > 0 {
				p := g.CTM.transformPoint(Point{X: args[0].Float64(), Y: args[1].Float64()})
				pPrev := currentPath[len(currentPath)-1]
				lines = append(lines, Line{
					P0:    Point{X: pPrev.X, Y: height - pPrev.Y},
					P1:    Point{X: p.X, Y: height - p.Y},
					Width: 1.0,
				})
				currentPath = append(currentPath, p)
			}

		case "S", "s", "b", "B": // Stroke path
			currentPath = nil
		}
	})

	// Separate and merge horizontal/vertical lines from vector paths and rects
	hLinesFromRects, vLinesFromRects := ExtractRulingLines(rects, 3.0)

	var hLines []Line
	var vLines []Line

	for _, l := range lines {
		if l.IsHorizontal(2.0) {
			hLines = append(hLines, l)
		} else if l.IsVertical(2.0) {
			vLines = append(vLines, l)
		}
	}

	hLines = append(hLines, hLinesFromRects...)
	vLines = append(vLines, vLinesFromRects...)

	mergedHLines := MergeHorizontalLines(hLines, 2.0)
	mergedVLines := MergeVerticalLines(vLines, 2.0)

	// Sort tokens by Y, then X
	sort.Slice(rawTokens, func(i, j int) bool {
		if math.Abs(rawTokens[i].BBox.Y0-rawTokens[j].BBox.Y0) > 3.0 {
			return rawTokens[i].BBox.Y0 < rawTokens[j].BBox.Y0
		}
		return rawTokens[i].BBox.X0 < rawTokens[j].BBox.X0
	})

	return &PageData{
		PageNum:         pageNum,
		Width:           width,
		Height:          height,
		Tokens:          rawTokens,
		HorizontalLines: mergedHLines,
		VerticalLines:   mergedVLines,
		Rects:           rects,
	}
}

func standardCharWidth(ch rune, baseFont string) float64 {
	if strings.Contains(strings.ToLower(baseFont), "courier") {
		return 600.0
	}
	switch {
	case ch == ' ':
		return 278.0
	case ch >= '0' && ch <= '9':
		return 556.0
	case ch == '.' || ch == ',' || ch == ':' || ch == ';':
		return 278.0
	case ch == '(' || ch == ')':
		return 333.0
	case ch == '-' || ch == '–' || ch == '—':
		return 333.0
	case ch == '$' || ch == '€' || ch == '£' || ch == '%':
		return 556.0
	case ch >= 'a' && ch <= 'z':
		switch ch {
		case 'i', 'l':
			return 222.0
		case 'f', 't', 'j', 'r':
			return 333.0
		case 'm', 'w':
			return 833.0
		default:
			return 556.0
		}
	case ch >= 'A' && ch <= 'Z':
		switch ch {
		case 'I', 'J':
			return 278.0
		case 'M', 'W':
			return 833.0
		default:
			return 667.0
		}
	default:
		return 500.0
	}
}

func getPageDimensions(page pdf.Page) (float64, float64) {
	mb := page.V.Key("MediaBox")
	if mb.Kind() == pdf.Array && mb.Len() == 4 {
		x0 := mb.Index(0).Float64()
		y0 := mb.Index(1).Float64()
		x1 := mb.Index(2).Float64()
		y1 := mb.Index(3).Float64()
		w := math.Abs(x1 - x0)
		h := math.Abs(y1 - y0)
		if w > 0 && h > 0 {
			return w, h
		}
	}

	parent := page.V.Key("Parent")
	if !parent.IsNull() {
		mb = parent.Key("MediaBox")
		if mb.Kind() == pdf.Array && mb.Len() == 4 {
			x0 := mb.Index(0).Float64()
			y0 := mb.Index(1).Float64()
			x1 := mb.Index(2).Float64()
			y1 := mb.Index(3).Float64()
			w := math.Abs(x1 - x0)
			h := math.Abs(y1 - y0)
			if w > 0 && h > 0 {
				return w, h
			}
		}
	}

	return 612.0, 792.0
}
