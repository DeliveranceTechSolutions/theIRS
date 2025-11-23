# theIRS - IRS Form 990 Data Extraction Tool

A Go-based tool for downloading, extracting, and processing IRS Form 990 XML files into structured CSV format for analysis.

## Overview

The IRS publishes tax-exempt organization data (Form 990 filings) in XML format. This tool automates the process of:
- Downloading Form 990 data packages from the IRS website
- Extracting ZIP archives containing thousands of XML files
- Parsing XML data and converting it to CSV format with structured fields
- Processing large datasets efficiently using concurrent goroutines

## Features

- **Smart Download Management**: Sync command checks existing files and downloads only what's missing
- **Concurrent Processing**: Multi-threaded XML parsing for optimal performance
- **Comprehensive Data Extraction**: Extracts 100+ fields from Form 990 returns including:
  - Organization information (EIN, name, address, contact details)
  - Financial data (revenue, expenses, assets, liabilities)
  - Compensation data (officers, employees, contractors)
  - Program information (grants, foreign activities, unrelated business income)
  - Balance sheet details (beginning/end of year comparisons)
- **Path Traversal Protection**: Security measures against zip slip attacks
- **Progress Tracking**: Real-time feedback on download and processing status

## Installation

### Prerequisites

- Go 1.21 or higher
- Sufficient disk space (the dataset is large - plan for 50GB+)

### Setup

```bash
# Clone the repository
git clone https://github.com/yourusername/theIRS.git
cd theIRS

# Install dependencies
go mod download

# Build the executable
go build -o theIRS
```

## Usage

The tool provides several commands for different stages of the data pipeline:

### 1. Download Data Files (Recommended)

```bash
./theIRS sync
```

**What it does**: Checks what ZIP files are already downloaded and fetches only the missing ones from the IRS website. Safe to run multiple times.

**Output**: ZIP files in `./data/990_zips/`

### 2. Extract ZIP Files

```bash
./theIRS unzip
```

**What it does**: Extracts all ZIP files in `./data/990_zips/` to individual directories. Each ZIP contains thousands of XML files.

**Output**: Extracted XML files in `./data/990_zips/<archive_name>/`

### 3. Generate CSV Data

```bash
./theIRS csv
```

**What it does**: Processes all XML files and generates a comprehensive CSV file with structured data.

**Output**: `irs_990_data.csv` in the project root

### Advanced Commands

#### Download Schemas (for developers)

```bash
./theIRS schemas
```

Downloads XSD schema files from the IRS and generates Go structs for type-safe parsing.

#### Legacy Download (deprecated)

```bash
./theIRS zips
```

Downloads all ZIP files from scratch. Use `sync` instead for better control.

## Demo

Here's a typical workflow:

```bash
# Step 1: Download the data (only missing files)
./theIRS sync
# Output:
# Checking for missing zip files...
# Found 15 missing files. Downloading...
# [1/15] Downloading 2024_TEOS_XML_12A.zip...
# ✓ Successfully downloaded 2024_TEOS_XML_12A.zip
# ...
# Download complete! Downloaded 15 new files.

# Step 2: Extract the archives
./theIRS unzip
# Proceed? [y/n]: y
# Extracting 2024_TEOS_XML_12A.zip to ./data/990_zips/2024_TEOS_XML_12A...
# ✓ Successfully extracted 2024_TEOS_XML_12A.zip
# ...
# Extraction complete! Extracted 72 ZIP files.

# Step 3: Generate CSV
./theIRS csv
# Proceed? [y/n]: y
# Processing directory: data/990_zips/2024_TEOS_XML_12A
# Processed 1000 files
# Processed 2000 files
# ...
# Processing complete. Total files processed: 458923
# CSV generation complete! Check irs_990_data.csv
```

## CSV Output Fields

The generated CSV includes the following categories of data:

### Organization Identification
- FileName, EIN, OrganizationName, TaxYear, ReturnType
- AddressLine1, AddressLine2, City, State, ZIPCode, Country
- Phone, Website

### Financial Summary
- TotalRevenue, TotalExpenses, NetIncome
- TotalAssets, TotalLiabilities, NetAssets
- Revenue sources (Contributions, ProgramServiceRevenue, InvestmentIncome)
- Expense categories (ProgramServices, Management, Fundraising)

### Balance Sheet
- Beginning/End of Year (BOY/EOY) comparisons for:
  - Assets (Cash, Investments, Land, Buildings, Equipment)
  - Liabilities (Accounts Payable, Grants Payable, Mortgages, Bonds, Other Debt)
  - Net Assets

### People & Compensation
- BoardMembers, Volunteers, Employees
- Officer/Employee/Contractor Compensation

### Activities
- Grants (to organizations/individuals)
- Foreign activities and income
- Unrelated business income
- Political/lobbying activity indicators

### Filing Information
- Filing dates, tax periods, preparer information
- Return type indicators (Amended, Initial, Final)
- Schedule attachments (A-R)

## Project Structure

```
theIRS/
├── main.go              # CLI entry point and orchestration
├── crawler.go           # HTTP download logic for ZIP files and schemas
├── csv.go               # XML to CSV conversion with field mapping
├── parser.go            # Legacy XML parsing (deprecated in favor of csv.go)
├── schemas.go           # XSD schema processing and Go code generation
├── scan_all_eins.go     # Utility for searching specific EINs
├── data/
│   ├── 990_zips/        # Downloaded ZIP files and extracted XMLs
│   └── 990_xsd/         # XSD schema files
├── models/              # Generated Go structs from XSD schemas
└── xsd2go/              # XSD to Go conversion tool (submodule)
```

## Performance Considerations

- **Concurrent Processing**: Goroutines with semaphore-based rate limiting (configurable via MAXPROCS)
- **HTTP Connection Pooling**: Reuses connections with 100 max idle connections
- **Memory Efficiency**: Processes files individually to avoid loading entire dataset into memory
- **Disk I/O**: Uses buffered CSV writers and efficient file streaming
- **Progress Logging**: Logs every 1,000 files processed to track progress
- **Retry with Backoff**: Exponential backoff prevents overwhelming servers during retries
- **Request Timeouts**: 30-second default timeout prevents hanging on slow connections

## Data Source

All data is sourced from official IRS publications:
- Form 990 Downloads: https://www.irs.gov/charities-non-profits/form-990-series-downloads
- Schema Definitions: https://www.irs.gov/charities-non-profits/tax-exempt-organization-search-teos-schemas

## Code Quality & Reliability

This codebase has been thoroughly reviewed and improved with a focus on reliability, error handling, and fault tolerance:

### Recent Improvements (2025)

**Error Handling & Reliability**:
- ✅ Eliminated all `panic()` calls - replaced with proper error returns
- ✅ Removed `log.Fatal()` from processing loops - continues on individual file errors
- ✅ Standardized error handling across all files with consistent `fmt.Errorf` wrapping
- ✅ Added comprehensive error checking for all file and network operations

**Resource Management**:
- ✅ Fixed critical defer-in-loop resource leaks in HTTP download functions
- ✅ Proper file handle cleanup with deferred Close() in correct scopes
- ✅ Fixed variable shadowing bugs (zip.File shadowing issue resolved)
- ✅ All resources properly closed even in error paths

**Network Resilience**:
- ✅ HTTP retry logic with exponential backoff (3 retries max)
- ✅ Request timeouts (30 seconds default)
- ✅ HTTP connection pooling with configurable limits
- ✅ Context-based cancellation support
- ✅ Graceful handling of 4xx vs 5xx errors

**Concurrency**:
- ✅ Goroutine rate limiting (prevents resource exhaustion)
- ✅ Fixed race conditions in shared map access
- ✅ Each goroutine uses isolated data structures
- ✅ Thread-safe CSV writing with mutex protection
- ✅ Configurable concurrency level (MAXPROCS)

**Code Quality**:
- ✅ Fixed chmod syntax (`+x` not `x+a`)
- ✅ Consistent directory permissions (0755 throughout)
- ✅ Efficient string concatenation using strings.Builder
- ✅ Proper use of filepath.Join for cross-platform paths
- ✅ Comprehensive logging with progress indicators
- ✅ Separated scan_eins tool into its own command

### Fault Tolerance Features

The tool now gracefully handles:
- Network timeouts and temporary failures
- Malformed or corrupt XML files
- Missing directories or files
- Individual download failures (continues with other files)
- HTTP errors (distinguishes between client and server errors)
- Partial data scenarios

## Security

The tool includes security measures:
- **Zip Slip Protection**: Path traversal validation for all ZIP extraction operations
- **Safe File Operations**: Checks file existence and permissions before writing
- **HTTP Validation**: Validates response codes and content before processing
- **No Arbitrary Command Execution**: Fixed shell command construction
- **Resource Limits**: Prevents resource exhaustion through rate limiting
- **Error Isolation**: Individual file failures don't crash entire process

## License

See LICENSE file for details.

## Contributing

Contributions welcome! Please ensure code follows Go best practices and includes appropriate error handling.

## Future Enhancements

- [ ] Add command-line flags for configuration (output path, concurrency level, retry count)
- [ ] Implement progress bars for long-running operations
- [ ] Add filtering options (by year, state, revenue range)
- [ ] Support for incremental CSV updates (resume after interruption)
- [ ] Database export options (PostgreSQL, SQLite)
- [ ] JSON output format option
- [ ] Metrics/telemetry (success rate, processing speed)
- [ ] Compression support for output CSV
- [ ] Parallel CSV writing with data sharding

## Reliability Testing

To verify the improvements:

```bash
# Test basic functionality
./theIRS help

# Test with small dataset (if you have sample data)
./theIRS csv

# Monitor resource usage
# The tool should now handle errors gracefully without crashing
```

## Troubleshooting

If you encounter issues:

1. **Out of disk space**: The full dataset requires 50GB+. Check `df -h`
2. **Network errors**: The tool will retry automatically (3 attempts with exponential backoff)
3. **Permission errors**: Ensure you have write access to `./data/` directory
4. **Too many open files**: The tool now limits concurrent operations, but you may need to increase system limits: `ulimit -n 4096`
5. **Memory issues**: Reduce MAXPROCS in parser.go if needed (default: 12)

Check logs for detailed error messages - the tool now provides comprehensive error context.
