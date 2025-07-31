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

func confirmation(s string, tries int) bool {
    r := bufio.NewReader(os.Stdin)

    for ; tries > 0; tries-- {
        fmt.Printf("%s Proceed? [y/n]: ", s)

        res, err := r.ReadString('\n')
        if err != nil {
            log.Fatal(err)
        }
        // Empty input (i.e. "\n")
        if len(res) < 2 {
            continue
        }

        return strings.ToLower(strings.TrimSpace(res))[0] == 'y'
    }

    return false
}

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Nah need a command")
        return
    } else if len(os.Args) > 2 {
        fmt.Println("too many")
        return 
    }



    switch os.Args[1] {
    case "zips":
        proceed := confirmation(`
        This will delete any and all zip files in the ./data/990_zips directory.
        Do not proceed with this command if you have already used it.

        `, 3)
        if proceed {
            zips, err := UnpackZips()
            if err != nil {
                fmt.Println(err)
            }
            fmt.Println(zips)
        } else {
            fmt.Println("Aborting")
        }
        break

    case "sync":
        proceed := confirmation(`
        This will check what zip files are already downloaded and download only the missing ones.
        This is safe to run multiple times.
        
        `, 3)
        if proceed {
            if err := CheckAndDownloadMissingZips(); err != nil {
                fmt.Printf("Error: %v\n", err)
            } else {
                fmt.Println("Sync complete!")
            }
        } else {
            fmt.Println("Aborting")
        }
        break

    case "schemas":
        versions, err := UnpackSchemas()
        if err != nil {
            fmt.Println(err)
        }
        links := generateLinks(versions) 
        fmt.Println(links)
        UnzipSchemas()
        files, err := GlobWalk("./data/990_xsd/output", "*.xsd")
        if err != nil {
            fmt.Println(err)
        }
        fmt.Println(files) 

        cmd := exec.Command("bash", "chmod x+a ./models.sh; ./models.sh")
        if err := cmd.Run(); err != nil {
            fmt.Println("pipeline failed to run", err)
        } else {
            log.Println("Completed pipeline collapse")
        }

        break

    case "unzip":
        proceed := confirmation(`
        This will extract all ZIP files in the ./data/990_zips directory.
        Each ZIP file will be extracted to its own directory.
        
        `, 3)
        if proceed {
            if err := ExtractAllZips(); err != nil {
                fmt.Printf("Error: %v\n", err)
            } else {
                fmt.Println("Unzip complete!")
            }
        } else {
            fmt.Println("Aborting")
        }
        break

    case "csv":
        proceed := confirmation(`
        This will process all XML files in the ./data/990_zips directories
        and create a comprehensive CSV file with IRS Form 990 data.
        
        Output file: irs_990_data.csv
        
        `, 3)
        if proceed {
            if err := ProcessAllDirectories(); err != nil {
                fmt.Printf("Error: %v\n", err)
            } else {
                fmt.Println("CSV generation complete! Check irs_990_data.csv")
            }
        } else {
            fmt.Println("Aborting")
        }
        break

    default:
        fmt.Println("the argument provided doesn't exist")
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
    for _, entry := range entries {
        if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
            continue
        }
        
        zipPath := filepath.Join(zipDir, entry.Name())
        extractDir := filepath.Join(zipDir, strings.TrimSuffix(entry.Name(), ".zip"))
        
        fmt.Printf("Extracting %s to %s...\n", entry.Name(), extractDir)
        
        if err := extractZip(zipPath, extractDir); err != nil {
            fmt.Printf("Error extracting %s: %v\n", entry.Name(), err)
            continue
        }
        
        extractedCount++
        fmt.Printf("✓ Successfully extracted %s\n", entry.Name())
    }
    
    fmt.Printf("Extraction complete! Extracted %d ZIP files.\n", extractedCount)
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

