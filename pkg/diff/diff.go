package diff

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/grid"
)

var cleanPunctRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// DiffStatus indicates whether line items matched, changed, or restated.
type DiffStatus string

const (
	StatusMatch       DiffStatus = "MATCH"
	StatusRestatement DiffStatus = "RESTATEMENT"
	StatusNewItem     DiffStatus = "NEW_ITEM"
	StatusDroppedItem DiffStatus = "DROPPED_ITEM"
)

// RowDiff represents a reconciled row comparison across two reporting periods.
type RowDiff struct {
	Label         string     `json:"label"`
	GAAPTag       string     `json:"gaap_tag,omitempty"`
	Period        string     `json:"period"`
	BaseValue     float64    `json:"base_value"`
	TargetValue   float64    `json:"target_value"`
	Delta         float64    `json:"delta"`
	DeltaPct      float64    `json:"delta_pct"`
	IsRestatement bool       `json:"is_restatement"`
	Status        DiffStatus `json:"status"`
}

// TableDiff represents the structural and arithmetic diff between two financial statements.
type TableDiff struct {
	BaseTableID        string     `json:"base_table_id"`
	TargetTableID      string     `json:"target_table_id"`
	Title              string     `json:"title"`
	RowDiffs           []*RowDiff `json:"row_diffs"`
	RestatementCount   int        `json:"restatement_count"`
	TotalComparedItems int        `json:"total_compared_items"`
}

// DocumentDiff aggregates comparisons across all tables between two filings.
type DocumentDiff struct {
	BasePath           string       `json:"base_path"`
	TargetPath         string       `json:"target_path"`
	TableDiffs         []*TableDiff `json:"table_diffs"`
	TotalRestatements  int          `json:"total_restatements"`
	TotalComparedItems int          `json:"total_compared_items"`
}

// Engine performs multi-period financial reconciliation and restatement audits.
type Engine struct {
	Tolerance float64
}

// NewEngine creates a Diff Engine with standard penny tolerance.
func NewEngine(tolerance float64) *Engine {
	if tolerance <= 0 {
		tolerance = 0.01
	}
	return &Engine{Tolerance: tolerance}
}

// CompareDocuments compares tables from a base filing against a target filing.
func (e *Engine) CompareDocuments(baseTables, targetTables []*grid.Table, basePath, targetPath string) *DocumentDiff {
	docDiff := &DocumentDiff{
		BasePath:   basePath,
		TargetPath: targetPath,
		TableDiffs: make([]*TableDiff, 0),
	}

	for _, tTbl := range targetTables {
		// Find matching base table by title or index
		var bTbl *grid.Table
		for _, candidate := range baseTables {
			if strings.EqualFold(cleanKey(candidate.Title), cleanKey(tTbl.Title)) {
				bTbl = candidate
				break
			}
		}

		if bTbl != nil {
			tDiff := e.CompareTables(bTbl, tTbl)
			docDiff.TableDiffs = append(docDiff.TableDiffs, tDiff)
			docDiff.TotalRestatements += tDiff.RestatementCount
			docDiff.TotalComparedItems += tDiff.TotalComparedItems
		}
	}

	return docDiff
}

// CompareTables evaluates two tables for historical restatements and changes.
func (e *Engine) CompareTables(baseTbl, targetTbl *grid.Table) *TableDiff {
	tDiff := &TableDiff{
		BaseTableID:   baseTbl.ID,
		TargetTableID: targetTbl.ID,
		Title:         targetTbl.Title,
		RowDiffs:      make([]*RowDiff, 0),
	}

	// Index base rows by GAAP tag and clean label
	baseByGAAP := make(map[string]*grid.Row)
	baseByLabel := make(map[string]*grid.Row)

	for _, r := range baseTbl.DataRows() {
		if r.GAAPTag != "" {
			baseByGAAP[r.GAAPTag] = r
		}
		baseByLabel[cleanKey(r.Label)] = r
	}

	// Identify overlapping historical columns (e.g. "2024" column in both filings)
	commonPeriods := make(map[string]struct {
		baseColIdx   int
		targetColIdx int
	})

	for tIdx, tCol := range targetTbl.Columns {
		if tIdx == 0 {
			continue
		}
		tKey := cleanPeriodKey(tCol.Name)
		for bIdx, bCol := range baseTbl.Columns {
			if bIdx == 0 {
				continue
			}
			bKey := cleanPeriodKey(bCol.Name)
			if tKey != "" && bKey != "" && tKey == bKey {
				commonPeriods[tKey] = struct {
					baseColIdx   int
					targetColIdx int
				}{baseColIdx: bIdx, targetColIdx: tIdx}
			}
		}
	}

	// Compare rows in target table
	for _, tRow := range targetTbl.DataRows() {
		// Match against base table: prefer exact clean row label first, then unique GAAPTag
		var bRow *grid.Row
		if r, ok := baseByLabel[cleanKey(tRow.Label)]; ok {
			bRow = r
		} else if tRow.GAAPTag != "" {
			bRow = baseByGAAP[tRow.GAAPTag]
		}

		if bRow == nil {
			// Item did not exist in base filing
			tDiff.RowDiffs = append(tDiff.RowDiffs, &RowDiff{
				Label:   tRow.Label,
				GAAPTag: tRow.GAAPTag,
				Status:  StatusNewItem,
			})
			continue
		}

		// Check common historical periods for restatements
		for periodName, pair := range commonPeriods {
			bCell := bRow.GetCell(pair.baseColIdx)
			tCell := tRow.GetCell(pair.targetColIdx)

			if bCell != nil && tCell != nil && bCell.IsNumeric && tCell.IsNumeric {
				tDiff.TotalComparedItems++
				delta := tCell.Value - bCell.Value
				isRestatement := math.Abs(delta) > e.Tolerance

				var deltaPct float64
				if bCell.Value != 0 {
					deltaPct = (delta / math.Abs(bCell.Value)) * 100.0
				}

				status := StatusMatch
				if isRestatement {
					status = StatusRestatement
					tDiff.RestatementCount++
				}

				tDiff.RowDiffs = append(tDiff.RowDiffs, &RowDiff{
					Label:         tRow.Label,
					GAAPTag:       tRow.GAAPTag,
					Period:        periodName,
					BaseValue:     bCell.Value,
					TargetValue:   tCell.Value,
					Delta:         delta,
					DeltaPct:      deltaPct,
					IsRestatement: isRestatement,
					Status:        status,
				})
			}
		}
	}

	return tDiff
}

// FormatMarkdown produces a formatted comparison table.
func (td *TableDiff) FormatMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Table Comparison: %s\n\n", td.Title))
	sb.WriteString(fmt.Sprintf("**Compared Items:** %d | **Restatements Detected:** %d\n\n",
		td.TotalComparedItems, td.RestatementCount))

	sb.WriteString("| Line Item | Period | Base (Original) | Target (Restated) | Delta | Delta % | Status |\n")
	sb.WriteString("|:---|:---|---:|---:|---:|---:|:---|\n")

	for _, r := range td.RowDiffs {
		statusBadge := string(r.Status)
		if r.IsRestatement {
			statusBadge = "**RESTATEMENT**"
		}

		sb.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.2f | %.2f | %.2f%% | %s |\n",
			r.Label, r.Period, r.BaseValue, r.TargetValue, r.Delta, r.DeltaPct, statusBadge))
	}

	return sb.String()
}

// ExportJSON writes the diff to a structured JSON file.
func (dd *DocumentDiff) ExportJSON(outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(dd, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

func cleanKey(s string) string {
	lower := strings.ToLower(s)
	return cleanPunctRegex.ReplaceAllString(lower, "")
}

func cleanPeriodKey(colName string) string {
	// Look for 4-digit year in column header
	re := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	if m := re.FindString(colName); m != "" {
		return m
	}
	return cleanKey(colName)
}
