# tablerip Export Format Specifications

This document describes the schema and structure of data files exported by tablerip.

## 1. Apache Parquet Schema

Each extracted table is written as a columnar Parquet file with dictionary encoding and Snappy compression.

```
message TableRow {
  required int64 row_index;
  required int32 hierarchy_level;
  required binary row_label (STRING);
  optional binary section (STRING);
  repeated ColumnMetric metrics;
}

message ColumnMetric {
  required binary column_name (STRING);
  optional double value;
  optional binary raw_text (STRING);
  optional binary currency (STRING);
  required int32 page_number;
  required BoundingBox bbox;
  required boolean is_audited;
  required boolean audit_passed;
}

message BoundingBox {
  required double x0;
  required double y0;
  required double x1;
  required double y1;
}
```

## 2. SQLite Relational Schema

When exporting to SQLite, tablerip creates structured tables and a citation metadata table:

```sql
-- Table data
CREATE TABLE financial_statement (
    row_id INTEGER PRIMARY KEY,
    line_number INTEGER,
    hierarchy_level INTEGER,
    line_item TEXT NOT NULL,
    col_2024 REAL,
    col_2025 REAL,
    section TEXT
);

-- Coordinate citation table for grounding
CREATE TABLE cell_citations (
    citation_id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT,
    row_id INTEGER,
    column_name TEXT,
    page_number INTEGER,
    bbox_x0 REAL,
    bbox_y0 REAL,
    bbox_x1 REAL,
    bbox_y1 REAL,
    raw_text TEXT
);
```

## 3. JSON Output Schema

JSON export provides complete document grounding:

```json
{
  "document": "sec_filings/aapl_10k_2025.pdf",
  "tables": [
    {
      "table_id": "tbl_001",
      "title": "Consolidated Statements of Operations",
      "pages": [67, 68],
      "columns": [
        {
          "index": 0,
          "name": "Line Item",
          "span": [0, 0]
        },
        {
          "index": 1,
          "name": "Three Months Ended.September 30.2025",
          "span": [1, 1]
        }
      ],
      "rows": [
        {
          "row_index": 0,
          "level": 0,
          "label": "Total net sales",
          "is_total": true,
          "cells": [
            {
              "column_index": 1,
              "raw": "94,930",
              "value": 94930.0,
              "currency": "USD",
              "page": 67,
              "bbox": [120.4, 340.2, 180.1, 352.0],
              "audit_status": "VERIFIED"
            }
          ]
        }
      ],
      "arithmetic_audit": {
        "status": "PASSED",
        "confidence_score": 1.0,
        "checked_invariants": 14,
        "discrepancies": []
      }
    }
  ]
}
```
