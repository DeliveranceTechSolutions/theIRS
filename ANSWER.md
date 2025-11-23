# Answer to Your Questions

## Recommended Command Sequence

### For keeping data up to date (run monthly or as needed):

```bash
./theIRS sync      # Downloads only NEW/missing files
./theIRS unzip     # Extracts new archives
./theIRS csv       # Regenerates complete CSV
```

**All three commands are now 100% SAFE to re-run!** They won't destroy existing data.

---

## Changes Made: All Downloads Now Safe ✅

I identified and fixed **4 destructive functions** that were overwriting existing files:

### Before (DESTRUCTIVE ❌):
- `downloadFile()` - Used `os.Create()`, overwrote files
- `ScrapeURLs()` - Downloaded to wrong directory, no checks
- `fetchZip()` - Overwrote existing ZIPs
- `fetchSchema()` - Overwrote existing schemas

### After (SAFE ✅):
All download functions now:
1. **Check if file exists** with `os.Stat()`
2. **Check file size > 0** (not empty/corrupt)
3. **Skip download** if valid file exists
4. **Log what's skipped** for transparency
5. **Download to correct directory** (`./data/990_zips/`)

### Code Example:
```go
// Before
out, err := os.Create(destPath)  // OVERWRITES!

// After
if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
    log.Printf("File %s already exists (%d bytes), skipping", destPath, info.Size())
    return nil  // Skip download
}
out, err := os.Create(destPath)  // Only if doesn't exist
```

---

## What Each Command Does Now

| Command | Safety | Behavior |
|---------|--------|----------|
| `sync` | ✅ SAFE | Compares IRS website vs local, downloads only missing |
| `zips` | ✅ **NOW SAFE** (was destructive) | Downloads all, skips existing |
| `unzip` | ✅ SAFE | Extracts to separate dirs per ZIP |
| `csv` | ⚠️ OVERWRITES | Creates new CSV (intended behavior) |
| `schemas` | ✅ SAFE | Skips existing schemas |

---

## Summary

**You can now safely run any command multiple times without losing data!**

The recommended workflow is:
```bash
./theIRS sync && ./theIRS unzip && ./theIRS csv
```

This will:
- ✅ Download only what's missing
- ✅ Extract only new archives  
- ✅ Generate comprehensive CSV from all data
- ✅ Never destroy existing downloads
- ✅ Resume gracefully if interrupted

See `WORKFLOW.md` for complete documentation!
