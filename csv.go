package main

import (
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// XMLToCSVProcessor handles converting XML files to CSV format
type XMLToCSVProcessor struct {
	outputFile *os.File
	csvWriter  *csv.Writer
	fieldMap   map[string]int
	header     []string
	mu         sync.Mutex
	processed  atomic.Int64
}

// NewXMLToCSVProcessor creates a new processor
func NewXMLToCSVProcessor(outputPath string) (*XMLToCSVProcessor, error) {
	file, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}

	writer := csv.NewWriter(file)
	
	// Initialize with common IRS 990 fields
	header := []string{
		"FileName",
		"EIN",
		"OrganizationName",
		"TaxYear",
		"ReturnType",
		"TotalRevenue",
		"TotalExpenses",
		"NetAssets",
		"TotalAssets",
		"TotalLiabilities",
		"ProgramServiceRevenue",
		"InvestmentIncome",
		"Contributions",
		"Grants",
		"Salaries",
		"ProfessionalFees",
		"Occupancy",
		"OtherExpenses",
		"AddressLine1",
		"AddressLine2",
		"City",
		"State",
		"ZIPCode",
		"Country",
		"Phone",
		"Website",
		"Mission",
		"PrimaryExemptPurpose",
		"OfficerCompensation",
		"EmployeeCompensation",
		"IndependentContractorCompensation",
		"TotalCompensation",
		"BoardMembers",
		"Volunteers",
		"Employees",
		"TotalIndividuals",
		"PoliticalCampaignActivity",
		"LobbyingActivity",
		"ForeignActivities",
		"ForeignAddress",
		"ForeignIncome",
		"ForeignExpenses",
		"RelatedOrganizations",
		"Subsidiaries",
		"JointVentures",
		"Partnerships",
		"UnrelatedBusinessIncome",
		"UnrelatedBusinessExpenses",
		"NetUnrelatedBusinessIncome",
		"ExcessBenefitTransactions",
		"LoansToOfficers",
		"LoansFromOfficers",
		"BusinessTransactions",
		"GrantsToOrganizations",
		"GrantsToIndividuals",
		"TotalGrants",
		"AssetsBOY",
		"AssetsEOY",
		"LiabilitiesBOY",
		"LiabilitiesEOY",
		"NetAssetsBOY",
		"NetAssetsEOY",
		"CashBOY",
		"CashEOY",
		"InvestmentsBOY",
		"InvestmentsEOY",
		"LandBOY",
		"LandEOY",
		"BuildingsBOY",
		"BuildingsEOY",
		"EquipmentBOY",
		"EquipmentEOY",
		"OtherAssetsBOY",
		"OtherAssetsEOY",
		"AccountsPayableBOY",
		"AccountsPayableEOY",
		"GrantsPayableBOY",
		"GrantsPayableEOY",
		"OtherLiabilitiesBOY",
		"OtherLiabilitiesEOY",
		"MortgagesBOY",
		"MortgagesEOY",
		"NotesPayableBOY",
		"NotesPayableEOY",
		"BondsBOY",
		"BondsEOY",
		"OtherDebtBOY",
		"OtherDebtEOY",
		"TotalDebtBOY",
		"TotalDebtEOY",
		"RevenueFromGovernment",
		"RevenueFromContributions",
		"RevenueFromProgramServices",
		"RevenueFromInvestment",
		"RevenueFromOther",
		"ExpensesForProgramServices",
		"ExpensesForManagement",
		"ExpensesForFundraising",
		"NetIncome",
		"FilingDate",
		"TaxPeriodBegin",
		"TaxPeriodEnd",
		"FormVersion",
		"SoftwareID",
		"SoftwareVersion",
		"PreparerName",
		"PreparerFirm",
		"PreparerAddress",
		"PreparerPhone",
		"PreparerEmail",
		"SignatureDate",
		"SignatureName",
		"SignatureTitle",
		"AmendedReturn",
		"InitialReturn",
		"FinalReturn",
		"Terminated",
		"DisasterRelief",
		"ElectronicFiling",
		"PaperFiling",
		"ExtensionFiled",
		"ExtensionGranted",
		"ExtensionExpiration",
		"PublicInspection",
		"ScheduleA",
		"ScheduleB",
		"ScheduleC",
		"ScheduleD",
		"ScheduleE",
		"ScheduleF",
		"ScheduleG",
		"ScheduleH",
		"ScheduleI",
		"ScheduleJ",
		"ScheduleK",
		"ScheduleL",
		"ScheduleM",
		"ScheduleN",
		"ScheduleO",
		"ScheduleR",
		"AdditionalData",
	}

	// Create field map for quick lookup
	fieldMap := make(map[string]int)
	for i, field := range header {
		fieldMap[field] = i
	}

	// Write header
	if err := writer.Write(header); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to write header: %w", err)
	}
	writer.Flush()

	return &XMLToCSVProcessor{
		outputFile: file,
		csvWriter:  writer,
		fieldMap:   fieldMap,
		header:     header,
	}, nil
}

// Close closes the processor and flushes data
func (p *XMLToCSVProcessor) Close() error {
	p.csvWriter.Flush()
	return p.outputFile.Close()
}

// ProcessDirectory processes all XML files in a directory
func (p *XMLToCSVProcessor) ProcessDirectory(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, runtime.NumCPU()*2) // Limit concurrent processing

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".xml") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			if err := p.processXMLFile(path); err != nil {
				log.Printf("Error processing %s: %v", path, err)
			}
		}(filePath)
	}

	wg.Wait()
	return nil
}

// processXMLFile processes a single XML file
func (p *XMLToCSVProcessor) processXMLFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Initialize record with empty strings
	record := make([]string, len(p.header))
	for i := range record {
		record[i] = ""
	}

	// Set filename
	record[p.fieldMap["FileName"]] = filepath.Base(filePath)

	// Parse XML and extract data
	decoder := xml.NewDecoder(file)
	if err := p.extractXMLData(decoder, record); err != nil {
		return fmt.Errorf("failed to parse XML: %w", err)
	}

	// Write record to CSV
	p.mu.Lock()
	if err := p.csvWriter.Write(record); err != nil {
		p.mu.Unlock()
		return fmt.Errorf("failed to write record: %w", err)
	}
	p.mu.Unlock()

	// Increment counter
	processed := p.processed.Add(1)
	if processed%1000 == 0 {
		log.Printf("Processed %d files", processed)
	}

	return nil
}

// extractXMLData extracts relevant data from XML and populates the record
func (p *XMLToCSVProcessor) extractXMLData(decoder *xml.Decoder, record []string) error {
	var pathStack []string
	var currentText string
	var inElement bool

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error parsing XML: %v", err)
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			pathStack = append(pathStack, t.Name.Local)
			inElement = true
			currentText = ""

		case xml.CharData:
			if inElement {
				currentText += string(t)
			}

		case xml.EndElement:
			if inElement {
				text := strings.TrimSpace(currentText)
				if text != "" {
					fullPath := strings.Join(pathStack, ".")
					p.mapFieldToRecord(fullPath, text, record)
				}
			}
			if len(pathStack) > 0 {
				pathStack = pathStack[:len(pathStack)-1]
			}
			inElement = false
		}
	}

	return nil
}

// mapFieldToRecord maps XML data to CSV record fields
func (p *XMLToCSVProcessor) mapFieldToRecord(path, value string, record []string) {
	// Direct field mappings based on actual IRS 990 XML structure
	fieldMappings := map[string]string{
		"Return.ReturnHeader.Filer.EIN": "EIN",
		"Return.ReturnHeader.Filer.BusinessName.BusinessNameLine1Txt": "OrganizationName",
		"Return.ReturnHeader.TaxYr": "TaxYear",
		"Return.ReturnHeader.ReturnTypeCd": "ReturnType",
		"Return.ReturnHeader.Filer.USAddress.AddressLine1Txt": "AddressLine1",
		"Return.ReturnHeader.Filer.USAddress.AddressLine2Txt": "AddressLine2",
		"Return.ReturnHeader.Filer.USAddress.CityNm": "City",
		"Return.ReturnHeader.Filer.USAddress.StateAbbreviationCd": "State",
		"Return.ReturnHeader.Filer.USAddress.ZIPCd": "ZIPCode",
		"Return.ReturnHeader.Filer.PhoneNum": "Phone",
		"Return.ReturnHeader.ReturnTs": "FilingDate",
		"Return.ReturnHeader.TaxPeriodBeginDt": "TaxPeriodBegin",
		"Return.ReturnHeader.TaxPeriodEndDt": "TaxPeriodEnd",
		"Return.ReturnHeader.PreparerPersonGrp.PreparerPersonNm": "PreparerName",
		"Return.ReturnHeader.PreparerFirmGrp.PreparerFirmName.BusinessNameLine1Txt": "PreparerFirm",
		"Return.ReturnHeader.BusinessOfficerGrp.PersonNm": "SignatureName",
		"Return.ReturnHeader.BusinessOfficerGrp.PersonTitleTxt": "SignatureTitle",
		"Return.ReturnHeader.BusinessOfficerGrp.SignatureDt": "SignatureDate",
		
		// Financial data mappings
		"ReturnData.IRS990.CYTotalRevenueAmt": "TotalRevenue",
		"ReturnData.IRS990.CYTotalExpensesAmt": "TotalExpenses",
		"ReturnData.IRS990.TotalAssetsGrp.EOYAmt": "TotalAssets",
		"ReturnData.IRS990.TotalLiabilitiesGrp.EOYAmt": "TotalLiabilities",
		"ReturnData.IRS990.CYProgramServiceRevenueAmt": "ProgramServiceRevenue",
		"ReturnData.IRS990.CYInvestmentIncomeAmt": "InvestmentIncome",
		"ReturnData.IRS990.CYContributionsGrantsAmt": "Contributions",
		"ReturnData.IRS990.CYGrantsAndSimilarPaidAmt": "Grants",
		"ReturnData.IRS990.CYSalariesCompEmpBnftPaidAmt": "Salaries",
		"ReturnData.IRS990.CYOtherExpensesAmt": "OtherExpenses",
		"ReturnData.IRS990.TotalAssetsBOYAmt": "AssetsBOY",
		"ReturnData.IRS990.TotalAssetsEOYAmt": "AssetsEOY",
		"ReturnData.IRS990.TotalLiabilitiesBOYAmt": "LiabilitiesBOY",
		"ReturnData.IRS990.TotalLiabilitiesEOYAmt": "LiabilitiesEOY",
		"ReturnData.IRS990.NetAssetsOrFundBalancesBOYAmt": "NetAssetsBOY",
		"ReturnData.IRS990.NetAssetsOrFundBalancesEOYAmt": "NetAssetsEOY",
		"ReturnData.IRS990.MissionDesc": "Mission",
		"ReturnData.IRS990.TotalProgramServiceExpensesAmt": "ExpensesForProgramServices",
	}

	// Check for direct mapping
	if field, exists := fieldMappings[path]; exists {
		if idx, ok := p.fieldMap[field]; ok {
			record[idx] = value
		}
		return
	}

	// Check for partial matches and common patterns
	lowerPath := strings.ToLower(path)

	// Revenue patterns
	if strings.Contains(lowerPath, "revenue") || strings.Contains(lowerPath, "income") {
		if strings.Contains(lowerPath, "total") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["TotalRevenue"]; ok && record[idx] == "" {
				record[idx] = value
			}
		} else if strings.Contains(lowerPath, "program") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["ProgramServiceRevenue"]; ok && record[idx] == "" {
				record[idx] = value
			}
		} else if strings.Contains(lowerPath, "investment") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["InvestmentIncome"]; ok && record[idx] == "" {
				record[idx] = value
			}
		} else if strings.Contains(lowerPath, "contribution") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["Contributions"]; ok && record[idx] == "" {
				record[idx] = value
			}
		}
	}

	// Expense patterns
	if strings.Contains(lowerPath, "expense") || strings.Contains(lowerPath, "cost") {
		if strings.Contains(lowerPath, "total") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["TotalExpenses"]; ok && record[idx] == "" {
				record[idx] = value
			}
		} else if strings.Contains(lowerPath, "program") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["ExpensesForProgramServices"]; ok && record[idx] == "" {
				record[idx] = value
			}
		} else if strings.Contains(lowerPath, "management") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["ExpensesForManagement"]; ok && record[idx] == "" {
				record[idx] = value
			}
		} else if strings.Contains(lowerPath, "fundraising") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["ExpensesForFundraising"]; ok && record[idx] == "" {
				record[idx] = value
			}
		}
	}

	// Asset patterns
	if strings.Contains(lowerPath, "asset") {
		if strings.Contains(lowerPath, "total") && strings.Contains(lowerPath, "amt") {
			if strings.Contains(lowerPath, "boy") {
				if idx, ok := p.fieldMap["AssetsBOY"]; ok && record[idx] == "" {
					record[idx] = value
				}
			} else if strings.Contains(lowerPath, "eoy") {
				if idx, ok := p.fieldMap["AssetsEOY"]; ok && record[idx] == "" {
					record[idx] = value
				}
			} else {
				if idx, ok := p.fieldMap["TotalAssets"]; ok && record[idx] == "" {
					record[idx] = value
				}
			}
		} else if strings.Contains(lowerPath, "net") && strings.Contains(lowerPath, "amt") {
			if strings.Contains(lowerPath, "boy") {
				if idx, ok := p.fieldMap["NetAssetsBOY"]; ok && record[idx] == "" {
					record[idx] = value
				}
			} else if strings.Contains(lowerPath, "eoy") {
				if idx, ok := p.fieldMap["NetAssetsEOY"]; ok && record[idx] == "" {
					record[idx] = value
				}
			} else {
				if idx, ok := p.fieldMap["NetAssets"]; ok && record[idx] == "" {
					record[idx] = value
				}
			}
		}
	}

	// Liability patterns
	if strings.Contains(lowerPath, "liability") {
		if strings.Contains(lowerPath, "total") && strings.Contains(lowerPath, "amt") {
			if strings.Contains(lowerPath, "boy") {
				if idx, ok := p.fieldMap["LiabilitiesBOY"]; ok && record[idx] == "" {
					record[idx] = value
				}
			} else if strings.Contains(lowerPath, "eoy") {
				if idx, ok := p.fieldMap["LiabilitiesEOY"]; ok && record[idx] == "" {
					record[idx] = value
				}
			} else {
				if idx, ok := p.fieldMap["TotalLiabilities"]; ok && record[idx] == "" {
					record[idx] = value
				}
			}
		}
	}

	// Compensation patterns
	if strings.Contains(lowerPath, "compensation") || strings.Contains(lowerPath, "salary") {
		if strings.Contains(lowerPath, "officer") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["OfficerCompensation"]; ok && record[idx] == "" {
				record[idx] = value
			}
		} else if strings.Contains(lowerPath, "employee") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["EmployeeCompensation"]; ok && record[idx] == "" {
				record[idx] = value
			}
		} else if strings.Contains(lowerPath, "total") && strings.Contains(lowerPath, "amt") {
			if idx, ok := p.fieldMap["TotalCompensation"]; ok && record[idx] == "" {
				record[idx] = value
			}
		}
	}

	// Boolean indicators
	if value == "true" || value == "1" || value == "X" {
		if strings.Contains(lowerPath, "amended") {
			if idx, ok := p.fieldMap["AmendedReturn"]; ok {
				record[idx] = "Yes"
			}
		} else if strings.Contains(lowerPath, "initial") {
			if idx, ok := p.fieldMap["InitialReturn"]; ok {
				record[idx] = "Yes"
			}
		} else if strings.Contains(lowerPath, "final") {
			if idx, ok := p.fieldMap["FinalReturn"]; ok {
				record[idx] = "Yes"
			}
		} else if strings.Contains(lowerPath, "terminated") {
			if idx, ok := p.fieldMap["Terminated"]; ok {
				record[idx] = "Yes"
			}
		} else if strings.Contains(lowerPath, "electronic") {
			if idx, ok := p.fieldMap["ElectronicFiling"]; ok {
				record[idx] = "Yes"
			}
		}
	}
}

// ProcessAllDirectories processes all extracted directories
func ProcessAllDirectories() error {
	processor, err := NewXMLToCSVProcessor("irs_990_data.csv")
	if err != nil {
		return fmt.Errorf("failed to create processor: %w", err)
	}
	defer processor.Close()

	baseDir := "data/990_zips"
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("failed to read base directory: %w", err)
	}

	// Process each directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(baseDir, entry.Name())
		log.Printf("Processing directory: %s", dirPath)
		
		if err := processor.ProcessDirectory(dirPath); err != nil {
			log.Printf("Error processing directory %s: %v", dirPath, err)
			continue
		}
	}

	log.Printf("Processing complete. Total files processed: %d", processor.processed.Load())
	return nil
} 