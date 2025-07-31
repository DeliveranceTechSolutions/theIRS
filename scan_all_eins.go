package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type EINScanner struct {
	processed int
	found     int
	errors    int
	targetEIN string
}

func main() {
	scanner := &EINScanner{
		targetEIN: "921844425",
	}
	
	dir := "data/990_zips"
	
	fmt.Printf("Scanning for EIN %s in all directories under %s...\n", scanner.targetEIN, dir)
	
	// Walk through all XML files in all subdirectories
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".xml") {
			scanner.processed++
			if scanner.processed%10000 == 0 {
				fmt.Printf("Processed %d files, found %d matches, %d errors\n", 
					scanner.processed, scanner.found, scanner.errors)
			}
			
			if err := scanner.scanFile(path); err != nil {
				scanner.errors++
				if scanner.processed%1000 == 0 { // Only log errors occasionally to avoid spam
					log.Printf("Error scanning %s: %v", path, err)
				}
			}
		}
		
		return nil
	})
	
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("\nScan complete!\n")
	fmt.Printf("Total files processed: %d\n", scanner.processed)
	fmt.Printf("Total matches for EIN %s: %d\n", scanner.targetEIN, scanner.found)
	fmt.Printf("Total errors: %d\n", scanner.errors)
	
	if scanner.found == 0 {
		fmt.Printf("\n❌ EIN %s was NOT found in any of the XML files.\n", scanner.targetEIN)
	} else {
		fmt.Printf("\n✅ EIN %s was found %d times!\n", scanner.targetEIN, scanner.found)
	}
}

func (s *EINScanner) scanFile(filepath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	decoder := xml.NewDecoder(file)
	
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "EIN" {
				var ein string
				if err := decoder.DecodeElement(&ein, &t); err != nil {
					continue
				}
				
				// Check if this EIN matches our target
				if ein == s.targetEIN {
					s.found++
					fmt.Printf("🎯 FOUND EIN %s in file: %s\n", ein, filepath)
				}
			}
		}
	}
	
	return nil
} 