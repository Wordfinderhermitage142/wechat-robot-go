package wechat

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestPKCS7Pad(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		blockSize int
		wantLen   int
	}{
		{
			name:      "empty data",
			data:      []byte{},
			blockSize: 16,
			wantLen:   16,
		},
		{
			name:      "exact block size",
			data:      bytes.Repeat([]byte("a"), 16),
			blockSize: 16,
			wantLen:   32, // adds full block of padding
		},
		{
			name:      "less than block size",
			data:      []byte("hello"),
			blockSize: 16,
			wantLen:   16,
		},
		{
			name:      "one byte short of block",
			data:      bytes.Repeat([]byte("a"), 15),
			blockSize: 16,
			wantLen:   16,
		},
		{
			name:      "one byte over block",
			data:      bytes.Repeat([]byte("a"), 17),
			blockSize: 16,
			wantLen:   32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			padded := pkcs7Pad(tt.data, tt.blockSize)
			if len(padded) != tt.wantLen {
				t.Errorf("pkcs7Pad() got len %d, want %d", len(padded), tt.wantLen)
			}
			// Verify padding bytes are correct
			paddingValue := padded[len(padded)-1]
			for i := len(padded) - int(paddingValue); i < len(padded); i++ {
				if padded[i] != paddingValue {
					t.Errorf("invalid padding byte at index %d: got %d, want %d", i, padded[i], paddingValue)
				}
			}
		})
	}
}

func TestPKCS7Unpad(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantLen int
		wantErr bool
	}{
		{
			name:    "valid padding 5",
			data:    append([]byte("hello"), bytes.Repeat([]byte{11}, 11)...),
			wantLen: 5,
			wantErr: false,
		},
		{
			name:    "valid padding full block",
			data:    bytes.Repeat([]byte{16}, 16),
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "invalid padding zero",
			data:    []byte{1, 2, 3, 0},
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "invalid padding too large",
			data:    []byte{1, 2, 3, 20},
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "invalid padding inconsistent bytes",
			data:    []byte{1, 2, 3, 4, 5, 3, 3, 2}, // last byte says 2 but second-to-last is 3
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pkcs7Unpad(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("pkcs7Unpad() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(result) != tt.wantLen {
				t.Errorf("pkcs7Unpad() got len %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestAESECBRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 16) // 16-byte key

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{
			name:      "empty",
			plaintext: []byte{},
		},
		{
			name:      "single byte",
			plaintext: []byte{0x01},
		},
		{
			name:      "exactly one block",
			plaintext: bytes.Repeat([]byte{0xAB}, 16),
		},
		{
			name:      "multiple blocks",
			plaintext: bytes.Repeat([]byte{0xCD}, 48),
		},
		{
			name:      "non-multiple of block size",
			plaintext: bytes.Repeat([]byte{0xEF}, 37),
		},
		{
			name:      "large data",
			plaintext: bytes.Repeat([]byte{0x12}, 1024),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := encryptAESECB(tt.plaintext, key)
			if err != nil {
				t.Fatalf("encryptAESECB() error = %v", err)
			}

			decrypted, err := decryptAESECB(encrypted, key)
			if err != nil {
				t.Fatalf("decryptAESECB() error = %v", err)
			}

			if !bytes.Equal(decrypted, tt.plaintext) {
				t.Errorf("round trip failed: got %v, want %v", decrypted, tt.plaintext)
			}
		})
	}
}

func TestAESECBKnownVector(t *testing.T) {
	// NIST AES-128 ECB test vector
	// Key: 2b7e151628aed2a6abf7158809cf4f3c
	// Plaintext: 6bc1bee22e409f96e93d7e117393172a
	// Ciphertext: 3ad77bb40d7a3660a89ecaf32466ef97
	key, _ := hex.DecodeString("2b7e151628aed2a6abf7158809cf4f3c")
	plaintext, _ := hex.DecodeString("6bc1bee22e409f96e93d7e117393172a")
	expectedCiphertext, _ := hex.DecodeString("3ad77bb40d7a3660a89ecaf32466ef97")

	// Since our encryptAESECB adds PKCS7 padding, we need to test block-level encryption manually
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}

	ciphertext := make([]byte, len(plaintext))
	block.Encrypt(ciphertext, plaintext)

	if !bytes.Equal(ciphertext, expectedCiphertext) {
		t.Errorf("AES-128-ECB encryption mismatch:\ngot:  %x\nwant: %x", ciphertext, expectedCiphertext)
	}

	// Test decryption
	decrypted := make([]byte, len(ciphertext))
	block.Decrypt(decrypted, ciphertext)

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("AES-128-ECB decryption mismatch:\ngot:  %x\nwant: %x", decrypted, plaintext)
	}
}

func TestEncryptAESECBInvalidKey(t *testing.T) {
	tests := []struct {
		name    string
		keyLen  int
		wantErr bool
	}{
		{"valid 16 bytes", 16, false},
		{"valid 24 bytes", 24, false},
		{"valid 32 bytes", 32, false},
		{"invalid 15 bytes", 15, true},
		{"invalid 17 bytes", 17, true},
		{"invalid 0 bytes", 0, true},
		{"invalid 8 bytes", 8, true},
	}

	plaintext := []byte("test data")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := bytes.Repeat([]byte{0x42}, tt.keyLen)
			_, err := encryptAESECB(plaintext, key)
			if (err != nil) != tt.wantErr {
				t.Errorf("encryptAESECB() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecryptAESECBInvalidData(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 16)

	// Create a valid ciphertext by encrypting some data first
	validPlaintext := []byte("test")
	validCiphertext, _ := encryptAESECB(validPlaintext, key)

	tests := []struct {
		name       string
		ciphertext []byte
		wantErr    bool
	}{
		{
			name:       "valid encrypted data",
			ciphertext: validCiphertext,
			wantErr:    false,
		},
		{
			name:       "not multiple of block size",
			ciphertext: bytes.Repeat([]byte{0x01}, 17),
			wantErr:    true,
		},
		{
			name:       "empty",
			ciphertext: []byte{},
			wantErr:    true,
		},
		{
			name:       "15 bytes",
			ciphertext: bytes.Repeat([]byte{0x01}, 15),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decryptAESECB(tt.ciphertext, key)
			if (err != nil) != tt.wantErr {
				t.Errorf("decryptAESECB() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateAESKey(t *testing.T) {
	// Test key length
	key1, err := generateAESKey()
	if err != nil {
		t.Fatalf("generateAESKey() error = %v", err)
	}
	if len(key1) != 16 {
		t.Errorf("generateAESKey() key length = %d, want 16", len(key1))
	}

	// Test uniqueness
	key2, err := generateAESKey()
	if err != nil {
		t.Fatalf("generateAESKey() error = %v", err)
	}
	if bytes.Equal(key1, key2) {
		t.Error("generateAESKey() generated identical keys")
	}
}

func TestMediaManager_UploadFile(t *testing.T) {
	testData := []byte("hello world test data")

	// Mock server for getuploadurl
	var capturedUploadData []byte
	cdnServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		capturedUploadData, _ = io.ReadAll(r.Body)
		w.Header().Set("x-encrypted-param", "test-encrypted-param")
		w.WriteHeader(http.StatusOK)
	}))
	defer cdnServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getuploadurl" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ret": 0, "upload_param": "test-param"}`))
		}
	}))
	defer apiServer.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewClient(apiServer.URL, &http.Client{}, logger, "1.0.3")
	manager := NewMediaManager(client, logger)
	// Set CDN base URL separately
	manager.SetCDNBaseURL(cdnServer.URL)

	result, err := manager.UploadFile(context.Background(), testData, "test-user-id", "image")
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}

	// Verify result
	if result.AESKey == "" {
		t.Error("UploadFile() AESKey is empty")
	}
	if result.FileKey == "" {
		t.Error("UploadFile() FileKey is empty")
	}
	if result.EncryptedParam != "test-encrypted-param" {
		t.Errorf("UploadFile() EncryptedParam = %s, want test-encrypted-param", result.EncryptedParam)
	}
	if result.FileSize != len(testData) {
		t.Errorf("UploadFile() FileSize = %d, want %d", result.FileSize, len(testData))
	}

	// Verify uploaded data can be decrypted back
	aesKey, _ := hex.DecodeString(result.AESKey)
	decrypted, err := decryptAESECB(capturedUploadData, aesKey)
	if err != nil {
		t.Fatalf("decrypt captured data error = %v", err)
	}
	if !bytes.Equal(decrypted, testData) {
		t.Errorf("decrypted data mismatch: got %s, want %s", decrypted, testData)
	}
}

func TestMediaManager_UploadFileRetry(t *testing.T) {
	testData := []byte("retry test data")
	attemptCount := 0

	cdnServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			// First attempt returns 500
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
			return
		}
		// Second attempt succeeds
		w.Header().Set("x-encrypted-param", "success-param")
		w.WriteHeader(http.StatusOK)
	}))
	defer cdnServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ret": 0, "upload_param": "test-param"}`))
	}))
	defer apiServer.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewClient(apiServer.URL, &http.Client{}, logger, "1.0.3")
	manager := NewMediaManager(client, logger)
	manager.SetCDNBaseURL(cdnServer.URL)

	result, err := manager.UploadFile(context.Background(), testData, "test-user-id", "file")
	if err != nil {
		t.Fatalf("UploadFile() with retry error = %v", err)
	}

	if attemptCount != 2 {
		t.Errorf("expected 2 attempts, got %d", attemptCount)
	}
	if result.EncryptedParam != "success-param" {
		t.Errorf("EncryptedParam = %s, want success-param", result.EncryptedParam)
	}
}

func TestMediaManager_UploadFile4xxNoRetry(t *testing.T) {
	testData := []byte("4xx test data")
	attemptCount := 0

	cdnServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer cdnServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ret": 0, "upload_param": "test-param"}`))
	}))
	defer apiServer.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewClient(apiServer.URL, &http.Client{}, logger, "1.0.3")
	manager := NewMediaManager(client, logger)
	manager.SetCDNBaseURL(cdnServer.URL)

	_, err := manager.UploadFile(context.Background(), testData, "test-user-id", "file")
	if err == nil {
		t.Fatal("UploadFile() expected error for 4xx")
	}

	if attemptCount != 1 {
		t.Errorf("expected 1 attempt (no retry for 4xx), got %d", attemptCount)
	}
}

func TestMediaManager_DownloadFile(t *testing.T) {
	originalData := []byte("original test content for download")
	aesKey := bytes.Repeat([]byte{0x42}, 16)
	aesKeyHex := hex.EncodeToString(aesKey)
	// DownloadFileWithKey expects base64-encoded key
	aesKeyBase64 := base64.StdEncoding.EncodeToString([]byte(aesKeyHex))

	// Encrypt the data
	encrypted, err := encryptAESECB(originalData, aesKey)
	if err != nil {
		t.Fatalf("encrypt test data error = %v", err)
	}

	// Mock CDN server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write(encrypted)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewClient("http://unused", &http.Client{}, logger, "1.0.3")
	manager := NewMediaManager(client, logger)

	result, err := manager.DownloadFile(context.Background(), server.URL, aesKeyBase64)
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}

	if !bytes.Equal(result, originalData) {
		t.Errorf("DownloadFile() result mismatch:\ngot:  %s\nwant: %s", result, originalData)
	}
}

func TestMediaManager_DownloadFileInvalidKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(bytes.Repeat([]byte{0x01}, 16))
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewClient("http://unused", &http.Client{}, logger, "1.0.3")
	manager := NewMediaManager(client, logger)

	// Invalid hex string
	_, err := manager.DownloadFile(context.Background(), server.URL, "invalid-hex")
	if err == nil {
		t.Error("DownloadFile() expected error for invalid hex key")
	}
}

func TestBuildImageItem(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewClient("http://unused", &http.Client{}, logger, "1.0.3")
	manager := NewMediaManager(client, logger)

	result := &UploadResult{
		AESKey:         "0123456789abcdef0123456789abcdef",
		FileKey:        "fedcba9876543210fedcba9876543210",
		EncryptedParam: "test-encrypted-param",
		FileSize:       12345,
		CipherSize:     12368, // encrypted size (padded to 16 bytes)
	}

	item := manager.BuildImageItem(result, 800, 600)

	if item.Type != ItemTypeImage {
		t.Errorf("Type = %d, want %d", item.Type, ItemTypeImage)
	}
	if item.ImageItem == nil {
		t.Fatal("ImageItem is nil")
	}
	// ImageItem now uses Media field with CDNMedia struct
	if item.ImageItem.Media == nil {
		t.Fatal("ImageItem.Media is nil")
	}
	if item.ImageItem.Media.EncryptQueryParam != result.EncryptedParam {
		t.Errorf("EncryptQueryParam = %s, want %s", item.ImageItem.Media.EncryptQueryParam, result.EncryptedParam)
	}
	if item.ImageItem.Media.AESKey == "" {
		t.Error("ImageItem.Media.AESKey is empty")
	}
	// Verify AES key encoding: should be base64(hex_string), NOT base64(raw_bytes)
	// hex_string = "0123456789abcdef0123456789abcdef" (32 chars)
	// base64(hex_string) = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" (44 chars)
	expectedAESKey := "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	if item.ImageItem.Media.AESKey != expectedAESKey {
		t.Errorf("AESKey = %s, want %s (base64 of hex string)", item.ImageItem.Media.AESKey, expectedAESKey)
	}
	if item.ImageItem.Media.EncryptType != 1 {
		t.Errorf("EncryptType = %d, want 1", item.ImageItem.Media.EncryptType)
	}
	if item.ImageItem.MidSize != result.CipherSize {
		t.Errorf("MidSize = %d, want %d", item.ImageItem.MidSize, result.CipherSize)
	}
}

func TestBuildFileItem(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewClient("http://unused", &http.Client{}, logger, "1.0.3")
	manager := NewMediaManager(client, logger)

	result := &UploadResult{
		AESKey:         "fedcba9876543210fedcba9876543210",
		FileKey:        "0123456789abcdef0123456789abcdef",
		EncryptedParam: "file-encrypted-param",
		FileSize:       98765,
		CipherSize:     98776, // padded size
	}

	item := manager.BuildFileItem(result, "document.pdf")

	if item.Type != ItemTypeFile {
		t.Errorf("Type = %d, want %d", item.Type, ItemTypeFile)
	}
	if item.FileItem == nil {
		t.Fatal("FileItem is nil")
	}
	// FileItem now uses Media field
	if item.FileItem.Media == nil {
		t.Fatal("FileItem.Media is nil")
	}
	if item.FileItem.Media.EncryptQueryParam != result.EncryptedParam {
		t.Errorf("EncryptQueryParam = %s, want %s", item.FileItem.Media.EncryptQueryParam, result.EncryptedParam)
	}
	if item.FileItem.Media.AESKey == "" {
		t.Error("FileItem.Media.AESKey is empty")
	}
	if item.FileItem.FileName != "document.pdf" {
		t.Errorf("FileName = %s, want document.pdf", item.FileItem.FileName)
	}
	if item.FileItem.Length != "98765" {
		t.Errorf("Length = %s, want 98765", item.FileItem.Length)
	}
}

func TestBuildVideoItem(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewClient("http://unused", &http.Client{}, logger, "1.0.3")
	manager := NewMediaManager(client, logger)

	result := &UploadResult{
		AESKey:         "abcdef0123456789abcdef0123456789",
		FileKey:        "fedcba9876543210fedcba9876543210",
		EncryptedParam: "video-encrypted-param",
		FileSize:       1234567,
		CipherSize:     1234576, // padded size
	}

	item := manager.BuildVideoItem(result, 1920, 1080, 30000)

	if item.Type != ItemTypeVideo {
		t.Errorf("Type = %d, want %d", item.Type, ItemTypeVideo)
	}
	if item.VideoItem == nil {
		t.Fatal("VideoItem is nil")
	}
	// VideoItem now uses Media field
	if item.VideoItem.Media == nil {
		t.Fatal("VideoItem.Media is nil")
	}
	if item.VideoItem.Media.EncryptQueryParam != result.EncryptedParam {
		t.Errorf("EncryptQueryParam = %s, want %s", item.VideoItem.Media.EncryptQueryParam, result.EncryptedParam)
	}
	if item.VideoItem.Media.AESKey == "" {
		t.Error("VideoItem.Media.AESKey is empty")
	}
	if item.VideoItem.VideoSize != result.FileSize {
		t.Errorf("VideoSize = %d, want %d", item.VideoItem.VideoSize, result.FileSize)
	}
	if item.VideoItem.PlayLength != 30000 {
		t.Errorf("PlayLength = %d, want 30000", item.VideoItem.PlayLength)
	}
	if item.VideoItem.ThumbWidth != 1920 {
		t.Errorf("ThumbWidth = %d, want 1920", item.VideoItem.ThumbWidth)
	}
	if item.VideoItem.ThumbHeight != 1080 {
		t.Errorf("ThumbHeight = %d, want 1080", item.VideoItem.ThumbHeight)
	}
}
