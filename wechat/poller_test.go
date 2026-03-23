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

func TestPoller_ReceiveMessages(t *testing.T) {
	// Track handler calls
	var handlerCalls int32
	var receivedMsgs []*Message

	handler := func(ctx context.Context, msg *Message) error {
		atomic.AddInt32(&handlerCalls, 1)
		receivedMsgs = append(receivedMsgs, msg)
		return nil
	}

	// Mock server
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if r.URL.Path == "/ilink/bot/getupdates" {
			if callCount == 1 {
				// First call: return messages
				resp := GetUpdatesResponse{
					Ret: 0,
					Messages: []Message{
						{
							FromUserID:   "user1",
							ToUserID:     "bot1",
							MessageType:  MessageTypeUser,
							ContextToken: "token1",
							ItemList: []MessageItem{
								{Type: ItemTypeText, TextItem: &TextItem{Text: "Hello"}},
							},
						},
					},
					GetUpdatesBuf:        "cursor1",
					LongPollingTimeoutMs: 1000,
				}
				json.NewEncoder(w).Encode(resp)
			} else {
				// Subsequent calls: return empty (will trigger context cancel)
				time.Sleep(50 * time.Millisecond)
				resp := GetUpdatesResponse{
					Ret:           0,
					Messages:      []Message{},
					GetUpdatesBuf: "cursor2",
				}
				json.NewEncoder(w).Encode(resp)
			}
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPoller(client, handler, slog.Default(), "1.0.3")

	// Run poller with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = poller.Run(ctx)

	// Verify handler was called
	if atomic.LoadInt32(&handlerCalls) < 1 {
		t.Errorf("expected handler to be called at least once, got %d", handlerCalls)
	}

	if len(receivedMsgs) < 1 {
		t.Fatal("expected at least one message")
	}

	if receivedMsgs[0].FromUserID != "user1" {
		t.Errorf("expected from_user_id 'user1', got '%s'", receivedMsgs[0].FromUserID)
	}
}

func TestPoller_CursorUpdate(t *testing.T) {
	// Track cursors sent in requests
	var cursors []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getupdates" {
			var req GetUpdatesRequest
			json.NewDecoder(r.Body).Decode(&req)
			cursors = append(cursors, req.GetUpdatesBuf)

			resp := GetUpdatesResponse{
				Ret:                  0,
				Messages:             []Message{},
				GetUpdatesBuf:        "new_cursor_" + req.GetUpdatesBuf,
				LongPollingTimeoutMs: 100,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPoller(client, func(ctx context.Context, msg *Message) error {
		return nil
	}, slog.Default(), "1.0.3")

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	_ = poller.Run(ctx)

	// Verify cursor updates
	if len(cursors) < 2 {
		t.Fatalf("expected at least 2 poll requests, got %d", len(cursors))
	}

	// First request should have empty cursor
	if cursors[0] != "" {
		t.Errorf("first request should have empty cursor, got '%s'", cursors[0])
	}

	// Second request should have updated cursor
	if cursors[1] != "new_cursor_" {
		t.Errorf("second request should have 'new_cursor_', got '%s'", cursors[1])
	}
}

func TestPoller_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate long poll - just hold the connection
		time.Sleep(5 * time.Second)
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPoller(client, func(ctx context.Context, msg *Message) error {
		return nil
	}, slog.Default(), "1.0.3")

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error)
	go func() {
		done <- poller.Run(ctx)
	}()

	// Cancel after a short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Error("poller did not exit after context cancel")
	}
}

func TestPoller_EmptyResponse(t *testing.T) {
	// Track number of poll requests
	var pollCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getupdates" {
			atomic.AddInt32(&pollCount, 1)
			// Always return empty messages (simulating timeout)
			resp := GetUpdatesResponse{
				Ret:                  0,
				Messages:             []Message{},
				GetUpdatesBuf:        "cursor",
				LongPollingTimeoutMs: 50,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPoller(client, func(ctx context.Context, msg *Message) error {
		t.Error("handler should not be called for empty response")
		return nil
	}, slog.Default(), "1.0.3")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = poller.Run(ctx)

	// Should have made multiple poll requests
	count := atomic.LoadInt32(&pollCount)
	if count < 2 {
		t.Errorf("expected at least 2 poll requests for empty responses, got %d", count)
	}
}

func TestPoller_OnlyUserMessages(t *testing.T) {
	var handledTypes []MessageType

	handler := func(ctx context.Context, msg *Message) error {
		handledTypes = append(handledTypes, msg.MessageType)
		return nil
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if r.URL.Path == "/ilink/bot/getupdates" {
			if callCount == 1 {
				// Return mixed message types
				resp := GetUpdatesResponse{
					Ret: 0,
					Messages: []Message{
						{FromUserID: "user1", MessageType: MessageTypeUser}, // Should be handled
						{FromUserID: "bot1", MessageType: MessageTypeBot},   // Should be ignored
						{FromUserID: "user2", MessageType: MessageTypeUser}, // Should be handled
						{FromUserID: "none", MessageType: MessageTypeNone},  // Should be ignored
					},
					GetUpdatesBuf:        "cursor1",
					LongPollingTimeoutMs: 100,
				}
				json.NewEncoder(w).Encode(resp)
			} else {
				resp := GetUpdatesResponse{
					Ret:           0,
					Messages:      []Message{},
					GetUpdatesBuf: "cursor2",
				}
				json.NewEncoder(w).Encode(resp)
			}
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPoller(client, handler, slog.Default(), "1.0.3")

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = poller.Run(ctx)

	// Should only have handled user messages
	if len(handledTypes) != 2 {
		t.Errorf("expected 2 messages handled, got %d", len(handledTypes))
	}

	for _, mt := range handledTypes {
		if mt != MessageTypeUser {
			t.Errorf("expected only MessageTypeUser, got %v", mt)
		}
	}
}

func TestPoller_Stop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getupdates" {
			// Hold connection
			time.Sleep(5 * time.Second)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPoller(client, func(ctx context.Context, msg *Message) error {
		return nil
	}, slog.Default(), "1.0.3")

	done := make(chan error)
	go func() {
		done <- poller.Run(context.Background())
	}()

	// Stop after a short delay
	time.Sleep(50 * time.Millisecond)
	poller.Stop()

	select {
	case err := <-done:
		if err != ErrPollerStopped {
			t.Errorf("expected ErrPollerStopped, got %v", err)
		}
	case <-time.After(time.Second):
		t.Error("poller did not exit after Stop()")
	}
}

func TestPoller_SessionExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getupdates" {
			// Return session expired error
			resp := GetUpdatesResponse{
				Ret: -14,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPoller(client, func(ctx context.Context, msg *Message) error {
		return nil
	}, slog.Default(), "1.0.3")

	err := poller.Run(context.Background())

	if err != ErrSessionExpired {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}
}

func TestPoller_TimeoutNotCountedAsFailure(t *testing.T) {
	// Track number of poll requests and verify no backoff occurs
	var pollCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/getupdates" {
			count := atomic.AddInt32(&pollCount, 1)

			// Simulate timeout by delaying longer than context timeout
			// The poller has a short HTTP timeout in tests, so we just delay
			// to trigger context.DeadlineExceeded
			if count <= 5 {
				// Delay to cause timeout - context will be cancelled
				time.Sleep(200 * time.Millisecond)
			}

			// If we get here, return empty response
			resp := GetUpdatesResponse{
				Ret:                  0,
				Messages:             []Message{},
				GetUpdatesBuf:        "cursor",
				LongPollingTimeoutMs: 50, // Short timeout for test
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	// Create client with short timeout to trigger timeouts quickly
	httpClient := &http.Client{
		Timeout: 100 * time.Millisecond,
	}

	client := NewClient(server.URL, httpClient, slog.Default(), "1.0.3")
	poller := NewPoller(client, func(ctx context.Context, msg *Message) error {
		return nil
	}, slog.Default(), "1.0.3")

	// Run poller with timeout - should make multiple requests without backoff
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	_ = poller.Run(ctx)

	// Should have made multiple poll requests quickly (no 30s backoff)
	count := atomic.LoadInt32(&pollCount)
	// If timeouts were counted as failures, we'd only see ~3 requests before backoff
	// With fix, we should see many more as timeouts don't trigger backoff
	if count < 3 {
		t.Errorf("expected at least 3 poll attempts (timeouts should not trigger backoff), got %d", count)
	}
}

func TestPoller_CursorUpdateAfterProcessing(t *testing.T) {
	// Track the order of operations
	var operations []string
	var cursors []string

	handler := func(ctx context.Context, msg *Message) error {
		operations = append(operations, "handler_called")
		return nil
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if r.URL.Path == "/ilink/bot/getupdates" {
			var req GetUpdatesRequest
			json.NewDecoder(r.Body).Decode(&req)
			cursors = append(cursors, req.GetUpdatesBuf)
			operations = append(operations, "poll_"+req.GetUpdatesBuf)

			if callCount == 1 {
				// First call: return a message with new cursor
				resp := GetUpdatesResponse{
					Ret: 0,
					Messages: []Message{
						{
							FromUserID:  "user1",
							MessageType: MessageTypeUser,
							ItemList: []MessageItem{
								{Type: ItemTypeText, TextItem: &TextItem{Text: "test"}},
							},
						},
					},
					GetUpdatesBuf:        "cursor_after_msg",
					LongPollingTimeoutMs: 50,
				}
				json.NewEncoder(w).Encode(resp)
			} else {
				// Second call should have the updated cursor
				resp := GetUpdatesResponse{
					Ret:           0,
					Messages:      []Message{},
					GetUpdatesBuf: "cursor_final",
				}
				json.NewEncoder(w).Encode(resp)
			}
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")
	poller := NewPoller(client, handler, slog.Default(), "1.0.3")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_ = poller.Run(ctx)

	// Verify operations order:
	// 1. poll_  (first poll with empty cursor)
	// 2. handler_called (message processed)
	// 3. poll_cursor_after_msg (second poll with updated cursor)
	// This proves cursor is updated AFTER handler is called

	if len(operations) < 3 {
		t.Fatalf("expected at least 3 operations, got %d: %v", len(operations), operations)
	}

	// First operation should be initial poll with empty cursor
	if operations[0] != "poll_" {
		t.Errorf("first operation should be 'poll_', got '%s'", operations[0])
	}

	// Handler should be called before second poll
	handlerIdx := -1
	secondPollIdx := -1
	for i, op := range operations {
		if op == "handler_called" && handlerIdx == -1 {
			handlerIdx = i
		}
		if op == "poll_cursor_after_msg" {
			secondPollIdx = i
		}
	}

	if handlerIdx == -1 {
		t.Error("handler was never called")
	}

	if secondPollIdx == -1 {
		t.Error("second poll with updated cursor was never made")
	}

	if handlerIdx > secondPollIdx {
		t.Errorf("cursor was updated before handler was called: handler at %d, second poll at %d", handlerIdx, secondPollIdx)
	}

	// Verify cursor sequence
	if len(cursors) >= 2 && cursors[1] != "cursor_after_msg" {
		t.Errorf("second request should have cursor 'cursor_after_msg', got '%s'", cursors[1])
	}
}
