<p align="center">
  <img src="docs/images/logo.png?raw=true" alt="tablerip Logo" width="130" style="border-radius: 24px;" />
</p>

<h1 align="center">tablerip</h1>

<p align="center">
  <strong>Pure Go Autonomous Financial Table Extraction and Arithmetic Reconciliation Engine</strong><br />
  Spatial Projection Profiles • Cross-Page Continuation Stitching • Zero Cgo • Apache Parquet Ingestion
</p>

<p align="center">
  <img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License" />
  <img src="https://img.shields.io/badge/memory-%3C%2025MB%20RAM-success?style=flat-square" alt="Memory Ceiling" />
  <img src="https://img.shields.io/badge/dependencies-0%20Cgo%20%2F%200%20Python-blue?style=flat-square" alt="Zero Dependencies" />
</p>

<p align="center">
  <img src="docs/images/tablerip_demo.gif?raw=true" alt="tablerip Interactive Web Cockpit Demo" width="860" style="border-radius: 8px; box-shadow: 0 8px 24px rgba(0,0,0,0.3);" />
</p>

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

## Visual Interface and Cockpit

tablerip includes an embedded cockpit on port 8787 for dual-view spatial verification, live cell correction, and relational SQL exploration:

### Tabular Grid and Spatial Vector Map
Reconstructed tabular data on the right pane paired with color-coded vector glyph bounding boxes and ruling lines on the left.

<p align="center">
  <img src="docs/images/cockpit_grid_view.png?raw=true" alt="tablerip Dual-Pane Grid View" width="860" style="border-radius: 6px; border: 1px solid #30363d;" />
</p>

### Mathematical Balance Verification
Real-time evaluation of accounting invariants, confidence score, and invariant check cards with exact mathematical delta tracking.

<p align="center">
  <img src="docs/images/cockpit_audit_view.png?raw=true" alt="tablerip Arithmetic Audit View" width="860" style="border-radius: 6px; border: 1px solid #30363d;" />
</p>

### In-Memory SQL Console and Multi-Device Adaptation
Direct SQL query execution against pure Go in-memory SQLite tables, with pre-built query templates and responsive multi-device adaptation.

<p align="center">
  <img src="docs/images/cockpit_sql_view.png?raw=true" alt="tablerip SQL Console View" width="860" style="border-radius: 6px; border: 1px solid #30363d;" />
</p>

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

Open `http://localhost:8787` in your browser. Features interactive SVG zoom and pan, live in-place cell editing with real-time arithmetic re-audit, and pre-built SQL query templates.

### Download SEC Filings from EDGAR

Locate, download, and extract financial tables directly from SEC EDGAR by company ticker:

```bash
./bin/tablerip fetch \
  --ticker AAPL \
  --form 10-K \
  --year 2024 \
  --out-dir ./data/sec/ \
  --extract
```

### Multi-Period Reconciliation & Restatement Audit

Reconcile historical periods across two filings and detect restatements or revisions:

```bash
./bin/tablerip diff \
  --base ./sec_filings/aapl_10k_2024.pdf \
  --target ./sec_filings/aapl_10k_2025.pdf \
  --out ./reports/restatement_diff.json
```

### Parallel Batch Processing

Process entire directories of financial filings with a bounded worker pool:

```bash
./bin/tablerip batch \
  --input-dir ./filings/ \
  --out-dir ./batch_output/ \
  --format all \
  --concurrency 4
```

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
│       └── main.go              # CLI command router and subcommands
├── pkg/
│   ├── pdf/                     # Pure Go PDF token extraction and geometry
│   ├── grid/                    # Spatial projection, headers, rows, stitcher, footnotes
│   ├── auditor/                 # Mathematical balance checks, normalizer, GAAP taxonomy
│   ├── batch/                   # Bounded concurrent worker pool for batch ingestion
│   ├── diff/                    # Multi-period reconciliation and restatement auditor
│   ├── edgar/                   # SEC EDGAR client and filing ingestor
│   ├── exporter/                # Parquet, SQLite, DuckDB, JSON, CSV writers
│   ├── fixtures/                # Synthetic authentic SEC filing generator
│   └── cockpit/                 # Embedded HTTP server and dual-pane web UI
├── docs/                        # Architecture, algorithms, and format specs
├── test/                        # End-to-end integration and benchmark tests
├── go.mod
└── go.sum
```

## License

MIT
