# tablerip: Architecture and Engineering Master Plan
Autonomous Multi-Page Financial Table Extraction & Parquet Ingestion Engine
Target: High-Precision Go Engine (< 25MB RAM) with Embedded Audit Cockpit

---

## 1. Problem Statement and Scope

### 1.1 Structural Complexity of Enterprise Financial Documents
Financial reports (such as SEC Form 10-K, Form 10-Q, audited balance sheets, cash flow statements, and credit risk schedules) present the most challenging document structures for automated data pipelines:
1. **Multi-Level and Spanning Headers**: Columns are organized in hierarchical tiers (e.g., *Three Months Ended September 30* spanning *2025* and *2024*, with sub-columns *Unaudited* and *Audited*).
2. **Indented Row Hierarchies**: Financial statements use subtle typographic indentations (2 spaces, 4 spaces, em-quad spaces) to indicate parent-child relationships (e.g., *Operating Expenses* -> *Research & Development* -> *Stock-Based Compensation*).
3. **Multi-Page Table Continuations**: A single table frequently spans 3 to 10 consecutive pages, with recurring running headers, footnotes, or page numbers breaking grid continuity.
4. **Catastrophic OCR/LLM Hallucinations**: Standard OCR and multimodal LLMs frequently shift numerical values by one column or drop negative parenthetical signs (e.g., treating `(452)` as `452`), introducing critical silent errors into quantitative financial models.

### 1.2 The tablerip Solution
`tablerip` is a pure Go high-precision layout analyzer and financial table extractor:
- **Geometric Vector Extraction**: Parses PDF content streams directly in Go, extracting exact character bounding boxes $(x_0, y_0, x_1, y_1)$, font size vectors, and horizontal/vertical vector stroke lines.
- **Topological Grid Reconstruction**: Uses recursive spatial projection to identify column boundaries, merged header spans, and row baseline clusters without heavy computer vision dependencies.
- **Multi-Page Continuation Stitcher**: Aligns fragmented tables across page breaks by fingerprinting column anchor coordinates and running header patterns.
- **Mathematical Balance Auditor**: Automatically evaluates row-column sum invariants ($\sum \text{Items} = \text{Total}$, $\text{Assets} = \text{Liabilities} + \text{Equity}$) to verify arithmetic consistency before ingestion.
- **Zero-Copy High-Throughput Export**: Streams extracted structured data directly into Apache Parquet, DuckDB, SQLite, and typed JSON with bounding-box citations.
- **Embedded Web Cockpit**: A visual interface on port 8787 featuring side-by-side PDF rendering, interactive cell grid bounding boxes, arithmetic discrepancy alerts, and live Parquet schema exploration.

---

## 2. System Architecture

```
+-------------------------------------------------------------+
|                      PDF Input Stream                       |
|        SEC 10-K / 10-Q | Annual Reports | Financial PDFs    |
+-------------------------------------------------------------+
                               |
                               v
+-------------------------------------------------------------+
|                     tablerip Core Engine                    |
|                                                             |
|  +---------------------+        +------------------------+  |
|  | PDF Content Stream  |------->| Geometric Text Token   |  |
|  | Parser (Pure Go)    |        | Spatial Vector Map     |  |
|  +---------------------+        +------------------------+  |
|                                              |              |
|                                              v              |
|  +-------------------------------------------------------+  |
|  |            Topological Grid Reconstruction            |  |
|  |  - Horizontal / Vertical Stroke Line Segmenter        |  |
|  |  - Spatial Projection Profile (X-Cut & Y-Cut)         |  |
|  |  - Multi-Tier Column Header Tree Builder              |  |
|  |  - Row Indentation Level Classifier                   |  |
|  +-------------------------------------------------------+  |
|                                              |              |
|                                              v              |
|  +-------------------------------------------------------+  |
|  |           Multi-Page Continuation Stitcher            |  |
|  |  - Running Header & Page Number Filtering             |  |
|  |  - Column Alignment Fingerprint Matcher               |  |
|  |  - Seamless Multi-Page Table Fusion                   |  |
|  +-------------------------------------------------------+  |
|                                              |              |
|                                              v              |
|  +-------------------------------------------------------+  |
|  |             Mathematical Balance Auditor              |  |
|  |  - Row Sum Verification (Item1 + Item2 == Subtotal)   |  |
|  |  - Balance Sheet Invariant Check                      |  |
|  |  - Sign & Currency Symbol Normalization ($(123) -> -123|  |
|  +-------------------------------------------------------+  |
|                               |                             |
|               +---------------+---------------+             |
|               |                               |             |
+---------------|-------------------------------|-------------+
                |                               |
                v                               v
+-----------------------------------+  +----------------------+
|       Export & Ingestion Engine   |  | Embedded Cockpit     |
|  1. Apache Parquet Files          |  | (Port 8787)          |
|  2. Direct DuckDB / SQLite Table  |  | - PDF vs Grid Overlay|
|  3. Typed JSON Schema             |  | - Discrepancy Alerts |
|  4. Clean Formatted CSV           |  | - Parquet Inspector  |
+-----------------------------------+  +----------------------+
```

---

## 3. Mathematical & Algorithmic Formulation

### 3.1 Spatial Projection Profile (X-Cut and Y-Cut)
Given a set of $N$ text tokens on a page, each token $i$ is defined by its bounding box $B_i = [x_{0, i}, y_{0, i}, x_{1, i}, y_{1, i}]$ and text string $t_i$.

The 1D horizontal projection histogram $H(y)$ across vertical coordinates is defined as:

$$H(y) = \sum_{i=1}^N \mathbf{1}\left(y_{0, i} \le y \le y_{1, i}\right)$$

Row boundaries correspond to local minima in $H(y)$ where $H(y) = 0$. Similarly, column separator channels correspond to valleys in the vertical projection histogram $V(x)$:

$$V(x) = \sum_{i=1}^N \mathbf{1}\left(x_{0, i} \le x \le x_{1, i}\right)$$

### 3.2 Indentation Depth Hierarchical Level
For rows within a financial table, the left margin $x_{0}$ of the row label text determines its hierarchical level $L \in \{0, 1, 2, \dots\}$:

$$L(\text{row}_k) = \text{round}\left( \frac{x_{0, k} - x_{\text{base}}}{\Delta x_{\text{indent}}} \right)$$

Where $x_{\text{base}}$ is the baseline left margin of the table and $\Delta x_{\text{indent}}$ is the estimated indentation step (typically 10 to 18 points).

### 3.3 Arithmetic Balance Invariant Audit
For any identified total row $R_{\text{total}}$ and its preceding sub-item rows $\{R_1, R_2, \dots, R_m\}$ with indentation level $L(R_j) = L(R_{\text{total}}) + 1$:

$$\epsilon_{\text{audit}}(c) = \left| V(R_{\text{total}}, c) - \sum_{j=1}^m \sigma_j \cdot V(R_j, c) \right|$$

Where $V(R, c)$ is the parsed numeric value in column $c$, $\sigma_j \in \{+1, -1\}$ handles explicit subtractions (e.g., less accumulated depreciation), and $\epsilon_{\text{audit}}$ must equal $0.00$ within a floating-point tolerance of $10^{-2}$. Any non-zero $\epsilon$ raises an **Arithmetic Audit Flag**.

### 3.4 Multi-Page Column Fingerprinting
To determine whether Table $T_B$ on Page $k+1$ continues Table $T_A$ on Page $k$, we compute the column alignment similarity score:

$$S_{\text{col}}(T_A, T_B) = \frac{1}{|C_A|} \sum_{c \in C_A} \exp\left( -\frac{(x_{\text{center}}(c, T_A) - x_{\text{center}}(c, T_B))^2}{2 \sigma_{\text{align}}^2} \right)$$

If $S_{\text{col}} > 0.92$, $T_B$ is stitched into $T_A$, skipping repeated running headers.

---

## 4. Component Design and Subsystems

### 4.1 Pure Go PDF Parser (`pkg/pdf`)
- Zero Cgo/Python dependencies.
- Extracts character codes, glyph bounding boxes, font metadata, and line draw primitives (`re`, `m`, `l`, `c` PDF operators).
- Handles rotated pages, scaled matrices, and encrypted/compressed PDF streams.

### 4.2 Grid Topology Engine (`pkg/grid`)
- Reconstructs 2D cell grid structure from text boxes and vector ruling lines.
- Spanning Header Resolver: Links multi-tiered headers into composite column names (e.g., `Revenue.ThreeMonthsEnded.2025.Unaudited`).
- Negative Number & Unit Parser: Normalizes accounting parenthesis `(1,234.5)` into `-1234.5`, and detects scale annotations (e.g., *"in millions, except per share data"*).

### 4.3 Arithmetic Auditor (`pkg/auditor`)
- Analyzes semantic label keywords (*"Total Assets"*, *"Operating Income"*, *"Net Income"*, *"Subtotal"*).
- Executes horizontal across-column equations (e.g., $\text{Gross Profit} = \text{Revenue} - \text{COGS}$).
- Executes vertical down-column summations.
- Outputs an Audit Confidence Score ($0.0 \dots 1.0$) with flagged discrepancy cell coordinates.

### 4.4 Ingestion & Exporters (`pkg/exporter`)
- **Apache Parquet Exporter**: High-performance typed binary serialization with dictionary encoding and Snappy compression.
- **DuckDB / SQLite Exporter**: Generates schema-aligned SQL tables directly from extracted grids.
- **JSON & CSV Exporter**: Generates structured JSON with full cell bounding-box citations for LLM grounding.

### 4.5 Embedded Web Cockpit (`pkg/cockpit`)
- Serves interactive UI on port 8787.
- Features:
  - **Dual-Pane Document Viewer**: Left pane renders the source PDF page with color-coded bounding-box highlights; right pane renders the interactive data grid.
  - **Arithmetic Audit Inspector**: Highlights reconciled totals in green; highlights discrepancies in red with mathematical breakdown.
  - **Parquet Schema & SQL Query Runner**: Lets users execute instant SQL queries directly against extracted tables in the browser.

---

## 5. Command-Line Interface (CLI)

```bash
# Extract all financial tables from SEC 10-K and export directly to Parquet
tablerip extract \
  --input ./sec_filings/aapl_10k_2025.pdf \
  --format parquet \
  --out-dir ./data/tables/ \
  --audit \
  --cockpit

# Extract balance sheet and execute strict arithmetic verification
tablerip audit \
  --input ./sec_filings/msft_balance_sheet.pdf \
  --tolerance 0.01 \
  --strict

# Convert PDF tables to an embedded DuckDB database file
tablerip export \
  --input ./sec_filings/nvda_q3.pdf \
  --duckdb ./db/financials.duckdb \
  --table-prefix "nvda_2025_"

# Launch the interactive Audit Cockpit directly
tablerip cockpit \
  --pdf ./sec_filings/aapl_10k_2025.pdf \
  --port 8787
```

---

## 6. Directory Structure

```
B:\workgit\tablerip\
├── cmd/
│   └── tablerip/
│       └── main.go                  # Main entry point & CLI commands
├── pkg/
│   ├── pdf/
│   │   ├── parser.go                # Pure Go PDF stream & token reader
│   │   ├── geom.go                  # Bounding box & coordinate transforms
│   │   └── lines.go                 # Vector stroke line detector
│   ├── grid/
│   │   ├── segmenter.go             # X-Cut / Y-Cut projection profiler
│   │   ├── headers.go               # Multi-level hierarchical header builder
│   │   ├── rows.go                  # Row indentation & parent-child matcher
│   │   └── stitcher.go              # Multi-page table continuation engine
│   ├── auditor/
│   │   ├── auditor.go               # Arithmetic equation verifier
│   │   ├── normalizer.go            # Currency, parenthesis, & scale parser
│   │   └── rules.go                 # Accounting balance rules & heuristics
│   ├── exporter/
│   │   ├── parquet.go               # Apache Parquet streaming writer
│   │   ├── duckdb.go                # DuckDB / SQLite table serializer
│   │   ├── csv.go                   # Standard CSV exporter
│   │   └── json.go                  # Citation-indexed JSON exporter
│   └── cockpit/
│       ├── server.go                # Embedded HTTP server
│       └── web/
│           ├── index.html           # Dual-pane PDF and Grid UI
│           ├── app.js               # Grid overlay & SQL playground
│           └── style.css            # Dark tech clean UI styling
├── test/
│   ├── sample_10k_test.go           # End-to-end multi-page extraction tests
│   └── auditor_test.go              # Arithmetic reconciliation unit tests
├── go.mod
├── go.sum
├── README.md
└── MASTER_PLAN.md
```

---

## 7. Implementation Roadmap

### Phase 1: Pure Go PDF Parser & Token Geometry
- Initialize Go module `github.com/mtlglabs/tablerip`.
- Implement PDF stream reader for character positions, bounding boxes, and lines.
- Build spatial coordinate indexing using R-Tree / Interval trees.

### Phase 2: Topological Grid Reconstruction
- Implement horizontal and vertical projection profiles ($H(y)$ and $V(x)$).
- Implement multi-level column header hierarchy tree builder.
- Implement row indentation classifier and unit normalizer (`$(123)` -> `-123`).

### Phase 3: Multi-Page Continuation Stitcher
- Implement page boundary detection and running header identification.
- Build column alignment similarity scoring.
- Stitch fragmented multi-page tables into single consolidated entities.

### Phase 4: Arithmetic Balance Auditor
- Implement row and column summation reconciliation ($\sum \text{parts} = \text{Total}$).
- Implement accounting invariant checks for balance sheets and income statements.
- Generate cell-level discrepancy alerts and confidence scores.

### Phase 5: High-Performance Exporters
- Implement streaming Apache Parquet writer with Snappy compression.
- Implement direct DuckDB / SQLite database export.
- Build citation-indexed JSON output with exact PDF bounding boxes.

### Phase 6: Embedded Web Cockpit
- Build dual-pane interface on port 8787.
- Render PDF page canvas with interactive SVG bounding boxes.
- Implement in-browser SQL query runner against extracted tables.

### Phase 7: Verification and Benchmarks
- Benchmark extraction speed: $< 250\text{ms}$ per 10-page financial filing.
- Memory ceiling: strictly $< 25\text{MB}$ RSS.
- Test accuracy on real SEC 10-K filings with zero arithmetic audit discrepancies.
