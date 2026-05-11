package tools

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// NewS3ListTool creates a tool for listing S3 objects
func NewS3ListTool() *Tool {
	return &Tool{
		Name:        "s3_list",
		DisplayName: "List S3 Objects",
		Description: "List objects in an AWS S3 bucket. Authentication is handled automatically via configured credentials.",
		Icon:        "FolderOpen",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"aws", "s3", "list", "files", "objects", "bucket"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"bucket": map[string]interface{}{
					"type":        "string",
					"description": "S3 bucket name",
				},
				"prefix": map[string]interface{}{
					"type":        "string",
					"description": "Filter objects by prefix (folder path)",
				},
				"max_keys": map[string]interface{}{
					"type":        "number",
					"description": "Maximum number of objects to return (default 100)",
				},
			},
			"required": []string{"bucket"},
		},
		Execute: executeS3List,
	}
}

// NewS3UploadTool creates a tool for uploading to S3
func NewS3UploadTool() *Tool {
	return &Tool{
		Name:        "s3_upload",
		DisplayName: "Upload to S3",
		Description: "Upload content to an AWS S3 bucket. Authentication is handled automatically.",
		Icon:        "Upload",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"aws", "s3", "upload", "put", "file"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"bucket": map[string]interface{}{
					"type":        "string",
					"description": "S3 bucket name",
				},
				"key": map[string]interface{}{
					"type":        "string",
					"description": "Object key (file path in bucket)",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to upload",
				},
				"content_type": map[string]interface{}{
					"type":        "string",
					"description": "Content type (e.g., 'text/plain', 'application/json')",
				},
			},
			"required": []string{"bucket", "key", "content"},
		},
		Execute: executeS3Upload,
	}
}

// NewS3DownloadTool creates a tool for downloading from S3
func NewS3DownloadTool() *Tool {
	return &Tool{
		Name:        "s3_download",
		DisplayName: "Download from S3",
		Description: "Download an object from AWS S3. Authentication is handled automatically.",
		Icon:        "Download",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"aws", "s3", "download", "get", "file"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"bucket": map[string]interface{}{
					"type":        "string",
					"description": "S3 bucket name",
				},
				"key": map[string]interface{}{
					"type":        "string",
					"description": "Object key (file path in bucket)",
				},
			},
			"required": []string{"bucket", "key"},
		},
		Execute: executeS3Download,
	}
}

// NewS3DeleteTool creates a tool for deleting S3 objects
func NewS3DeleteTool() *Tool {
	return &Tool{
		Name:        "s3_delete",
		DisplayName: "Delete S3 Object",
		Description: "Delete an object from AWS S3. Authentication is handled automatically.",
		Icon:        "Trash2",
		Source:      ToolSourceBuiltin,
		Category:    "integration",
		Keywords:    []string{"aws", "s3", "delete", "remove", "file"},
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"credential_id": map[string]interface{}{
					"type":        "string",
					"description": "INTERNAL: Auto-injected by system. Do not set manually.",
				},
				"bucket": map[string]interface{}{
					"type":        "string",
					"description": "S3 bucket name",
				},
				"key": map[string]interface{}{
					"type":        "string",
					"description": "Object key to delete",
				},
			},
			"required": []string{"bucket", "key"},
		},
		Execute: executeS3Delete,
	}
}

type s3Config struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Bucket          string
}

func getS3Config(args map[string]interface{}) (*s3Config, error) {
	credData, err := GetCredentialData(args, "aws_s3")
	if err != nil {
		return nil, fmt.Errorf("failed to get AWS credentials: %w", err)
	}

	accessKey, _ := credData["access_key_id"].(string)
	secretKey, _ := credData["secret_access_key"].(string)
	region, _ := credData["region"].(string)
	bucket, _ := credData["bucket"].(string)

	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("access_key_id and secret_access_key are required")
	}

	if region == "" {
		region = "us-east-1"
	}

	return &s3Config{
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		Region:          region,
		Bucket:          bucket,
	}, nil
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func hashSHA256(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func s3Request(method, bucket, key, region, accessKey, secretKey string, body []byte, contentType string) (*http.Response, error) {
	host := fmt.Sprintf("%s.s3.%s.amazonaws.com", bucket, region)
	endpoint := fmt.Sprintf("https://%s%s", host, key)

	t := time.Now().UTC()
	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	var bodyReader io.Reader
	payloadHash := hashSHA256("")
	if body != nil {
		bodyReader = bytes.NewReader(body)
		payloadHash = hashSHA256(string(body))
	}

	req, err := http.NewRequest(method, endpoint, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Host", host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Create canonical request
	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	if contentType != "" {
		signedHeaders = append(signedHeaders, "content-type")
	}
	sort.Strings(signedHeaders)
	signedHeadersStr := strings.Join(signedHeaders, ";")

	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", host, payloadHash, amzDate)
	if contentType != "" {
		canonicalHeaders = fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n", contentType, host, payloadHash, amzDate)
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		key,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeadersStr,
		payloadHash,
	)

	// Create string to sign
	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, region)
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		amzDate,
		credentialScope,
		hashSHA256(canonicalRequest),
	)

	// Calculate signature
	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	// Add authorization header
	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		accessKey,
		credentialScope,
		signedHeadersStr,
		signature,
	)
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 60 * time.Second}
	return client.Do(req)
}

type listBucketResult struct {
	XMLName  xml.Name `xml:"ListBucketResult"`
	Contents []struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
	} `xml:"Contents"`
}

func executeS3List(args map[string]interface{}) (string, error) {
	cfg, err := getS3Config(args)
	if err != nil {
		return "", err
	}

	bucket, _ := args["bucket"].(string)
	if bucket == "" {
		bucket = cfg.Bucket
	}
	if bucket == "" {
		return "", fmt.Errorf("bucket is required")
	}

	key := "/"
	if prefix, ok := args["prefix"].(string); ok && prefix != "" {
		key = "/?prefix=" + prefix
	}

	maxKeys := 100
	if mk, ok := args["max_keys"].(float64); ok && mk > 0 {
		maxKeys = int(mk)
	}
	if strings.Contains(key, "?") {
		key += fmt.Sprintf("&max-keys=%d", maxKeys)
	} else {
		key += fmt.Sprintf("?max-keys=%d", maxKeys)
	}

	resp, err := s3Request("GET", bucket, key, cfg.Region, cfg.AccessKeyID, cfg.SecretAccessKey, nil, "")
	if err != nil {
		return "", fmt.Errorf("S3 request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("S3 error (status %d): %s", resp.StatusCode, string(body))
	}

	var result listBucketResult
	if err := xml.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	objects := make([]map[string]interface{}, 0)
	for _, obj := range result.Contents {
		objects = append(objects, map[string]interface{}{
			"key":           obj.Key,
			"size":          obj.Size,
			"last_modified": obj.LastModified,
			"storage_class": obj.StorageClass,
		})
	}

	response := map[string]interface{}{
		"success": true,
		"bucket":  bucket,
		"count":   len(objects),
		"objects": objects,
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

func executeS3Upload(args map[string]interface{}) (string, error) {
	cfg, err := getS3Config(args)
	if err != nil {
		return "", err
	}

	bucket, _ := args["bucket"].(string)
	if bucket == "" {
		bucket = cfg.Bucket
	}
	if bucket == "" {
		return "", fmt.Errorf("bucket is required")
	}

	key, _ := args["key"].(string)
	content, _ := args["content"].(string)
	if key == "" || content == "" {
		return "", fmt.Errorf("key and content are required")
	}

	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}

	contentType := "text/plain"
	if ct, ok := args["content_type"].(string); ok && ct != "" {
		contentType = ct
	}

	resp, err := s3Request("PUT", bucket, key, cfg.Region, cfg.AccessKeyID, cfg.SecretAccessKey, []byte(content), contentType)
	if err != nil {
		return "", fmt.Errorf("S3 request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("S3 error (status %d): %s", resp.StatusCode, string(body))
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Object uploaded successfully",
		"bucket":  bucket,
		"key":     strings.TrimPrefix(key, "/"),
		"size":    len(content),
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

func executeS3Download(args map[string]interface{}) (string, error) {
	cfg, err := getS3Config(args)
	if err != nil {
		return "", err
	}

	bucket, _ := args["bucket"].(string)
	if bucket == "" {
		bucket = cfg.Bucket
	}
	if bucket == "" {
		return "", fmt.Errorf("bucket is required")
	}

	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}

	resp, err := s3Request("GET", bucket, key, cfg.Region, cfg.AccessKeyID, cfg.SecretAccessKey, nil, "")
	if err != nil {
		return "", fmt.Errorf("S3 request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("S3 error (status %d): %s", resp.StatusCode, string(body))
	}

	response := map[string]interface{}{
		"success":      true,
		"bucket":       bucket,
		"key":          strings.TrimPrefix(key, "/"),
		"content":      string(body),
		"size":         len(body),
		"content_type": resp.Header.Get("Content-Type"),
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

func executeS3Delete(args map[string]interface{}) (string, error) {
	cfg, err := getS3Config(args)
	if err != nil {
		return "", err
	}

	bucket, _ := args["bucket"].(string)
	if bucket == "" {
		bucket = cfg.Bucket
	}
	if bucket == "" {
		return "", fmt.Errorf("bucket is required")
	}

	key, _ := args["key"].(string)
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	if !strings.HasPrefix(key, "/") {
		key = "/" + key
	}

	resp, err := s3Request("DELETE", bucket, key, cfg.Region, cfg.AccessKeyID, cfg.SecretAccessKey, nil, "")
	if err != nil {
		return "", fmt.Errorf("S3 request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("S3 error (status %d): %s", resp.StatusCode, string(body))
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Object deleted successfully",
		"bucket":  bucket,
		"key":     strings.TrimPrefix(key, "/"),
	}

	jsonResult, _ := json.MarshalIndent(response, "", "  ")
	return string(jsonResult), nil
}

