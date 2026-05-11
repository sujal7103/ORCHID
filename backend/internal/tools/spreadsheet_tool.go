package tools

import (
	"clara-agents/internal/filecache"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// NewReadSpreadsheetTool creates a tool for reading Excel/CSV/spreadsheet files
func NewReadSpreadsheetTool() *Tool {
	return &Tool{
		Name:        "read_spreadsheet",
		DisplayName: "Read Spreadsheet",
		Description: `Reads and parses spreadsheet files (Excel, CSV, TSV). Returns structured data with headers and rows.

SUPPORTED FORMATS:
- .xlsx (Excel 2007+, OpenXML) ✓
- .xls (Legacy Excel) - converted to text
- .csv (Comma-separated values) ✓
- .tsv (Tab-separated values) ✓

USE THIS TOOL when user uploads:
- Excel files (.xlsx, .xls)
- CSV/TSV data files
- Any spreadsheet for analysis

Returns: headers, rows (as arrays), row count, and sheet names for Excel files.`,
		Icon: "FileSpreadsheet",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_id": map[string]interface{}{
					"type":        "string",
					"description": "The file ID of the uploaded spreadsheet (from upload response)",
				},
				"sheet_name": map[string]interface{}{
					"type":        "string",
					"description": "For Excel files: specific sheet name to read. If not provided, reads the first/active sheet.",
				},
				"max_rows": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of data rows to return (default: 200, max: 1000). Headers are always included.",
				},
				"include_empty": map[string]interface{}{
					"type":        "boolean",
					"description": "Include empty rows in output (default: false)",
				},
			},
			"required": []string{"file_id"},
		},
		Execute:  executeReadSpreadsheet,
		Source:   ToolSourceBuiltin,
		Category: "data_sources",
		Keywords: []string{"excel", "xlsx", "xls", "csv", "tsv", "spreadsheet", "sheet", "workbook", "data", "table", "read", "parse"},
	}
}

func executeReadSpreadsheet(args map[string]interface{}) (string, error) {
	// Extract parameters
	fileID, ok := args["file_id"].(string)
	if !ok || fileID == "" {
		return "", fmt.Errorf("file_id parameter is required")
	}

	sheetName := ""
	if sn, ok := args["sheet_name"].(string); ok {
		sheetName = sn
	}

	maxRows := 200
	if mr, ok := args["max_rows"].(float64); ok {
		maxRows = int(mr)
		if maxRows > 1000 {
			maxRows = 1000
		}
		if maxRows < 1 {
			maxRows = 1
		}
	}

	includeEmpty := false
	if ie, ok := args["include_empty"].(bool); ok {
		includeEmpty = ie
	}

	// Extract user context
	userID, _ := args["__user_id__"].(string)
	conversationID, _ := args["__conversation_id__"].(string)
	delete(args, "__user_id__")
	delete(args, "__conversation_id__")

	log.Printf("📊 [READ-SPREADSHEET] Reading file_id=%s sheet=%s maxRows=%d (user=%s)", fileID, sheetName, maxRows, userID)

	// Get file from cache
	fileCacheService := filecache.GetService()
	var file *filecache.CachedFile
	var err error

	if userID != "" && conversationID != "" {
		file, err = fileCacheService.GetByUserAndConversation(fileID, userID, conversationID)
		if err != nil {
			file, _ = fileCacheService.Get(fileID)
			if file != nil && file.UserID != userID {
				return "", fmt.Errorf("access denied: you don't have permission to read this file")
			}
		}
	} else {
		file, _ = fileCacheService.Get(fileID)
	}

	if file == nil {
		return "", fmt.Errorf("file not found or expired. Files are available for 30 minutes after upload")
	}

	// Determine file type
	filename := strings.ToLower(file.Filename)
	mimeType := strings.ToLower(file.MimeType)

	var result *SpreadsheetResult

	switch {
	case strings.HasSuffix(filename, ".xlsx") ||
		mimeType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		result, err = readXLSX(file, sheetName, maxRows, includeEmpty)

	case strings.HasSuffix(filename, ".xls") ||
		mimeType == "application/vnd.ms-excel":
		result, err = readXLS(file, maxRows)

	case strings.HasSuffix(filename, ".csv") ||
		mimeType == "text/csv":
		result, err = readCSV(file, maxRows, ',', includeEmpty)

	case strings.HasSuffix(filename, ".tsv") ||
		mimeType == "text/tab-separated-values":
		result, err = readCSV(file, maxRows, '\t', includeEmpty)

	default:
		return "", fmt.Errorf("unsupported file type: %s. Supported: .xlsx, .xls, .csv, .tsv", file.MimeType)
	}

	if err != nil {
		log.Printf("❌ [READ-SPREADSHEET] Failed to read %s: %v", file.Filename, err)
		return "", err
	}

	// Build response
	response := map[string]interface{}{
		"success":          true,
		"file_id":          file.FileID,
		"filename":         file.Filename,
		"file_type":        result.FileType,
		"headers":          result.Headers,
		"column_count":     len(result.Headers),
		"rows":             result.Rows,
		"row_count":        len(result.Rows),
		"total_rows":       result.TotalRows,
		"truncated":        result.Truncated,
		"sheet_name":       result.SheetName,
		"available_sheets": result.SheetNames,
	}

	// Add preview text for LLM context
	preview := generatePreview(result)
	response["preview"] = preview

	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	log.Printf("✅ [READ-SPREADSHEET] Read %s: %d columns, %d rows (sheet: %s)",
		file.Filename, len(result.Headers), len(result.Rows), result.SheetName)

	return string(responseJSON), nil
}

// SpreadsheetResult holds parsed spreadsheet data
type SpreadsheetResult struct {
	FileType   string
	Headers    []string
	Rows       [][]string
	TotalRows  int
	Truncated  bool
	SheetName  string
	SheetNames []string
}

// readXLSX reads .xlsx files using excelize
func readXLSX(file *filecache.CachedFile, sheetName string, maxRows int, includeEmpty bool) (*SpreadsheetResult, error) {
	if file.FilePath == "" {
		return nil, fmt.Errorf("file content not available")
	}

	// Open Excel file
	f, err := excelize.OpenFile(file.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Get sheet names
	sheetNames := f.GetSheetList()
	if len(sheetNames) == 0 {
		return nil, fmt.Errorf("no sheets found in workbook")
	}

	// Select sheet
	targetSheet := sheetName
	if targetSheet == "" {
		// Use active sheet or first sheet
		targetSheet = f.GetSheetName(f.GetActiveSheetIndex())
		if targetSheet == "" {
			targetSheet = sheetNames[0]
		}
	}

	// Verify sheet exists
	sheetExists := false
	for _, s := range sheetNames {
		if s == targetSheet {
			sheetExists = true
			break
		}
	}
	if !sheetExists {
		return nil, fmt.Errorf("sheet '%s' not found. Available sheets: %v", targetSheet, sheetNames)
	}

	// Read all rows
	rows, err := f.GetRows(targetSheet)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet '%s': %w", targetSheet, err)
	}

	if len(rows) == 0 {
		return &SpreadsheetResult{
			FileType:   "xlsx",
			Headers:    []string{},
			Rows:       [][]string{},
			TotalRows:  0,
			SheetName:  targetSheet,
			SheetNames: sheetNames,
		}, nil
	}

	// First row is headers
	headers := rows[0]

	// Normalize headers (trim whitespace, handle empty)
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
		if headers[i] == "" {
			headers[i] = fmt.Sprintf("Column_%d", i+1)
		}
	}

	// Process data rows
	dataRows := make([][]string, 0, len(rows)-1)
	totalRows := 0

	for i := 1; i < len(rows); i++ {
		row := rows[i]

		// Skip empty rows unless requested
		if !includeEmpty && isEmptyRow(row) {
			continue
		}

		totalRows++

		// Pad row to match header count
		for len(row) < len(headers) {
			row = append(row, "")
		}
		// Trim row if longer than headers
		if len(row) > len(headers) {
			row = row[:len(headers)]
		}

		// Trim cell values
		for j := range row {
			row[j] = strings.TrimSpace(row[j])
		}

		if len(dataRows) < maxRows {
			dataRows = append(dataRows, row)
		}
	}

	return &SpreadsheetResult{
		FileType:   "xlsx",
		Headers:    headers,
		Rows:       dataRows,
		TotalRows:  totalRows,
		Truncated:  totalRows > maxRows,
		SheetName:  targetSheet,
		SheetNames: sheetNames,
	}, nil
}

// readXLS handles legacy .xls files
func readXLS(file *filecache.CachedFile, maxRows int) (*SpreadsheetResult, error) {
	// For .xls files, try to read as text or return helpful error
	// Note: Full .xls support would require additional library like github.com/extrame/xls

	if file.FilePath == "" {
		return nil, fmt.Errorf("file content not available")
	}

	// Read raw content
	content, err := os.ReadFile(file.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Try to extract text content (basic approach)
	// .xls is a binary format, but we can try to extract readable strings
	text := extractTextFromBinary(content)

	if text == "" {
		return nil, fmt.Errorf("legacy .xls format detected. Please convert to .xlsx or .csv for full support. You can do this in Excel: File > Save As > Excel Workbook (.xlsx)")
	}

	// Return extracted text as single column
	lines := strings.Split(text, "\n")
	headers := []string{"Extracted_Content"}
	rows := make([][]string, 0)

	for i, line := range lines {
		if i >= maxRows {
			break
		}
		line = strings.TrimSpace(line)
		if line != "" {
			rows = append(rows, []string{line})
		}
	}

	return &SpreadsheetResult{
		FileType:   "xls",
		Headers:    headers,
		Rows:       rows,
		TotalRows:  len(rows),
		Truncated:  len(lines) > maxRows,
		SheetName:  "Sheet1",
		SheetNames: []string{"Sheet1"},
	}, nil
}

// readCSV reads CSV/TSV files
func readCSV(file *filecache.CachedFile, maxRows int, delimiter rune, includeEmpty bool) (*SpreadsheetResult, error) {
	var content string

	if file.FilePath != "" {
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		content = string(data)
	} else if file.ExtractedText != nil {
		content = file.ExtractedText.String()
	} else {
		return nil, fmt.Errorf("file content not available")
	}

	// Parse CSV
	reader := csv.NewReader(strings.NewReader(content))
	reader.Comma = delimiter
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) == 0 {
		return &SpreadsheetResult{
			FileType:   "csv",
			Headers:    []string{},
			Rows:       [][]string{},
			TotalRows:  0,
			SheetName:  filepath.Base(file.Filename),
			SheetNames: []string{filepath.Base(file.Filename)},
		}, nil
	}

	// First row is headers
	headers := records[0]
	for i, h := range headers {
		headers[i] = strings.TrimSpace(h)
		if headers[i] == "" {
			headers[i] = fmt.Sprintf("Column_%d", i+1)
		}
	}

	// Process data rows
	dataRows := make([][]string, 0, len(records)-1)
	totalRows := 0

	for i := 1; i < len(records); i++ {
		row := records[i]

		if !includeEmpty && isEmptyRow(row) {
			continue
		}

		totalRows++

		// Normalize row length
		for len(row) < len(headers) {
			row = append(row, "")
		}
		if len(row) > len(headers) {
			row = row[:len(headers)]
		}

		for j := range row {
			row[j] = strings.TrimSpace(row[j])
		}

		if len(dataRows) < maxRows {
			dataRows = append(dataRows, row)
		}
	}

	fileType := "csv"
	if delimiter == '\t' {
		fileType = "tsv"
	}

	return &SpreadsheetResult{
		FileType:   fileType,
		Headers:    headers,
		Rows:       dataRows,
		TotalRows:  totalRows,
		Truncated:  totalRows > maxRows,
		SheetName:  filepath.Base(file.Filename),
		SheetNames: []string{filepath.Base(file.Filename)},
	}, nil
}

// isEmptyRow checks if a row has no content
func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// extractTextFromBinary tries to extract readable text from binary files
func extractTextFromBinary(data []byte) string {
	var builder strings.Builder
	var current strings.Builder

	for _, b := range data {
		// Keep printable ASCII and common whitespace
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			current.WriteByte(b)
		} else {
			// If we have accumulated text, write it
			text := strings.TrimSpace(current.String())
			if len(text) > 3 { // Only keep strings longer than 3 chars
				if builder.Len() > 0 {
					builder.WriteString("\n")
				}
				builder.WriteString(text)
			}
			current.Reset()
		}
	}

	// Don't forget the last segment
	text := strings.TrimSpace(current.String())
	if len(text) > 3 {
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(text)
	}

	return builder.String()
}

// generatePreview creates a text preview of the data for LLM context
func generatePreview(result *SpreadsheetResult) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("=== %s Data Preview ===\n", strings.ToUpper(result.FileType)))
	builder.WriteString(fmt.Sprintf("Sheet: %s | Columns: %d | Rows: %d", result.SheetName, len(result.Headers), result.TotalRows))
	if result.Truncated {
		builder.WriteString(fmt.Sprintf(" (showing first %d)", len(result.Rows)))
	}
	builder.WriteString("\n\n")

	// Headers
	builder.WriteString("Headers: ")
	builder.WriteString(strings.Join(result.Headers, " | "))
	builder.WriteString("\n\n")

	// Sample rows (first 5)
	sampleCount := 5
	if len(result.Rows) < sampleCount {
		sampleCount = len(result.Rows)
	}

	if sampleCount > 0 {
		builder.WriteString("Sample data:\n")
		for i := 0; i < sampleCount; i++ {
			builder.WriteString(fmt.Sprintf("  Row %d: %s\n", i+1, strings.Join(result.Rows[i], " | ")))
		}
	}

	return builder.String()
}
