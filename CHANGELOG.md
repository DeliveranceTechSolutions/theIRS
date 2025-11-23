# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased] - 2025-11-23

### Made All Download Commands Non-Destructive

#### Added
- **File Existence Checks**: All download functions now check if files exist before downloading
- **Size Validation**: Skips files only if size > 0 (prevents keeping corrupt/empty files)
- **Skip Logging**: Clear log messages show what's being skipped vs downloaded
- **Proper Directory Handling**: `ScrapeURLs()` now downloads to `./data/990_zips/` not current directory

#### Fixed
- **`downloadFile()`**: Now skips existing files instead of overwriting
- **`ScrapeURLs()` (zips command)**: Fixed to skip existing files and use correct directory
- **`fetchZip()`**: Now checks for existing files before downloading
- **`fetchSchema()`**: Now skips existing schema files

#### Changed
- **`zips` command behavior**: No longer destructive - updated confirmation message
- All download operations now log file sizes when skipping

**Result**: All commands (`sync`, `zips`, `unzip`, `csv`, `schemas`) can now be safely re-run without data loss!

---

## [Unreleased] - 2025-11-22

### Major Code Quality Overhaul

#### Added
- **HTTP Retry Logic**: All network requests now retry up to 3 times with exponential backoff
- **Request Timeouts**: 30-second timeout on all HTTP requests to prevent hanging
- **Connection Pooling**: HTTP client with 100 max idle connections for better performance
- **Context Support**: All network operations support context-based cancellation
- **Goroutine Rate Limiting**: Semaphore-based limiting prevents resource exhaustion
- **Help Command**: Added `help`, `-h`, and `--help` flags with comprehensive usage information
- **Progress Indicators**: Log every 1,000 files processed with detailed progress information
- **Comprehensive Error Messages**: All errors now provide full context with error wrapping

#### Fixed
- **Critical Resource Leaks**:
  - Fixed defer-in-loop bug in `ScrapeURLs()` that leaked file handles (72+ handles)
  - Fixed unclosed file handle in `parser.go:generateRows()`
  - Moved all defers out of loops in schema processing

- **Race Conditions**:
  - Fixed shared map access in `parser.go` - each goroutine now uses isolated data
  - Added mutex protection for CSV writer operations
  - Fixed global `fileYear` variable synchronization

- **Panic & Fatal Errors**:
  - Eliminated 4 `panic()` calls in `parser.go` (replaced with proper error returns)
  - Removed 3 `log.Fatal()` calls that killed entire program on single file errors
  - All panics in defer blocks removed (extremely dangerous pattern)

- **Error Handling**:
  - Replaced 15+ instances of `fmt.Println(err)` with proper error returns
  - Added nil checks before accessing `http.Response.Body`
  - Fixed ignored return values from `os.Mkdir`, `os.MkdirAll`, `CSV.Write()`, etc.
  - Standardized error handling with `fmt.Errorf` and `%w` wrapping

- **Variable Shadowing**:
  - Fixed critical bug in `parser.go:197` where local variable `f` shadowed zip.File
  - Cleaned up variable naming inconsistencies

- **Code Quality**:
  - Fixed chmod syntax from `chmod x+a` to `chmod +x` in `main.go:111`
  - Removed unnecessary `break` statements in switch cases (5 instances)
  - Changed directory permissions from inconsistent `0777`/`0755` to uniform `0755`
  - Replaced inefficient string concatenation loops with `strings.Builder`
  - Replaced string concatenation paths with `filepath.Join` for cross-platform compatibility

- **Security**:
  - Enhanced zip slip protection with better error messages
  - Fixed shell command construction to prevent injection
  - Added HTTP status code validation before processing responses

#### Changed
- **Function Signatures**: Many functions now return errors instead of panicking
  - `ParseXMLs()` → `ParseXMLs() error`
  - `UnzipXMLs()` → `UnzipXMLs() error`
  - `ScrapeURLs()` → `ScrapeURLs() error`
  - `downloadFile()` now returns errors instead of printing them
  - `fetchSchema()` → `fetchSchema() error`
  - `fetchZip()` → `fetchZip() (string, error)`
  - `confirmation()` → `confirmation() (bool, error)`
  - `SchemaGenerator()` → `SchemaGenerator() error`

- **Error Handling Philosophy**: Individual file failures no longer crash the program
  - Processing continues on errors, with detailed logging
  - Errors are collected and reported at the end
  - User can review which files failed and retry if needed

- **Concurrency Model**:
  - Added semaphore to limit concurrent goroutines (prevents resource exhaustion)
  - Each goroutine now uses isolated `Xmler` struct with own Record map
  - CSV writer access properly synchronized with mutex

- **HTTP Behavior**:
  - Server errors (5xx) trigger retries
  - Client errors (4xx) fail immediately (no retry)
  - Exponential backoff: 2s, 4s, 8s between retries
  - All requests include timeout context

- **Project Structure**:
  - Moved `scan_all_eins.go` to `cmd/scan_eins/main.go` (separate binary)
  - Made EIN parameter a command-line argument instead of hard-coded

#### Removed
- Removed all `panic()` calls from production code paths
- Removed `log.Fatal()` from processing loops
- Removed hard-coded EIN in scan tool
- Removed duplicate error logging patterns

### Performance Improvements
- HTTP connection reuse with connection pooling
- Reduced goroutine spawning through rate limiting
- More efficient string operations using `strings.Builder`
- Better file I/O patterns with proper buffering

### Fault Tolerance
The codebase now gracefully handles:
- Network timeouts and temporary failures (retries automatically)
- Malformed or corrupt XML files (logs and continues)
- Missing directories or files (creates or skips as appropriate)
- Individual download failures (continues with other files)
- HTTP errors (distinguishes between client and server errors)
- Partial data scenarios (processes what's available)

### Testing
- ✅ Builds successfully with `go build`
- ✅ Help command displays correctly
- ✅ All commands parse correctly
- ✅ Error messages are user-friendly and actionable

## Previous Versions

### [Initial Release]
- Basic functionality for downloading and processing IRS Form 990 data
- XML to CSV conversion
- Schema processing support
