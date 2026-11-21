package exporter

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alexandrmotologa/tablerip/pkg/grid"
	_ "modernc.org/sqlite"
)

var identSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// SQLiteExporter creates structured SQLite database tables with citations.
type SQLiteExporter struct{}

// NewSQLiteExporter creates a new SQLiteExporter.
func NewSQLiteExporter() *SQLiteExporter {
	return &SQLiteExporter{}
}

// ExportTable writes a table and its citations to a SQLite database.
func (se *SQLiteExporter) ExportTable(table *grid.Table, dbPath string, tablePrefix string) error {
	if table == nil {
		return fmt.Errorf("table is nil")
	}

	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database: %w", err)
	}
	defer db.Close()

	return se.ExportTableToDB(table, db, tablePrefix)
}

// ExportTableToDB writes the table directly to an open sql.DB connection.
func (se *SQLiteExporter) ExportTableToDB(table *grid.Table, db *sql.DB, tablePrefix string) error {
	tableName := sanitizeIdentifier(tablePrefix + table.ID)
	citationTableName := tableName + "_citations"

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Build column definitions for data table
	var colDefs []string
	colDefs = append(colDefs, "row_id INTEGER PRIMARY KEY")
	colDefs = append(colDefs, "hierarchy_level INTEGER")
	colDefs = append(colDefs, "row_label TEXT NOT NULL")
	colDefs = append(colDefs, "section TEXT")
	colDefs = append(colDefs, "is_total BOOLEAN")

	var valueColNames []string
	for colIdx := 1; colIdx < len(table.Columns); colIdx++ {
		cName := sanitizeIdentifier("col_" + table.Columns[colIdx].Name)
		colDefs = append(colDefs, fmt.Sprintf("%s REAL", cName))
		valueColNames = append(valueColNames, cName)
	}

	createDataSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s);", tableName, strings.Join(colDefs, ", "))
	if _, err := tx.Exec(createDataSQL); err != nil {
		return fmt.Errorf("failed to create data table: %w", err)
	}

	createCitationSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		citation_id INTEGER PRIMARY KEY AUTOINCREMENT,
		row_id INTEGER,
		col_index INTEGER,
		col_name TEXT,
		page_number INTEGER,
		bbox_x0 REAL,
		bbox_y0 REAL,
		bbox_x1 REAL,
		bbox_y1 REAL,
		raw_text TEXT,
		value REAL,
		audited INTEGER,
		audit_passed INTEGER
	);`, citationTableName)
	if _, err := tx.Exec(createCitationSQL); err != nil {
		return fmt.Errorf("failed to create citations table: %w", err)
	}

	// Prepare data insert
	placeholders := make([]string, 5+len(valueColNames))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	insertCols := []string{"row_id", "hierarchy_level", "row_label", "section", "is_total"}
	insertCols = append(insertCols, valueColNames...)

	insertDataSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);",
		tableName, strings.Join(insertCols, ", "), strings.Join(placeholders, ", "))
	dataStmt, err := tx.Prepare(insertDataSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare data insert: %w", err)
	}
	defer dataStmt.Close()

	// Prepare citation insert
	insertCitationSQL := fmt.Sprintf(`INSERT INTO %s 
		(row_id, col_index, col_name, page_number, bbox_x0, bbox_y0, bbox_x1, bbox_y1, raw_text, value, audited, audit_passed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`, citationTableName)
	citStmt, err := tx.Prepare(insertCitationSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare citation insert: %w", err)
	}
	defer citStmt.Close()

	// Insert rows
	for _, row := range table.DataRows() {
		args := []any{
			row.Index,
			row.Level,
			row.Label,
			row.Section,
			row.IsTotal,
		}

		for colIdx := 1; colIdx < len(table.Columns); colIdx++ {
			cell := row.GetCell(colIdx)
			if cell != nil && cell.IsNumeric {
				args = append(args, cell.Value)
			} else {
				args = append(args, nil)
			}

			// Insert citation
			if cell != nil {
				audInt := 0
				if cell.Audited {
					audInt = 1
				}
				passInt := 0
				if cell.AuditPass {
					passInt = 1
				}
				_, err := citStmt.Exec(
					row.Index,
					colIdx,
					table.Columns[colIdx].Name,
					row.PageNum,
					cell.BBox.X0,
					cell.BBox.Y0,
					cell.BBox.X1,
					cell.BBox.Y1,
					cell.RawText,
					cell.Value,
					audInt,
					passInt,
				)
				if err != nil {
					return fmt.Errorf("failed to insert citation: %w", err)
				}
			}
		}

		if _, err := dataStmt.Exec(args...); err != nil {
			return fmt.Errorf("failed to insert row %d: %w", row.Index, err)
		}
	}

	return tx.Commit()
}

func sanitizeIdentifier(s string) string {
	cleaned := identSanitizer.ReplaceAllString(s, "_")
	cleaned = strings.Trim(cleaned, "_")
	if len(cleaned) == 0 {
		return "item"
	}
	if cleaned[0] >= '0' && cleaned[0] <= '9' {
		cleaned = "t_" + cleaned
	}
	return cleaned
}
