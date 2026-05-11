package tools

import (
	"clara-agents/internal/filecache"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// NewReadDataFileTool creates the read_data_file tool for reading CSV/JSON/text files
func NewReadDataFileTool() *Tool {
	return &Tool{
		Name:        "read_data_file",
		DisplayName: "Read Data File",
		Description: "Reads the content of CSV, JSON, Excel, or text files that were uploaded. Returns the file content in the specified format.",
		Icon:        "FileSpreadsheet",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file_id": map[string]interface{}{
					"type":        "string",
					"description": "The file ID of the uploaded data file (from the upload response)",
				},
				"format": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"raw", "json", "csv_rows"},
					"description": "Output format: 'raw' returns text as-is, 'json' parses JSON files, 'csv_rows' returns CSV as array of rows. Default is 'raw'",
				},
				"max_rows": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of rows to return for CSV files (default 100, max 500)",
				},
			},
			"required": []string{"file_id"},
		},
		Execute:  executeReadDataFile,
		Source:   ToolSourceBuiltin,
		Category: "data_sources",
		Keywords: []string{"csv", "json", "excel", "text", "data", "read", "file", "spreadsheet", "txt", "content"},
	}
}

func executeReadDataFile(args map[string]interface{}) (string, error) {
	// Extract file_id parameter
	fileID, ok := args["file_id"].(string)
	if !ok || fileID == "" {
		return "", fmt.Errorf("file_id parameter is required and must be a string")
	}

	// Extract format parameter (default to "raw")
	format := "raw"
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}

	// Extract max_rows parameter (default 100, max 500)
	maxRows := 100
	if mr, ok := args["max_rows"].(float64); ok {
		maxRows = int(mr)
		if maxRows > 500 {
			maxRows = 500
		}
		if maxRows < 1 {
			maxRows = 1
		}
	}

	// Extract user context (injected by tool executor)
	userID, _ := args["__user_id__"].(string)
	conversationID, _ := args["__conversation_id__"].(string)

	// Clean up internal parameters
	delete(args, "__user_id__")
	delete(args, "__conversation_id__")

	log.Printf("📊 [READ-DATA-FILE] Reading data file_id=%s format=%s (user=%s)", fileID, format, userID)

	// Get file cache service
	fileCacheService := filecache.GetService()

	// Try to get file with ownership validation
	var file *filecache.CachedFile
	var err error

	if userID != "" && conversationID != "" {
		file, err = fileCacheService.GetByUserAndConversation(fileID, userID, conversationID)
		if err != nil {
			// Try just by user (conversation might not match in workflow context)
			file, _ = fileCacheService.Get(fileID)
			if file != nil && file.UserID != userID {
				log.Printf("🚫 [READ-DATA-FILE] Access denied: file %s belongs to different user", fileID)
				return "", fmt.Errorf("access denied: you don't have permission to read this file")
			}
		}
	} else {
		// Fallback: get file without strict ownership check (for workflow context)
		file, _ = fileCacheService.Get(fileID)
	}

	if file == nil {
		log.Printf("❌ [READ-DATA-FILE] File not found: %s", fileID)
		return "", fmt.Errorf("file not found or has expired. Files are only available for 30 minutes after upload")
	}

	// Validate file type is a data file
	supportedTypes := map[string]string{
		"text/csv":                          "csv",
		"application/json":                  "json",
		"text/plain":                        "text",
		"application/vnd.ms-excel":          "excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": "excel",
		"text/tab-separated-values": "csv",
	}

	fileType, supported := supportedTypes[file.MimeType]
	if !supported {
		// Check by extension if mime type not recognized
		if strings.HasSuffix(strings.ToLower(file.Filename), ".csv") {
			fileType = "csv"
		} else if strings.HasSuffix(strings.ToLower(file.Filename), ".json") {
			fileType = "json"
		} else if strings.HasSuffix(strings.ToLower(file.Filename), ".txt") {
			fileType = "text"
		} else {
			log.Printf("⚠️ [READ-DATA-FILE] Unsupported file type: %s", file.MimeType)
			return "", fmt.Errorf("unsupported file type: %s. Use read_document for PDF/DOCX/PPTX files", file.MimeType)
		}
	}

	// Read file content
	var content string
	if file.FilePath != "" {
		// File stored on disk (images, CSV, etc.)
		data, err := os.ReadFile(file.FilePath)
		if err != nil {
			log.Printf("❌ [READ-DATA-FILE] Failed to read file from disk: %v", err)
			return "", fmt.Errorf("failed to read file: %v", err)
		}
		content = string(data)
	} else if file.ExtractedText != nil {
		// File stored in memory (PDFs)
		content = file.ExtractedText.String()
	} else {
		log.Printf("⚠️ [READ-DATA-FILE] No content available for file: %s", fileID)
		return "", fmt.Errorf("no content available for this file")
	}

	// Process based on format and file type
	var response map[string]interface{}

	switch format {
	case "json":
		if fileType == "json" {
			// Parse and return JSON
			var jsonData interface{}
			if err := json.Unmarshal([]byte(content), &jsonData); err != nil {
				return "", fmt.Errorf("failed to parse JSON: %v", err)
			}
			response = map[string]interface{}{
				"success":   true,
				"file_id":   file.FileID,
				"filename":  file.Filename,
				"mime_type": file.MimeType,
				"size":      file.Size,
				"format":    "json",
				"data":      jsonData,
			}
		} else {
			return "", fmt.Errorf("json format only supported for JSON files, this is a %s file", fileType)
		}

	case "csv_rows":
		if fileType == "csv" {
			// Parse CSV and return as rows
			reader := csv.NewReader(strings.NewReader(content))
			records, err := reader.ReadAll()
			if err != nil {
				return "", fmt.Errorf("failed to parse CSV: %v", err)
			}

			// Limit rows
			totalRows := len(records)
			if len(records) > maxRows {
				records = records[:maxRows]
			}

			// Extract headers and data
			var headers []string
			var rows [][]string
			if len(records) > 0 {
				headers = records[0]
				if len(records) > 1 {
					rows = records[1:]
				}
			}

			response = map[string]interface{}{
				"success":    true,
				"file_id":    file.FileID,
				"filename":   file.Filename,
				"mime_type":  file.MimeType,
				"size":       file.Size,
				"format":     "csv_rows",
				"headers":    headers,
				"rows":       rows,
				"total_rows": totalRows - 1, // Exclude header
				"returned_rows": len(rows),
				"truncated":  totalRows > maxRows,
			}
		} else {
			return "", fmt.Errorf("csv_rows format only supported for CSV files, this is a %s file", fileType)
		}

	default: // "raw"
		// Return raw content
		// Truncate if too large for LLM context (100KB limit)
		truncated := false
		if len(content) > 100000 {
			content = content[:100000]
			truncated = true
		}

		response = map[string]interface{}{
			"success":   true,
			"file_id":   file.FileID,
			"filename":  file.Filename,
			"mime_type": file.MimeType,
			"size":      file.Size,
			"format":    "raw",
			"content":   content,
			"truncated": truncated,
		}
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	log.Printf("✅ [READ-DATA-FILE] Successfully read data file %s (format=%s)", file.Filename, format)

	return string(responseJSON), nil
}
