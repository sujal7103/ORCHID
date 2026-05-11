package utils

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
)

const (
	// MaxPPTXSlides limits the number of slides to process
	MaxPPTXSlides = 200
)

// PPTXMetadata contains information about a PPTX file
type PPTXMetadata struct {
	SlideCount int
	WordCount  int
	Text       string
}

// ValidatePPTX checks if a file is a valid PPTX by checking ZIP structure
func ValidatePPTX(data []byte) error {
	reader := bytes.NewReader(data)
	zipReader, err := zip.NewReader(reader, int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid PPTX: not a valid ZIP file: %w", err)
	}

	// Check for required PPTX files
	hasContentTypes := false
	hasSlides := false

	for _, file := range zipReader.File {
		if file.Name == "[Content_Types].xml" {
			hasContentTypes = true
		}
		if strings.HasPrefix(file.Name, "ppt/slides/slide") && strings.HasSuffix(file.Name, ".xml") {
			hasSlides = true
		}
	}

	if !hasContentTypes {
		return fmt.Errorf("invalid PPTX: missing [Content_Types].xml")
	}
	if !hasSlides {
		return fmt.Errorf("invalid PPTX: no slides found")
	}

	return nil
}

// ExtractPPTXText extracts text from a PPTX file
func ExtractPPTXText(data []byte) (*PPTXMetadata, error) {
	reader := bytes.NewReader(data)
	zipReader, err := zip.NewReader(reader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to open PPTX: %w", err)
	}

	// Collect slide files and sort them by slide number
	type slideFile struct {
		num  int
		file *zip.File
	}
	var slides []slideFile

	for _, file := range zipReader.File {
		if strings.HasPrefix(file.Name, "ppt/slides/slide") && strings.HasSuffix(file.Name, ".xml") {
			// Extract slide number from filename (e.g., "ppt/slides/slide1.xml" -> 1)
			baseName := path.Base(file.Name)
			numStr := strings.TrimPrefix(baseName, "slide")
			numStr = strings.TrimSuffix(numStr, ".xml")
			num, err := strconv.Atoi(numStr)
			if err != nil {
				continue
			}
			slides = append(slides, slideFile{num: num, file: file})
		}
	}

	// Sort slides by number
	sort.Slice(slides, func(i, j int) bool {
		return slides[i].num < slides[j].num
	})

	// Limit slides for security
	if len(slides) > MaxPPTXSlides {
		slides = slides[:MaxPPTXSlides]
	}

	var textBuilder strings.Builder
	slideCount := len(slides)

	// Extract text from each slide
	for _, slide := range slides {
		rc, err := slide.file.Open()
		if err != nil {
			continue
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		// Parse XML and extract text
		slideText := extractTextFromPPTXML(content)
		if slideText != "" {
			textBuilder.WriteString(fmt.Sprintf("\n--- Slide %d ---\n", slide.num))
			textBuilder.WriteString(slideText)
			textBuilder.WriteString("\n")
		}

		// Check size limit
		if textBuilder.Len() > MaxExtractedTextSize {
			textBuilder.WriteString("\n... [Content truncated - size limit reached]")
			break
		}
	}

	extractedText := textBuilder.String()
	extractedText = cleanDocumentText(extractedText)

	// Final size check
	if len(extractedText) > MaxExtractedTextSize {
		extractedText = extractedText[:MaxExtractedTextSize] + "\n... [Content truncated]"
	}

	wordCount := countWords(extractedText)

	return &PPTXMetadata{
		SlideCount: slideCount,
		WordCount:  wordCount,
		Text:       extractedText,
	}, nil
}

// extractTextFromPPTXML parses PPTX slide XML and extracts text content
func extractTextFromPPTXML(xmlContent []byte) string {
	var textBuilder strings.Builder
	decoder := xml.NewDecoder(bytes.NewReader(xmlContent))

	// Track text runs within paragraphs
	inTextParagraph := false
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
			// Track paragraph boundaries (a:p is DrawingML paragraph)
			if t.Name.Local == "p" && strings.Contains(t.Name.Space, "drawingml") {
				inTextParagraph = true
				paragraphText.Reset()
			}
		case xml.EndElement:
			// End of paragraph - add newline
			if t.Name.Local == "p" && strings.Contains(t.Name.Space, "drawingml") {
				if inTextParagraph && paragraphText.Len() > 0 {
					textBuilder.WriteString(paragraphText.String())
					textBuilder.WriteString("\n")
				}
				inTextParagraph = false
			}
		case xml.CharData:
			// Collect text content (a:t is DrawingML text)
			text := strings.TrimSpace(string(t))
			if text != "" && inTextParagraph {
				if paragraphText.Len() > 0 {
					paragraphText.WriteString(" ")
				}
				paragraphText.WriteString(text)
			}
		}
	}

	return textBuilder.String()
}

// GetPPTXPreview returns the first N characters of text as a preview
func GetPPTXPreview(text string, maxChars int) string {
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
