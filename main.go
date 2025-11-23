package main

import (
    "archive/zip"
    "bufio"
    "fmt"
    "io"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

func confirmation(s string, tries int) (bool, error) {
    r := bufio.NewReader(os.Stdin)

    for ; tries > 0; tries-- {
        fmt.Printf("%s Proceed? [y/n]: ", s)

        res, err := r.ReadString('\n')
        if err != nil {
            return false, fmt.Errorf("failed to read input: %w", err)
        }
        // Empty input (i.e. "\n")
        if len(res) < 2 {
            continue
        }

        return strings.ToLower(strings.TrimSpace(res))[0] == 'y', nil
    }

    return false, nil
}

func printUsage() {
    fmt.Println("theIRS - IRS Form 990 Data Extraction Tool")
    fmt.Println()
    fmt.Println("Usage: theIRS <command>")
    fmt.Println()
    fmt.Println("Commands:")
    fmt.Println("  sync      Check and download missing ZIP files (recommended)")
    fmt.Println("  unzip     Extract all ZIP files to directories")
    fmt.Println("  csv       Process XML files and generate CSV output")
    fmt.Println("  schemas   Download and process XSD schema files (for developers)")
    fmt.Println("  zips      Download all ZIP files from scratch (deprecated, use sync)")
    fmt.Println("  help      Show this help message")
    fmt.Println()
    fmt.Println("Example workflow:")
    fmt.Println("  ./theIRS sync    # Download missing files")
    fmt.Println("  ./theIRS unzip   # Extract ZIP archives")
    fmt.Println("  ./theIRS csv     # Generate CSV from XML files")
}

func main() {
    if len(os.Args) < 2 {
        printUsage()
        return
    } else if len(os.Args) > 2 {
        fmt.Println("Error: Too many arguments")
        printUsage()
        return
    }

    switch os.Args[1] {
    case "help", "-h", "--help":
        printUsage()

    case "zips":
        proceed, err := confirmation(`
        This will download all ZIP files from the IRS website.
        Files that already exist will be skipped automatically.
        Note: The 'sync' command is recommended instead as it's more efficient.

        `, 3)
        if err != nil {
            fmt.Printf("Error reading confirmation: %v\n", err)
            os.Exit(1)
        }
        if proceed {
            zips, err := UnpackZips()
            if err != nil {
                fmt.Printf("Error: %v\n", err)
                os.Exit(1)
            }
            log.Printf("Downloaded %d zip files", len(zips))
        } else {
            fmt.Println("Aborting")
        }

    case "sync":
        proceed, err := confirmation(`
        This will check what zip files are already downloaded and download only the missing ones.
        This is safe to run multiple times.

        `, 3)
        if err != nil {
            fmt.Printf("Error reading confirmation: %v\n", err)
            os.Exit(1)
        }
        if proceed {
            if err := CheckAndDownloadMissingZips(); err != nil {
                fmt.Printf("Error: %v\n", err)
                os.Exit(1)
            } else {
                fmt.Println("Sync complete!")
            }
        } else {
            fmt.Println("Aborting")
        }

    case "schemas":
        versions, err := UnpackSchemas()
        if err != nil {
            fmt.Printf("Error unpacking schemas: %v\n", err)
            os.Exit(1)
        }
        links := generateLinks(versions)
        log.Printf("Generated %d schema links", len(links))

        if err := UnzipSchemas(); err != nil {
            fmt.Printf("Error unzipping schemas: %v\n", err)
            os.Exit(1)
        }

        files, err := GlobWalk("./data/990_xsd/output", "*.xsd")
        if err != nil {
            fmt.Printf("Error globbing XSD files: %v\n", err)
            os.Exit(1)
        }
        log.Printf("Found %d XSD files", len(files))

        cmd := exec.Command("bash", "-c", "chmod +x ./models.sh && ./models.sh")
        if err := cmd.Run(); err != nil {
            fmt.Printf("Pipeline failed to run: %v\n", err)
            os.Exit(1)
        } else {
            log.Println("Completed pipeline collapse")
        }

    case "unzip":
        proceed, err := confirmation(`
        This will extract all ZIP files in the ./data/990_zips directory.
        Each ZIP file will be extracted to its own directory.

        `, 3)
        if err != nil {
            fmt.Printf("Error reading confirmation: %v\n", err)
            os.Exit(1)
        }
        if proceed {
            if err := ExtractAllZips(); err != nil {
                fmt.Printf("Error: %v\n", err)
                os.Exit(1)
            } else {
                fmt.Println("Unzip complete!")
            }
        } else {
            fmt.Println("Aborting")
        }

    case "csv":
        proceed, err := confirmation(`
        This will process all XML files in the ./data/990_zips directories
        and create a comprehensive CSV file with IRS Form 990 data.

        Output file: irs_990_data.csv

        `, 3)
        if err != nil {
            fmt.Printf("Error reading confirmation: %v\n", err)
            os.Exit(1)
        }
        if proceed {
            if err := ProcessAllDirectories(); err != nil {
                fmt.Printf("Error: %v\n", err)
                os.Exit(1)
            } else {
                fmt.Println("CSV generation complete! Check irs_990_data.csv")
            }
        } else {
            fmt.Println("Aborting")
        }

    default:
        fmt.Printf("Error: Unknown command '%s'\n\n", os.Args[1])
        printUsage()
        os.Exit(1)
    }
}

func generateLinks(versions map[string]Version) []string {
    var links []string  

    for _, version := range versions {
        links = append(links, fmt.Sprintf(`https://www.irs.gov/pub/irs-tege/%s`, version.Schedule))
    }

    return links
}

// ExtractAllZips extracts all ZIP files in the data/990_zips directory
func ExtractAllZips() error {
    zipDir := "./data/990_zips"

    // Read all files in the directory
    entries, err := os.ReadDir(zipDir)
    if err != nil {
        return fmt.Errorf("failed to read directory: %w", err)
    }

    var extractedCount int
    var skippedCount int

    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
            continue
        }

        zipPath := filepath.Join(zipDir, entry.Name())
        extractDir := filepath.Join(zipDir, strings.TrimSuffix(entry.Name(), ".zip"))

        // Check if already extracted (directory exists and has files)
        if dirInfo, err := os.Stat(extractDir); err == nil && dirInfo.IsDir() {
            // Check if directory has content
            dirEntries, err := os.ReadDir(extractDir)
            if err == nil && len(dirEntries) > 0 {
                skippedCount++
                fmt.Printf("⏭  Skipping %s (already extracted, %d files)\n", entry.Name(), len(dirEntries))
                continue
            }
        }

        fmt.Printf("Extracting %s to %s...\n", entry.Name(), extractDir)

        if err := extractZip(zipPath, extractDir); err != nil {
            fmt.Printf("Error extracting %s: %v\n", entry.Name(), err)
            continue
        }

        extractedCount++
        fmt.Printf("✓ Successfully extracted %s\n", entry.Name())
    }

    fmt.Printf("\nExtraction complete!\n")
    fmt.Printf("  Extracted: %d ZIP files\n", extractedCount)
    fmt.Printf("  Skipped:   %d ZIP files (already extracted)\n", skippedCount)
    return nil
}

// extractZip extracts a single ZIP file to the specified directory
func extractZip(zipPath, extractDir string) error {
    // Open the ZIP file
    reader, err := zip.OpenReader(zipPath)
    if err != nil {
        return fmt.Errorf("failed to open ZIP file: %w", err)
    }
    defer reader.Close()
    
    // Create the extraction directory
    if err := os.MkdirAll(extractDir, 0755); err != nil {
        return fmt.Errorf("failed to create extraction directory: %w", err)
    }
    
    // Extract each file in the ZIP
    for _, file := range reader.File {
        filePath := filepath.Join(extractDir, file.Name)
        
        // Check for path traversal
        if !strings.HasPrefix(filePath, filepath.Clean(extractDir)+string(os.PathSeparator)) {
            return fmt.Errorf("illegal file path: %s", filePath)
        }
        
        if file.FileInfo().IsDir() {
            // Create directory
            if err := os.MkdirAll(filePath, file.Mode()); err != nil {
                return fmt.Errorf("failed to create directory: %w", err)
            }
            continue
        }
        
        // Create parent directories for the file
        if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
            return fmt.Errorf("failed to create parent directories: %w", err)
        }
        
        // Open the file in the ZIP
        zipFile, err := file.Open()
        if err != nil {
            return fmt.Errorf("failed to open file in ZIP: %w", err)
        }
        
        // Create the output file
        outputFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
        if err != nil {
            zipFile.Close()
            return fmt.Errorf("failed to create output file: %w", err)
        }
        
        // Copy the file contents
        if _, err := io.Copy(outputFile, zipFile); err != nil {
            zipFile.Close()
            outputFile.Close()
            return fmt.Errorf("failed to copy file contents: %w", err)
        }
        
        zipFile.Close()
        outputFile.Close()
    }
    
    return nil
}

