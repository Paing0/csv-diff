/**
 * CSV Diff - Modern shadcn/ui Frontend
 * Features: Batch-by-batch loading (100 by 100), auto-scroll pagination,
 * safe DOM rendering, synchronized scrolling, search, and keyboard diff navigation.
 */

(function () {
  'use strict';

  // Constants
  const BATCH_SIZE = 100;

  // Application State
  const state = {
    sessionId: null,
    origFile: null,
    modFile: null,
    meta: null,
    summary: null,
    columns: [],
    displayedRows: [],
    totalDiffs: 0,
    activeDiffIndex: -1,
    currentFilter: 'all',
    searchQuery: '',
    isLoading: false,
    isBatchLoadingAll: false,
    cancelBatchLoad: false,
    autoLoadOnScroll: true,
    syncScroll: true, // Synced scrolling by default
    hiddenColumns: new Set(),
    columnWidths: {},
    theme: localStorage.getItem('csvdiff_theme') || (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'),
  };

  // DOM Elements
  const el = {
    // Header & Theme
    brandHomeBtn: document.getElementById('brandHomeBtn'),
    themeToggleBtn: document.getElementById('themeToggleBtn'),
    themeIconSun: document.getElementById('themeIconSun'),
    themeIconMoon: document.getElementById('themeIconMoon'),
    newComparisonBtn: document.getElementById('newComparisonBtn'),

    // Sections
    uploadSection: document.getElementById('uploadSection'),
    configSection: document.getElementById('configSection'),
    resultsSection: document.getElementById('resultsSection'),
    loadingOverlay: document.getElementById('loadingOverlay'),
    loadingText: document.getElementById('loadingText'),
    loadingBatchSub: document.getElementById('loadingBatchSub'),

    // Upload
    originalDropzone: document.getElementById('originalDropzone'),
    originalFileInput: document.getElementById('originalFileInput'),
    originalFileInfo: document.getElementById('originalFileInfo'),
    originalFileName: document.getElementById('originalFileName'),
    originalFileSize: document.getElementById('originalFileSize'),

    modifiedDropzone: document.getElementById('modifiedDropzone'),
    modifiedFileInput: document.getElementById('modifiedFileInput'),
    modifiedFileInfo: document.getElementById('modifiedFileInfo'),
    modifiedFileName: document.getElementById('modifiedFileName'),
    modifiedFileSize: document.getElementById('modifiedFileSize'),

    hasHeaderCheckbox: document.getElementById('hasHeaderCheckbox'),
    inspectFilesBtn: document.getElementById('inspectFilesBtn'),
    uploadError: document.getElementById('uploadError'),
    uploadErrorMsg: document.getElementById('uploadErrorMsg'),

    // Config
    comparisonModeRadios: document.getElementsByName('comparisonMode'),
    pkSelectorWrapper: document.getElementById('pkSelectorWrapper'),
    pkColumnSelect: document.getElementById('pkColumnSelect'),

    columnModeTabs: document.querySelectorAll('.tab-trigger'),
    columnFilterModeRadios: document.getElementsByName('columnFilterMode'),
    columnFilterWrapper: document.getElementById('columnFilterWrapper'),
    columnSearchInput: document.getElementById('columnSearchInput'),
    columnsCheckboxList: document.getElementById('columnsCheckboxList'),
    columnFilterInfo: document.getElementById('columnFilterInfo'),
    selectAllColsBtn: document.getElementById('selectAllColsBtn'),
    deselectAllColsBtn: document.getElementById('deselectAllColsBtn'),
    resetColsBtn: document.getElementById('resetColsBtn'),

    startCompareBtn: document.getElementById('startCompareBtn'),
    cancelCompareBtn: document.getElementById('cancelCompareBtn'),
    configError: document.getElementById('configError'),
    configErrorMsg: document.getElementById('configErrorMsg'),

    // Summary & Metrics
    sumOrigTag: document.getElementById('sumOrigTag'),
    sumModTag: document.getElementById('sumModTag'),
    statTotalDiffs: document.getElementById('statTotalDiffs'),
    statModified: document.getElementById('statModified'),
    statAdded: document.getElementById('statAdded'),
    statRemoved: document.getElementById('statRemoved'),
    statChangedCells: document.getElementById('statChangedCells'),
    statUnchanged: document.getElementById('statUnchanged'),

    // Toolbar & Navigation
    filterTabs: document.querySelectorAll('.filter-tab'),
    searchDiffsInput: document.getElementById('searchDiffsInput'),
    clearSearchBtn: document.getElementById('clearSearchBtn'),
    columnsDropdownWrapper: document.getElementById('columnsDropdownWrapper'),
    columnsVisibilityBtn: document.getElementById('columnsVisibilityBtn'),
    columnsHiddenBadge: document.getElementById('columnsHiddenBadge'),
    columnsVisibilityMenu: document.getElementById('columnsVisibilityMenu'),
    resetColumnsVisibilityBtn: document.getElementById('resetColumnsVisibilityBtn'),
    columnsVisibilitySearch: document.getElementById('columnsVisibilitySearch'),
    columnsVisibilityList: document.getElementById('columnsVisibilityList'),
    dynamicTableStyles: document.getElementById('dynamicTableStyles'),
    prevDiffBtn: document.getElementById('prevDiffBtn'),
    nextDiffBtn: document.getElementById('nextDiffBtn'),
    diffPositionIndicator: document.getElementById('diffPositionIndicator'),
    syncScrollCheckbox: document.getElementById('syncScrollCheckbox'),
    autoLoadScrollCheckbox: document.getElementById('autoLoadScrollCheckbox'),

    // Tables
    noDiffsCard: document.getElementById('noDiffsCard'),
    tablesWrapper: document.getElementById('tablesWrapper'),
    paneOriginal: document.getElementById('paneOriginal'),
    paneModified: document.getElementById('paneModified'),
    scrollAreaOrig: document.getElementById('scrollAreaOrig'),
    scrollAreaMod: document.getElementById('scrollAreaMod'),
    theadOriginal: document.getElementById('theadOriginal'),
    tbodyOriginal: document.getElementById('tbodyOriginal'),
    theadModified: document.getElementById('theadModified'),
    tbodyModified: document.getElementById('tbodyModified'),
    sentinelOrig: document.getElementById('infiniteSentinelOrig'),
    sentinelMod: document.getElementById('infiniteSentinelMod'),

    // Batch Loader / Pagination
    paginationBar: document.getElementById('paginationBar'),
    displayedCount: document.getElementById('displayedCount'),
    totalDiffCount: document.getElementById('totalDiffCount'),
    batchProgressLabel: document.getElementById('batchProgressLabel'),
    showMoreBtn: document.getElementById('showMoreBtn'),
    loadAllBtn: document.getElementById('loadAllBtn'),
  };

  // Helper: Format bytes
  function formatBytes(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  // Helper: Debounce
  function debounce(fn, delay) {
    let timer = null;
    return function (...args) {
      clearTimeout(timer);
      timer = setTimeout(() => fn.apply(this, args), delay);
    };
  }

  // Theme Management
  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    if (theme === 'dark') {
      el.themeIconSun.style.display = 'block';
      el.themeIconMoon.style.display = 'none';
    } else {
      el.themeIconSun.style.display = 'none';
      el.themeIconMoon.style.display = 'block';
    }
    localStorage.setItem('csvdiff_theme', theme);
    state.theme = theme;
  }

  function toggleTheme() {
    applyTheme(state.theme === 'dark' ? 'light' : 'dark');
  }

  // File Picker & Dropzone Setup
  function setupFilePickers() {
    setupDropzone(el.originalDropzone, el.originalFileInput, (file) => {
      state.origFile = file;
      updateFileInfo(file, el.originalFileInfo, el.originalFileName, el.originalFileSize, el.originalDropzone);
      validateFilesReady();
    });

    setupDropzone(el.modifiedDropzone, el.modifiedFileInput, (file) => {
      state.modFile = file;
      updateFileInfo(file, el.modifiedFileInfo, el.modifiedFileName, el.modifiedFileSize, el.modifiedDropzone);
      validateFilesReady();
    });

    el.inspectFilesBtn.addEventListener('click', handleInspectFiles);
  }

  function setupDropzone(dropzone, fileInput, onFileSelected) {
    dropzone.addEventListener('click', (e) => {
      if (e.target !== fileInput) fileInput.click();
    });

    dropzone.addEventListener('keydown', (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        fileInput.click();
      }
    });

    fileInput.addEventListener('change', () => {
      if (fileInput.files.length > 0) {
        onFileSelected(fileInput.files[0]);
      }
    });

    ['dragenter', 'dragover'].forEach((eventName) => {
      dropzone.addEventListener(eventName, (e) => {
        e.preventDefault();
        e.stopPropagation();
        dropzone.classList.add('dragover');
      });
    });

    ['dragleave', 'drop'].forEach((eventName) => {
      dropzone.addEventListener(eventName, (e) => {
        e.preventDefault();
        e.stopPropagation();
        dropzone.classList.remove('dragover');
      });
    });

    dropzone.addEventListener('drop', (e) => {
      const dt = e.dataTransfer;
      if (dt.files && dt.files.length > 0) {
        onFileSelected(dt.files[0]);
      }
    });
  }

  function updateFileInfo(file, infoEl, nameEl, sizeEl, dropzone) {
    nameEl.textContent = file.name;
    sizeEl.textContent = formatBytes(file.size);
    infoEl.style.display = 'inline-flex';
    dropzone.classList.add('has-file');
  }

  function validateFilesReady() {
    const ready = state.origFile !== null && state.modFile !== null;
    el.inspectFilesBtn.disabled = !ready;
    hideAlert(el.uploadError, el.uploadErrorMsg);
  }

  // Upload & Inspect Files
  async function handleInspectFiles() {
    if (!state.origFile || !state.modFile) return;

    hideAlert(el.uploadError, el.uploadErrorMsg);
    showLoading('Inspecting and validating CSV files...');

    const formData = new FormData();
    formData.append('original_file', state.origFile);
    formData.append('modified_file', state.modFile);
    formData.append('has_header', el.hasHeaderCheckbox.checked ? 'true' : 'false');

    try {
      const response = await fetch('/api/upload', {
        method: 'POST',
        body: formData,
      });

      const data = await response.json();
      hideLoading();

      if (!response.ok) {
        showAlert(el.uploadError, el.uploadErrorMsg, data.error || 'Failed to inspect files');
        return;
      }

      state.sessionId = data.session_id;
      state.meta = data;

      showConfigSection(data);
    } catch (err) {
      hideLoading();
      showAlert(el.uploadError, el.uploadErrorMsg, 'Connection error: ' + err.message);
    }
  }

  // Show Config Section
  function showConfigSection(data) {
    el.uploadSection.style.display = 'none';
    el.configSection.style.display = 'block';
    el.newComparisonBtn.style.display = 'inline-flex';

    // Populate Primary Key select
    el.pkColumnSelect.replaceChildren();
    data.all_columns.forEach((col) => {
      const opt = document.createElement('option');
      opt.value = col;
      opt.textContent = col;
      el.pkColumnSelect.appendChild(opt);
    });

    // Populate Column checkboxes
    populateColumnsList(data.all_columns);
    setColumnFilterTab('all');
  }

  function populateColumnsList(columns) {
    el.columnsCheckboxList.replaceChildren();
    columns.forEach((col) => {
      const item = document.createElement('label');
      item.className = 'column-item-label';
      item.dataset.colName = col.toLowerCase();

      const checkbox = document.createElement('input');
      checkbox.type = 'checkbox';
      checkbox.value = col;
      checkbox.checked = true;
      checkbox.className = 'col-checkbox';

      const labelSpan = document.createElement('span');
      labelSpan.textContent = col;

      item.appendChild(checkbox);
      item.appendChild(labelSpan);
      el.columnsCheckboxList.appendChild(item);
    });
  }

  function setColumnFilterTab(mode) {
    el.columnModeTabs.forEach((tab) => {
      tab.classList.toggle('active', tab.dataset.colMode === mode);
    });

    // Sync hidden radios
    for (const r of el.columnFilterModeRadios) {
      r.checked = (r.value === mode);
    }

    const isFiltered = (mode !== 'all');
    el.columnFilterWrapper.style.display = isFiltered ? 'block' : 'none';

    if (mode === 'include') {
      el.columnFilterInfo.textContent = 'Only checked columns will participate in difference comparison.';
    } else if (mode === 'exclude') {
      el.columnFilterInfo.textContent = 'Checked columns will be EXCLUDED from value comparison.';
    }
  }

  function setupConfigListeners() {
    Array.from(el.comparisonModeRadios).forEach((radio) => {
      radio.addEventListener('change', () => {
        const isPK = radio.value === 'primary_key';
        el.pkSelectorWrapper.style.display = isPK ? 'block' : 'none';
      });
    });

    el.columnModeTabs.forEach((tab) => {
      tab.addEventListener('click', () => {
        setColumnFilterTab(tab.dataset.colMode);
      });
    });

    // Column search filter
    el.columnSearchInput.addEventListener('input', (e) => {
      const q = e.target.value.toLowerCase().trim();
      const items = el.columnsCheckboxList.querySelectorAll('.column-item-label');
      items.forEach((item) => {
        item.style.display = item.dataset.colName.includes(q) ? 'flex' : 'none';
      });
    });

    el.selectAllColsBtn.addEventListener('click', () => {
      el.columnsCheckboxList.querySelectorAll('.col-checkbox').forEach((cb) => (cb.checked = true));
    });

    el.deselectAllColsBtn.addEventListener('click', () => {
      el.columnsCheckboxList.querySelectorAll('.col-checkbox').forEach((cb) => (cb.checked = false));
    });

    el.resetColsBtn.addEventListener('click', () => {
      el.columnsCheckboxList.querySelectorAll('.col-checkbox').forEach((cb) => (cb.checked = true));
      el.columnSearchInput.value = '';
      el.columnsCheckboxList.querySelectorAll('.column-item-label').forEach((item) => (item.style.display = 'flex'));
    });

    el.startCompareBtn.addEventListener('click', handleStartCompare);
    el.cancelCompareBtn.addEventListener('click', handleCancelCompare);
  }

  function getSelectedRadioValue(radios) {
    for (const r of radios) {
      if (r.checked) return r.value;
    }
    return '';
  }

  // Compare Execution
  async function handleStartCompare() {
    hideAlert(el.configError, el.configErrorMsg);

    const mode = getSelectedRadioValue(el.comparisonModeRadios);
    const filterMode = getSelectedRadioValue(el.columnFilterModeRadios);

    const selectedCols = [];
    el.columnsCheckboxList.querySelectorAll('.col-checkbox:checked').forEach((cb) => {
      selectedCols.push(cb.value);
    });

    const options = {
      session_id: state.sessionId,
      mode: mode,
      primary_key_col: mode === 'primary_key' ? el.pkColumnSelect.value : '',
      column_filter: {
        mode: filterMode,
        columns: selectedCols,
      },
      has_header: el.hasHeaderCheckbox.checked,
    };

    showLoading('Running comparison in Go engine...');
    el.cancelCompareBtn.style.display = 'inline-flex';

    try {
      const response = await fetch('/api/compare', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(options),
      });

      const data = await response.json();
      hideLoading();
      el.cancelCompareBtn.style.display = 'none';

      if (!response.ok) {
        let msg = data.error || 'Comparison failed';
        if (data.summary && data.summary.duplicate_keys && data.summary.duplicate_keys.length > 0) {
          const dupes = data.summary.duplicate_keys
            .slice(0, 5)
            .map((d) => `"${d.key}" in ${d.file_source} (rows ${d.rows.join(', ')})`)
            .join('; ');
          msg += `: Duplicate keys: ${dupes}`;
        }
        showAlert(el.configError, el.configErrorMsg, msg);
        return;
      }

      state.summary = data.summary;
      state.columns = data.summary.columns;
      state.displayedRows = data.rows || [];
      state.totalDiffs = data.summary.total_differing_rows;

      showResultsSection(data);
    } catch (err) {
      hideLoading();
      el.cancelCompareBtn.style.display = 'none';
      showAlert(el.configError, el.configErrorMsg, 'Connection error: ' + err.message);
    }
  }

  async function handleCancelCompare() {
    if (!state.sessionId) return;
    try {
      await fetch('/api/cancel', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_id: state.sessionId }),
      });
    } catch (err) {
      console.warn('Cancel request error:', err);
    }
  }

  // Show Results
  function showResultsSection(data) {
    el.configSection.style.display = 'none';
    el.resultsSection.style.display = 'block';

    if (state.meta) {
      el.sumOrigTag.textContent = `${state.meta.original.file_name} (${state.meta.original.row_count} rows)`;
      el.sumModTag.textContent = `${state.meta.modified.file_name} (${state.meta.modified.row_count} rows)`;
    }

    const s = data.summary;
    el.statTotalDiffs.textContent = s.total_differing_rows.toLocaleString();
    el.statModified.textContent = s.modified_rows.toLocaleString();
    el.statAdded.textContent = s.added_rows.toLocaleString();
    el.statRemoved.textContent = s.removed_rows.toLocaleString();
    el.statChangedCells.textContent = s.total_changed_cells.toLocaleString();
    el.statUnchanged.textContent = s.unchanged_rows.toLocaleString();

    state.currentFilter = 'all';
    state.searchQuery = '';
    state.activeDiffIndex = -1;
    el.searchDiffsInput.value = '';
    el.clearSearchBtn.style.display = 'none';
    el.filterTabs.forEach((t) => t.classList.toggle('active', t.dataset.filter === 'all'));

    state.hiddenColumns.clear();
    state.columnWidths = {};
    updateDynamicTableStyles();
    updateColumnsBtnBadge();
    initColumnsVisibilityUI(s.columns);

    renderTableHeaders(s.columns);

    if (s.total_differing_rows === 0) {
      el.noDiffsCard.style.display = 'block';
      el.tablesWrapper.style.display = 'none';
      el.paginationBar.style.display = 'none';
    } else {
      el.noDiffsCard.style.display = 'none';
      el.tablesWrapper.style.display = 'grid';
      el.paginationBar.style.display = 'flex';

      renderTableRows(state.displayedRows, false);
      updateBatchUI(s.total_differing_rows, data.has_more);
    }
  }

  // Synchronized Scrolling
  function setupSynchronizedScrolling() {
    let isSyncingOrig = false;
    let isSyncingMod = false;

    el.scrollAreaOrig.addEventListener('scroll', () => {
      if (state.syncScroll && !isSyncingOrig) {
        isSyncingMod = true;
        el.scrollAreaMod.scrollTop = el.scrollAreaOrig.scrollTop;
        el.scrollAreaMod.scrollLeft = el.scrollAreaOrig.scrollLeft;
      }
      isSyncingOrig = false;
      checkInfiniteScroll(el.scrollAreaOrig);
    });

    el.scrollAreaMod.addEventListener('scroll', () => {
      if (state.syncScroll && !isSyncingMod) {
        isSyncingMod = true;
        el.scrollAreaOrig.scrollTop = el.scrollAreaMod.scrollTop;
        el.scrollAreaMod.scrollLeft = el.scrollAreaMod.scrollLeft;
      }
      isSyncingMod = false;
      checkInfiniteScroll(el.scrollAreaMod);
    });

    if (el.syncScrollCheckbox) {
      el.syncScrollCheckbox.checked = state.syncScroll;
      el.syncScrollCheckbox.addEventListener('change', (e) => {
        state.syncScroll = e.target.checked;
        if (state.syncScroll) {
          el.scrollAreaMod.scrollTop = el.scrollAreaOrig.scrollTop;
          el.scrollAreaMod.scrollLeft = el.scrollAreaOrig.scrollLeft;
        }
      });
    }
  }



  // Infinite Scroll Trigger (Auto-load 100 by 100)
  function checkInfiniteScroll(scrollArea) {
    if (!state.autoLoadOnScroll || state.isLoading || state.isBatchLoadingAll) return;
    if (state.displayedRows.length >= state.totalDiffs) return;

    const threshold = 180; // Trigger when within 180px of bottom
    const distanceFromBottom = scrollArea.scrollHeight - (scrollArea.scrollTop + scrollArea.clientHeight);

    if (distanceFromBottom <= threshold) {
      loadNextBatch(false);
    }
  }

  // Column Visibility Popover Management
  function initColumnsVisibilityUI(columns) {
    if (!el.columnsVisibilityList) return;
    el.columnsVisibilityList.replaceChildren();

    columns.forEach((col, idx) => {
      const item = document.createElement('label');
      item.className = 'dropdown-col-item';
      item.dataset.colName = col.toLowerCase();

      const checkbox = document.createElement('input');
      checkbox.type = 'checkbox';
      checkbox.value = col;
      checkbox.checked = !state.hiddenColumns.has(col);
      checkbox.dataset.colIdx = idx;
      checkbox.dataset.colName = col;

      checkbox.addEventListener('change', () => {
        const visibleCount = state.columns.length - state.hiddenColumns.size;
        if (!checkbox.checked && visibleCount <= 1) {
          // Keep at least one column visible
          checkbox.checked = true;
          return;
        }

        if (checkbox.checked) {
          state.hiddenColumns.delete(col);
        } else {
          state.hiddenColumns.add(col);
        }

        updateDynamicTableStyles();
        updatePlaceholderColspans();
        updateColumnsBtnBadge();
        syncRowHeights(state.displayedRows, false);
      });

      const span = document.createElement('span');
      span.textContent = col;

      item.appendChild(checkbox);
      item.appendChild(span);
      el.columnsVisibilityList.appendChild(item);
    });

    if (el.columnsVisibilitySearch) {
      el.columnsVisibilitySearch.value = '';
    }
  }

  function updateColumnsBtnBadge() {
    if (!el.columnsHiddenBadge) return;
    const count = state.hiddenColumns.size;
    if (count > 0) {
      el.columnsHiddenBadge.textContent = `${state.columns.length - count}/${state.columns.length}`;
      el.columnsHiddenBadge.style.display = 'inline-flex';
    } else {
      el.columnsHiddenBadge.style.display = 'none';
    }
  }

  function updatePlaceholderColspans() {
    const visibleCount = Math.max(1, state.columns.length - state.hiddenColumns.size);
    const placeholders = document.querySelectorAll('.cell-empty-placeholder');
    placeholders.forEach((td) => {
      td.colSpan = visibleCount;
    });
  }

  function updateDynamicTableStyles() {
    if (!el.dynamicTableStyles) return;
    const rules = [];

    // Hidden columns
    state.hiddenColumns.forEach((colName) => {
      const idx = state.columns.indexOf(colName);
      if (idx !== -1) {
        rules.push(`#tableOriginal .col-idx-${idx}, #tableModified .col-idx-${idx} { display: none !important; }`);
      }
    });

    // Custom resized column widths
    Object.keys(state.columnWidths).forEach((key) => {
      const idx = parseInt(key, 10);
      const width = state.columnWidths[key];
      if (!isNaN(idx) && width > 0) {
        rules.push(`#tableOriginal .col-idx-${idx}, #tableModified .col-idx-${idx} { width: ${width}px !important; min-width: ${width}px !important; max-width: ${width}px !important; }`);
      }
    });

    el.dynamicTableStyles.textContent = rules.join('\n');
  }

  // Column Resizing via Drag Handles
  function initResizeDrag(e, colIdx) {
    e.preventDefault();
    e.stopPropagation();

    const handle = e.target;
    if (handle.classList && handle.classList.add) {
      handle.classList.add('resizing');
    }
    if (document.body && document.body.classList) {
      document.body.classList.add('resizing-columns');
    }

    const th = handle.closest ? handle.closest('th') : null;
    const startX = e.clientX;
    const startWidth = (th && th.getBoundingClientRect) ? th.getBoundingClientRect().width : (state.columnWidths[colIdx] || 180);

    function onMouseMove(ev) {
      const deltaX = ev.clientX - startX;
      const newWidth = Math.max(60, Math.min(800, Math.round(startWidth + deltaX)));
      state.columnWidths[colIdx] = newWidth;
      updateDynamicTableStyles();
    }

    function onMouseUp() {
      if (handle.classList && handle.classList.remove) {
        handle.classList.remove('resizing');
      }
      if (document.body && document.body.classList) {
        document.body.classList.remove('resizing-columns');
      }
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
      syncRowHeights(state.displayedRows, false);
    }

    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
  }

  function handleResizeDblClick(e, colIdx) {
    e.preventDefault();
    e.stopPropagation();
    delete state.columnWidths[colIdx];
    updateDynamicTableStyles();
    syncRowHeights(state.displayedRows, false);
  }

  // Render Table Headers
  function renderTableHeaders(columns) {
    el.theadOriginal.replaceChildren();
    el.theadModified.replaceChildren();

    const trO = document.createElement('tr');
    const thRowO = document.createElement('th');
    thRowO.className = 'row-num-cell';
    thRowO.textContent = '#';
    trO.appendChild(thRowO);

    const trM = document.createElement('tr');
    const thRowM = document.createElement('th');
    thRowM.className = 'row-num-cell';
    thRowM.textContent = '#';
    trM.appendChild(thRowM);

    columns.forEach((col, idx) => {
      // Original Table Header
      const th1 = document.createElement('th');
      th1.className = `col-idx-${idx}`;
      th1.dataset.colIdx = idx;
      th1.dataset.colName = col;
      th1.title = `${col} (Drag edge to resize, double-click to reset)`;

      const span1 = document.createElement('span');
      span1.className = 'th-content';
      span1.textContent = col;
      th1.appendChild(span1);

      const resize1 = document.createElement('div');
      resize1.className = 'col-resize-handle';
      resize1.dataset.colIdx = idx;
      resize1.title = 'Drag border to resize, double-click to reset';
      resize1.addEventListener('mousedown', (e) => initResizeDrag(e, idx));
      resize1.addEventListener('dblclick', (e) => handleResizeDblClick(e, idx));
      th1.appendChild(resize1);

      trO.appendChild(th1);

      // Modified Table Header
      const th2 = document.createElement('th');
      th2.className = `col-idx-${idx}`;
      th2.dataset.colIdx = idx;
      th2.dataset.colName = col;
      th2.title = `${col} (Drag edge to resize, double-click to reset)`;

      const span2 = document.createElement('span');
      span2.className = 'th-content';
      span2.textContent = col;
      th2.appendChild(span2);

      const resize2 = document.createElement('div');
      resize2.className = 'col-resize-handle';
      resize2.dataset.colIdx = idx;
      resize2.title = 'Drag border to resize, double-click to reset';
      resize2.addEventListener('mousedown', (e) => initResizeDrag(e, idx));
      resize2.addEventListener('dblclick', (e) => handleResizeDblClick(e, idx));
      th2.appendChild(resize2);

      trM.appendChild(th2);
    });

    el.theadOriginal.appendChild(trO);
    el.theadModified.appendChild(trM);
  }

  // Render Table Rows (Using DocumentFragment for 60fps performance)
  function renderTableRows(rows, append = false) {
    if (!append) {
      el.tbodyOriginal.replaceChildren();
      el.tbodyModified.replaceChildren();
    }

    const fragOrig = document.createDocumentFragment();
    const fragMod = document.createDocumentFragment();
    const visibleCount = Math.max(1, state.columns.length - state.hiddenColumns.size);

    rows.forEach((rowDiff, idx) => {
      const rowIndex = append ? state.displayedRows.length - rows.length + idx : idx;

      const trOrig = document.createElement('tr');
      trOrig.id = `row-orig-${rowIndex}`;
      trOrig.className = `row-${rowDiff.status}`;
      trOrig.dataset.diffIndex = rowIndex;

      const trMod = document.createElement('tr');
      trMod.id = `row-mod-${rowIndex}`;
      trMod.className = `row-${rowDiff.status}`;
      trMod.dataset.diffIndex = rowIndex;

      trOrig.addEventListener('click', () => setActiveDiff(rowIndex));
      trMod.addEventListener('click', () => setActiveDiff(rowIndex));

      // Row Hover Sync
      const handleRowMouseEnter = () => {
        trOrig.classList.add('row-hover-synced');
        trMod.classList.add('row-hover-synced');
      };
      const handleRowMouseLeave = () => {
        trOrig.classList.remove('row-hover-synced');
        trMod.classList.remove('row-hover-synced');
      };
      trOrig.addEventListener('mouseenter', handleRowMouseEnter);
      trOrig.addEventListener('mouseleave', handleRowMouseLeave);
      trMod.addEventListener('mouseenter', handleRowMouseEnter);
      trMod.addEventListener('mouseleave', handleRowMouseLeave);

      // Row numbers
      const tdNumOrig = document.createElement('td');
      tdNumOrig.className = 'row-num-cell';
      tdNumOrig.textContent = rowDiff.original_row_num > 0 ? rowDiff.original_row_num : '-';
      trOrig.appendChild(tdNumOrig);

      const tdNumMod = document.createElement('td');
      tdNumMod.className = 'row-num-cell';
      tdNumMod.textContent = rowDiff.modified_row_num > 0 ? rowDiff.modified_row_num : '-';
      trMod.appendChild(tdNumMod);

      // Render cells
      if (rowDiff.status === 'added') {
        const tdPlaceholder = document.createElement('td');
        tdPlaceholder.colSpan = visibleCount;
        tdPlaceholder.className = 'cell-empty-placeholder';
        tdPlaceholder.textContent = '[Record added in modified version]';
        trOrig.appendChild(tdPlaceholder);

        rowDiff.cells.forEach((cell, cellIdx) => {
          const td = document.createElement('td');
          td.className = `col-idx-${cellIdx}`;
          td.dataset.colIdx = cellIdx;
          td.dataset.colName = cell.col_name;
          if (cellIdx === 0) {
            const pill = document.createElement('span');
            pill.className = 'cell-pill pill-add';
            pill.textContent = '+ Added';
            td.appendChild(pill);
          }
          renderCellValue(td, cell.modified_value);
          trMod.appendChild(td);
        });
      } else if (rowDiff.status === 'removed') {
        rowDiff.cells.forEach((cell, cellIdx) => {
          const td = document.createElement('td');
          td.className = `col-idx-${cellIdx}`;
          td.dataset.colIdx = cellIdx;
          td.dataset.colName = cell.col_name;
          if (cellIdx === 0) {
            const pill = document.createElement('span');
            pill.className = 'cell-pill pill-rem';
            pill.textContent = '- Removed';
            td.appendChild(pill);
          }
          renderCellValue(td, cell.original_value);
          trOrig.appendChild(td);
        });

        const tdPlaceholder = document.createElement('td');
        tdPlaceholder.colSpan = visibleCount;
        tdPlaceholder.className = 'cell-empty-placeholder';
        tdPlaceholder.textContent = '[Record removed from original version]';
        trMod.appendChild(tdPlaceholder);
      } else {
        rowDiff.cells.forEach((cell, cellIdx) => {
          const tdOrig = document.createElement('td');
          const tdMod = document.createElement('td');
          tdOrig.className = `col-idx-${cellIdx}`;
          tdOrig.dataset.colIdx = cellIdx;
          tdOrig.dataset.colName = cell.col_name;

          tdMod.className = `col-idx-${cellIdx}`;
          tdMod.dataset.colIdx = cellIdx;
          tdMod.dataset.colName = cell.col_name;

          if (cell.status === 'modified') {
            tdOrig.classList.add('cell-mod-diff');
            tdMod.classList.add('cell-mod-diff');

            if (cellIdx === 0) {
              const pillO = document.createElement('span');
              pillO.className = 'cell-pill pill-mod';
              pillO.textContent = 'Mod';
              tdOrig.appendChild(pillO);

              const pillM = document.createElement('span');
              pillM.className = 'cell-pill pill-mod';
              pillM.textContent = 'Mod';
              tdMod.appendChild(pillM);
            }

            renderCellChunks(tdOrig, cell.chunks_original, cell.original_value);
            renderCellChunks(tdMod, cell.chunks_modified, cell.modified_value);
          } else {
            renderCellValue(tdOrig, cell.original_value);
            renderCellValue(tdMod, cell.modified_value);
          }

          trOrig.appendChild(tdOrig);
          trMod.appendChild(tdMod);
        });
      }

      fragOrig.appendChild(trOrig);
      fragMod.appendChild(trMod);
    });

    el.tbodyOriginal.appendChild(fragOrig);
    el.tbodyModified.appendChild(fragMod);

    // Synchronize row heights across both sides to ensure exact side-by-side alignment
    syncRowHeights(rows, append);

    updateNavPosition();
  }

  // Unified row height alignment
  function syncRowHeights(rows, append) {
    requestAnimationFrame(() => {
      const startIndex = append ? state.displayedRows.length - rows.length : 0;
      const count = rows.length;

      for (let i = 0; i < count; i++) {
        const idx = startIndex + i;
        const rOrig = document.getElementById(`row-orig-${idx}`);
        const rMod = document.getElementById(`row-mod-${idx}`);
        if (!rOrig || !rMod) continue;

        // Reset any inline height to measure natural client height
        rOrig.style.height = '';
        rMod.style.height = '';

        const hOrig = rOrig.getBoundingClientRect().height;
        const hMod = rMod.getBoundingClientRect().height;
        const maxHeight = Math.max(hOrig, hMod);

        if (maxHeight > 0) {
          rOrig.style.height = `${maxHeight}px`;
          rMod.style.height = `${maxHeight}px`;
        }
      }
    });
  }

  // Safe Cell Rendering
  function renderCellValue(td, valPtr) {
    if (valPtr === null || valPtr === undefined) {
      const span = document.createElement('span');
      span.className = 'cell-empty-placeholder';
      span.textContent = '[missing]';
      td.appendChild(span);
      return;
    }

    if (valPtr === '') {
      const span = document.createElement('span');
      span.className = 'cell-empty-placeholder';
      span.textContent = '""';
      td.appendChild(span);
      return;
    }

    td.appendChild(document.createTextNode(valPtr));
  }

  // Fine-Grained Character Diff Rendering
  function renderCellChunks(td, chunks, fallbackVal) {
    if (!chunks || chunks.length === 0) {
      renderCellValue(td, fallbackVal);
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

  // Batch-by-Batch UI Updates
  function updateBatchUI(total, hasMore) {
    state.totalDiffs = total;
    const displayed = state.displayedRows.length;
    el.displayedCount.textContent = displayed.toLocaleString();
    el.totalDiffCount.textContent = total.toLocaleString();

    const currentBatchNum = Math.ceil(displayed / BATCH_SIZE) || 1;
    const totalBatches = Math.ceil(total / BATCH_SIZE) || 1;

    if (displayed >= total || !hasMore) {
      el.batchProgressLabel.textContent = `All ${total.toLocaleString()} rows loaded (${totalBatches} batches)`;
      el.showMoreBtn.style.display = 'none';
      el.loadAllBtn.style.display = 'none';
    } else {
      el.batchProgressLabel.textContent = `Batch ${currentBatchNum} of ${totalBatches}`;
      el.showMoreBtn.style.display = 'inline-flex';
      el.loadAllBtn.style.display = 'inline-flex';
    }
  }

  // Load a single 100-row batch
  async function loadNextBatch(showOverlay = true) {
    if (!state.sessionId || state.isLoading) return false;

    state.isLoading = true;
    if (showOverlay) {
      showLoading(`Loading batch (${state.displayedRows.length + 1} - ${state.displayedRows.length + BATCH_SIZE})...`);
    }

    const offset = state.displayedRows.length;
    const url = `/api/results?session_id=${encodeURIComponent(state.sessionId)}&offset=${offset}&limit=${BATCH_SIZE}&filter=${encodeURIComponent(state.currentFilter)}&search=${encodeURIComponent(state.searchQuery)}`;

    try {
      const response = await fetch(url);
      const data = await response.json();
      state.isLoading = false;
      if (showOverlay) hideLoading();

      if (!response.ok) {
        console.error('Failed to load batch:', data.error);
        return false;
      }

      const newRows = data.rows || [];
      if (newRows.length > 0) {
        state.displayedRows = state.displayedRows.concat(newRows);
        renderTableRows(newRows, true);
      }
      updateBatchUI(data.total_filtered, data.has_more);
      return data.has_more && newRows.length > 0;
    } catch (err) {
      state.isLoading = false;
      if (showOverlay) hideLoading();
      console.error('Batch load error:', err);
      return false;
    }
  }

  // Progressive Batch-by-Batch Loader for all remaining rows
  async function handleLoadAllBatches() {
    if (!state.sessionId || state.isBatchLoadingAll) return;

    state.isBatchLoadingAll = true;
    state.cancelBatchLoad = false;

    showLoading('Loading batches progressively...');
    el.loadingBatchSub.style.display = 'block';

    const totalBatches = Math.ceil(state.totalDiffs / BATCH_SIZE) || 1;

    while (state.displayedRows.length < state.totalDiffs && !state.cancelBatchLoad) {
      const currentBatch = Math.floor(state.displayedRows.length / BATCH_SIZE) + 1;
      el.loadingBatchSub.textContent = `Loaded ${state.displayedRows.length.toLocaleString()} of ${state.totalDiffs.toLocaleString()} rows (Batch ${currentBatch} of ${totalBatches})...`;

      const hasMore = await loadNextBatch(false);
      if (!hasMore) break;

      // Yield briefly to browser event loop to keep UI completely responsive
      await new Promise((resolve) => setTimeout(resolve, 25));
    }

    state.isBatchLoadingAll = false;
    hideLoading();
    el.loadingBatchSub.style.display = 'none';
  }

  // Filter & Search Handling
  async function refreshFilteredResults() {
    if (!state.sessionId) return;

    showLoading('Filtering rows...');
    state.displayedRows = [];

    const url = `/api/results?session_id=${encodeURIComponent(state.sessionId)}&offset=0&limit=${BATCH_SIZE}&filter=${encodeURIComponent(state.currentFilter)}&search=${encodeURIComponent(state.searchQuery)}`;

    try {
      const response = await fetch(url);
      const data = await response.json();
      hideLoading();

      if (!response.ok) {
        alert(data.error || 'Failed to query results');
        return;
      }

      state.displayedRows = data.rows || [];
      state.activeDiffIndex = -1;

      renderTableRows(state.displayedRows, false);
      updateBatchUI(data.total_filtered, data.has_more);
    } catch (err) {
      hideLoading();
      alert('Network error: ' + err.message);
    }
  }

  // Difference Navigation
  function setActiveDiff(idx) {
    if (idx < 0 || idx >= state.displayedRows.length) return;

    if (state.activeDiffIndex >= 0) {
      const prevOrig = document.getElementById(`row-orig-${state.activeDiffIndex}`);
      const prevMod = document.getElementById(`row-mod-${state.activeDiffIndex}`);
      if (prevOrig) prevOrig.classList.remove('row-active-highlight');
      if (prevMod) prevMod.classList.remove('row-active-highlight');
    }

    state.activeDiffIndex = idx;
    const curOrig = document.getElementById(`row-orig-${idx}`);
    const curMod = document.getElementById(`row-mod-${idx}`);
    if (curOrig) curOrig.classList.add('row-active-highlight');
    if (curMod) curMod.classList.add('row-active-highlight');

    if (curOrig) {
      curOrig.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }
    if (curMod) {
      curMod.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }

    updateNavPosition();
  }

  function updateNavPosition() {
    const total = state.displayedRows.length;
    if (total === 0) {
      el.diffPositionIndicator.textContent = '- / -';
      el.prevDiffBtn.disabled = true;
      el.nextDiffBtn.disabled = true;
      return;
    }

    const current = state.activeDiffIndex >= 0 ? state.activeDiffIndex + 1 : '-';
    el.diffPositionIndicator.textContent = `${current} of ${total}`;
    el.prevDiffBtn.disabled = (state.activeDiffIndex <= 0);
    el.nextDiffBtn.disabled = (state.activeDiffIndex >= total - 1);
  }

  // Setup Results & Controls Listeners
  function setupResultsListeners() {
    // Filter Tabs
    el.filterTabs.forEach((tab) => {
      tab.addEventListener('click', () => {
        el.filterTabs.forEach((t) => t.classList.remove('active'));
        tab.classList.add('active');
        state.currentFilter = tab.dataset.filter;
        refreshFilteredResults();
      });
    });

    // Search Input
    const debouncedSearch = debounce(() => {
      state.searchQuery = el.searchDiffsInput.value.trim();
      el.clearSearchBtn.style.display = state.searchQuery ? 'block' : 'none';
      refreshFilteredResults();
    }, 280);

    el.searchDiffsInput.addEventListener('input', debouncedSearch);
    el.clearSearchBtn.addEventListener('click', () => {
      el.searchDiffsInput.value = '';
      el.clearSearchBtn.style.display = 'none';
      state.searchQuery = '';
      refreshFilteredResults();
    });

    // Navigation Buttons
    el.prevDiffBtn.addEventListener('click', () => {
      if (state.activeDiffIndex > 0) setActiveDiff(state.activeDiffIndex - 1);
    });

    el.nextDiffBtn.addEventListener('click', () => {
      if (state.activeDiffIndex < state.displayedRows.length - 1) setActiveDiff(state.activeDiffIndex + 1);
    });

    // Auto-load checkbox
    el.autoLoadScrollCheckbox.addEventListener('change', (e) => {
      state.autoLoadOnScroll = e.target.checked;
    });

    // Keyboard Shortcuts (J / K or Alt+Down / Alt+Up)
    window.addEventListener('keydown', (e) => {
      if (el.resultsSection.style.display !== 'block') return;
      if (document.activeElement && (document.activeElement.tagName === 'INPUT' || document.activeElement.tagName === 'SELECT')) {
        return;
      }

      if (e.key === 'j' || e.key === 'J' || (e.altKey && e.key === 'ArrowDown')) {
        e.preventDefault();
        if (state.activeDiffIndex < state.displayedRows.length - 1) {
          setActiveDiff(state.activeDiffIndex + 1);
        } else if (state.displayedRows.length < state.totalDiffs) {
          // If at end of loaded batch, fetch next batch and navigate!
          loadNextBatch(false).then(() => {
            if (state.activeDiffIndex < state.displayedRows.length - 1) {
              setActiveDiff(state.activeDiffIndex + 1);
            }
          });
        }
      } else if (e.key === 'k' || e.key === 'K' || (e.altKey && e.key === 'ArrowUp')) {
        e.preventDefault();
        if (state.activeDiffIndex > 0) {
          setActiveDiff(state.activeDiffIndex - 1);
        }
      }
    });

    // Batch Load Buttons
    el.showMoreBtn.addEventListener('click', () => loadNextBatch(true));
    el.loadAllBtn.addEventListener('click', handleLoadAllBatches);

    // Columns Visibility Popover
    if (el.columnsVisibilityBtn) {
      el.columnsVisibilityBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        const isOpen = el.columnsVisibilityMenu.style.display === 'flex';
        el.columnsVisibilityMenu.style.display = isOpen ? 'none' : 'flex';
        el.columnsDropdownWrapper.classList.toggle('open', !isOpen);
        el.columnsVisibilityBtn.setAttribute('aria-expanded', !isOpen ? 'true' : 'false');
      });
    }

    if (el.resetColumnsVisibilityBtn) {
      el.resetColumnsVisibilityBtn.addEventListener('click', () => {
        state.hiddenColumns.clear();
        if (el.columnsVisibilityList) {
          el.columnsVisibilityList.querySelectorAll('input[type="checkbox"]').forEach((cb) => {
            cb.checked = true;
          });
        }
        updateDynamicTableStyles();
        updatePlaceholderColspans();
        updateColumnsBtnBadge();
        syncRowHeights(state.displayedRows, false);
      });
    }

    if (el.columnsVisibilitySearch) {
      el.columnsVisibilitySearch.addEventListener('input', (e) => {
        const q = e.target.value.toLowerCase().trim();
        const items = el.columnsVisibilityList.querySelectorAll('.dropdown-col-item');
        items.forEach((item) => {
          item.style.display = item.dataset.colName.includes(q) ? 'flex' : 'none';
        });
      });
    }

    // Close columns dropdown when clicking outside
    document.addEventListener('click', (e) => {
      if (el.columnsDropdownWrapper && !el.columnsDropdownWrapper.contains(e.target)) {
        if (el.columnsVisibilityMenu) el.columnsVisibilityMenu.style.display = 'none';
        el.columnsDropdownWrapper.classList.remove('open');
        if (el.columnsVisibilityBtn) el.columnsVisibilityBtn.setAttribute('aria-expanded', 'false');
      }
    });

    // New Comparison
    el.newComparisonBtn.addEventListener('click', resetToUpload);
    if (el.brandHomeBtn) {
      el.brandHomeBtn.addEventListener('click', resetToUpload);
      el.brandHomeBtn.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          resetToUpload();
        }
      });
    }
  }

  function resetToUpload() {
    state.sessionId = null;
    state.origFile = null;
    state.modFile = null;
    state.meta = null;
    state.summary = null;
    state.displayedRows = [];
    state.activeDiffIndex = -1;
    state.hiddenColumns.clear();
    state.columnWidths = {};
    if (el.dynamicTableStyles) el.dynamicTableStyles.textContent = '';
    if (el.columnsHiddenBadge) el.columnsHiddenBadge.style.display = 'none';

    el.originalDropzone.classList.remove('has-file');
    el.originalFileInfo.style.display = 'none';
    el.originalFileInput.value = '';

    el.modifiedDropzone.classList.remove('has-file');
    el.modifiedFileInfo.style.display = 'none';
    el.modifiedFileInput.value = '';

    el.inspectFilesBtn.disabled = true;

    el.uploadSection.style.display = 'block';
    el.configSection.style.display = 'none';
    el.resultsSection.style.display = 'none';
    el.newComparisonBtn.style.display = 'none';

    hideAlert(el.uploadError, el.uploadErrorMsg);
    hideAlert(el.configError, el.configErrorMsg);
  }

  // Alert & Loading Helpers
  function showAlert(container, msgEl, msg) {
    msgEl.textContent = msg;
    container.style.display = 'flex';
  }

  function hideAlert(container, msgEl) {
    if (msgEl) msgEl.textContent = '';
    container.style.display = 'none';
  }

  function showLoading(msg) {
    state.isLoading = true;
    el.loadingText.textContent = msg || 'Loading...';
    el.loadingOverlay.style.display = 'flex';
  }

  function hideLoading() {
    state.isLoading = false;
    el.loadingOverlay.style.display = 'none';
  }

  // App Initialization
  function init() {
    applyTheme(state.theme);
    el.themeToggleBtn.addEventListener('click', toggleTheme);

    setupFilePickers();
    setupConfigListeners();
    setupResultsListeners();
    setupSynchronizedScrolling();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
