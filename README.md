# CSV Diff - Production-Ready CSV Comparison Application

A fast, production-ready, cross-platform CSV comparison application built with **Go** for the comparison engine and backend HTTP server, and **modern vanilla HTML, CSS, and JavaScript** for the responsive user interface.

The application runs entirely locally on your workstation, operates offline without external network dependencies, parses CSVs strictly according to RFC 4180 rules, handles all Unicode scripts and complex emoji sequences accurately, and embeds all static assets into a single standalone executable.

---

## Table of Contents

1. [Application Overview](#application-overview)
2. [Features](#features)
3. [Project Structure](#project-structure)
4. [Development Requirements](#development-requirements)
5. [Running the Application](#running-the-application)
6. [Running Tests](#running-tests)
7. [Cross-Platform Building](#cross-platform-building)
8. [Comparison Semantics](#comparison-semantics)
   - [Position-Based Comparison](#position-based-comparison)
   - [Primary-Key Comparison](#primary-key-comparison)
   - [Include and Exclude Column Behavior](#include-and-exclude-column-behavior)
   - [Difference Classification](#difference-classification)
   - [The 100-Differing-Row Display Limit & Pagination](#the-100-differing-row-display-limit--pagination)
9. [Unicode & Text Comparison Details](#unicode--text-comparison-details)
10. [Performance & Architecture](#performance--architecture)
11. [Security & Local Data Handling](#security--local-data-handling)
12. [Known Limitations](#known-limitations)

---

## Application Overview

Comparing CSV files reliably is notoriously error-prone when naive line diffing or basic string comparison is used. Real-world CSVs often feature:
- Multiline quoted fields containing newline characters (`\n` or `\r\n`).
- Quoted commas and escaped quotes (`""`).
- Southeast Asian scripts such as Burmese (မြန်မာစာ) with complex consonant stacks and combining marks.
- Zero-Width Joiner (ZWJ) family emojis and skin-tone modifiers.
- Differing record orders or column header arrangements.
- Large row counts where rendering every row into the DOM would freeze the browser.

`CSV Diff` addresses these challenges with a dedicated Go comparison engine utilizing Myers diff on grapheme clusters, synchronized side-by-side tables, row-level caching, and strict XSS sanitization.

---

## Features

- **Zero-Dependency Single Binary**: HTML, CSS, and JavaScript are embedded directly into the Go executable via `embed.FS`. No external web server, Node.js, or runtime installation is required to run the released binary.
- **Two Comparison Modes**:
  - **Positional Comparison (Default)**: Compares records by their row index in the CSV file. Ideal when row order is meaningful.
  - **Primary-Key Comparison**: Matches records by a unique identifier column, accommodating row reordering, differing column arrangements, and missing rows.
- **Adjustable & Hidable Table Columns**:
  - Resizable column widths: Drag any column header border in either table to dynamically resize both side-by-side tables synchronously in real time; double-click the handle to reset to default.
  - Column visibility toggle: Use the **Columns** dropdown popover in the toolbar with search filtering to show or hide specific columns dynamically without re-running the comparison.
- **Strict Header Validation**:
  - Both CSV files are validated to ensure they have equal header column names. Mismatches in column count or names immediately display clear, actionable errors on upload.
- **Selective Column Inclusion & Exclusion**:
  - `Include Selected`: Compares only explicitly selected columns.
  - `Exclude Selected`: Compares all columns except those excluded.
  - In primary-key mode, the primary key column continues to identify and match records even if excluded from value comparison.
- **Duplicate & Empty Primary Key Validation**:
  - Identifies duplicate or empty key values in either file and reports exact row numbers without silently discarding data.
- **Fine-Grained Character-Level Highlighting**:
  - Changes within modified cells are highlighted down to individual inserted and deleted characters using Myers diff on grapheme clusters.
  - Never splits UTF-8 multibyte characters or separates combining accents/viramas.
- **Initial 100-Differing-Row Limit & Seamless Pagination**:
  - The initial view displays up to the first 100 rows with actual differences. Unchanged rows are hidden from the diff viewport.
  - Offers a **Show Next 100 Rows** button and a **Load All Remaining Rows** button.
  - Results are cached in the Go session manager, avoiding expensive re-diffing on pagination.
- **Independent & Optional Synchronized Scrolling**:
  - The Original and Modified tables feature completely independent horizontal and vertical scrollbars, preventing layout distortions when one file contains multiline records or differing line lengths.
  - A **Sync Scroll** toggle in the toolbar allows users to re-enable lockstep scrolling whenever desired.
  - Jumping between differences (via Prev/Next buttons or keyboard shortcuts `J` / `K`) automatically scrolls both panes to bring the corresponding difference into view.
- **Difference Navigation & Search**:
  - Keyboard shortcuts (`J` / `K` or `Alt+Down` / `Alt+Up`) to jump through differing rows.
  - Substring search across all cell values and primary keys.
  - Filter by difference type: *All*, *Modified Only*, *Added Only*, *Removed Only*.
- **Theming**:
  - Modern light theme by default, with dark theme toggle stored in `localStorage`.
- **Port Conflict Handling & Auto-Launch**:
  - Binds to `127.0.0.1:8080` by default; automatically probes subsequent ports if occupied.
  - Automatically launches the default web browser on startup (can be disabled via `-no-browser`).

---

## Project Structure

```
csv-diff/
├── cmd/
│   └── csv-diff/
│       └── main.go              # CLI flags, server bootstrap, signal handling, browser launcher
├── internal/
│   ├── browser/
│   │   └── browser.go           # Cross-platform browser launcher (Windows, macOS, Linux)
│   ├── csvparser/
│   │   ├── parser.go            # RFC 4180 CSV parser, BOM stripper, line tracking
│   │   └── parser_test.go       # Comprehensive parser unit tests
│   ├── diff/
│   │   ├── engine.go            # Positional & primary-key comparison engine
│   │   ├── engine_test.go       # Diff engine tests (edge cases, Unicode, large inputs)
│   │   ├── grapheme.go          # UAX #29 grapheme cluster tokenizer (Burmese, emoji, ZWJ)
│   │   ├── textdiff.go          # Myers diff algorithm for character/token edits
│   │   └── textdiff_test.go     # Unit tests for character-level diffing
│   ├── model/
│   │   └── model.go             # Domain models (diff chunks, cells, rows, summaries, sessions)
│   ├── server/
│   │   ├── server.go            # HTTP server, port probe, route mux, graceful shutdown
│   │   ├── handlers.go          # REST API endpoints (/upload, /compare, /results, /cancel)
│   │   └── server_test.go       # Integration tests with net/http/httptest
│   └── session/
│       ├── session.go           # Thread-safe in-memory session manager, query & cache
│       └── session_test.go      # Session pagination and expiration tests
├── scripts/
│   ├── build.ps1                # Cross-compilation script for Windows PowerShell
│   └── build.sh                 # Cross-compilation script for Bash / Linux / macOS
├── testdata/                    # Sample CSVs (simple, Unicode/Burmese, PK reordered, large)
├── tests/
│   └── frontend/
│       └── frontend_test.js     # Automated headless tests for UI logic & XSS prevention
├── web/
│   ├── embed.go                 # go:embed wrapper for static assets
│   └── static/
│       ├── index.html           # Main UI template
│       ├── css/
│       │   └── style.css        # Responsive stylesheet with light/dark variables
│       └── js/
│           └── app.js           # Vanilla JavaScript frontend with safe DOM rendering
├── go.mod                       # Go module definition
└── README.md                    # Comprehensive documentation
```

---

## Development Requirements

- **Go**: Version 1.22 or higher (tested with Go 1.27).
- **Node.js** *(Optional)*: Version 18+ only if executing the automated headless frontend test suite (`node --test`).
- **Operating Systems**: Windows 10/11, macOS (Intel & Apple Silicon), or Linux (Ubuntu, Debian, Fedora, Arch, etc.).

---

## Running the Application

### Running directly with Go
```bash
go run ./cmd/csv-diff
```

### Global Installation (Run `csv-diff` from anywhere)

#### On Windows (PowerShell)
You can run the automated installer:
```powershell
powershell -File scripts/install.ps1
```
Or use the standard Go installer:
```powershell
go install ./cmd/csv-diff
```
This places `csv-diff.exe` into your `%USERPROFILE%\go\bin` directory (and adds it to your user `PATH`). You can then launch `csv-diff` directly from any command prompt or PowerShell window:
```powershell
csv-diff
```

#### On macOS / Linux
Run the installer script:
```bash
chmod +x scripts/install.sh
./scripts/install.sh
```
Or via Go:
```bash
go install ./cmd/csv-diff
```

### Command-Line Options
```bash
./csv-diff [options]

Options:
  -host string
        Host address to bind HTTP server (default "127.0.0.1")
  -port int
        Port to listen on (default 8080; auto-increments if in use)
  -max-upload-mb int
        Maximum CSV upload size in megabytes (default 50)
  -no-browser
        Disable automatically opening the web browser on launch
```

**Example:**
```bash
./csv-diff -port 9000 -max-upload-mb 100 -no-browser
```

Once running, navigate to `http://127.0.0.1:8080` (or the port displayed in the console).

---

## Running Tests

### Complete Go Test Suite
Run all unit and integration tests across all packages:
```bash
go test -v ./...
```

### Race Detection
Run all Go tests with the race detector enabled to verify concurrency safety:
```bash
go test -race ./...
```

### Static Analysis & Formatting
```bash
go vet ./...
go fmt ./...
```

### Headless Frontend & XSS Tests
Run the automated frontend test suite using Node.js:
```bash
node --test tests/frontend/frontend_test.js
```

---

## Cross-Platform Building

All targets compile to standalone binaries with zero CGO dependencies (`CGO_ENABLED=0`).

### Build for all platforms using PowerShell (Windows)
```powershell
powershell -File scripts/build.ps1
```

### Build for all platforms using Bash (macOS / Linux)
```bash
chmod +x scripts/build.sh
./scripts/build.sh
```

### Individual Target Compilation Commands

#### Windows (64-bit AMD64)
```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o bin/csv-diff-windows-amd64.exe ./cmd/csv-diff
```

#### macOS Apple Silicon (ARM64)
```bash
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o bin/csv-diff-darwin-arm64 ./cmd/csv-diff
```

#### macOS Intel (AMD64)
```bash
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o bin/csv-diff-darwin-amd64 ./cmd/csv-diff
```

#### Linux (64-bit AMD64)
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/csv-diff-linux-amd64 ./cmd/csv-diff
```

#### Linux (64-bit ARM64)
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/csv-diff-linux-arm64 ./cmd/csv-diff
```

*Note: Binaries built on non-Apple hosts will require local code-signing or `xattr -d com.apple.quarantine <binary>` on macOS if Gatekeeper flags unsigned binaries.*

---

## Comparison Semantics

### Position-Based Comparison
- Compares record 1 with record 1, record 2 with record 2, and so forth.
- Suitable when physical row sequence is meaningful and files are structurally synchronized.
- If the modified file has more rows than the original, the extra trailing rows are marked as **Added**.
- If the original file has more rows than the modified, the extra trailing rows are marked as **Removed**.
- *Caveat*: If a row is inserted in the middle of a file, subsequent positional rows will be shifted, resulting in widespread modification reports. For reordered or inserted datasets, use **Primary-Key Comparison**.

### Primary-Key Comparison
- The user selects a column header to serve as the unique record identifier.
- Records are indexed by this key value. Rows with matching keys are paired regardless of their physical line position.
- Unmatched original records are classified as **Removed**.
- Unmatched modified records are classified as **Added**.
- Matched records with differing values in eligible columns are classified as **Modified**.
- Columns are mapped between files by header name, allowing the two CSV files to have columns in different orders.
- **Ordering**: Results are presented in deterministic order: original rows in their original file sequence, followed by newly added modified rows in their modified file sequence.
- **Duplicate & Empty Key Handling**: If either file contains duplicate key values or empty keys, comparison halts with a descriptive error listing the duplicate values and row numbers.

### Include and Exclude Column Behavior
- **Compare All Columns (Default)**: Evaluates all eligible columns for changes.
- **Include Selected**: Compares *only* the user-checked columns. Any difference in an unselected column is ignored.
- **Exclude Selected**: Compares all columns *except* the user-checked columns.
- **Key Column Preservation**: In primary-key mode, excluding the key column from value comparison removes it from cell difference counting, but it remains fully active for record pairing.
- **Row Classification Impact**: If a row differs *only* in an excluded column, that row is classified as **Unchanged** and will not appear in the differing rows view.

### Difference Classification
| Status | Meaning | Original Side View | Modified Side View |
|---|---|---|---|
| `modified` | Paired row with value changes in eligible columns | Original values with deleted text highlighted | Modified values with inserted text highlighted |
| `added` | Record exists only in Modified CSV | `[Added row: record absent in original]` | Full row values with `+ Added` badge |
| `removed` | Record exists only in Original CSV | Full row values with `- Removed` badge | `[Removed row: record absent in modified]` |

- Changed cells within a modified row are counted individually for cell statistics, while the row itself counts as exactly **one differing row**.
- Unchanged rows are filtered out of the results view.

### The 100-Differing-Row Display Limit & Pagination
- The application initially delivers and displays up to **100 differing rows**, not the first 100 physical lines of the CSV.
- Rows are retrieved from the Go in-memory session cache without re-running the comparison engine.
- Users can click **Show Next 100 Rows** to load the next 100 rows, or **Load All Remaining Rows** to display the entire diff table.
- Difference counters display: `Showing X of Y differing rows (Z remaining)`.

---

## Unicode & Text Comparison Details

1. **Exact Cell Value Decoding**:
   - Comparison is case-sensitive by default.
   - Whitespace (including leading and trailing spaces or tabs) is preserved exactly and not trimmed.
   - Visual code-point differences (such as NFC `é` `\u00e9` vs NFD `e + combining acute` `\u0065\u0301`) are detected as differences.
2. **Empty vs Missing Cells**:
   - An **empty cell** (`""`) is a field explicitly present in the record with zero characters.
   - A **missing cell** (`nil`) occurs when a CSV row has fewer delimiters than the table schema.
   - An empty cell compared against a missing cell is flagged as a difference and rendered with an italic `[missing]` placeholder.
3. **Line Endings & Multiline Fields**:
   - File record delimiters (CRLF `\r\n` vs LF `\n`) are normalized during parsing so line-ending conventions between Windows and Unix do not trigger false diffs.
   - Quoted multiline fields preserve internal newline characters as part of the string value and are compared byte-for-byte.
4. **Grapheme-Cluster-Aware Text Diffing**:
   - When modified cells differ, text is tokenized into user-perceived grapheme clusters according to Unicode standard rules.
   - Burmese stacked consonants (`U+1039` virama), asat marks (`U+103A`), combining vowels, and medial diacritics are grouped with their base consonants.
   - Multi-person emoji sequences with Zero-Width Joiners (`\u200D`) and skin-tone modifiers remain atomic units and are never split into broken characters.
   - If edit distance exceeds 400 operations or token count exceeds 2,000, the system gracefully falls back to whole-cell highlighting.

---

## Performance & Architecture

- **Streaming CSV Parser**: Input files are parsed once upon upload using Go's standard `encoding/csv` reader wrapped with a BOM detector.
- **In-Memory Hash Indexing**: Primary-key matching utilizes Go maps (`map[string]int`), enabling $O(N + M)$ lookup times even for hundreds of thousands of records.
- **Bounded Memory Myers Diff**: The Myers diff algorithm enforces a maximum edit threshold (`maxMyersD = 400`), preventing quadratic runaway times on adversarial inputs.
- **Session Cache**: Comparison results are cached with a configurable TTL (default 2 hours) and automatically pruned by a background cleaner.
- **Cooperative Context Cancellation**: Comparison loops periodically poll `ctx.Done()`, enabling immediate termination when a user requests cancellation or during server shutdown.

---

## Security & Local Data Handling

- **100% Local Execution**: All comparison logic executes on the local machine (`127.0.0.1`). Uploaded CSVs are never transmitted over the internet or to external cloud services.
- **Untrusted Content Sanitization (XSS Defense)**: The frontend strictly avoids `innerHTML` when rendering CSV content. All cell values and header strings are rendered using `document.createTextNode()` and `textContent`. Malicious payloads (e.g., `<script>alert(1)</script>`) are safely rendered as plain text.
- **Upload Size Limits**: Requests are wrapped with `http.MaxBytesReader` (default limit 50 MB, configurable via `-max-upload-mb`) to prevent memory exhaustion attacks.
- **Graceful Shutdown**: The server catches `SIGINT` and `SIGTERM` signals and terminates cleanly within a 10-second grace period.

---

## Known Limitations

1. **Very Large Files (>100MB)**:
   - For CSVs exceeding hundreds of megabytes with hundreds of thousands of differences, loading all rows simultaneously into the browser DOM will consume significant client memory. It is recommended to use the 100-row pagination and difference search to review subsets.
2. **Missing Key Recovery**:
   - In primary-key mode, rows missing the primary-key value cannot be reliably matched and are treated as validation errors. Ensure identifiers are populated prior to comparison.
3. **CSV Dialect Variations**:
   - Semicolon-delimited or tab-delimited files require converting delimiters to commas before comparison, or configuring the CSV parser options. Standard comma delimiters with RFC 4180 quotes are supported out of the box.

---

## License

MIT License. Designed and built for production reliability.
