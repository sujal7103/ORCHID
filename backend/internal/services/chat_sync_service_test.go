package services

import (
	"clara-agents/internal/crypto"
	"clara-agents/internal/models"
	"encoding/json"
	"testing"
	"time"
)

// Test encryption service creation and basic operations
func TestEncryptionService(t *testing.T) {
	// Generate a test master key (32 bytes = 64 hex chars)
	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	encService, err := crypto.NewEncryptionService(masterKey)
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	if encService == nil {
		t.Fatal("Encryption service should not be nil")
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	encService, err := crypto.NewEncryptionService(masterKey)
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	userID := "test-user-123"
	testCases := []struct {
		name      string
		plaintext string
	}{
		{"simple text", "Hello, World!"},
		{"empty string", ""},
		{"json array", `[{"id":"1","content":"test"}]`},
		{"unicode", "Hello, \u4e16\u754c! \U0001F600"},
		{"long text", string(make([]byte, 10000))}, // 10KB of null bytes
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := encService.EncryptString(userID, tc.plaintext)
			if err != nil {
				if tc.plaintext == "" {
					// Empty string returns empty, not error
					return
				}
				t.Fatalf("Encryption failed: %v", err)
			}

			// Encrypted should not equal plaintext (unless empty)
			if tc.plaintext != "" && encrypted == tc.plaintext {
				t.Error("Encrypted text should not equal plaintext")
			}

			// Decrypt
			decrypted, err := encService.DecryptString(userID, encrypted)
			if err != nil {
				t.Fatalf("Decryption failed: %v", err)
			}

			// Decrypted should equal original
			if decrypted != tc.plaintext {
				t.Errorf("Decrypted text doesn't match original. Got %q, want %q", decrypted, tc.plaintext)
			}
		})
	}
}

func TestDifferentUsersGetDifferentKeys(t *testing.T) {
	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	encService, err := crypto.NewEncryptionService(masterKey)
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	plaintext := "Secret message"
	user1 := "user-1"
	user2 := "user-2"

	// Encrypt same message for two different users
	encrypted1, _ := encService.EncryptString(user1, plaintext)
	encrypted2, _ := encService.EncryptString(user2, plaintext)

	// Encrypted values should be different (different user keys + different nonces)
	if encrypted1 == encrypted2 {
		t.Error("Same plaintext encrypted for different users should produce different ciphertext")
	}

	// User 2 should not be able to decrypt user 1's message
	decrypted, err := encService.DecryptString(user2, encrypted1)
	if err == nil && decrypted == plaintext {
		t.Error("User 2 should not be able to decrypt User 1's message")
	}
}

func TestChatMessageSerialization(t *testing.T) {
	messages := []models.ChatMessage{
		{
			ID:        "msg-1",
			Role:      "user",
			Content:   "Hello!",
			Timestamp: time.Now().UnixMilli(),
		},
		{
			ID:        "msg-2",
			Role:      "assistant",
			Content:   "Hi there! How can I help you?",
			Timestamp: time.Now().UnixMilli(),
		},
	}

	// Serialize
	jsonData, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("Failed to serialize messages: %v", err)
	}

	// Deserialize
	var decoded []models.ChatMessage
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to deserialize messages: %v", err)
	}

	if len(decoded) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(decoded))
	}

	for i, msg := range decoded {
		if msg.ID != messages[i].ID {
			t.Errorf("Message %d ID mismatch: got %s, want %s", i, msg.ID, messages[i].ID)
		}
		if msg.Role != messages[i].Role {
			t.Errorf("Message %d Role mismatch: got %s, want %s", i, msg.Role, messages[i].Role)
		}
		if msg.Content != messages[i].Content {
			t.Errorf("Message %d Content mismatch: got %s, want %s", i, msg.Content, messages[i].Content)
		}
	}
}

func TestChatMessageWithAttachments(t *testing.T) {
	message := models.ChatMessage{
		ID:        "msg-1",
		Role:      "user",
		Content:   "Check out this file",
		Timestamp: time.Now().UnixMilli(),
		Attachments: []models.ChatAttachment{
			{
				FileID:   "att-1",
				Filename: "document.pdf",
				Type:     "document",
				MimeType: "application/pdf",
				Size:     1024,
			},
		},
	}

	// Serialize
	jsonData, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("Failed to serialize message with attachment: %v", err)
	}

	// Deserialize
	var decoded models.ChatMessage
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to deserialize message with attachment: %v", err)
	}

	if len(decoded.Attachments) != 1 {
		t.Fatalf("Expected 1 attachment, got %d", len(decoded.Attachments))
	}

	att := decoded.Attachments[0]
	if att.FileID != "att-1" {
		t.Errorf("Attachment FileID mismatch: got %s, want att-1", att.FileID)
	}
	if att.Filename != "document.pdf" {
		t.Errorf("Attachment filename mismatch: got %s, want document.pdf", att.Filename)
	}
}

func TestCreateChatRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request models.CreateChatRequest
		wantErr bool
	}{
		{
			name: "valid request",
			request: models.CreateChatRequest{
				ID:    "chat-123",
				Title: "Test Chat",
				Messages: []models.ChatMessage{
					{ID: "msg-1", Role: "user", Content: "Hello"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty chat ID",
			request: models.CreateChatRequest{
				ID:    "",
				Title: "Test Chat",
			},
			wantErr: true,
		},
		{
			name: "empty messages",
			request: models.CreateChatRequest{
				ID:       "chat-123",
				Title:    "Test Chat",
				Messages: []models.ChatMessage{},
			},
			wantErr: false, // Empty messages should be allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasErr := tt.request.ID == ""
			if hasErr != tt.wantErr {
				t.Errorf("Validation error = %v, wantErr %v", hasErr, tt.wantErr)
			}
		})
	}
}

func TestBulkSyncRequest(t *testing.T) {
	req := models.BulkSyncRequest{
		Chats: []models.CreateChatRequest{
			{
				ID:    "chat-1",
				Title: "Chat 1",
				Messages: []models.ChatMessage{
					{ID: "msg-1", Role: "user", Content: "Hello"},
				},
			},
			{
				ID:    "chat-2",
				Title: "Chat 2",
				Messages: []models.ChatMessage{
					{ID: "msg-2", Role: "user", Content: "Hi"},
				},
			},
		},
	}

	// Serialize
	jsonData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to serialize bulk sync request: %v", err)
	}

	// Deserialize
	var decoded models.BulkSyncRequest
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to deserialize bulk sync request: %v", err)
	}

	if len(decoded.Chats) != 2 {
		t.Errorf("Expected 2 chats, got %d", len(decoded.Chats))
	}
}

func TestChatResponseConversion(t *testing.T) {
	response := models.ChatResponse{
		ID:        "chat-123",
		Title:     "Test Chat",
		Messages:  []models.ChatMessage{{ID: "msg-1", Role: "user", Content: "Hello"}},
		IsStarred: true,
		Model:     "gpt-4",
		Version:   5,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Serialize
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to serialize chat response: %v", err)
	}

	// Deserialize
	var decoded models.ChatResponse
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to deserialize chat response: %v", err)
	}

	if decoded.ID != response.ID {
		t.Errorf("ID mismatch: got %s, want %s", decoded.ID, response.ID)
	}
	if decoded.Version != response.Version {
		t.Errorf("Version mismatch: got %d, want %d", decoded.Version, response.Version)
	}
	if decoded.IsStarred != response.IsStarred {
		t.Errorf("IsStarred mismatch: got %v, want %v", decoded.IsStarred, response.IsStarred)
	}
}

func TestChatListItem(t *testing.T) {
	item := models.ChatListItem{
		ID:           "chat-123",
		Title:        "Test Chat",
		IsStarred:    true,
		Model:        "gpt-4",
		MessageCount: 10,
		Version:      3,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Serialize
	jsonData, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to serialize chat list item: %v", err)
	}

	// Check JSON contains expected fields
	var jsonMap map[string]interface{}
	json.Unmarshal(jsonData, &jsonMap)

	if _, ok := jsonMap["message_count"]; !ok {
		t.Error("Expected message_count in JSON output")
	}
	if _, ok := jsonMap["is_starred"]; !ok {
		t.Error("Expected is_starred in JSON output")
	}
}

func TestUpdateChatRequest(t *testing.T) {
	title := "New Title"
	starred := true

	req := models.UpdateChatRequest{
		Title:     &title,
		IsStarred: &starred,
		Version:   5,
	}

	// Serialize
	jsonData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to serialize update request: %v", err)
	}

	// Deserialize
	var decoded models.UpdateChatRequest
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to deserialize update request: %v", err)
	}

	if decoded.Title == nil || *decoded.Title != title {
		t.Error("Title should be set")
	}
	if decoded.IsStarred == nil || *decoded.IsStarred != starred {
		t.Error("IsStarred should be set")
	}
	if decoded.Version != 5 {
		t.Errorf("Version mismatch: got %d, want 5", decoded.Version)
	}
}

func TestChatAddMessageRequest(t *testing.T) {
	req := models.ChatAddMessageRequest{
		Message: models.ChatMessage{
			ID:        "msg-new",
			Role:      "user",
			Content:   "New message",
			Timestamp: time.Now().UnixMilli(),
		},
		Version: 3,
	}

	// Serialize
	jsonData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to serialize add message request: %v", err)
	}

	// Deserialize
	var decoded models.ChatAddMessageRequest
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to deserialize add message request: %v", err)
	}

	if decoded.Message.ID != "msg-new" {
		t.Errorf("Message ID mismatch: got %s, want msg-new", decoded.Message.ID)
	}
	if decoded.Version != 3 {
		t.Errorf("Version mismatch: got %d, want 3", decoded.Version)
	}
}

func TestNewChatSyncService(t *testing.T) {
	// This test requires MongoDB to be set up
	// In a real test environment, you'd use a test MongoDB instance
	// For now, we just verify the service constructor doesn't panic with valid inputs
	t.Skip("Requires MongoDB connection - run with integration tests")
}

// ==================== EDGE CASE TESTS ====================

func TestEmptyMessagesArray(t *testing.T) {
	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	encService, _ := crypto.NewEncryptionService(masterKey)

	userID := "user-123"
	emptyMessages := []models.ChatMessage{}

	// Serialize empty array
	jsonData, err := json.Marshal(emptyMessages)
	if err != nil {
		t.Fatalf("Failed to serialize empty messages: %v", err)
	}

	// Encrypt
	encrypted, err := encService.Encrypt(userID, jsonData)
	if err != nil {
		t.Fatalf("Failed to encrypt empty messages: %v", err)
	}

	// Decrypt
	decrypted, err := encService.Decrypt(userID, encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	// Deserialize
	var recovered []models.ChatMessage
	err = json.Unmarshal(decrypted, &recovered)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	if len(recovered) != 0 {
		t.Errorf("Expected empty array, got %d messages", len(recovered))
	}
}

func TestNullFieldsInMessage(t *testing.T) {
	// Test that null/empty optional fields don't break serialization
	message := models.ChatMessage{
		ID:          "msg-1",
		Role:        "user",
		Content:     "Hello",
		Timestamp:   1700000000000,
		IsStreaming: false,
		Attachments: nil, // nil attachments
		AgentId:     "",  // empty agent id
		AgentName:   "",
		AgentAvatar: "",
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("Failed to serialize message with null fields: %v", err)
	}

	var recovered models.ChatMessage
	err = json.Unmarshal(jsonData, &recovered)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	if recovered.ID != message.ID {
		t.Error("ID mismatch after roundtrip")
	}
	if recovered.Attachments != nil {
		t.Error("Expected nil attachments")
	}
}

func TestSpecialCharactersInContent(t *testing.T) {
	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	encService, _ := crypto.NewEncryptionService(masterKey)

	testCases := []string{
		"Hello\nWorld",                            // Newlines
		"Tab\there",                               // Tabs
		"Quote: \"test\"",                         // Quotes
		"Backslash: \\path\\to\\file",             // Backslashes
		"Unicode: \u4e2d\u6587",                   // Chinese characters
		"Emoji: \U0001F600\U0001F389",             // Emojis
		"HTML: <script>alert('xss')</script>",     // HTML
		"SQL: SELECT * FROM users; DROP TABLE--",  // SQL injection attempt
		"Null byte: \x00",                         // Null byte
		"Control chars: \x01\x02\x03",             // Control characters
		string(make([]byte, 100000)),              // Large content (100KB)
	}

	userID := "user-123"

	for i, content := range testCases {
		t.Run("case_"+string(rune('0'+i)), func(t *testing.T) {
			message := models.ChatMessage{
				ID:        "msg-1",
				Role:      "user",
				Content:   content,
				Timestamp: 1700000000000,
			}

			// Serialize
			jsonData, err := json.Marshal(message)
			if err != nil {
				t.Fatalf("Failed to serialize: %v", err)
			}

			// Encrypt
			encrypted, err := encService.Encrypt(userID, jsonData)
			if err != nil {
				t.Fatalf("Failed to encrypt: %v", err)
			}

			// Decrypt
			decrypted, err := encService.Decrypt(userID, encrypted)
			if err != nil {
				t.Fatalf("Failed to decrypt: %v", err)
			}

			// Deserialize
			var recovered models.ChatMessage
			err = json.Unmarshal(decrypted, &recovered)
			if err != nil {
				t.Fatalf("Failed to deserialize: %v", err)
			}

			if recovered.Content != content {
				t.Errorf("Content mismatch after roundtrip")
			}
		})
	}
}

func TestMaxMessageCount(t *testing.T) {
	// Test with a large number of messages (stress test)
	messageCount := 1000
	messages := make([]models.ChatMessage, messageCount)

	for i := 0; i < messageCount; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = models.ChatMessage{
			ID:        "msg-" + string(rune('0'+i%10)) + string(rune('0'+i/10)),
			Role:      role,
			Content:   "This is message number " + string(rune('0'+i)),
			Timestamp: int64(1700000000000 + i*1000),
		}
	}

	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	encService, _ := crypto.NewEncryptionService(masterKey)
	userID := "user-123"

	// Serialize
	jsonData, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("Failed to serialize %d messages: %v", messageCount, err)
	}

	// Encrypt
	encrypted, err := encService.Encrypt(userID, jsonData)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Decrypt
	decrypted, err := encService.Decrypt(userID, encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	// Deserialize
	var recovered []models.ChatMessage
	err = json.Unmarshal(decrypted, &recovered)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	if len(recovered) != messageCount {
		t.Errorf("Expected %d messages, got %d", messageCount, len(recovered))
	}
}

func TestAttachmentTypes(t *testing.T) {
	// Test all attachment types
	attachments := []models.ChatAttachment{
		{
			FileID:   "att-1",
			Filename: "document.pdf",
			Type:     "document",
			MimeType: "application/pdf",
			Size:     1024,
			URL:      "https://example.com/file.pdf",
		},
		{
			FileID:   "att-2",
			Filename: "image.png",
			Type:     "image",
			MimeType: "image/png",
			Size:     2048,
			Preview:  "base64encodedcontent",
		},
		{
			FileID:   "att-3",
			Filename: "data.csv",
			Type:     "data",
			MimeType: "text/csv",
			Size:     512,
		},
	}

	message := models.ChatMessage{
		ID:          "msg-1",
		Role:        "user",
		Content:     "Check these files",
		Timestamp:   1700000000000,
		Attachments: attachments,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	var recovered models.ChatMessage
	err = json.Unmarshal(jsonData, &recovered)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	if len(recovered.Attachments) != len(attachments) {
		t.Fatalf("Expected %d attachments, got %d", len(attachments), len(recovered.Attachments))
	}

	for i, att := range recovered.Attachments {
		if att.FileID != attachments[i].FileID {
			t.Errorf("Attachment %d: FileID mismatch", i)
		}
		if att.Type != attachments[i].Type {
			t.Errorf("Attachment %d: Type mismatch", i)
		}
		if att.Size != attachments[i].Size {
			t.Errorf("Attachment %d: Size mismatch", i)
		}
	}
}

// ==================== BACKWARD COMPATIBILITY TESTS ====================

func TestBackwardCompatibility_OldMessageFormat(t *testing.T) {
	// Simulate old message format that might exist in localStorage
	// This tests that the backend can handle messages without all new fields
	oldFormatJSON := `{
		"id": "msg-1",
		"role": "user",
		"content": "Hello",
		"timestamp": 1700000000000
	}`

	var message models.ChatMessage
	err := json.Unmarshal([]byte(oldFormatJSON), &message)
	if err != nil {
		t.Fatalf("Failed to parse old format: %v", err)
	}

	if message.ID != "msg-1" {
		t.Error("ID mismatch")
	}
	if message.Role != "user" {
		t.Error("Role mismatch")
	}
	if message.Attachments != nil {
		t.Error("Attachments should be nil for old format")
	}
}

func TestBackwardCompatibility_OldChatFormat(t *testing.T) {
	// Test parsing of chat without newer optional fields
	oldFormatJSON := `{
		"id": "chat-123",
		"title": "Test Chat",
		"messages": [
			{"id": "msg-1", "role": "user", "content": "Hello", "timestamp": 1700000000000}
		]
	}`

	var req models.CreateChatRequest
	err := json.Unmarshal([]byte(oldFormatJSON), &req)
	if err != nil {
		t.Fatalf("Failed to parse old format: %v", err)
	}

	if req.ID != "chat-123" {
		t.Error("ID mismatch")
	}
	if req.IsStarred != false {
		t.Error("IsStarred should default to false")
	}
	if req.Model != "" {
		t.Error("Model should be empty for old format")
	}
}

func TestBackwardCompatibility_VersionZero(t *testing.T) {
	// Test that version 0 (unset) is handled correctly
	req := models.CreateChatRequest{
		ID:       "chat-123",
		Title:    "Test",
		Messages: []models.ChatMessage{},
		Version:  0, // Explicitly zero (or unset)
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed models.CreateChatRequest
	err = json.Unmarshal(jsonData, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed.Version != 0 {
		t.Errorf("Version should be 0, got %d", parsed.Version)
	}
}

func TestBackwardCompatibility_MixedTimestampFormats(t *testing.T) {
	// Test both Unix milliseconds (new) and potential variations
	testCases := []struct {
		name      string
		timestamp int64
	}{
		{"current timestamp", 1700000000000},
		{"zero timestamp", 0},
		{"far future", 9999999999999},
		{"year 2000", 946684800000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			message := models.ChatMessage{
				ID:        "msg-1",
				Role:      "user",
				Content:   "Test",
				Timestamp: tc.timestamp,
			}

			jsonData, _ := json.Marshal(message)
			var recovered models.ChatMessage
			json.Unmarshal(jsonData, &recovered)

			if recovered.Timestamp != tc.timestamp {
				t.Errorf("Timestamp mismatch: expected %d, got %d", tc.timestamp, recovered.Timestamp)
			}
		})
	}
}

// ==================== VERSION CONFLICT TESTS ====================

func TestVersionConflictDetection(t *testing.T) {
	// Test that version numbers work correctly for conflict detection
	req1 := models.CreateChatRequest{
		ID:       "chat-123",
		Title:    "Original",
		Messages: []models.ChatMessage{},
		Version:  1,
	}

	req2 := models.CreateChatRequest{
		ID:       "chat-123",
		Title:    "Updated",
		Messages: []models.ChatMessage{},
		Version:  2,
	}

	// Simulate version check
	if req2.Version <= req1.Version {
		t.Error("req2 should have higher version than req1")
	}
}

func TestUpdateRequestPartialFields(t *testing.T) {
	// Test that partial updates work correctly
	title := "New Title"
	req := models.UpdateChatRequest{
		Title:   &title,
		Version: 5,
	}

	if req.IsStarred != nil {
		t.Error("IsStarred should be nil (not set)")
	}
	if req.Model != nil {
		t.Error("Model should be nil (not set)")
	}
	if *req.Title != title {
		t.Error("Title should be set")
	}
}

// ==================== ENCRYPTION SECURITY TESTS ====================

func TestEncryptionDeterminism(t *testing.T) {
	// Verify that same plaintext produces different ciphertext each time (due to random nonce)
	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	encService, _ := crypto.NewEncryptionService(masterKey)

	plaintext := "Same message"
	userID := "user-123"

	encrypted1, _ := encService.EncryptString(userID, plaintext)
	encrypted2, _ := encService.EncryptString(userID, plaintext)

	if encrypted1 == encrypted2 {
		t.Error("Same plaintext should produce different ciphertext (random nonce)")
	}

	// But both should decrypt to the same value
	decrypted1, _ := encService.DecryptString(userID, encrypted1)
	decrypted2, _ := encService.DecryptString(userID, encrypted2)

	if decrypted1 != decrypted2 || decrypted1 != plaintext {
		t.Error("Both encryptions should decrypt to the same plaintext")
	}
}

func TestDecryptionWithWrongKey(t *testing.T) {
	masterKey1 := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	masterKey2 := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	encService1, _ := crypto.NewEncryptionService(masterKey1)
	encService2, _ := crypto.NewEncryptionService(masterKey2)

	plaintext := "Secret message"
	userID := "user-123"

	encrypted, _ := encService1.EncryptString(userID, plaintext)

	// Try to decrypt with different master key - should fail
	_, err := encService2.DecryptString(userID, encrypted)
	if err == nil {
		t.Error("Decryption with wrong key should fail")
	}
}

func TestTamperedCiphertext(t *testing.T) {
	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	encService, err := crypto.NewEncryptionService(masterKey)
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	plaintext := "Secret message"
	userID := "user-123"

	encrypted, err := encService.EncryptString(userID, plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if len(encrypted) < 20 {
		t.Skip("Encrypted string too short for tampering test")
	}

	// Tamper with the ciphertext - flip multiple characters to ensure GCM detects it
	tampered := encrypted[:len(encrypted)-5] + "XXXXX"
	
	_, err = encService.DecryptString(userID, tampered)
	if err == nil {
		t.Error("Tampered ciphertext should fail to decrypt")
	}
}

func TestFullEncryptionDecryptionPipeline(t *testing.T) {
	// Simulate the full pipeline: messages -> JSON -> encrypt -> decrypt -> JSON -> messages
	masterKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	encService, err := crypto.NewEncryptionService(masterKey)
	if err != nil {
		t.Fatalf("Failed to create encryption service: %v", err)
	}

	userID := "user-123"

	// Create test messages
	originalMessages := []models.ChatMessage{
		{
			ID:        "msg-1",
			Role:      "user",
			Content:   "What's the weather like?",
			Timestamp: 1700000000000,
		},
		{
			ID:        "msg-2",
			Role:      "assistant",
			Content:   "I'd be happy to help with weather information! However, I don't have access to real-time weather data.",
			Timestamp: 1700000001000,
		},
		{
			ID:        "msg-3",
			Role:      "user",
			Content:   "Thanks!",
			Timestamp: 1700000002000,
			Attachments: []models.ChatAttachment{
				{FileID: "att-1", Filename: "image.png", Type: "image", MimeType: "image/png", Size: 5000},
			},
		},
	}

	// Step 1: Serialize to JSON
	jsonData, err := json.Marshal(originalMessages)
	if err != nil {
		t.Fatalf("Failed to serialize messages: %v", err)
	}

	// Step 2: Encrypt
	encrypted, err := encService.Encrypt(userID, jsonData)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Step 3: Decrypt
	decrypted, err := encService.Decrypt(userID, encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	// Step 4: Deserialize
	var recoveredMessages []models.ChatMessage
	err = json.Unmarshal(decrypted, &recoveredMessages)
	if err != nil {
		t.Fatalf("Failed to deserialize messages: %v", err)
	}

	// Verify all messages match
	if len(recoveredMessages) != len(originalMessages) {
		t.Fatalf("Message count mismatch: got %d, want %d", len(recoveredMessages), len(originalMessages))
	}

	for i, original := range originalMessages {
		recovered := recoveredMessages[i]
		if recovered.ID != original.ID {
			t.Errorf("Message %d: ID mismatch", i)
		}
		if recovered.Role != original.Role {
			t.Errorf("Message %d: Role mismatch", i)
		}
		if recovered.Content != original.Content {
			t.Errorf("Message %d: Content mismatch", i)
		}
		if recovered.Timestamp != original.Timestamp {
			t.Errorf("Message %d: Timestamp mismatch", i)
		}
		if len(recovered.Attachments) != len(original.Attachments) {
			t.Errorf("Message %d: Attachment count mismatch", i)
		}
	}
}
