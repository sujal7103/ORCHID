package document

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/cdproto/page"
	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// GeneratedDocument represents a generated document
type GeneratedDocument struct {
	DocumentID     string
	UserID         string
	ConversationID string
	Filename       string
	FilePath       string
	Size           int64
	DownloadURL    string
	ContentType    string // MIME type for download (e.g., "application/pdf", "text/plain")
	CreatedAt      time.Time
	Downloaded     bool
	DownloadedAt   *time.Time
}

// Service handles document generation and management
type Service struct {
	outputDir string
	documents map[string]*GeneratedDocument
	mu        sync.RWMutex
}

var (
	serviceInstance *Service
	serviceOnce     sync.Once
)

// GetService returns the singleton document service
func GetService() *Service {
	serviceOnce.Do(func() {
		outputDir := "./generated"
		if err := os.MkdirAll(outputDir, 0700); err != nil {
			log.Printf("⚠️  Warning: Could not create generated directory: %v", err)
		}
		serviceInstance = &Service{
			outputDir: outputDir,
			documents: make(map[string]*GeneratedDocument),
		}
	})
	return serviceInstance
}

// GenerateDocumentFromHTML creates a PDF document from custom HTML content
func (s *Service) GenerateDocumentFromHTML(htmlContent, filename, title, userID, conversationID string) (*GeneratedDocument, error) {
	// Wrap HTML in complete document structure if not already present
	fullHTML := htmlContent

	// Check if HTML is a complete document (has <!DOCTYPE or <html>)
	hasDoctype := bytes.Contains([]byte(htmlContent), []byte("<!DOCTYPE")) ||
	              bytes.Contains([]byte(htmlContent), []byte("<!doctype"))
	hasHTML := bytes.Contains([]byte(htmlContent), []byte("<html"))

	// If not a complete HTML document, wrap it with basic structure
	if !hasDoctype && !hasHTML {
		fullHTML = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        * {
            margin: 0 !important;
            padding: 0 !important;
            box-sizing: border-box;
        }
        html, body {
            width: 100%%;
            height: 100%%;
            margin: 0 !important;
            padding: 0 !important;
        }
    </style>
</head>
<body>
%s
</body>
</html>`, title, htmlContent)
	}

	// Generate unique document ID and filename
	documentID := uuid.New().String()
	safeFilename := filename + ".pdf"
	filePath := filepath.Join(s.outputDir, documentID+".pdf")

	// Convert HTML to PDF using chromedp
	if err := s.generatePDF(fullHTML, filePath); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Create document record
	doc := &GeneratedDocument{
		DocumentID:     documentID,
		UserID:         userID,
		ConversationID: conversationID,
		Filename:       safeFilename,
		FilePath:       filePath,
		Size:           fileInfo.Size(),
		DownloadURL:    fmt.Sprintf("/api/download/%s", documentID),
		ContentType:    "application/pdf",
		CreatedAt:      time.Now(),
		Downloaded:     false,
	}

	// Store document
	s.mu.Lock()
	s.documents[documentID] = doc
	s.mu.Unlock()

	log.Printf("📄 [DOCUMENT-SERVICE] Generated custom HTML PDF: %s (%d bytes)", safeFilename, fileInfo.Size())

	return doc, nil
}

// GenerateDocument creates a PDF document from markdown content (deprecated - use GenerateDocumentFromHTML)
// Kept for backward compatibility with existing code
func (s *Service) GenerateDocument(content, filename, title, userID, conversationID string) (*GeneratedDocument, error) {
	// Convert markdown to HTML with GFM extensions
	var htmlBuf bytes.Buffer
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (includes Table, Strikethrough, Linkify, TaskList)
		),
	)
	if err := md.Convert([]byte(content), &htmlBuf); err != nil {
		return nil, fmt.Errorf("failed to convert markdown: %w", err)
	}

	// Wrap in HTML template with basic styling
	fullHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <style>
        body {
            font-family: 'Segoe UI', Arial, sans-serif;
            line-height: 1.6;
            max-width: 800px;
            margin: 0 auto;
            padding: 40px 20px;
            color: #333;
        }
        h1, h2, h3 { color: #2c3e50; }
        code { background-color: #f4f4f4; padding: 2px 6px; border-radius: 3px; }
        pre { background-color: #2d2d2d; color: #f8f8f2; padding: 16px; border-radius: 6px; }
        table { border-collapse: collapse; width: 100%%; margin: 20px 0; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background-color: #3498db; color: white; }
    </style>
</head>
<body>
    %s
</body>
</html>`, title, htmlBuf.String())

	// Generate unique document ID and filename
	documentID := uuid.New().String()
	safeFilename := filename + ".pdf"
	filePath := filepath.Join(s.outputDir, documentID+".pdf")

	// Convert HTML to PDF using chromedp
	if err := s.generatePDF(fullHTML, filePath); err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Create document record
	doc := &GeneratedDocument{
		DocumentID:     documentID,
		UserID:         userID,
		ConversationID: conversationID,
		Filename:       safeFilename,
		FilePath:       filePath,
		Size:           fileInfo.Size(),
		DownloadURL:    fmt.Sprintf("/api/download/%s", documentID),
		ContentType:    "application/pdf",
		CreatedAt:      time.Now(),
		Downloaded:     false,
	}

	// Store document
	s.mu.Lock()
	s.documents[documentID] = doc
	s.mu.Unlock()

	log.Printf("📄 [DOCUMENT-SERVICE] Generated PDF from markdown: %s (%d bytes)", safeFilename, fileInfo.Size())

	return doc, nil
}

// generatePDF converts HTML to PDF using chromedp
func (s *Service) generatePDF(htmlContent, outputPath string) error {
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
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Generate PDF
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
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuffer, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithDisplayHeaderFooter(false).
				WithMarginTop(0).
				WithMarginBottom(0).
				WithMarginLeft(0).
				WithMarginRight(0).
				WithPaperWidth(8.27).  // A4 width in inches
				WithPaperHeight(11.69). // A4 height in inches
				WithScale(1.0).         // 100% scale, no shrinking
				Do(ctx)
			return err
		}),
	); err != nil {
		return err
	}

	// Write PDF to file with restricted permissions (owner read/write only for security)
	if err := os.WriteFile(outputPath, pdfBuffer, 0600); err != nil {
		return err
	}

	return nil
}

// GetDocument retrieves a document by ID
func (s *Service) GetDocument(documentID string) (*GeneratedDocument, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, exists := s.documents[documentID]
	return doc, exists
}

// MarkDownloaded marks a document as downloaded
func (s *Service) MarkDownloaded(documentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if doc, exists := s.documents[documentID]; exists {
		now := time.Now()
		doc.Downloaded = true
		doc.DownloadedAt = &now
		log.Printf("✅ [DOCUMENT-SERVICE] Document downloaded: %s", doc.Filename)
	}
}

// CleanupDownloadedDocuments deletes documents that have been downloaded
func (s *Service) CleanupDownloadedDocuments() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cleanedCount := 0

	for docID, doc := range s.documents {
		shouldDelete := false

		// Delete if downloaded AND 5 minutes passed (fast path)
		if doc.Downloaded && doc.DownloadedAt != nil {
			if now.Sub(*doc.DownloadedAt) > 5*time.Minute {
				shouldDelete = true
				log.Printf("🗑️  [DOCUMENT-SERVICE] Deleting downloaded document: %s (downloaded %v ago)",
					doc.Filename, now.Sub(*doc.DownloadedAt))
			}
		}

		// Delete if created over 10 minutes ago (main TTL - privacy-first)
		if now.Sub(doc.CreatedAt) > 10*time.Minute {
			shouldDelete = true
			log.Printf("🗑️  [DOCUMENT-SERVICE] Deleting expired document: %s (created %v ago)",
				doc.Filename, now.Sub(doc.CreatedAt))
		}

		if shouldDelete {
			// Delete file from disk
			if err := os.Remove(doc.FilePath); err != nil && !os.IsNotExist(err) {
				log.Printf("⚠️  Failed to delete document file %s: %v", doc.FilePath, err)
			}

			// Remove from map
			delete(s.documents, docID)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		log.Printf("✅ [DOCUMENT-SERVICE] Cleaned up %d documents", cleanedCount)
	}
}

// GenerateTextFile creates a text-based file with the given content and extension
func (s *Service) GenerateTextFile(content, filename, extension, userID, conversationID string) (*GeneratedDocument, error) {
	// Sanitize extension (remove leading dot if present)
	if len(extension) > 0 && extension[0] == '.' {
		extension = extension[1:]
	}

	// Generate unique document ID and filename
	documentID := uuid.New().String()
	safeFilename := filename + "." + extension
	filePath := filepath.Join(s.outputDir, documentID+"."+extension)

	// Write content to file with restricted permissions (owner read/write only for security)
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		return nil, fmt.Errorf("failed to write text file: %w", err)
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Get appropriate content type
	contentType := getContentTypeForExtension(extension)

	// Create document record
	doc := &GeneratedDocument{
		DocumentID:     documentID,
		UserID:         userID,
		ConversationID: conversationID,
		Filename:       safeFilename,
		FilePath:       filePath,
		Size:           fileInfo.Size(),
		DownloadURL:    fmt.Sprintf("/api/download/%s", documentID),
		ContentType:    contentType,
		CreatedAt:      time.Now(),
		Downloaded:     false,
	}

	// Store document
	s.mu.Lock()
	s.documents[documentID] = doc
	s.mu.Unlock()

	log.Printf("📝 [DOCUMENT-SERVICE] Generated text file: %s (%d bytes)", safeFilename, fileInfo.Size())

	return doc, nil
}

// getContentTypeForExtension returns the MIME type for a given file extension
func getContentTypeForExtension(ext string) string {
	contentTypes := map[string]string{
		// Text formats
		"txt":  "text/plain",
		"text": "text/plain",
		"log":  "text/plain",

		// Data formats
		"json": "application/json",
		"yaml": "application/x-yaml",
		"yml":  "application/x-yaml",
		"xml":  "application/xml",
		"csv":  "text/csv",
		"tsv":  "text/tab-separated-values",

		// Config formats
		"ini":  "text/plain",
		"toml": "application/toml",
		"env":  "text/plain",
		"conf": "text/plain",
		"cfg":  "text/plain",

		// Web formats
		"html": "text/html",
		"htm":  "text/html",
		"css":  "text/css",
		"js":   "application/javascript",
		"mjs":  "application/javascript",
		"ts":   "application/typescript",
		"tsx":  "application/typescript",
		"jsx":  "text/jsx",

		// Markdown
		"md":       "text/markdown",
		"markdown": "text/markdown",

		// Programming languages
		"py":    "text/x-python",
		"go":    "text/x-go",
		"rs":    "text/x-rust",
		"java":  "text/x-java",
		"c":     "text/x-c",
		"cpp":   "text/x-c++",
		"h":     "text/x-c",
		"hpp":   "text/x-c++",
		"cs":    "text/x-csharp",
		"rb":    "text/x-ruby",
		"php":   "text/x-php",
		"swift": "text/x-swift",
		"kt":    "text/x-kotlin",
		"scala": "text/x-scala",
		"r":     "text/x-r",

		// Shell scripts
		"sh":   "application/x-sh",
		"bash": "application/x-sh",
		"zsh":  "application/x-sh",
		"ps1":  "application/x-powershell",
		"bat":  "application/x-msdos-program",
		"cmd":  "application/x-msdos-program",

		// Database
		"sql": "application/sql",

		// Other
		"diff":  "text/x-diff",
		"patch": "text/x-diff",
	}

	if contentType, ok := contentTypes[ext]; ok {
		return contentType
	}

	// Default to text/plain for unknown extensions
	return "text/plain"
}
