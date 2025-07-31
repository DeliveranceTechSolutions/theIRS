package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"golang.org/x/net/html"
)

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

func ScrapeURLs() {
    var template string
    for year := currentStart; year <= currentYear; year++ {
        counter := 12
        for counter > 0 {
            if year < 2021 {
                template = upTo20 + fmt.Sprintf(`%d/download990xml_%d_%d.zip`, year, year, counter)
            } else {
                template = upTo20 + fmt.Sprintf(`%d/%d_TEOS_XML_%02dA.zip`, year, year, counter)
            }
            fmt.Println(template)
            res, err := http.Get(template)
            if err != nil {
                fmt.Println(err)
            }
            defer res.Body.Close()

            out, err := os.Create(fmt.Sprintf(`%d_%d.zip`, year, counter))
            if err != nil {
                fmt.Println(err)
            }
            defer out.Close()

            _, err = io.Copy(out, res.Body)
            fmt.Println(err)
            counter--
        }
    }
}

func UnpackSchemas() (map[string]Version, error) {
    ledger = make(map[string]Version)
    res, err := http.Get("https://www.irs.gov/charities-non-profits/tax-exempt-organization-search-teos-schemas")
    if err != nil {
        fmt.Println(err)
    }
    defer res.Body.Close()

    doc, err := html.Parse(res.Body)
    if err != nil {
        fmt.Println(err)
    }

    os.Mkdir("./data/990_xsd", 0777)

    var walk func(*html.Node)
    walk = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "a" {
            for _, attr := range n.Attr {
                if attr.Key == "href" {
                    if strings.Contains(attr.Val, ".zip") {
                        fetchSchema(attr.Val)
                    }
                }
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            walk(c)
        }
    }
    walk(doc)
    fmt.Printf("%+v", ledger)
    return ledger, nil
}

func UnpackZips() ([]string, error) {
    res, err := http.Get("https://www.irs.gov/charities-non-profits/form-990-series-downloads")
    if err != nil {
        fmt.Println(err)
    }
    defer res.Body.Close()

    doc, err := html.Parse(res.Body)
    if err != nil {
        fmt.Println(err)
    }

    os.Mkdir(`./data/990_zips/`, 0777) 

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
        zipData = append(zipData, fetchZip(uri))
    }
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


func fetchSchema(uri string) {
    fmt.Println(uri)
    res, err := http.Get(uri)
    if err != nil {
        fmt.Println(err)
    }
    defer res.Body.Close()

    year := splitYear(uri, "schema")
    if year != fileYear {
        fileTracker.Store(0)
        fileYear = year
    }
    fmt.Println(year)
    out, err := os.Create(fmt.Sprintf(`./data/990_xsd/%s`, year))
    if err != nil {
        fmt.Println(err)
    }
    defer out.Close()

    _, err = io.Copy(out, res.Body)
    if err != nil {
        log.Println(err)
    }
}

func fetchZip(uri string) string {
    res, err := http.Get(uri)
    if err != nil {
        fmt.Println(err)
    }
    defer res.Body.Close()

    // Extract the filename from the URL
    urlParts := strings.Split(uri, "/")
    if len(urlParts) == 0 {
        fmt.Println("Invalid URL:", uri)
        return ""
    }
    
    filename := urlParts[len(urlParts)-1]
    
    // Create the full path for the downloaded file
    tracker := fmt.Sprintf(`./data/990_zips/%s`, filename)
    out, err := os.Create(tracker)
    if err != nil {
        fmt.Println(err)
    }
    defer out.Close()

    _, err = io.Copy(out, res.Body)
    if err != nil {
        fmt.Println("Error copying file:", err)
    } else {
        fmt.Printf("Downloaded: %s\n", filename)
    }
    
    return tracker 
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
	res, err := http.Get("https://www.irs.gov/charities-non-profits/form-990-series-downloads")
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
		return fmt.Errorf("file already exists and has content")
	}
	
	// Download the file
	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer res.Body.Close()
	
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %d", res.StatusCode)
	}
	
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
