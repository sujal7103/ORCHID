package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"clara-agents/internal/securefile"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// NewHTMLToPDFTool creates a new HTML to PDF converter tool
func NewHTMLToPDFTool() *Tool {
	return &Tool{
		Name:        "html_to_pdf",
		DisplayName: "HTML to PDF Converter",
		Description: `Convert HTML content or a URL to a high-quality PDF document. Uses a headless Chromium browser for pixel-perfect rendering with JavaScript execution support.

Use cases:
- Convert HTML reports, invoices, or documents to PDF
- Generate PDFs from HTML templates with dynamic data
- Capture live webpages as PDF (via URL)
- Render JavaScript-heavy pages (React, Vue, charts, etc.)

Supports:
- Full CSS styling (flexbox, grid, custom fonts, animations)
- JavaScript execution and dynamic content rendering
- External stylesheets, fonts, and images
- URL rendering (capture any webpage as PDF)
- Custom page sizes and orientations`,
		Icon:     "FileText",
		Source:   ToolSourceBuiltin,
		Category: "output",
		Keywords: []string{"html", "pdf", "convert", "document", "export", "report", "invoice", "print", "url", "webpage"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"html": map[string]interface{}{
					"type":        "string",
					"description": "The HTML content to convert to PDF. Can include inline CSS, JavaScript, external stylesheets, and images. Either 'html' or 'url' must be provided.",
				},
				"url": map[string]interface{}{
					"type":        "string",
					"description": "URL of a webpage to convert to PDF. The page will be fully rendered including JavaScript. Either 'html' or 'url' must be provided.",
				},
				"filename": map[string]interface{}{
					"type":        "string",
					"description": "Output filename for the PDF (without extension). Default: 'document'",
				},
				"page_size": map[string]interface{}{
					"type":        "string",
					"description": "Page size: 'A4', 'Letter', 'Legal', 'A3', 'A5', or 'Tabloid'. Default: 'A4'",
					"enum":        []string{"A4", "Letter", "Legal", "A3", "A5", "Tabloid"},
				},
				"landscape": map[string]interface{}{
					"type":        "boolean",
					"description": "Use landscape orientation. Default: false (portrait)",
				},
				"margin": map[string]interface{}{
					"type":        "string",
					"description": "Page margins: 'none', 'minimal', 'normal', or 'wide'. Default: 'normal'",
					"enum":        []string{"none", "minimal", "normal", "wide"},
				},
				"print_background": map[string]interface{}{
					"type":        "boolean",
					"description": "Include background colors and images. Default: true",
				},
				"scale": map[string]interface{}{
					"type":        "number",
					"description": "Scale of the PDF (0.1 to 2.0). Default: 1.0",
				},
			},
			"required": []string{},
		},
		Execute: executeHTMLToPDFNative,
	}
}

func executeHTMLToPDFNative(args map[string]interface{}) (string, error) {
	// Extract HTML content or URL (one is required)
	html, hasHTML := args["html"].(string)
	url, hasURL := args["url"].(string)

	if (!hasHTML || html == "") && (!hasURL || url == "") {
		return "", fmt.Errorf("either 'html' content or 'url' is required")
	}

	// Determine mode
	useURL := hasURL && url != ""

	// Extract optional parameters with defaults
	filename := "document"
	if fn, ok := args["filename"].(string); ok && fn != "" {
		filename = fn
	}
	filename = strings.TrimSuffix(filename, ".pdf")

	pageSize := "A4"
	if ps, ok := args["page_size"].(string); ok && ps != "" {
		pageSize = ps
	}

	landscape := false
	if ls, ok := args["landscape"].(bool); ok {
		landscape = ls
	}

	margin := "normal"
	if m, ok := args["margin"].(string); ok && m != "" {
		margin = m
	}

	printBackground := true
	if pb, ok := args["print_background"].(bool); ok {
		printBackground = pb
	}

	scale := 1.0
	if s, ok := args["scale"].(float64); ok && s >= 0.1 && s <= 2.0 {
		scale = s
	}

	// Convert margin to actual values (in inches)
	marginConfig := getMarginConfig(margin)

	// Get page dimensions based on page size
	paperWidth, paperHeight := getPaperSize(pageSize)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Configure Chrome options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("allow-running-insecure-content", true),
	)

	// Check for Chromium path in environment
	if chromePath := os.Getenv("CHROME_BIN"); chromePath != "" {
		opts = append(opts, chromedp.ExecPath(chromePath))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	taskCtx, taskCancel := chromedp.NewContext(allocCtx)
	defer taskCancel()

	var pdfData []byte
	var actions []chromedp.Action

	if useURL {
		// Navigate to URL
		actions = append(actions,
			chromedp.Navigate(url),
			chromedp.WaitReady("body"),
			// Wait for fonts and images to load
			chromedp.Sleep(2*time.Second),
			// Wait for document.fonts.ready
			chromedp.Evaluate(`document.fonts.ready`, nil),
			chromedp.Sleep(500*time.Millisecond),
		)
	} else {
		// Set HTML content directly using base64 encoding for reliability
		// Base64 encoding handles all special characters properly
		encodedHTML := base64.StdEncoding.EncodeToString([]byte(html))
		dataURL := "data:text/html;base64," + encodedHTML
		actions = append(actions,
			chromedp.Navigate(dataURL),
			chromedp.WaitReady("body"),
			// Wait for fonts and images to load
			chromedp.Sleep(2*time.Second),
			// Wait for document.fonts.ready
			chromedp.Evaluate(`document.fonts.ready`, nil),
			chromedp.Sleep(500*time.Millisecond),
		)
	}

	// Add PDF generation action
	actions = append(actions,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfData, _, err = page.PrintToPDF().
				WithPaperWidth(paperWidth).
				WithPaperHeight(paperHeight).
				WithMarginTop(marginConfig.Top).
				WithMarginBottom(marginConfig.Bottom).
				WithMarginLeft(marginConfig.Left).
				WithMarginRight(marginConfig.Right).
				WithLandscape(landscape).
				WithPrintBackground(printBackground).
				WithScale(scale).
				WithPreferCSSPageSize(false).
				Do(ctx)
			return err
		}),
	)

	// Execute all actions
	startTime := time.Now()
	if err := chromedp.Run(taskCtx, actions...); err != nil {
		return "", fmt.Errorf("failed to generate PDF: %w", err)
	}
	executionTime := time.Since(startTime).Seconds()

	if len(pdfData) == 0 {
		return "", fmt.Errorf("PDF generation produced empty result")
	}

	// Store PDF using secure file service
	secureFileSvc := securefile.GetService()
	secureResult, err := secureFileSvc.CreateFile(
		"system",
		pdfData,
		filename+".pdf",
		"application/pdf",
	)

	// Build response
	response := map[string]interface{}{
		"success":        true,
		"filename":       filename + ".pdf",
		"execution_time": executionTime,
	}

	if useURL {
		response["source"] = "url"
		response["url"] = url
	} else {
		response["source"] = "html"
	}

	if err != nil {
		// If secure file creation fails, return error
		return "", fmt.Errorf("failed to store PDF: %w", err)
	}

	// Include secure file info with download URL
	response["files"] = []map[string]interface{}{
		{
			"filename":     secureResult.Filename,
			"download_url": secureResult.DownloadURL,
			"access_code":  secureResult.AccessCode,
			"size":         secureResult.Size,
			"expires_at":   secureResult.ExpiresAt.Format("2006-01-02 15:04:05 MST"),
			"expires_in":   "30 days",
		},
	}
	response["file_count"] = 1
	response["message"] = fmt.Sprintf("PDF '%s.pdf' generated successfully (%d bytes). Download link valid for 30 days.", filename, len(pdfData))

	jsonResponse, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResponse), nil
}

// MarginConfigFloat holds margin values for PDF in inches
type MarginConfigFloat struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// getMarginConfig returns margin values based on preset name (in inches)
func getMarginConfig(margin string) MarginConfigFloat {
	switch margin {
	case "none":
		return MarginConfigFloat{0, 0, 0, 0}
	case "minimal":
		return MarginConfigFloat{0.25, 0.25, 0.25, 0.25}
	case "normal":
		return MarginConfigFloat{0.5, 0.5, 0.5, 0.5}
	case "wide":
		return MarginConfigFloat{1.0, 1.0, 1.0, 1.0}
	default:
		return MarginConfigFloat{0.5, 0.5, 0.5, 0.5}
	}
}

// getPaperSize returns paper dimensions in inches for common page sizes
func getPaperSize(pageSize string) (width, height float64) {
	switch pageSize {
	case "A4":
		return 8.27, 11.69
	case "Letter":
		return 8.5, 11
	case "Legal":
		return 8.5, 14
	case "A3":
		return 11.69, 16.54
	case "A5":
		return 5.83, 8.27
	case "Tabloid":
		return 11, 17
	default:
		return 8.27, 11.69 // A4 default
	}
}
