package wechat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBot(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		bot := NewBot()
		if bot.client == nil {
			t.Error("client should not be nil")
		}
		if bot.auth == nil {
			t.Error("auth should not be nil")
		}
		if bot.typing == nil {
			t.Error("typing manager should not be nil")
		}
		if bot.media == nil {
			t.Error("media manager should not be nil")
		}
		if bot.config.baseURL != DefaultBaseURL {
			t.Errorf("baseURL = %q, want %q", bot.config.baseURL, DefaultBaseURL)
		}
		if bot.config.tokenFile != DefaultTokenFile {
			t.Errorf("tokenFile = %q, want %q", bot.config.tokenFile, DefaultTokenFile)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		customLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		customHTTPClient := &http.Client{Timeout: 10 * time.Second}
		bot := NewBot(
			WithBaseURL("https://custom.api.com"),
			WithTokenFile("custom-token.json"),
			WithHTTPClient(customHTTPClient),
			WithLogger(customLogger),
			WithChannelVersion("2.0.0"),
		)
		if bot.config.baseURL != "https://custom.api.com" {
			t.Errorf("baseURL = %q, want %q", bot.config.baseURL, "https://custom.api.com")
		}
		if bot.config.tokenFile != "custom-token.json" {
			t.Errorf("tokenFile = %q, want %q", bot.config.tokenFile, "custom-token.json")
		}
		if bot.config.channelVersion != "2.0.0" {
			t.Errorf("channelVersion = %q, want %q", bot.config.channelVersion, "2.0.0")
		}
	})
}

func TestBot_RunWithoutLogin(t *testing.T) {
	bot := NewBot()
	ctx := context.Background()

	// Register a handler so that's not the error
	bot.OnMessage(func(ctx context.Context, msg *Message) error {
		return nil
	})

	err := bot.Run(ctx)
	if err != ErrNotLoggedIn {
		t.Errorf("Run without login should return ErrNotLoggedIn, got %v", err)
	}
}

func TestBot_RunWithoutHandler(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"ret": 0})
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	// Manually set token to simulate logged in state
	bot.client.SetToken("test-token")

	ctx := context.Background()
	err := bot.Run(ctx)
	if err != ErrNoHandler {
		t.Errorf("Run without handler should return ErrNoHandler, got %v", err)
	}
}

func TestBot_FullFlow(t *testing.T) {
	var handlerCalled atomic.Int32
	var receivedMsg *Message
	var replySent atomic.Int32

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			json.NewEncoder(w).Encode(QRCodeResponse{
				QRCode:           "test-qrcode",
				QRCodeImgContent: "test-image-content",
			})
		case "/ilink/bot/get_qrcode_status":
			json.NewEncoder(w).Encode(QRCodeStatus{
				Status:   "confirmed",
				BotToken: "test-bot-token",
			})
		case "/ilink/bot/getupdates":
			// Only return message on first call
			if handlerCalled.Load() == 0 {
				json.NewEncoder(w).Encode(GetUpdatesResponse{
					Ret: 0,
					Messages: []Message{
						{
							FromUserID:   "user-123",
							ToUserID:     "bot-456",
							MessageType:  MessageTypeUser,
							MessageState: MessageStateNew,
							ContextToken: "ctx-token-789",
							ItemList: []MessageItem{
								{
									Type:     ItemTypeText,
									TextItem: &TextItem{Text: "Hello Bot!"},
								},
							},
						},
					},
					GetUpdatesBuf:        "buf-1",
					LongPollingTimeoutMs: 1000, // Short timeout for test
				})
			} else {
				// Subsequent calls return empty (simulate timeout)
				json.NewEncoder(w).Encode(GetUpdatesResponse{
					Ret:                  0,
					Messages:             []Message{},
					GetUpdatesBuf:        "buf-2",
					LongPollingTimeoutMs: 1000,
				})
			}
		case "/ilink/bot/sendmessage":
			replySent.Add(1)
			json.NewEncoder(w).Encode(SendMessageResponse{Ret: 0})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create temp token file
	tmpFile, err := os.CreateTemp("", "bot-test-token-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Create bot
	bot := NewBot(
		WithBaseURL(server.URL),
		WithTokenFile(tmpFile.Name()),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Login
	var qrCodeReceived string
	err = bot.Login(ctx, func(qrCodeImgContent string) {
		qrCodeReceived = qrCodeImgContent
	})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if qrCodeReceived != "test-image-content" {
		t.Errorf("QR code content = %q, want %q", qrCodeReceived, "test-image-content")
	}
	if bot.client.Token() != "test-bot-token" {
		t.Errorf("Token = %q, want %q", bot.client.Token(), "test-bot-token")
	}

	// Register handler
	bot.OnMessage(func(ctx context.Context, msg *Message) error {
		handlerCalled.Add(1)
		receivedMsg = msg
		// Reply to the message
		return bot.Reply(ctx, msg, "Echo: "+msg.Text())
	})

	// Run in goroutine (will be cancelled by context)
	runDone := make(chan error, 1)
	go func() {
		runDone <- bot.Run(ctx)
	}()

	// Wait for handler to be called or timeout
	timeout := time.After(3 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

waitLoop:
	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for handler to be called")
		case <-ticker.C:
			if handlerCalled.Load() > 0 && replySent.Load() > 0 {
				break waitLoop
			}
		}
	}

	// Cancel context to stop bot
	cancel()

	// Wait for Run to return
	select {
	case <-runDone:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("Run did not return after context cancel")
	}

	// Verify handler was called
	if handlerCalled.Load() == 0 {
		t.Error("handler was not called")
	}

	// Verify message content
	if receivedMsg == nil {
		t.Fatal("received message is nil")
	}
	if receivedMsg.FromUserID != "user-123" {
		t.Errorf("FromUserID = %q, want %q", receivedMsg.FromUserID, "user-123")
	}
	if receivedMsg.Text() != "Hello Bot!" {
		t.Errorf("Text = %q, want %q", receivedMsg.Text(), "Hello Bot!")
	}

	// Verify reply was sent
	if replySent.Load() == 0 {
		t.Error("reply was not sent")
	}
}

func TestBot_Reply(t *testing.T) {
	var sentRequest *SendMessageRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			sentRequest = &SendMessageRequest{}
			json.NewDecoder(r.Body).Decode(sentRequest)
			json.NewEncoder(w).Encode(SendMessageResponse{Ret: 0})
		}
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	bot.client.SetToken("test-token")

	msg := &Message{
		FromUserID:   "user-123",
		ContextToken: "ctx-token-456",
	}

	ctx := context.Background()
	err := bot.Reply(ctx, msg, "Hello back!")
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}

	if sentRequest == nil {
		t.Fatal("no request sent")
	}
	if sentRequest.Msg.ToUserID != "user-123" {
		t.Errorf("ToUserID = %q, want %q", sentRequest.Msg.ToUserID, "user-123")
	}
	if sentRequest.Msg.ContextToken != "ctx-token-456" {
		t.Errorf("ContextToken = %q, want %q", sentRequest.Msg.ContextToken, "ctx-token-456")
	}
	if sentRequest.Msg.ItemList[0].TextItem.Text != "Hello back!" {
		t.Errorf("Text = %q, want %q", sentRequest.Msg.ItemList[0].TextItem.Text, "Hello back!")
	}
}

func TestBot_SendTyping(t *testing.T) {
	var typingRequest *SendTypingRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			json.NewEncoder(w).Encode(GetConfigResponse{
				Ret:          0,
				TypingTicket: "typing-ticket-123",
			})
		case "/ilink/bot/sendtyping":
			typingRequest = &SendTypingRequest{}
			json.NewDecoder(r.Body).Decode(typingRequest)
			json.NewEncoder(w).Encode(SendTypingResponse{Ret: 0})
		}
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	bot.client.SetToken("test-token")

	ctx := context.Background()
	err := bot.SendTyping(ctx, "user-123")
	if err != nil {
		t.Fatalf("SendTyping failed: %v", err)
	}

	if typingRequest == nil {
		t.Fatal("no typing request sent")
	}
	if typingRequest.ToUserID != "user-123" {
		t.Errorf("ToUserID = %q, want %q", typingRequest.ToUserID, "user-123")
	}
	if typingRequest.TypingTicket != "typing-ticket-123" {
		t.Errorf("TypingTicket = %q, want %q", typingRequest.TypingTicket, "typing-ticket-123")
	}
	if typingRequest.Status != TypingStatusStart {
		t.Errorf("Status = %d, want %d", typingRequest.Status, TypingStatusStart)
	}
}

func TestBot_StopTyping(t *testing.T) {
	var typingRequest *SendTypingRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			json.NewEncoder(w).Encode(GetConfigResponse{
				Ret:          0,
				TypingTicket: "typing-ticket-123",
			})
		case "/ilink/bot/sendtyping":
			typingRequest = &SendTypingRequest{}
			json.NewDecoder(r.Body).Decode(typingRequest)
			json.NewEncoder(w).Encode(SendTypingResponse{Ret: 0})
		}
	}))
	defer server.Close()

	bot := NewBot(WithBaseURL(server.URL))
	bot.client.SetToken("test-token")

	ctx := context.Background()
	err := bot.StopTyping(ctx, "user-456")
	if err != nil {
		t.Fatalf("StopTyping failed: %v", err)
	}

	if typingRequest == nil {
		t.Fatal("no typing request sent")
	}
	if typingRequest.ToUserID != "user-456" {
		t.Errorf("ToUserID = %q, want %q", typingRequest.ToUserID, "user-456")
	}
	if typingRequest.Status != TypingStatusStop {
		t.Errorf("Status = %d, want %d", typingRequest.Status, TypingStatusStop)
	}
}

func TestBot_Client(t *testing.T) {
	bot := NewBot()
	client := bot.Client()
	if client == nil {
		t.Error("Client() should not return nil")
	}
	if client != bot.client {
		t.Error("Client() should return the internal client")
	}
}

func TestBot_HandlerError(t *testing.T) {
	// Test that handler errors are logged but don't interrupt polling
	var handlerCallCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getupdates" {
			// Return messages a few times
			if handlerCallCount.Load() < 3 {
				json.NewEncoder(w).Encode(GetUpdatesResponse{
					Ret: 0,
					Messages: []Message{
						{
							FromUserID:   "user-123",
							MessageType:  MessageTypeUser,
							ContextToken: "ctx-token",
							ItemList: []MessageItem{
								{Type: ItemTypeText, TextItem: &TextItem{Text: "test"}},
							},
						},
					},
					LongPollingTimeoutMs: 100,
				})
			} else {
				json.NewEncoder(w).Encode(GetUpdatesResponse{
					Ret:                  0,
					LongPollingTimeoutMs: 100,
				})
			}
		}
	}))
	defer server.Close()

	// Create a discarding logger
	bot := NewBot(
		WithBaseURL(server.URL),
		WithLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))),
	)
	bot.client.SetToken("test-token")

	// Register a handler that always returns an error
	bot.OnMessage(func(ctx context.Context, msg *Message) error {
		handlerCallCount.Add(1)
		return ErrNotLoggedIn // Return some error
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Run in goroutine
	done := make(chan error, 1)
	go func() {
		done <- bot.Run(ctx)
	}()

	// Wait for multiple handler calls (proving errors don't stop polling)
	timeout := time.After(1500 * time.Millisecond)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			if handlerCallCount.Load() < 2 {
				t.Errorf("handler should be called multiple times despite errors, got %d calls", handlerCallCount.Load())
			}
			cancel()
			return
		case <-ticker.C:
			if handlerCallCount.Load() >= 2 {
				// Success - handler was called multiple times despite errors
				cancel()
				return
			}
		}
	}
}
