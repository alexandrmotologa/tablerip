document.addEventListener('DOMContentLoaded', () => {
  let docData = null;
  let tablesData = [];
  let currentTable = null;
  let currentPage = 1;

  // DOM Elements
  const docStatus = document.getElementById('docStatus');
  const prevPageBtn = document.getElementById('prevPageBtn');
  const nextPageBtn = document.getElementById('nextPageBtn');
  const pageIndicator = document.getElementById('pageIndicator');
  const tableSelect = document.getElementById('tableSelect');
  const vectorOverlay = document.getElementById('vectorOverlay');
  const tokenLayer = document.getElementById('tokenLayer');
  const pageCanvas = document.getElementById('pageCanvas');
  const tableHead = document.getElementById('tableHead');
  const tableBody = document.getElementById('tableBody');
  const cellDetails = document.getElementById('cellDetails');
  const tabButtons = document.querySelectorAll('.tab-btn');
  const tabPanes = document.querySelectorAll('.tab-pane');
  const runSqlBtn = document.getElementById('runSqlBtn');
  const sqlInput = document.getElementById('sqlInput');
  const sqlResults = document.getElementById('sqlResults');

  // Exports
  document.getElementById('exportParquetBtn').addEventListener('click', () => triggerExport('parquet'));
  document.getElementById('exportSQLiteBtn').addEventListener('click', () => triggerExport('sqlite'));
  document.getElementById('exportJSONBtn').addEventListener('click', () => triggerExport('json'));

  function triggerExport(format) {
    if (!currentTable) return;
    window.open(`/api/export?table=${encodeURIComponent(currentTable.id)}&format=${format}`, '_blank');
  }

  // Tabs navigation
  tabButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      tabButtons.forEach(b => b.classList.remove('active'));
      tabPanes.forEach(p => p.classList.remove('active'));
      btn.classList.add('active');
      const target = document.getElementById(btn.dataset.tab);
      if (target) target.classList.add('active');
    });
  });

  // Page Navigation
  prevPageBtn.addEventListener('click', () => {
    if (currentPage > 1) {
      currentPage--;
      renderPage(currentPage);
    }
  });

  nextPageBtn.addEventListener('click', () => {
    if (docData && currentPage < docData.num_pages) {
      currentPage++;
      renderPage(currentPage);
    }
  });

  tableSelect.addEventListener('change', (e) => {
    const selectedId = e.target.value;
    currentTable = tablesData.find(t => t.table.id === selectedId);
    if (currentTable) {
      if (currentTable.table.pages && currentTable.table.pages.length > 0) {
        currentPage = currentTable.table.pages[0];
      }
      renderTable(currentTable.table);
      renderAudit(currentTable.audit_report);
      renderPage(currentPage);
    }
  });

  // Initialize
  loadData();

  async function loadData() {
    try {
      docStatus.textContent = 'Loading Document...';
      const [docRes, tablesRes] = await Promise.all([
        fetch('/api/document'),
        fetch('/api/tables')
      ]);

      if (!docRes.ok || !tablesRes.ok) {
        throw new Error('Failed to load document or tables');
      }

      docData = await docRes.json();
      tablesData = await tablesRes.json();

      docStatus.textContent = `${docData.path.split(/[\\/]/).pop()} (${docData.num_pages} Pages)`;

      // Populate Table Selector
      tableSelect.innerHTML = '';
      tablesData.forEach((tExport, idx) => {
        const opt = document.createElement('option');
        opt.value = tExport.table.id;
        opt.textContent = `${tExport.table.id}: ${tExport.table.title || 'Table ' + (idx + 1)}`;
        tableSelect.appendChild(opt);
      });

      if (tablesData.length > 0) {
        currentTable = tablesData[0];
        if (currentTable.table.pages && currentTable.table.pages.length > 0) {
          currentPage = currentTable.table.pages[0];
        }
        renderTable(currentTable.table);
        renderAudit(currentTable.audit_report);
      }

      renderPage(currentPage);
    } catch (err) {
      docStatus.textContent = 'Error loading data: ' + err.message;
      docStatus.style.color = '#f85149';
    }
  }

  function renderPage(pageNum) {
    pageIndicator.textContent = `Page ${pageNum} / ${docData ? docData.num_pages : 1}`;
    if (!docData || !docData.pages || pageNum > docData.pages.length) return;

    const page = docData.pages[pageNum - 1];
    pageCanvas.style.width = `${page.width}px`;
    pageCanvas.style.height = `${page.height}px`;
    vectorOverlay.setAttribute('viewBox', `0 0 ${page.width} ${page.height}`);

    // Render ruling lines in SVG
    let svgContent = '';
    (page.horizontal_lines || []).forEach(line => {
      svgContent += `<line x1="${line.p0.x}" y1="${line.p0.y}" x2="${line.p1.x}" y2="${line.p1.y}" stroke="#30363d" stroke-width="${Math.max(1, line.width)}" />`;
    });
    (page.vertical_lines || []).forEach(line => {
      svgContent += `<line x1="${line.p0.x}" y1="${line.p0.y}" x2="${line.p1.x}" y2="${line.p1.y}" stroke="#30363d" stroke-width="${Math.max(1, line.width)}" />`;
    });
    vectorOverlay.innerHTML = svgContent;

    // Render token bounding boxes
    tokenLayer.innerHTML = '';
    (page.tokens || []).forEach((token, idx) => {
      const box = document.createElement('div');
      box.className = 'token-box';
      const w = Math.max(2, token.bbox.x1 - token.bbox.x0);
      const h = Math.max(2, token.bbox.y1 - token.bbox.y0);

      box.style.left = `${token.bbox.x0}px`;
      box.style.top = `${token.bbox.y0}px`;
      box.style.width = `${w}px`;
      box.style.height = `${h}px`;
      box.dataset.text = token.text;
      box.dataset.tokenIdx = idx;

      box.addEventListener('mouseenter', () => {
        cellDetails.textContent = `[Token] "${token.text}" | Font: ${token.font || 'Default'} (${token.font_size}pt) | Box: [${token.bbox.x0.toFixed(1)}, ${token.bbox.y0.toFixed(1)}, ${token.bbox.x1.toFixed(1)}, ${token.bbox.y1.toFixed(1)}]`;
      });

      tokenLayer.appendChild(box);
    });

    // Highlight cells belonging to current table on this page
    if (currentTable && currentTable.table) {
      highlightTableCellsOnPage(currentTable.table, pageNum);
    }
  }

  function highlightTableCellsOnPage(table, pageNum) {
    (table.rows || []).forEach(row => {
      (row.cells || []).forEach(cell => {
        if (cell.page_num === pageNum && cell.bbox && (cell.bbox.x1 > cell.bbox.x0)) {
          const w = cell.bbox.x1 - cell.bbox.x0;
          const h = cell.bbox.y1 - cell.bbox.y0;
          const cellHighlight = document.createElement('div');
          cellHighlight.className = 'token-box';

          if (row.is_total) {
            cellHighlight.classList.add('type-total');
          } else if (cell.col_index === 0) {
            cellHighlight.classList.add('type-label');
          } else if (cell.audited && !cell.audit_pass) {
            cellHighlight.classList.add('type-fail');
          } else {
            cellHighlight.classList.add('type-metric');
          }

          cellHighlight.style.left = `${cell.bbox.x0}px`;
          cellHighlight.style.top = `${cell.bbox.y0}px`;
          cellHighlight.style.width = `${w}px`;
          cellHighlight.style.height = `${h}px`;
          cellHighlight.id = `bbox_c_${cell.row_index}_${cell.col_index}`;

          cellHighlight.addEventListener('mouseenter', () => {
            cellDetails.textContent = `[Cell R${cell.row_index}:C${cell.col_index}] Value: ${cell.value} (Raw: "${cell.raw_text}") | Audited: ${cell.audited ? (cell.audit_pass ? 'PASS' : 'FAIL') : 'NO'} | BBox: [${cell.bbox.x0.toFixed(1)}, ${cell.bbox.y0.toFixed(1)}, ${cell.bbox.x1.toFixed(1)}, ${cell.bbox.y1.toFixed(1)}]`;
          });

          tokenLayer.appendChild(cellHighlight);
        }
      });
    });
  }

  function renderTable(table) {
    // Header
    tableHead.innerHTML = '';
    const trHead = document.createElement('tr');
    (table.columns || []).forEach((col, cIdx) => {
      const th = document.createElement('th');
      th.textContent = col.name;
      trHead.appendChild(th);
    });
    tableHead.appendChild(trHead);

    // Body
    tableBody.innerHTML = '';
    (table.rows || []).forEach(row => {
      if (row.is_header) return;

      const tr = document.createElement('tr');
      if (row.is_total) tr.classList.add('row-total');

      (table.columns || []).forEach((col, cIdx) => {
        const td = document.createElement('td');
        const cell = (row.cells || []).find(c => c.col_index === cIdx);

        if (cIdx === 0) {
          const indent = '&nbsp;&nbsp;'.repeat(row.level || 0);
          td.innerHTML = indent + (row.label || '');
        } else if (cell) {
          td.textContent = cell.raw_text || '';
          if (cell.audited) {
            if (cell.audit_pass) {
              td.classList.add('cell-audited-pass');
              td.innerHTML += ' <span class="audit-indicator text-success">✓</span>';
            } else {
              td.classList.add('cell-audited-fail');
              td.innerHTML += ' <span class="audit-indicator text-danger">✗</span>';
            }
          }
        }

        td.addEventListener('mouseenter', () => {
          if (cell) {
            cellDetails.textContent = `[Cell R${row.index}:C${cIdx}] "${cell.raw_text}" | Num: ${cell.value} | Page: ${cell.page_num} | Box: [${cell.bbox ? cell.bbox.x0.toFixed(1) : 0}, ${cell.bbox ? cell.bbox.y0.toFixed(1) : 0}, ${cell.bbox ? cell.bbox.x1.toFixed(1) : 0}, ${cell.bbox ? cell.bbox.y1.toFixed(1) : 0}]`;
            const boxElem = document.getElementById(`bbox_c_${row.index}_${cIdx}`);
            if (boxElem) boxElem.classList.add('active');
          }
        });

        td.addEventListener('mouseleave', () => {
          const boxElem = document.getElementById(`bbox_c_${row.index}_${cIdx}`);
          if (boxElem) boxElem.classList.remove('active');
        });

        tr.appendChild(td);
      });

      tableBody.appendChild(tr);
    });
  }

  function renderAudit(report) {
    const auditBadge = document.getElementById('auditBadge');
    const auditStatusVal = document.getElementById('auditStatusVal');
    const auditConfVal = document.getElementById('auditConfVal');
    const auditCheckedVal = document.getElementById('auditCheckedVal');
    const auditDiscVal = document.getElementById('auditDiscVal');
    const invariantsList = document.getElementById('invariantsList');

    if (!report) {
      auditBadge.textContent = 'N/A';
      auditStatusVal.textContent = 'NOT AUDITED';
      invariantsList.innerHTML = '<div class="empty-state">No audit executed for this table.</div>';
      return;
    }

    const confPct = (report.confidence_score * 100).toFixed(1);
    auditBadge.textContent = `${confPct}%`;
    auditConfVal.textContent = `${confPct}%`;
    auditCheckedVal.textContent = report.total_checked;
    auditDiscVal.textContent = report.discrepancy_count;

    if (report.passed) {
      auditStatusVal.textContent = 'PASSED';
      auditStatusVal.className = 'metric-val text-success';
      auditDiscVal.className = 'metric-val text-success';
    } else {
      auditStatusVal.textContent = 'DISCREPANCIES';
      auditStatusVal.className = 'metric-val text-danger';
      auditDiscVal.className = 'metric-val text-danger';
    }

    invariantsList.innerHTML = '';
    if (!report.checks || report.checks.length === 0) {
      invariantsList.innerHTML = '<div class="empty-state">No mathematical relations identified for verification.</div>';
      return;
    }

    report.checks.forEach(check => {
      const card = document.createElement('div');
      card.className = `invariant-card ${check.passed ? 'pass' : 'fail'}`;

      card.innerHTML = `
        <div class="inv-header">
          <span class="inv-title">${check.name} (Col ${check.col_index})</span>
          <span class="badge-audit" style="color: ${check.passed ? '#3fb950' : '#f85149'}; background: ${check.passed ? 'rgba(63,185,80,0.2)' : 'rgba(248,81,73,0.2)'}">
            ${check.passed ? 'PASS' : 'FAIL'}
          </span>
        </div>
        <div class="inv-formula">${check.formula}</div>
        <div class="inv-stats">
          <span>Expected: <strong>${check.expected.toLocaleString()}</strong></span>
          <span>Actual: <strong>${check.actual.toLocaleString()}</strong></span>
          <span>Delta: <strong>${check.delta.toFixed(4)}</strong></span>
        </div>
      `;

      invariantsList.appendChild(card);
    });
  }

  // SQL Console
  runSqlBtn.addEventListener('click', executeSQL);
  sqlInput.addEventListener('keydown', (e) => {
    if (e.ctrlKey && e.key === 'Enter') {
      executeSQL();
    }
  });

  async function executeSQL() {
    const query = sqlInput.value.trim();
    if (!query) return;

    sqlResults.innerHTML = '<div class="empty-state">Executing SQL query...</div>';

    try {
      const startTime = performance.now();
      const res = await fetch('/api/query', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sql: query })
      });

      const elapsed = (performance.now() - startTime).toFixed(2);
      const data = await res.json();

      if (!res.ok || data.error) {
        sqlResults.innerHTML = `<div class="empty-state" style="color: #f85149">Error: ${data.error || 'Execution failed'}</div>`;
        return;
      }

      if (!data.rows || data.rows.length === 0) {
        sqlResults.innerHTML = `<div class="empty-state">Query returned 0 rows (${elapsed} ms).</div>`;
        return;
      }

      let html = `<div style="margin-bottom: 8px; font-size: 11px; color: #8b949e">Returned ${data.rows.length} rows in ${elapsed} ms:</div>`;
      html += '<table class="financial-table"><thead><tr>';
      (data.columns || []).forEach(col => {
        html += `<th>${col}</th>`;
      });
      html += '</tr></thead><tbody>';

      data.rows.forEach(r => {
        html += '<tr>';
        (r || []).forEach((val, i) => {
          html += `<td>${val !== null ? val : '<em>NULL</em>'}</td>`;
        });
        html += '</tr>';
      });
      html += '</tbody></table>';

      sqlResults.innerHTML = html;
    } catch (err) {
      sqlResults.innerHTML = `<div class="empty-state" style="color: #f85149">Failed to execute: ${err.message}</div>`;
    }
  }
});
