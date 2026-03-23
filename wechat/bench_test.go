package wechat

import (
	"testing"
)

func BenchmarkEncryptAESECB(b *testing.B) {
	key := []byte("0123456789abcdef") // 16 bytes
	data := make([]byte, 1024*1024)   // 1MB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encryptAESECB(data, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecryptAESECB(b *testing.B) {
	key := []byte("0123456789abcdef")
	data := make([]byte, 1024*1024)
	encrypted, _ := encryptAESECB(data, key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decryptAESECB(encrypted, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPKCS7Pad(b *testing.B) {
	data := make([]byte, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pkcs7Pad(data, 16)
	}
}

func BenchmarkGenerateAESKey(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generateAESKey()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateFileKey(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateFileKey()
	}
}

func BenchmarkBuildImageItem(b *testing.B) {
	logger := NewClient("http://unused", nil, nil, "1.0.3")
	manager := NewMediaManager(logger, nil)

	result := &UploadResult{
		AESKey:         "0123456789abcdef0123456789abcdef",
		FileKey:        "fedcba9876543210fedcba9876543210",
		EncryptedParam: "test-encrypted-param",
		FileSize:       12345,
		CipherSize:     12368,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.BuildImageItem(result, 800, 600)
	}
}

func BenchmarkDownloadFileWithKey(b *testing.B) {
	// This benchmark measures the decryption overhead
	key := []byte("0123456789abcdef")
	data := make([]byte, 1024)
	encrypted, _ := encryptAESECB(data, key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = decryptAESECB(encrypted, key)
	}
}
