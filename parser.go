package main

import (
	"archive/zip"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

type Xmler struct {
    Record map[string][]string
    Writer *csv.Writer
}

var wg sync.WaitGroup
var rw sync.RWMutex
var myInt atomic.Int64

const (
    MAXPROCS = 12
)

func ParseXMLs() error {
    runtime.GOMAXPROCS(MAXPROCS)
    // _, header := Load()  // Load function not defined
    header := []string{"FileName", "EIN", "OrganizationName", "TaxYear", "ReturnType"} // Simple header
    sheet, err := os.Create("resolve.csv")
    if err != nil {
        return fmt.Errorf("failed to create output CSV: %w", err)
    }
    defer sheet.Close()
    writer := csv.NewWriter(sheet)

    if err := writer.Write(header); err != nil {
        return fmt.Errorf("failed to write CSV header: %w", err)
    }
    writer.Flush()
    if err := writer.Error(); err != nil {
        return fmt.Errorf("failed to flush CSV header: %w", err)
    }

    pathway := "./data/990_zips/"
    reader, err := os.ReadDir(pathway)
    if err != nil {
        return fmt.Errorf("failed to read directory %s: %w", pathway, err)
    }

    // Use buffered channel to limit concurrent goroutines
    semaphore := make(chan struct{}, MAXPROCS)
    var processingErrors sync.Map // Thread-safe error collection

    re := regexp.MustCompile(`.zip`)
    for _, zipper := range reader {
        if !re.Match([]byte(zipper.Name())) {
            dirPath := filepath.Join(pathway, zipper.Name())
            zReader, err := os.ReadDir(dirPath)
            if err != nil {
                log.Printf("Error reading directory %s: %v", dirPath, err)
                continue
            }

            wg.Add(1)
            semaphore <- struct{}{} // Acquire semaphore
            go func(path string, files []os.DirEntry) {
                defer wg.Done()
                defer func() { <-semaphore }() // Release semaphore

                // Each goroutine gets its own writer and record
                xmler := &Xmler{
                    Record: make(map[string][]string),
                    Writer: writer,
                }

                if err := xmler.generateRows(path, files); err != nil {
                    processingErrors.Store(path, err)
                    log.Printf("Error processing %s: %v", path, err)
                }
            }(dirPath, zReader)
        }
    }

    wg.Wait()
    writer.Flush()
    if err := writer.Error(); err != nil {
        return fmt.Errorf("failed to flush final CSV data: %w", err)
    }

    return nil
}

func (x *Xmler) generateRows(root string, files []os.DirEntry) error {
    for _, file := range files {
        if file.IsDir() {
            continue
        }

        filePath := filepath.Join(root, file.Name())
        f, err := os.Open(filePath)
        if err != nil {
            log.Printf("Error opening file %s: %v", filePath, err)
            continue // Skip this file, process others
        }

        decoder := xml.NewDecoder(f)
        if err := x.flatten(xml.StartElement{}, decoder, ""); err != nil {
            f.Close()
            log.Printf("Error parsing XML in %s: %v", filePath, err)
            continue
        }
        f.Close()
    }

    return nil
}

func (x *Xmler) flatten(element xml.StartElement, decoder *xml.Decoder, prefix string) error {
    var lastTag string
    for {
        tok, err := decoder.Token()
        if err == io.EOF {
            // Build row from record
            rw.Lock()
            var row []string
            for _, data := range x.Record {
                length := len(data)
                if length > 1 {
                    // Use strings.Builder for efficient concatenation
                    var builder strings.Builder
                    for _, b := range data {
                        builder.WriteString(b)
                    }
                    row = append(row, builder.String())
                } else if length == 1 {
                    row = append(row, data[0])
                } else {
                    row = append(row, "")
                }
            }

            if err := x.Writer.Write(row); err != nil {
                rw.Unlock()
                return fmt.Errorf("failed to write CSV row: %w", err)
            }
            x.Writer.Flush()
            if err := x.Writer.Error(); err != nil {
                rw.Unlock()
                return fmt.Errorf("failed to flush CSV writer: %w", err)
            }
            rw.Unlock()

            myInt.Add(1)
            if myInt.Load()%1000 == 0 {
                log.Printf("Processed %d files", myInt.Load())
            }
            return nil
        }
        if err != nil {
            return fmt.Errorf("XML parsing error: %w", err)
        }

        switch t := tok.(type) {
        case xml.StartElement:
            fullTag := prefix + "." + t.Name.Local
            lastTag = t.Name.Local
            element = t
            if err := x.flatten(t, decoder, fullTag); err != nil {
                return err
            }

        case xml.CharData:
            val := strings.TrimSpace(string(t))
            if val != "" {
                x.Record[prefix] = append(x.Record[prefix], val)
            }
            if _, ok := x.Record[lastTag]; !ok {
                x.Record[lastTag] = []string{}
            }

        case xml.EndElement:
            if t.Name.Local == element.Name.Local {
                return nil
            }
        }
    }
}

func UnzipXMLs() error {
    pathway := "./data/990_zips/"

    reader, err := os.ReadDir(pathway)
    if err != nil {
        return fmt.Errorf("failed to read directory %s: %w", pathway, err)
    }

    for _, zipper := range reader {
        if !strings.HasSuffix(strings.ToLower(zipper.Name()), ".zip") {
            continue
        }

        srcPath := filepath.Join(pathway, zipper.Name())
        destPath := strings.TrimSuffix(srcPath, filepath.Ext(srcPath))

        if err := unzipXMLs(srcPath, destPath); err != nil {
            log.Printf("Error unzipping %s: %v", srcPath, err)
            continue
        }
    }

    return nil
}

func unzipXMLs(src, dest string) error {
    r, err := zip.OpenReader(src)
    if err != nil {
        return fmt.Errorf("failed to open zip %s: %w", src, err)
    }
    defer func() {
        if closeErr := r.Close(); closeErr != nil {
            log.Printf("Error closing zip reader for %s: %v", src, closeErr)
        }
    }()

    if err := os.MkdirAll(dest, 0755); err != nil {
        return fmt.Errorf("failed to create destination directory %s: %w", dest, err)
    }

    for _, f := range r.File {
        if err := extractZipFile(f, dest); err != nil {
            log.Printf("Error extracting %s from %s: %v", f.Name, src, err)
            continue
        }
    }

    return nil
}

func extractZipFile(f *zip.File, dest string) error {
    rc, err := f.Open()
    if err != nil {
        return fmt.Errorf("failed to open file in zip: %w", err)
    }
    defer func() {
        if closeErr := rc.Close(); closeErr != nil {
            log.Printf("Error closing zip file %s: %v", f.Name, closeErr)
        }
    }()

    path := filepath.Join(dest, f.Name)
    if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
        return fmt.Errorf("illegal file path (zip slip protection): %s", path)
    }

    if f.FileInfo().IsDir() {
        if err := os.MkdirAll(path, 0755); err != nil {
            return fmt.Errorf("failed to create directory: %w", err)
        }
        return nil
    }

    if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return fmt.Errorf("failed to create parent directory: %w", err)
    }

    outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
    if err != nil {
        return fmt.Errorf("failed to create output file: %w", err)
    }
    defer func() {
        if closeErr := outFile.Close(); closeErr != nil {
            log.Printf("Error closing output file %s: %v", path, closeErr)
        }
    }()

    if _, err = io.Copy(outFile, rc); err != nil {
        return fmt.Errorf("failed to copy file contents: %w", err)
    }

    return nil
}

