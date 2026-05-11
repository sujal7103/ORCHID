package utils

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

const (
	// MaxPDFPages limits the number of pages to process
	MaxPDFPages = 100

	// MaxExtractedTextSize limits the extracted text size (1MB)
	MaxExtractedTextSize = 1024 * 1024
)

// PDFMetadata contains information about a PDF
type PDFMetadata struct {
	PageCount int
	WordCount int
	Text      string
}

// ValidatePDF checks if a file is a valid PDF by attempting to open it
func ValidatePDF(data []byte) error {
	reader := bytes.NewReader(data)
	_, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid PDF: %w", err)
	}
	return nil
}

// ExtractPDFText extracts text from a PDF file (provided as byte data)
func ExtractPDFText(data []byte) (*PDFMetadata, error) {
	// Create reader from byte data
	reader := bytes.NewReader(data)
	pdfReader, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}

	// Check page count
	totalPages := pdfReader.NumPage()
	if totalPages == 0 {
		return nil, fmt.Errorf("PDF has no pages")
	}

	// Limit page count for security
	if totalPages > MaxPDFPages {
		return nil, fmt.Errorf("PDF has too many pages (%d), max allowed is %d", totalPages, MaxPDFPages)
	}

	var textBuilder strings.Builder
	wordCount := 0

	// Extract text from each page
	for pageNum := 1; pageNum <= totalPages; pageNum++ {
		page := pdfReader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		// Get text content
		text, err := page.GetPlainText(nil)
		if err != nil {
			// Skip pages with extraction errors, don't fail completely
			continue
		}

		// Clean and add text
		cleaned := cleanPDFText(text)
		if cleaned != "" {
			textBuilder.WriteString(fmt.Sprintf("\n--- Page %d ---\n", pageNum))
			textBuilder.WriteString(cleaned)
			textBuilder.WriteString("\n")

			// Count words
			wordCount += countWords(cleaned)
		}

		// Check size limit
		if textBuilder.Len() > MaxExtractedTextSize {
			textBuilder.WriteString("\n... [Content truncated - size limit reached]")
			break
		}
	}

	extractedText := textBuilder.String()

	// Final size check
	if len(extractedText) > MaxExtractedTextSize {
		extractedText = extractedText[:MaxExtractedTextSize] + "\n... [Content truncated]"
	}

	return &PDFMetadata{
		PageCount: totalPages,
		WordCount: wordCount,
		Text:      extractedText,
	}, nil
}

// cleanPDFText cleans extracted PDF text
func cleanPDFText(text string) string {
	// Remove null bytes
	text = strings.ReplaceAll(text, "\x00", "")

	// Normalize whitespace
	text = normalizeWhitespace(text)

	// Trim
	text = strings.TrimSpace(text)

	return text
}

// normalizeWhitespace normalizes whitespace in text
func normalizeWhitespace(text string) string {
	var result strings.Builder
	lastWasSpace := false

	for _, r := range text {
		if unicode.IsSpace(r) {
			if !lastWasSpace {
				// Preserve newlines, convert other spaces to single space
				if r == '\n' {
					result.WriteRune('\n')
					lastWasSpace = false
				} else {
					result.WriteRune(' ')
					lastWasSpace = true
				}
			}
		} else {
			result.WriteRune(r)
			lastWasSpace = false
		}
	}

	return result.String()
}

// countWords counts the number of words in text
func countWords(text string) int {
	count := 0
	inWord := false

	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if inWord {
				count++
				inWord = false
			}
		} else {
			inWord = true
		}
	}

	// Count last word
	if inWord {
		count++
	}

	return count
}

// GetPDFPreview returns the first N characters of text as a preview
func GetPDFPreview(text string, maxChars int) string {
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
