# tablerip

A pure Go geometric layout analyzer and financial table extractor for multi-page SEC 10-K and 10-Q filings, balance sheets, and earnings statements.

tablerip parses PDF content streams directly without Python or Cgo runtime dependencies. It reconstructs multi-level column headers, captures row indentation hierarchies, stitches tables across page boundaries, verifies arithmetic invariants, and streams results directly into Apache Parquet, SQLite, and JSON.

## Core Capabilities

- **Pure Go Vector Parsing**: Operates under 25 MB RAM with zero external Cgo or OCR dependencies. Parses glyph coordinates, text bounding boxes, font scales, and vector strokes directly from PDF streams.
- **Topological Grid Reconstruction**: Uses spatial projection profiles (X-Cut and Y-Cut histograms) to locate cell boundaries, composite spanning headers, and nested parent-child row hierarchies.
- **Cross-Page Table Continuation**: Identifies running headers and footers across page breaks. Matches column fingerprints across consecutive pages to stitch multi-page statements into a single table.
- **Arithmetic Reconciliation Engine**: Verifies horizontal and vertical accounting invariants:
  - Vertical summations: sum of line items equals reported subtotals and totals.
  - Balance sheet identity: Total Assets equals Total Liabilities plus Stockholders Equity.
  - Sign and currency normalization: handles accounting parentheses `(1,234)` as negative numbers, strips currency symbols, and detects scale factors (millions or thousands).
- **Direct Multi-Format Exporters**:
  - Apache Parquet with dictionary encoding and Snappy compression.
  - Embedded SQLite database tables with cell bounding box citations.
  - Citation-indexed JSON with exact page coordinate bounds for quantitative downstream workflows.
  - Clean CSV with indentation levels.
- **Interactive Web Cockpit**: Embedded HTTP interface on port 8787 providing dual-pane PDF vector overlay, interactive grid inspector, arithmetic discrepancy highlights, and an integrated in-memory SQL console.

## Architecture

```
                  PDF Input Stream
            (SEC 10-K / 10-Q / Statements)
                         │
                         ▼
             tablerip Geometry Parser
         (Character Bounding Boxes & Paths)
                         │
                         ▼
            Spatial Projection Profiler
              (X-Cut & Y-Cut Histograms)
                         │
                         ▼
            Topological Grid Reconstructor
         (Spanning Headers & Row Indent Tree)
                         │
                         ▼
           Cross-Page Continuation Stitcher
            (Column Alignment Fingerprints)
                         │
                         ▼
             Arithmetic Balance Auditor
           (Invariant Checks & Discrepancies)
                         │
          ┌──────────────┴──────────────┐
          ▼                             ▼
   Multi-Format Exporters        Embedded Web Cockpit
   - Apache Parquet (Snappy)     (Port 8787)
   - SQLite / DuckDB SQL         - Dual-pane PDF / Grid
   - Citation JSON               - Invariant Breakdown
   - CSV                         - In-Memory SQL Console
```

## Installation

Requires Go 1.22 or higher.

```bash
git clone https://github.com/alexandrmotologa/tablerip.git
cd tablerip
go build -o bin/tablerip ./cmd/tablerip
```

## Quick Start

### Extract Tables to Apache Parquet

Extract financial tables from an SEC 10-K filing and export directly to Parquet:

```bash
./bin/tablerip extract \
  --input ./sec_filings/aapl_10k_2025.pdf \
  --format parquet \
  --out-dir ./data/tables/ \
  --audit
```

### Run Arithmetic Reconciliation

Audit balance sheet totals and flag arithmetic discrepancies:

```bash
./bin/tablerip audit \
  --input ./sec_filings/msft_balance_sheet.pdf \
  --tolerance 0.01 \
  --strict
```

### Export to SQLite Database

Convert extracted tables directly into an embedded SQLite database with schema DDL:

```bash
./bin/tablerip export \
  --input ./sec_filings/nvda_q3.pdf \
  --format sqlite \
  --out ./db/financials.sqlite \
  --table-prefix "nvda_2025_"
```

### Launch Interactive Web Cockpit

Start the dual-pane visual inspection cockpit on port 8787:

```bash
./bin/tablerip cockpit \
  --input ./sec_filings/aapl_10k_2025.pdf \
  --port 8787
```

Open `http://localhost:8787` in your browser.

## Mathematical Formulation

### Spatial Projection (X-Cut and Y-Cut)

Given $N$ text tokens with bounding boxes $B_i = [x_{0,i}, y_{0,i}, x_{1,i}, y_{1,i}]$, horizontal projection profile $H(y)$ identifies row baselines:

$$H(y) = \sum_{i=1}^N \mathbf{1}(y_{0,i} \le y \le y_{1,i})$$

Vertical projection profile $V(x)$ identifies column gutter intervals where $V(x) = 0$:

$$V(x) = \sum_{i=1}^N \mathbf{1}(x_{0,i} \le x \le x_{1,i})$$

### Multi-Page Column Fingerprinting

Column alignment similarity between table $T_A$ on page $k$ and table $T_B$ on page $k+1$ is evaluated by Gaussian kernel center comparison:

$$S_{\text{col}}(T_A, T_B) = \frac{1}{|C_A|} \sum_{c \in C_A} \exp\left( -\frac{(x_{\text{center}}(c, T_A) - x_{\text{center}}(c, T_B))^2}{2 \sigma^2} \right)$$

When $S_{\text{col}} > 0.90$, tables are stitched and redundant running headers are pruned.

### Arithmetic Invariant Verification

For any aggregate line item $R_{\text{total}}$ and preceding child items $\{R_1, \dots, R_m\}$:

$$\epsilon(c) = \left| V(R_{\text{total}}, c) - \sum_{j=1}^m \sigma_j V(R_j, c) \right|$$

Where $\sigma_j \in \{+1, -1\}$ accounts for deductions. If $\epsilon(c) > 0.01$, an audit flag is registered with cell coordinate references.

## Project Structure

```
tablerip/
├── cmd/
│   └── tablerip/
│       └── main.go              # CLI command router and flags
├── pkg/
│   ├── pdf/                     # Pure Go PDF token extraction and geometry
│   ├── grid/                    # Spatial projection, headers, rows, stitcher
│   ├── auditor/                 # Mathematical balance checks and normalizer
│   ├── exporter/                # Parquet, SQLite, DuckDB, JSON, CSV writers
│   └── cockpit/                 # Embedded HTTP server and web UI
├── docs/                        # Architecture, algorithms, and format specs
├── test/                        # End-to-end integration and benchmark tests
├── go.mod
└── go.sum
```

## License

MIT
