# theIRS Workflow Guide

## Quick Start - Recommended Workflow

### First Time Setup
```bash
# 1. Download missing data files (smart, incremental)
./theIRS sync

# 2. Extract ZIP archives
./theIRS unzip

# 3. Generate CSV from XML files
./theIRS csv
```

### Updating Existing Data
```bash
# Simply run the same commands - they're all safe and incremental!
./theIRS sync      # Only downloads NEW files
./theIRS unzip     # Only extracts NEW archives
./theIRS csv       # Regenerates CSV with all data
```

## Command Reference

### `sync` - Incremental Download (RECOMMENDED)
**Safety**: ✅ SAFE - Preserves existing data

```bash
./theIRS sync
```

**What it does:**
1. Fetches list of available files from IRS website
2. Compares with locally downloaded files
3. Downloads ONLY missing files
4. Skips files that already exist

**Use when:**
- First time setup
- Updating your dataset with new monthly releases
- Resuming after interrupted download
- You want to be safe and efficient

**Output location:** `./data/990_zips/*.zip`

---

### `unzip` - Extract Archives
**Safety**: ✅ SAFE - Extracts to separate directories

```bash
./theIRS unzip
```

**What it does:**
1. Scans `./data/990_zips/` for ZIP files
2. Extracts each to its own directory
3. Example: `2024_TEOS_XML_12A.zip` → `./data/990_zips/2024_TEOS_XML_12A/`

**Use when:**
- After downloading files with `sync` or `zips`
- Adding new ZIP files manually

**Output location:** `./data/990_zips/<archive_name>/` (one directory per ZIP)

---

### `csv` - Generate CSV Data
**Safety**: ⚠️ OVERWRITES - Creates new CSV file

```bash
./theIRS csv
```

**What it does:**
1. Scans all extracted XML files in `./data/990_zips/*/`
2. Parses each XML file (extracts 170+ fields)
3. Generates comprehensive CSV file

**Use when:**
- After extracting XML files
- You want to regenerate the complete dataset
- You've added new data and want updated CSV

**Output location:** `irs_990_data.csv` (in project root)

**Performance:**
- Processes ~1,000 files per log message
- Uses concurrent processing (12 goroutines default)
- Can process 100,000+ files

---

### `schemas` - Download XSD Schemas (Developer Tool)
**Safety**: ✅ SAFE - Skips existing schemas

```bash
./theIRS schemas
```

**What it does:**
1. Downloads IRS Form 990 XSD schema files
2. Extracts schemas
3. Runs `models.sh` to generate Go structs
4. Skips schemas that already exist

**Use when:**
- Developing new features
- Need to understand IRS XML structure
- Generating type-safe Go code from schemas

**Output location:**
- Schemas: `./data/990_xsd/`
- Generated code: `./data/990_xsd/output/generated_templates/`

---

### `zips` - Legacy Download (Now Safe!)
**Safety**: ✅ SAFE - Skips existing files (recently fixed!)

```bash
./theIRS zips
```

**What it does:**
1. Downloads all ZIP files from IRS website
2. **NOW SAFE**: Skips files that already exist
3. Downloads to `./data/990_zips/` directory

**Use when:**
- You prefer this method over `sync`
- **Recommendation**: Use `sync` instead - it's smarter and more efficient

**Note:** This command was previously destructive but has been fixed to skip existing files.

---

## Data Flow Diagram

```
IRS Website
    ↓
┌───────────────────────────┐
│  ./theIRS sync            │  ← Downloads missing ZIPs
│  Output: *.zip files      │
└───────────────────────────┘
    ↓
┌───────────────────────────┐
│  ./theIRS unzip           │  ← Extracts all ZIPs
│  Output: */xml files      │
└───────────────────────────┘
    ↓
┌───────────────────────────┐
│  ./theIRS csv             │  ← Processes all XMLs
│  Output: irs_990_data.csv │
└───────────────────────────┘
```

## File Structure After Full Run

```
theIRS/
├── theIRS                          # Main executable
├── irs_990_data.csv               # Final output (359MB+)
└── data/
    └── 990_zips/
        ├── 2019_01.zip            # Downloaded ZIPs
        ├── 2019_02.zip
        ├── 2019_01/               # Extracted directories
        │   ├── file1.xml
        │   ├── file2.xml
        │   └── ...
        ├── 2019_02/
        │   └── ...
        └── ...
```

## Safety Features (Recently Added!)

All download commands now include:

✅ **Skip Existing Files**: Checks `os.Stat()` before downloading
✅ **File Size Validation**: Only skips if file size > 0 (not empty/corrupt)
✅ **Logging**: Shows what's skipped vs. downloaded
✅ **No Overwrites**: Never destroys existing data
✅ **Resume Capability**: Can safely re-run after interruption

### Example Output:
```
2024/11/23 10:30:15 File 2024_TEOS_XML_12A.zip already exists (45123456 bytes), skipping
2024/11/23 10:30:16 Downloading: 2024_TEOS_XML_11A.zip
2024/11/23 10:30:45 Downloaded: 2024_TEOS_XML_11A.zip
```

## Monthly Update Workflow

The IRS releases new data monthly. To stay current:

```bash
# Run once per month (or whenever you want updates)
./theIRS sync      # Downloads new monthly release
./theIRS unzip     # Extracts new files only
./theIRS csv       # Regenerates CSV with all data
```

**Time estimates** (full dataset, first run):
- `sync`: 2-6 hours (depending on connection)
- `unzip`: 30-60 minutes
- `csv`: 1-3 hours (100,000+ files)

**Incremental updates** (monthly):
- `sync`: 5-15 minutes (1-2 new files)
- `unzip`: 2-5 minutes
- `csv`: 1-3 hours (reprocesses all)

## Common Scenarios

### Scenario 1: Complete Fresh Install
```bash
./theIRS sync
./theIRS unzip
./theIRS csv
```

### Scenario 2: Monthly Update
```bash
# Same commands! All are safe to re-run
./theIRS sync
./theIRS unzip
./theIRS csv
```

### Scenario 3: Resume Interrupted Download
```bash
# Simply re-run sync - it skips completed files
./theIRS sync
```

### Scenario 4: Regenerate CSV (data looks wrong)
```bash
# Just run csv again - it reprocesses all XML files
./theIRS csv
```

### Scenario 5: Add Specific Year Manually
```bash
# Download a specific file manually
curl -o data/990_zips/2020_TEOS_XML_05A.zip \
  https://apps.irs.gov/pub/epostcard/990/xml/2020/2020_TEOS_XML_05A.zip

# Extract and process
./theIRS unzip
./theIRS csv
```

## Troubleshooting

### "No space left on device"
Full dataset requires 50GB+. Check available space:
```bash
df -h
```

### "Too many open files"
Increase system limits:
```bash
ulimit -n 4096
```

### Network timeouts
The tool automatically retries (3 attempts with exponential backoff). If failures persist:
- Check internet connection
- IRS website may be down (try later)
- Use VPN if blocked

### CSV generation very slow
- Normal for 100,000+ files
- Runs 12 concurrent processors by default
- Check CPU usage with `top` or `htop`
- Reduce MAXPROCS in parser.go if memory constrained

### Want to process subset of data
Currently processes all XML files found. To limit:
```bash
# Move unwanted directories temporarily
mkdir data/990_zips_backup
mv data/990_zips/2019* data/990_zips_backup/

# Process only remaining years
./theIRS csv

# Restore later
mv data/990_zips_backup/* data/990_zips/
```

## Advanced Usage

### Check what's missing without downloading
The `sync` command shows what it will download before proceeding.

### Parallel downloads
Currently sequential. For faster downloads, consider:
- Better internet connection
- Running during off-peak hours
- IRS servers may rate-limit anyway

### Custom processing
The CSV contains 170+ fields. For custom analysis:
- Import `irs_990_data.csv` into your tool of choice
- Use pandas, Excel, PostgreSQL, etc.
- Fields include financials, compensation, grants, activities

## Help & Support

```bash
# Show command list
./theIRS help

# This comprehensive guide
cat WORKFLOW.md
```

For issues or improvements, check the GitHub repository or CHANGELOG.md for recent fixes.
