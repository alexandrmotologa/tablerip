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

If $S_{\text{col}} > 0.75$, $T_B$ is stitched into $T_A$, skipping repeated running headers.

---

## 4. Subsystems and Detailed Implementation Tasks

### Phase 1: Pure Go PDF Parser & Geometry Core (`pkg/pdf`)
- [x] Subtask 1.1: Initialize Go module `github.com/alexandrmotologa/tablerip` with dependencies.
- [x] Subtask 1.2: Coordinate transforms and Bounding Box data structures (`geom.go`).
- [x] Subtask 1.3: Text Token extractor from PDF content streams (`parser.go`).
- [x] Subtask 1.4: Line and path segment detector (`lines.go`).
- [x] Subtask 1.5: Spatial indexing and token spatial clustering.

### Phase 2: Topological Grid Reconstruction (`pkg/grid`)
- [x] Subtask 2.1: Horizontal and vertical projection profiling (`segmenter.go`).
- [x] Subtask 2.2: Adaptive valley detection with gutter thresholding.
- [x] Subtask 2.3: Spanning header resolver and multi-tier column names (`headers.go`).
- [x] Subtask 2.4: Row baseline clustering and indentation classifier (`rows.go`).
- [x] Subtask 2.5: Cell intersection matrix and tabular model builder (`table.go`).

### Phase 3: Multi-Page Continuation Stitcher (`pkg/grid`)
- [x] Subtask 3.1: Running header and page number filtering (`stitcher.go`).
- [x] Subtask 3.2: Gaussian column alignment similarity scoring.
- [x] Subtask 3.3: Consecutive page table fusion and header deduplication.

### Phase 4: Arithmetic Reconciliation & Normalizer (`pkg/auditor`)
- [x] Subtask 4.1: Currency, negative parenthesis, and scale normalizer (`normalizer.go`).
- [x] Subtask 4.2: Vertical summation invariant auditor (`auditor.go`).
- [x] Subtask 4.3: Balance sheet and income statement identity rules (`rules.go`).
- [x] Subtask 4.4: Discrepancy flagging and audit confidence scoring.

### Phase 5: High-Performance Exporters (`pkg/exporter`)
- [x] Subtask 5.1: Apache Parquet columnar exporter with Snappy (`parquet.go`).
- [x] Subtask 5.2: SQLite embedded database creator with citation table (`sqlite.go`).
- [x] Subtask 5.3: DuckDB SQL DDL / DML script generator (`duckdb.go`).
- [x] Subtask 5.4: Citation-indexed JSON and hierarchical CSV exporter (`json.go`, `csv.go`).

### Phase 6: Embedded Web Cockpit (`pkg/cockpit`)
- [x] Subtask 6.1: HTTP server on port 8787 with embedded assets (`server.go`).
- [x] Subtask 6.2: Dual-pane visual interface with SVG vector overlays (`web/index.html`, `web/style.css`).
- [x] Subtask 6.3: Interactive grid inspection and arithmetic discrepancy breakdown (`web/app.js`).
- [x] Subtask 6.4: In-browser SQL console for live query execution against extracted tables.

### Phase 7: CLI Interface & End-to-End Test Suite (`cmd/tablerip`, `test/`)
- [x] Subtask 7.1: CLI subcommands (`extract`, `audit`, `export`, `cockpit`, `benchmark`, `generate`).
- [x] Subtask 7.2: Synthetic multi-page financial PDF test generator (`pkg/fixtures`).
- [x] Subtask 7.3: Integration tests verifying arithmetic balance and multi-page stitching (`test/sample_10k_test.go`).
- [x] Subtask 7.4: Memory benchmark verifying < 25 MB RAM ceiling (verified at 1.75 MB - 6.6 MB peak).
