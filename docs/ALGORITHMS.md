# tablerip Algorithmic Specifications

This document outlines the mathematical formulations and algorithms implemented in tablerip.

## 1. Spatial Projection Profiles (Recursive X-Cut & Y-Cut)

Financial statements often omit visible grid lines, relying on typographic alignment. tablerip computes 1D discrete projection profiles over a continuous bounding box space.

### 1.1 Horizontal Projection (Y-Cut)

For a page region containing tokens $T = \{t_1, \dots, t_N\}$ where each token has vertical span $[y_{0,i}, y_{1,i}]$:

$$H(y) = \sum_{i=1}^N \mathbf{1}(y_{0,i} \le y \le y_{1,i})$$

Let $Y_{\text{valleys}} = \{y \mid H(y) = 0 \land H(y - \delta) > 0 \land H(y + \delta) > 0\}$.
Consecutive valleys delimit row bands $[r_{k,\text{top}}, r_{k,\text{bottom}}]$.

### 1.2 Vertical Projection (X-Cut)

Within each row band or across a cluster of rows:

$$V(x) = \sum_{i=1}^N \mathbf{1}(x_{0,i} \le x \le x_{1,i})$$

Valleys where $V(x) = 0$ over a width greater than minimum gutter threshold $W_{\text{gutter}}$ define column separator boundaries:

$$C = [x_{\text{left}}, x_{\text{right}}]$$

## 2. Row Indentation Hierarchy

Financial statements use left margin offsets to indicate line-item dependencies.

Let $x_{\text{base}} = \min_{k} x_{0}(r_k)$ be the minimum left coordinate across all row labels.
Let $\Delta x$ be the estimated indentation step, computed as the greatest common divisor of observed delta offsets:

$$L(r_k) = \text{round}\left( \frac{x_0(r_k) - x_{\text{base}}}{\Delta x} \right)$$

This maps each row to a tree depth:
- Level 0: Primary statement section (Operating expenses)
- Level 1: Sub-category (Research and development)
- Level 2: Component breakdown (Stock-based compensation)

## 3. Multi-Page Continuation Fingerprinting

To reliably stitch tables spanning page breaks, tablerip measures column alignment similarity using a Gaussian radial basis function.

Given table $T_1$ with column centers $U = \{u_1, \dots, u_m\}$ and table $T_2$ with column centers $W = \{w_1, \dots, w_n\}$:

$$S(T_1, T_2) = \frac{1}{|U|} \sum_{i=1}^{|U|} \max_{j} \exp\left( -\frac{(u_i - w_j)^2}{2 \sigma^2} \right)$$

Where $\sigma = 3.0$ points. If $S(T_1, T_2) \ge 0.90$, table $T_2$ is classified as a continuation of $T_1$.

## 4. Arithmetic Reconciliation Engine

### 4.1 Number Normalization

Financial values are parsed into IEEE 754 float64 / fixed-point representation:
1. Strip currency symbols: `$`, `€`, `£`, `¥`.
2. Accounting parentheses: `(1,234.50)` converts to `-1234.50`.
3. Trailing minus: `1,234.50-` converts to `-1234.50`.
4. Scale detection: detects header annotations such as "(in thousands)" ($10^3$) or "(in millions)" ($10^6$).

### 4.2 Invariant Equations

For a parent total row $R_{\text{total}}$ with children $\{R_1, \dots, R_m\}$:

$$\text{Error}(c) = \left| V(R_{\text{total}}, c) - \sum_{j=1}^m \sigma_j V(R_j, c) \right|$$

Where $\sigma_j = -1$ if row $j$ text contains "Less:" or "minus", and $+1$ otherwise.
An audit flag is raised if $\text{Error}(c) > 0.01$.
