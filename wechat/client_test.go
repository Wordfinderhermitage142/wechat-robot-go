package wechat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestGenerateUIN(t *testing.T) {
	uin := generateUIN()
	if uin == "" {
		t.Fatal("generateUIN returned empty string")
	}
	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(uin)
	if err != nil {
		t.Fatalf("generateUIN returned invalid base64: %v", err)
	}
	// Decoded should be a decimal number string
	_, err = strconv.ParseUint(string(decoded), 10, 64)
	if err != nil {
		t.Fatalf("decoded UIN is not a valid number: %v", err)
	}

	// Should be different each time (probabilistic)
	uin2 := generateUIN()
	if uin == uin2 {
		t.Log("warning: two consecutive UINs are the same (unlikely but possible)")
	}
}

func TestClientHeaders(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ret": 0})
	}))
	defer server.Close()

	client := NewClient(server.URL, http.DefaultClient, nil, "1.0.3")
	client.SetToken("test-token-123")

	var result map[string]interface{}
	err := client.Post(context.Background(), "/test", map[string]string{"key": "val"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify headers
	if ct := capturedHeaders.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if at := capturedHeaders.Get("AuthorizationType"); at != "ilink_bot_token" {
		t.Errorf("AuthorizationType = %q, want ilink_bot_token", at)
	}
	if auth := capturedHeaders.Get("Authorization"); auth != "Bearer test-token-123" {
		t.Errorf("Authorization = %q, want Bearer test-token-123", auth)
	}
	if uin := capturedHeaders.Get("X-Wechat-Uin"); uin == "" {
		t.Error("X-WECHAT-UIN header is missing")
	}
}

func TestClientGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ret": 0, "data": "hello"})
	}))
	defer server.Close()

	client := NewClient(server.URL, http.DefaultClient, nil, "1.0.3")
	var result map[string]interface{}
	err := client.Get(context.Background(), "/test", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["data"] != "hello" {
		t.Errorf("data = %v, want hello", result["data"])
	}
}

func TestClientHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, http.DefaultClient, nil, "1.0.3")
	var result map[string]interface{}
	err := client.Get(context.Background(), "/test", &result)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
