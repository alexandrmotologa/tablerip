# tablerip Architecture

This document describes the internal subsystems, pipeline data flow, and design trade-offs of the tablerip engine.

## 1. Design Principles

1. **Zero External Runtime**: Pure Go implementation without Cgo bindings, Python bridges, or heavy external visual dependencies. Compiles to a single portable binary.
2. **Deterministic Geometric Operations**: Avoids heuristic neural network guessing for table borders. Relies on exact PDF character glyph positions, vector ruling paths, and spatial projection histograms.
3. **Audited Ingestion**: Financial records are accompanied by mathematical reconciliation proofs and source document coordinates $(x_0, y_0, x_1, y_1, \text{page})$.
4. **Constrained Memory Ceiling**: Streams page by page with bounded buffer pools, operating well below 25 MB RAM for 100+ page corporate filings.

## 2. Pipeline Stages

### Stage 1: Geometric Token Extraction (`pkg/pdf`)

The PDF parser reads raw content streams, resolving transformation matrices, character encodings, and vector strokes. Each extracted token contains:
- Text string (UTF-8 normalized)
- Coordinate box in points: $X_0, Y_0, X_1, Y_1$ (origin at top-left or bottom-left normalized to top-left)
- Font name and point size
- Vector stroke lines (horizontal and vertical ruling lines)

### Stage 2: Spatial Projection Profiling (`pkg/grid`)

The spatial segmenter projects token bounding boxes onto horizontal and vertical axes:
- **Y-Cut Profile**: Identifies row baseline bands and vertical gaps between lines.
- **X-Cut Profile**: Identifies column boundaries and gutter widths within each table region.
- **Ruling Line Fusion**: Merges explicitly drawn vector border lines with projection valleys to handle both boxed and unbordered tables.

### Stage 3: Topological Header and Indentation Tree (`pkg/grid`)

- **Multi-Level Column Headers**: Recursively tracks parent header spans (for example, "Three Months Ended" spanning sub-columns "2024" and "2025"). Generates composite names using period delimiters.
- **Row Indentation Classifier**: Measures horizontal offset from the table's left margin. Assigns depth levels ($0, 1, 2$) to represent category hierarchies (e.g. Operating Expenses -> Research & Development).

### Stage 4: Cross-Page Table Continuation (`pkg/grid`)

When an SEC 10-K statement spans multiple pages:
1. Running headers and footers (page numbers, filing headers) are filtered out.
2. Column centers of the trailing table on page $k$ are compared against candidate tables on page $k+1$.
3. If the Gaussian similarity score exceeds 0.90, the rows are concatenated, and repeated header rows are discarded.

### Stage 5: Arithmetic Reconciliation (`pkg/auditor`)

The auditor extracts numbers and cleans formatting:
- Resolves accounting negatives `(1,234)` or `1,234-` into floating-point numbers.
- Identifies total and subtotal indicator keywords.
- Calculates sum of items versus reported totals:
  $$\epsilon = |V_{\text{total}} - \sum V_{\text{items}}|$$
- Reconciles balance sheet equations (Assets = Liabilities + Equity).
- Generates cell citations and confidence flags.

### Stage 6: High-Performance Exporters (`pkg/exporter`)

- **Apache Parquet**: Directly streams columnar chunks using Snappy compression.
- **SQLite**: Generates relational tables with typed columns (`DECIMAL`, `TEXT`, `INTEGER`) and citation metadata tables.
- **JSON**: Outputs nested structured documents containing full bounding-box coordinates for quantitative analysis.
- **CSV**: Outputs standard comma-delimited tables with hierarchical depth indicators.

### Stage 7: Embedded Cockpit (`pkg/cockpit`)

An embedded HTTP service on port 8787 delivers a zero-dependency web cockpit:
- Side-by-side layout: PDF vector visualization and reconstructed data grid.
- Visual inspection of bounding boxes and column cuts.
- Color-coded arithmetic audit flags.
- In-browser SQL console powered by an in-memory database engine.
