package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
)

const (
	maxRetries     = 3
	retryDelay     = 2 * time.Second
	requestTimeout = 30 * time.Second
)

var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// httpGetWithRetry performs an HTTP GET with retry logic and exponential backoff
func httpGetWithRetry(ctx context.Context, url string) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryDelay * time.Duration(1<<uint(attempt-1)) // Exponential backoff
			log.Printf("Retry %d/%d for %s after %v", attempt+1, maxRetries, url, delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("HTTP request failed (attempt %d/%d): %v", attempt+1, maxRetries, err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		// For non-200 status codes, decide whether to retry
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			// Server errors - retry
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			log.Printf("Server error (attempt %d/%d): %v", attempt+1, maxRetries, lastErr)
			continue
		}

		// Client errors (4xx) - don't retry
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

type Crawler struct {}

const (
    upTo20 = `https://apps.irs.gov/pub/epostcard/990/xml/`
    post20 = `https://apps.irs.gov/pub/epostcard/990/xml/`
    currentStart = 2019
    currentYear = 2025
)

var fileTracker atomic.Int32
var (
    fileYear = ""
)

type Version struct {
    Schedule string
    Major int
    Minor int
    Sep string
}

var ledger map[string]Version

func ScrapeURLs() error {
    // Ensure download directory exists
    if err := os.MkdirAll("./data/990_zips", 0755); err != nil {
        return fmt.Errorf("failed to create download directory: %w", err)
    }

    for year := currentStart; year <= currentYear; year++ {
        for counter := 12; counter > 0; counter-- {
            var template string
            if year < 2021 {
                template = upTo20 + fmt.Sprintf(`%d/download990xml_%d_%d.zip`, year, year, counter)
            } else {
                template = upTo20 + fmt.Sprintf(`%d/%d_TEOS_XML_%02dA.zip`, year, year, counter)
            }

            log.Printf("Downloading: %s", template)

            // Download to data directory, not current directory
            destPath := filepath.Join("./data/990_zips", fmt.Sprintf(`%d_%d.zip`, year, counter))
            if err := downloadFile(template, destPath); err != nil {
                log.Printf("Error downloading %s: %v", template, err)
                continue
            }
        }
    }
    return nil
}

func downloadFile(url, destPath string) error {
    // Check if file already exists and has content
    if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
        log.Printf("File %s already exists (%d bytes), skipping", destPath, info.Size())
        return nil
    }

    ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*2)
    defer cancel()

    res, err := httpGetWithRetry(ctx, url)
    if err != nil {
        return fmt.Errorf("HTTP GET failed: %w", err)
    }
    defer res.Body.Close()

    out, err := os.Create(destPath)
    if err != nil {
        return fmt.Errorf("failed to create file: %w", err)
    }
    defer out.Close()

    if _, err = io.Copy(out, res.Body); err != nil {
        return fmt.Errorf("failed to write file: %w", err)
    }

    log.Printf("Downloaded: %s", destPath)
    return nil
}

func UnpackSchemas() (map[string]Version, error) {
    ledger = make(map[string]Version)

    ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*2)
    defer cancel()

    res, err := httpGetWithRetry(ctx, "https://www.irs.gov/charities-non-profits/tax-exempt-organization-search-teos-schemas")
    if err != nil {
        return nil, fmt.Errorf("failed to fetch schema page: %w", err)
    }
    defer res.Body.Close()

    doc, err := html.Parse(res.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to parse HTML: %w", err)
    }

    if err := os.MkdirAll("./data/990_xsd", 0755); err != nil {
        return nil, fmt.Errorf("failed to create schema directory: %w", err)
    }

    var walk func(*html.Node)
    walk = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "a" {
            for _, attr := range n.Attr {
                if attr.Key == "href" {
                    if strings.Contains(attr.Val, ".zip") {
                        if err := fetchSchema(attr.Val); err != nil {
                            log.Printf("Error fetching schema %s: %v", attr.Val, err)
                        }
                    }
                }
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            walk(c)
        }
    }
    walk(doc)
    log.Printf("Fetched %d schema versions", len(ledger))
    return ledger, nil
}

func UnpackZips() ([]string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*2)
    defer cancel()

    res, err := httpGetWithRetry(ctx, "https://www.irs.gov/charities-non-profits/form-990-series-downloads")
    if err != nil {
        return nil, fmt.Errorf("failed to fetch downloads page: %w", err)
    }
    defer res.Body.Close()

    doc, err := html.Parse(res.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to parse HTML: %w", err)
    }

    if err := os.MkdirAll(`./data/990_zips/`, 0755); err != nil {
        return nil, fmt.Errorf("failed to create zips directory: %w", err)
    } 

    var links []string
    var walk func(*html.Node)
    walk = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "a" {
            for _, attr := range n.Attr {
                if attr.Key == "href" {
                    if strings.Contains(attr.Val, ".zip") {
                        links = append(links, attr.Val)
                    }
                }
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            walk(c)
        }
    }
    walk(doc)

    var zipData []string
    for _, uri := range links {
        tracker, err := fetchZip(uri)
        if err != nil {
            log.Printf("Error fetching zip %s: %v", uri, err)
            continue
        }
        zipData = append(zipData, tracker)
    }
    log.Printf("Downloaded %d zip files", len(zipData))
    return links, nil
}

func splitYear(uri string, strategy string) string {
    switch strategy {
    case "schema":
        fmt.Println(uri)
        res := strings.Split(uri, "/")
        schedule := res[5]
        
        var sep string
        var major, minor int
        var test, register []string
        if strings.Contains(schedule, "v") {
            test = strings.Split(schedule, "v")
            register = strings.Split(test[0], "-")
        } else {
            register = strings.Split(schedule, "-")
        }
         
        if len(test) > 0 {
            var erra error
            major, erra = strconv.Atoi(string(test[1][0]))
            if erra != nil {
                fmt.Println(erra)
            }

            minor, erra = strconv.Atoi(string(test[1][2]))
            if erra != nil {
                fmt.Println(erra)
            }
            sep = string(test[1][1])
        }

        year := register[len(register) - 1]
        key := year + ":" + string(register[0][3])
        if found, ok := ledger[key]; ok {
            if found.Major < major {
                ledger[key] = Version{
                    Schedule: schedule,
                    Major: major,
                    Minor: minor,
                    Sep: sep,
                }
            } else if found.Major == major {
                if found.Minor < minor {
                    ledger[key] = Version{
                        Schedule: schedule,
                        Major: major,
                        Minor: minor,
                        Sep: sep,
                    } 
                }
            }
        } else {
            ledger[key] = Version{
                Schedule: schedule,
                Major: major,
                Minor: minor,
                Sep: sep,
            } 
        }

        return res[5]
    case "zips":
        res := strings.Split(uri, "/")
        if len(res) >= 8 {
            return res[6]
        }
        return ""
    }

    return "" 
}


func fetchSchema(uri string) error {
    log.Printf("Fetching schema: %s", uri)

    year := splitYear(uri, "schema")
    if year != fileYear {
        fileTracker.Store(0)
        fileYear = year
    }

    outPath := filepath.Join("./data/990_xsd", year)

    // Check if schema already exists
    if info, err := os.Stat(outPath); err == nil && info.Size() > 0 {
        log.Printf("Schema %s already exists (%d bytes), skipping", year, info.Size())
        return nil
    }

    ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*2)
    defer cancel()

    res, err := httpGetWithRetry(ctx, uri)
    if err != nil {
        return fmt.Errorf("failed to fetch schema: %w", err)
    }
    defer res.Body.Close()

    out, err := os.Create(outPath)
    if err != nil {
        return fmt.Errorf("failed to create schema file: %w", err)
    }
    defer out.Close()

    if _, err = io.Copy(out, res.Body); err != nil {
        return fmt.Errorf("failed to write schema file: %w", err)
    }

    log.Printf("Downloaded schema: %s", year)
    return nil
}

func fetchZip(uri string) (string, error) {
    // Extract the filename from the URL
    urlParts := strings.Split(uri, "/")
    if len(urlParts) == 0 {
        return "", fmt.Errorf("invalid URL: %s", uri)
    }

    filename := urlParts[len(urlParts)-1]
    tracker := filepath.Join(`./data/990_zips/`, filename)

    // Check if file already exists and has content
    if info, err := os.Stat(tracker); err == nil && info.Size() > 0 {
        log.Printf("File %s already exists (%d bytes), skipping", filename, info.Size())
        return tracker, nil
    }

    ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*2)
    defer cancel()

    res, err := httpGetWithRetry(ctx, uri)
    if err != nil {
        return "", fmt.Errorf("failed to fetch zip: %w", err)
    }
    defer res.Body.Close()

    out, err := os.Create(tracker)
    if err != nil {
        return "", fmt.Errorf("failed to create zip file: %w", err)
    }
    defer out.Close()

    if _, err = io.Copy(out, res.Body); err != nil {
        return "", fmt.Errorf("failed to write zip file: %w", err)
    }

    log.Printf("Downloaded: %s", filename)
    return tracker, nil
}

// CheckAndDownloadMissingZips checks what files are already downloaded and downloads only the missing ones
func CheckAndDownloadMissingZips() error {
	fmt.Println("Checking for missing zip files...")
	
	// Get list of available files from IRS website
	availableFiles, err := getAvailableZipFiles()
	if err != nil {
		return fmt.Errorf("failed to get available files: %w", err)
	}
	
	// Get list of already downloaded files
	downloadedFiles, err := getDownloadedZipFiles()
	if err != nil {
		return fmt.Errorf("failed to get downloaded files: %w", err)
	}
	
	// Find missing files
	missingFiles := findMissingFiles(availableFiles, downloadedFiles)
	
	if len(missingFiles) == 0 {
		fmt.Println("✓ All files are already downloaded!")
		return nil
	}
	
	fmt.Printf("Found %d missing files. Downloading...\n", len(missingFiles))
	
	// Download missing files
	for i, url := range missingFiles {
		filename := extractFilenameFromURL(url)
		fmt.Printf("[%d/%d] Downloading %s...\n", i+1, len(missingFiles), filename)
		
		if err := downloadSingleFile(url, filename); err != nil {
			fmt.Printf("Error downloading %s: %v\n", filename, err)
			continue
		}
		
		fmt.Printf("✓ Successfully downloaded %s\n", filename)
	}
	
	fmt.Printf("Download complete! Downloaded %d new files.\n", len(missingFiles))
	return nil
}

// getAvailableZipFiles fetches the list of available zip files from the IRS website
func getAvailableZipFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*2)
	defer cancel()

	res, err := httpGetWithRetry(ctx, "https://www.irs.gov/charities-non-profits/form-990-series-downloads")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch IRS page: %w", err)
	}
	defer res.Body.Close()

	doc, err := html.Parse(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var links []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" && strings.Contains(attr.Val, ".zip") {
					links = append(links, attr.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	
	return links, nil
}

// getDownloadedZipFiles gets the list of already downloaded zip files
func getDownloadedZipFiles() ([]string, error) {
	zipDir := "./data/990_zips"
	
	// Ensure directory exists
	if err := os.MkdirAll(zipDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}
	
	entries, err := os.ReadDir(zipDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			files = append(files, entry.Name())
		}
	}
	
	return files, nil
}

// findMissingFiles compares available and downloaded files to find what's missing
func findMissingFiles(availableURLs, downloadedFiles []string) []string {
	// Create a map of downloaded filenames for quick lookup
	downloadedMap := make(map[string]bool)
	for _, file := range downloadedFiles {
		downloadedMap[file] = true
	}
	
	var missing []string
	for _, url := range availableURLs {
		filename := extractFilenameFromURL(url)
		if !downloadedMap[filename] {
			missing = append(missing, url)
		}
	}
	
	return missing
}

// extractFilenameFromURL extracts the filename from a URL
func extractFilenameFromURL(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// downloadSingleFile downloads a single file with proper error handling and progress
func downloadSingleFile(url, filename string) error {
	// Create the data directory if it doesn't exist
	zipDir := "./data/990_zips"
	if err := os.MkdirAll(zipDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	filePath := filepath.Join(zipDir, filename)

	// Check if file already exists and has size > 0
	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		log.Printf("File %s already exists, skipping", filename)
		return nil
	}

	// Download the file with retry
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*3)
	defer cancel()

	res, err := httpGetWithRetry(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer res.Body.Close()
	
	// Create the output file
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()
	
	// Copy the content with progress tracking
	written, err := io.Copy(out, res.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	// Verify the file has content
	if written == 0 {
		return fmt.Errorf("downloaded file is empty")
	}
	
	return nil
}
