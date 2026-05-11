package utils

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	// MaxDOCXPages is a soft limit for very large documents
	MaxDOCXPages = 500
)

// DOCXMetadata contains information about a DOCX file
type DOCXMetadata struct {
	PageCount int    // Estimated from content
	WordCount int
	Text      string
}

// ValidateDOCX checks if a file is a valid DOCX by checking ZIP structure
func ValidateDOCX(data []byte) error {
	reader := bytes.NewReader(data)
	zipReader, err := zip.NewReader(reader, int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid DOCX: not a valid ZIP file: %w", err)
	}

	// Check for required DOCX files
	hasContentTypes := false
	hasDocument := false

	for _, file := range zipReader.File {
		if file.Name == "[Content_Types].xml" {
			hasContentTypes = true
		}
		if file.Name == "word/document.xml" {
			hasDocument = true
		}
	}

	if !hasContentTypes {
		return fmt.Errorf("invalid DOCX: missing [Content_Types].xml")
	}
	if !hasDocument {
		return fmt.Errorf("invalid DOCX: missing word/document.xml")
	}

	return nil
}

// ExtractDOCXText extracts text from a DOCX file
func ExtractDOCXText(data []byte) (*DOCXMetadata, error) {
	reader := bytes.NewReader(data)
	zipReader, err := zip.NewReader(reader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open DOCX: %w", err)
	}

	var textBuilder strings.Builder

	// Find and read document.xml
	for _, file := range zipReader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open document.xml: %w", err)
			}
			defer rc.Close()

			content, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("failed to read document.xml: %w", err)
			}

			// Parse XML and extract text
			text := extractTextFromDOCXML(content)
			textBuilder.WriteString(text)
			break
		}
	}

	extractedText := textBuilder.String()
	extractedText = cleanDocumentText(extractedText)

	// Enforce size limit
	if len(extractedText) > MaxExtractedTextSize {
		extractedText = extractedText[:MaxExtractedTextSize] + "\n... [Content truncated]"
	}

	wordCount := countWords(extractedText)

	// Estimate page count (roughly 500 words per page)
	pageCount := (wordCount / 500) + 1
	if pageCount < 1 {
		pageCount = 1
	}

	return &DOCXMetadata{
		PageCount: pageCount,
		WordCount: wordCount,
		Text:      extractedText,
	}, nil
}

// extractTextFromDOCXML parses DOCX XML and extracts text content
func extractTextFromDOCXML(xmlContent []byte) string {
	var textBuilder strings.Builder
	decoder := xml.NewDecoder(bytes.NewReader(xmlContent))

	inParagraph := false
	paragraphText := strings.Builder{}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			// Track paragraph boundaries
			if t.Name.Local == "p" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				inParagraph = true
				paragraphText.Reset()
			}
		case xml.EndElement:
			// End of paragraph - add newline
			if t.Name.Local == "p" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				if inParagraph && paragraphText.Len() > 0 {
					textBuilder.WriteString(paragraphText.String())
					textBuilder.WriteString("\n")
				}
				inParagraph = false
			}
		case xml.CharData:
			// Collect text content
			text := strings.TrimSpace(string(t))
			if text != "" && inParagraph {
				if paragraphText.Len() > 0 {
					paragraphText.WriteString(" ")
				}
				paragraphText.WriteString(text)
			}
		}
	}

	return textBuilder.String()
}

// cleanDocumentText cleans extracted document text
func cleanDocumentText(text string) string {
	// Remove null bytes
	text = strings.ReplaceAll(text, "\x00", "")

	// Normalize multiple newlines to double newlines (paragraph breaks)
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	// Trim
	text = strings.TrimSpace(text)

	return text
}

// GetDOCXPreview returns the first N characters of text as a preview
func GetDOCXPreview(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}

	// Try to break at a word boundary
	preview := text[:maxChars]
	lastSpace := strings.LastIndex(preview, " ")
	if lastSpace > maxChars/2 {
		preview = preview[:lastSpace]
	}

	return preview + "..."
}
