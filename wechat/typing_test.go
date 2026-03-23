package wechat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestTypingManager_GetConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getconfig" {
			resp := GetConfigResponse{
				Ret:          0,
				TypingTicket: "test-ticket-abc123",
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	tm := NewTypingManager(client, slog.Default())

	ticket, err := tm.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	if ticket != "test-ticket-abc123" {
		t.Errorf("expected ticket 'test-ticket-abc123', got '%s'", ticket)
	}
}

func TestTypingManager_GetConfigCache(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getconfig" {
			atomic.AddInt32(&requestCount, 1)
			resp := GetConfigResponse{
				Ret:          0,
				TypingTicket: "cached-ticket",
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	tm := NewTypingManager(client, slog.Default())

	// First call - should hit server
	ticket1, err := tm.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("first GetConfig failed: %v", err)
	}

	// Second call - should use cache
	ticket2, err := tm.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("second GetConfig failed: %v", err)
	}

	// Third call - should still use cache
	ticket3, err := tm.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("third GetConfig failed: %v", err)
	}

	// All tickets should be the same
	if ticket1 != ticket2 || ticket2 != ticket3 {
		t.Errorf("tickets should be identical: %s, %s, %s", ticket1, ticket2, ticket3)
	}

	// Should only have made one request
	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}
}

func TestTypingManager_GetConfigCacheExpiry(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getconfig" {
			count := atomic.AddInt32(&requestCount, 1)
			resp := GetConfigResponse{
				Ret:          0,
				TypingTicket: "ticket-" + string(rune('0'+count)),
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	tm := NewTypingManager(client, slog.Default())

	// First call
	_, err := tm.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("first GetConfig failed: %v", err)
	}

	// Clear cache to simulate expiry
	tm.ClearCache()

	// Second call - should hit server again
	_, err = tm.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("second GetConfig failed: %v", err)
	}

	// Should have made two requests
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("expected 2 requests after cache clear, got %d", requestCount)
	}
}

func TestTypingManager_SendTyping(t *testing.T) {
	var typingRequest *SendTypingRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			resp := GetConfigResponse{
				Ret:          0,
				TypingTicket: "typing-ticket-xyz",
			}
			json.NewEncoder(w).Encode(resp)

		case "/ilink/bot/sendtyping":
			typingRequest = &SendTypingRequest{}
			json.NewDecoder(r.Body).Decode(typingRequest)
			resp := SendTypingResponse{Ret: 0}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	tm := NewTypingManager(client, slog.Default())

	err := tm.SendTyping(context.Background(), "user123")
	if err != nil {
		t.Fatalf("SendTyping failed: %v", err)
	}

	if typingRequest == nil {
		t.Fatal("typing request was not sent")
	}

	if typingRequest.ToUserID != "user123" {
		t.Errorf("expected to_user_id 'user123', got '%s'", typingRequest.ToUserID)
	}

	if typingRequest.TypingTicket != "typing-ticket-xyz" {
		t.Errorf("expected typing_ticket 'typing-ticket-xyz', got '%s'", typingRequest.TypingTicket)
	}

	if typingRequest.Status != TypingStatusStart {
		t.Errorf("expected status %d (start), got %d", TypingStatusStart, typingRequest.Status)
	}
}

func TestTypingManager_StopTyping(t *testing.T) {
	var typingRequest *SendTypingRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			resp := GetConfigResponse{
				Ret:          0,
				TypingTicket: "stop-ticket",
			}
			json.NewEncoder(w).Encode(resp)

		case "/ilink/bot/sendtyping":
			typingRequest = &SendTypingRequest{}
			json.NewDecoder(r.Body).Decode(typingRequest)
			resp := SendTypingResponse{Ret: 0}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	tm := NewTypingManager(client, slog.Default())

	err := tm.StopTyping(context.Background(), "user456")
	if err != nil {
		t.Fatalf("StopTyping failed: %v", err)
	}

	if typingRequest == nil {
		t.Fatal("typing request was not sent")
	}

	if typingRequest.ToUserID != "user456" {
		t.Errorf("expected to_user_id 'user456', got '%s'", typingRequest.ToUserID)
	}

	if typingRequest.Status != TypingStatusStop {
		t.Errorf("expected status %d (stop), got %d", TypingStatusStop, typingRequest.Status)
	}
}

func TestTypingManager_GetConfigError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getconfig" {
			resp := GetConfigResponse{
				Ret:    -1,
				ErrMsg: "config error",
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	tm := NewTypingManager(client, slog.Default())

	_, err := tm.GetConfig(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}

	if apiErr.Code != -1 {
		t.Errorf("expected error code -1, got %d", apiErr.Code)
	}
}

func TestTypingManager_SendTypingError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			resp := GetConfigResponse{
				Ret:          0,
				TypingTicket: "ticket",
			}
			json.NewEncoder(w).Encode(resp)

		case "/ilink/bot/sendtyping":
			resp := SendTypingResponse{
				Ret:    -2,
				ErrMsg: "typing error",
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	tm := NewTypingManager(client, slog.Default())

	err := tm.SendTyping(context.Background(), "user")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}

	if apiErr.Code != -2 {
		t.Errorf("expected error code -2, got %d", apiErr.Code)
	}
}

func TestTypingManager_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	tm := NewTypingManager(client, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := tm.GetConfig(ctx)
	if err == nil {
		t.Fatal("expected error due to context timeout")
	}
}
