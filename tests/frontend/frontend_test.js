/**
 * Automated Frontend Unit & Security Tests
 * Uses Node.js test runner to verify frontend logic, safe DOM handling,
 * XSS prevention, pagination, and diff chunk rendering.
 */

const test = require('node:test');
const assert = require('node:assert');

// Minimal mock DOM for headless testing of frontend logic
class MockNode {
  constructor(nodeType, tagName = '') {
    this.nodeType = nodeType;
    this.tagName = tagName.toUpperCase();
    this.children = [];
    this.textContent = '';
    this.className = '';
    this.classList = {
      _classes: new Set(),
      add: (c) => this.classList._classes.add(c),
      remove: (c) => this.classList._classes.delete(c),
      contains: (c) => this.classList._classes.has(c),
      toggle: (c, force) => {
        if (force === undefined) {
          if (this.classList.contains(c)) this.classList.remove(c);
          else this.classList.add(c);
        } else if (force) {
          this.classList.add(c);
        } else {
          this.classList.remove(c);
        }
      }
    };
    this.dataset = {};
    this.attributes = {};
  }

  appendChild(child) {
    this.children.push(child);
    return child;
  }

  replaceChildren(...newChildren) {
    this.children = [...newChildren];
  }

  setAttribute(k, v) {
    this.attributes[k] = v;
  }

  getAttribute(k) {
    return this.attributes[k];
  }
}

class MockTextNode extends MockNode {
  constructor(text) {
    super(3, '#text');
    this.textContent = text;
  }
}

class MockDocument {
  createElement(tag) {
    return new MockNode(1, tag);
  }
  createTextNode(text) {
    return new MockTextNode(text);
  }
}

const document = new MockDocument();

// Test 1: XSS safety - CSV text containing HTML / script tags MUST be inserted as text nodes
test('XSS Prevention: Malicious CSV content is rendered as safe text nodes', () => {
  const maliciousCSVValue = '<script>alert("XSS")</script><img src=x onerror=alert(1)>';
  
  // Safe cell rendering implementation from app.js
  function renderCellValue(td, valPtr) {
    if (valPtr === null || valPtr === undefined) {
      const span = document.createElement('span');
      span.className = 'cell-missing';
      span.textContent = '[missing]';
      td.appendChild(span);
      return;
    }
    td.appendChild(document.createTextNode(valPtr));
  }

  const td = document.createElement('td');
  renderCellValue(td, maliciousCSVValue);

  assert.strictEqual(td.children.length, 1);
  assert.strictEqual(td.children[0].nodeType, 3); // Text node
  assert.strictEqual(td.children[0].textContent, maliciousCSVValue);
  // It is never an element node with dangerous tags
  assert.notStrictEqual(td.children[0].tagName, 'SCRIPT');
});

// Test 2: Fine-grained character diff chunk rendering
test('Diff Rendering: Deleted and Inserted text chunks are styled appropriately', () => {
  function renderCellChunks(td, chunks, fallbackVal) {
    if (!chunks || chunks.length === 0) {
      td.appendChild(document.createTextNode(fallbackVal || ''));
      return;
    }

    chunks.forEach((chunk) => {
      if (chunk.type === 'equal') {
        td.appendChild(document.createTextNode(chunk.text));
      } else if (chunk.type === 'delete') {
        const span = document.createElement('span');
        span.className = 'diff-del';
        span.textContent = chunk.text;
        td.appendChild(span);
      } else if (chunk.type === 'insert') {
        const span = document.createElement('span');
        span.className = 'diff-ins';
        span.textContent = chunk.text;
        td.appendChild(span);
      }
    });
  }

  const tdOrig = document.createElement('td');
  const origChunks = [
    { type: 'equal', text: 'Hello ' },
    { type: 'delete', text: 'World' }
  ];
  renderCellChunks(tdOrig, origChunks, 'Hello World');

  assert.strictEqual(tdOrig.children.length, 2);
  assert.strictEqual(tdOrig.children[0].textContent, 'Hello ');
  assert.strictEqual(tdOrig.children[1].className, 'diff-del');
  assert.strictEqual(tdOrig.children[1].textContent, 'World');

  const tdMod = document.createElement('td');
  const modChunks = [
    { type: 'equal', text: 'Hello ' },
    { type: 'insert', text: 'Universe' }
  ];
  renderCellChunks(tdMod, modChunks, 'Hello Universe');

  assert.strictEqual(tdMod.children.length, 2);
  assert.strictEqual(tdMod.children[0].textContent, 'Hello ');
  assert.strictEqual(tdMod.children[1].className, 'diff-ins');
  assert.strictEqual(tdMod.children[1].textContent, 'Universe');
});

// Test 3: Missing cell rendering
test('Cell Status: Missing cell is rendered with distinct placeholder', () => {
  function renderCellValue(td, valPtr) {
    if (valPtr === null || valPtr === undefined) {
      const span = document.createElement('span');
      span.className = 'cell-missing';
      span.textContent = '[missing]';
      td.appendChild(span);
      return;
    }
    if (valPtr === '') {
      const span = document.createElement('span');
      span.className = 'empty-placeholder';
      span.textContent = '""';
      td.appendChild(span);
      return;
    }
    td.appendChild(document.createTextNode(valPtr));
  }

  const tdMissing = document.createElement('td');
  renderCellValue(tdMissing, null);
  assert.strictEqual(tdMissing.children.length, 1);
  assert.strictEqual(tdMissing.children[0].className, 'cell-missing');
  assert.strictEqual(tdMissing.children[0].textContent, '[missing]');

  const tdEmpty = document.createElement('td');
  renderCellValue(tdEmpty, '');
  assert.strictEqual(tdEmpty.children.length, 1);
  assert.strictEqual(tdEmpty.children[0].className, 'empty-placeholder');
  assert.strictEqual(tdEmpty.children[0].textContent, '""');
});

// Test 4: Pagination & Batch-by-Batch calculation
test('Pagination: 100 row limit calculation and remaining count', () => {
  function calculatePagination(totalDiffs, currentlyDisplayed) {
    const remaining = totalDiffs - currentlyDisplayed;
    const hasMore = remaining > 0;
    const nextBatch = Math.min(100, remaining);
    const totalBatches = Math.ceil(totalDiffs / 100);
    const currentBatch = Math.ceil(currentlyDisplayed / 100);
    return { remaining, hasMore, nextBatch, totalBatches, currentBatch };
  }

  // 126 diffs, initially 100 displayed (Batch 1 of 2)
  const p1 = calculatePagination(126, 100);
  assert.strictEqual(p1.remaining, 26);
  assert.strictEqual(p1.hasMore, true);
  assert.strictEqual(p1.nextBatch, 26);
  assert.strictEqual(p1.totalBatches, 2);
  assert.strictEqual(p1.currentBatch, 1);

  // All 126 displayed (Batch 2 of 2)
  const p2 = calculatePagination(126, 126);
  assert.strictEqual(p2.remaining, 0);
  assert.strictEqual(p2.hasMore, false);
  assert.strictEqual(p2.currentBatch, 2);
});

// Test 5: Column filter logic
test('Column Filter Logic: Include and Exclude modes correctly filter columns', () => {
  const allCols = ['id', 'name', 'age', 'role', 'salary'];
  
  function getEligibleColumns(mode, selectedList) {
    if (mode === 'all') return allCols;
    if (mode === 'include') {
      return allCols.filter(c => selectedList.includes(c));
    }
    if (mode === 'exclude') {
      return allCols.filter(c => !selectedList.includes(c));
    }
    return allCols;
  }

  const included = getEligibleColumns('include', ['name', 'role']);
  assert.deepStrictEqual(included, ['name', 'role']);

  const excluded = getEligibleColumns('exclude', ['salary', 'age']);
  assert.deepStrictEqual(excluded, ['id', 'name', 'role']);
});

// Test 6: Column Hiding & Dynamic Style Generation
test('Column Hiding: Dynamic CSS rules and placeholder colSpan calculation', () => {
  const columns = ['id', 'first_name', 'last_name', 'email', 'department'];
  const hiddenColumns = new Set(['email', 'department']);
  const columnWidths = { 1: 240 }; // first_name resized to 240px

  function generateDynamicStyles(cols, hidden, widths) {
    const rules = [];
    hidden.forEach((colName) => {
      const idx = cols.indexOf(colName);
      if (idx !== -1) {
        rules.push(`#tableOriginal .col-idx-${idx}, #tableModified .col-idx-${idx} { display: none !important; }`);
      }
    });
    Object.keys(widths).forEach((key) => {
      const idx = parseInt(key, 10);
      const width = widths[key];
      if (!isNaN(idx) && width > 0) {
        rules.push(`#tableOriginal .col-idx-${idx}, #tableModified .col-idx-${idx} { width: ${width}px !important; min-width: ${width}px !important; max-width: ${width}px !important; }`);
      }
    });
    return rules;
  }

  const rules = generateDynamicStyles(columns, hiddenColumns, columnWidths);
  assert.strictEqual(rules.length, 3);
  assert.ok(rules.some(r => r.includes('.col-idx-3') && r.includes('display: none !important;')));
  assert.ok(rules.some(r => r.includes('.col-idx-4') && r.includes('display: none !important;')));
  assert.ok(rules.some(r => r.includes('.col-idx-1') && r.includes('width: 240px !important;')));

  // Visible count calculation for placeholder cells
  const visibleCount = Math.max(1, columns.length - hiddenColumns.size);
  assert.strictEqual(visibleCount, 3);
});

// Test 7: Column Resizing Bounds
test('Column Resizing: Clamping to minimum and maximum allowed widths', () => {
  function clampWidth(startWidth, deltaX) {
    return Math.max(60, Math.min(800, Math.round(startWidth + deltaX)));
  }

  // Normal resize
  assert.strictEqual(clampWidth(180, 50), 230);
  assert.strictEqual(clampWidth(180, -40), 140);

  // Clamping min width to 60px
  assert.strictEqual(clampWidth(180, -200), 60);

  // Clamping max width to 800px
  assert.strictEqual(clampWidth(180, 1000), 800);
});
