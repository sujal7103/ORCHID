package presentation

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

// Slide represents a single slide/page in the presentation
// Each slide contains a complete standalone HTML document
type Slide struct {
	HTML string `json:"html"` // Complete HTML document (must include <html>, <head>, and <body> tags)
}

// PresentationConfig holds the presentation configuration
type PresentationConfig struct {
	Title  string  `json:"title"`
	Slides []Slide `json:"slides"` // Array of slides, each with custom HTML content
}

// GeneratedPresentation represents a generated presentation file
type GeneratedPresentation struct {
	PresentationID string
	UserID         string
	ConversationID string
	Filename       string
	FilePath       string
	Size           int64
	ContentType    string
	CreatedAt      time.Time
}

// Service handles presentation generation
type Service struct {
	outputDir     string
	presentations map[string]*GeneratedPresentation
	mu            sync.RWMutex
}

var (
	serviceInstance *Service
	serviceOnce     sync.Once
)

// GetService returns the singleton presentation service
func GetService() *Service {
	serviceOnce.Do(func() {
		outputDir := "./generated"
		if err := os.MkdirAll(outputDir, 0700); err != nil {
			log.Printf("⚠️  Warning: Could not create generated directory: %v", err)
		}
		serviceInstance = &Service{
			outputDir:     outputDir,
			presentations: make(map[string]*GeneratedPresentation),
		}
	})
	return serviceInstance
}

// GeneratePresentation creates a PDF presentation with custom HTML pages
// Each page is rendered in 16:9 landscape format with complete creative freedom
func (s *Service) GeneratePresentation(config PresentationConfig, userID, conversationID string) (*GeneratedPresentation, error) {
	// Validate
	if config.Title == "" {
		config.Title = "Presentation"
	}
	if len(config.Slides) == 0 {
		return nil, fmt.Errorf("presentation must have at least one slide")
	}

	// Generate multi-page HTML with CSS page breaks
	htmlContent := s.generateMultiPageHTML(config)

	// Generate unique ID and filename
	presentationID := uuid.New().String()
	safeFilename := sanitizeFilename(config.Title) + ".pdf"
	filePath := filepath.Join(s.outputDir, presentationID+".pdf")

	// Convert to PDF using chromedp (16:9 landscape)
	if err := s.generatePDFLandscape(htmlContent, filePath); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Create record
	pres := &GeneratedPresentation{
		PresentationID: presentationID,
		UserID:         userID,
		ConversationID: conversationID,
		Filename:       safeFilename,
		FilePath:       filePath,
		Size:           fileInfo.Size(),
		ContentType:    "application/pdf",
		CreatedAt:      time.Now(),
	}

	// Store
	s.mu.Lock()
	s.presentations[presentationID] = pres
	s.mu.Unlock()

	log.Printf("🎯 [PRESENTATION-SERVICE] Generated PDF presentation: %s (%d bytes, %d pages)", safeFilename, fileInfo.Size(), len(config.Slides))

	return pres, nil
}

// generateMultiPageHTML creates HTML document with each slide as a separate page
// Extracts content from each slide's HTML and renders directly (no iframes)
func (s *Service) generateMultiPageHTML(config PresentationConfig) string {
	var pagesHTML bytes.Buffer
	var allStyles bytes.Buffer

	for i, slide := range config.Slides {
		slideHTML := slide.HTML

		// Extract styles from <head> (everything between <style> and </style>)
		styleStart := strings.Index(strings.ToLower(slideHTML), "<style")
		styleEnd := strings.Index(strings.ToLower(slideHTML), "</style>")
		var styles string
		if styleStart != -1 && styleEnd != -1 {
			// Find the end of the opening <style> tag
			styleTagEnd := strings.Index(slideHTML[styleStart:], ">")
			if styleTagEnd != -1 {
				styleContentStart := styleStart + styleTagEnd + 1
				styles = slideHTML[styleContentStart:styleEnd]
			}
		}

		// Extract body content (everything between <body> and </body>)
		bodyStart := strings.Index(strings.ToLower(slideHTML), "<body")
		bodyEnd := strings.Index(strings.ToLower(slideHTML), "</body>")
		var bodyContent string
		if bodyStart != -1 && bodyEnd != -1 {
			// Find the end of the opening <body> tag
			bodyTagEnd := strings.Index(slideHTML[bodyStart:], ">")
			if bodyTagEnd != -1 {
				bodyContentStart := bodyStart + bodyTagEnd + 1
				bodyContent = slideHTML[bodyContentStart:bodyEnd]
			}
		}

		// Scope the styles to this specific slide using a unique class
		slideClass := fmt.Sprintf("slide-%d", i+1)
		if styles != "" {
			// Replace 'body' selector with the slide class since body content is now in a div
			scopedStyles := strings.ReplaceAll(styles, "body{", fmt.Sprintf(".%s{", slideClass))
			scopedStyles = strings.ReplaceAll(scopedStyles, "body {", fmt.Sprintf(".%s {", slideClass))
			allStyles.WriteString(fmt.Sprintf("/* Slide %d styles */\n%s\n\n", i+1, scopedStyles))
		}

		// Create a div for this slide with page-break-after
		pagesHTML.WriteString(fmt.Sprintf(`<div class="slide-page %s">%s</div>
`, slideClass, bodyContent))
	}

	// Generate full HTML document
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <style>
        @page {
            size: 10.67in 6in;
            margin: 0;
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        html, body {
            margin: 0;
            padding: 0;
        }

        .slide-page {
            width: 10.67in;
            height: 6in;
            page-break-after: always;
            page-break-inside: avoid;
            position: relative;
            overflow: hidden;
        }

        .slide-page:last-child {
            page-break-after: auto;
        }

        @media print {
            .slide-page {
                page-break-after: always !important;
                page-break-inside: avoid !important;
            }
            .slide-page:last-child {
                page-break-after: auto !important;
            }
        }

        /* Slide-specific styles */
        %s
    </style>
</head>
<body>
%s
</body>
</html>`, html.EscapeString(config.Title), allStyles.String(), pagesHTML.String())
}

// generatePDFLandscape converts HTML to PDF using chromedp with 16:9 landscape format
func (s *Service) generatePDFLandscape(htmlContent, outputPath string) error {
	// Create allocator options for headless Chrome
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath("/usr/bin/chromium-browser"),
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)

	// Create allocator context
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	// Create context
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set timeout
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Generate PDF in 16:9 landscape format
	var pdfBuffer []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(frameTree.Frame.ID, htmlContent).Do(ctx)
		}),
		chromedp.Sleep(3*time.Second), // Wait for fonts, images, and custom styles to load
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			// Use PreferCSSPageSize to ensure Chrome respects CSS page-break properties
			pdfBuffer, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithDisplayHeaderFooter(false).
				WithMarginTop(0).
				WithMarginBottom(0).
				WithMarginLeft(0).
				WithMarginRight(0).
				WithPaperWidth(10.67).  // 16:9 landscape width
				WithPaperHeight(6).     // 16:9 landscape height
				WithPreferCSSPageSize(true). // CRITICAL: Enables CSS page-break properties
				Do(ctx)
			return err
		}),
	); err != nil {
		return err
	}

	// Write PDF to file
	if err := os.WriteFile(outputPath, pdfBuffer, 0600); err != nil {
		return err
	}

	return nil
}

// sanitizeFilename removes invalid characters from filename
func sanitizeFilename(name string) string {
	// Replace invalid characters with underscore
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := name
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	// Limit length
	if len(result) > 50 {
		result = result[:50]
	}
	if result == "" {
		result = "presentation"
	}
	return result
}

// GetPresentation retrieves a presentation by ID
func (s *Service) GetPresentation(presentationID string) (*GeneratedPresentation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pres, exists := s.presentations[presentationID]
	return pres, exists
}

