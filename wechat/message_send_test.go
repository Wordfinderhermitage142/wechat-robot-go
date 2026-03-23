package wechat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendText(t *testing.T) {
	var receivedReq *SendMessageRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			receivedReq = &SendMessageRequest{}
			json.NewDecoder(r.Body).Decode(receivedReq)

			resp := SendMessageResponse{Ret: 0}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")

	err := SendText(context.Background(), client, "user123", "Hello World", "ctx-token-abc")
	if err != nil {
		t.Fatalf("SendText failed: %v", err)
	}

	if receivedReq == nil || receivedReq.Msg == nil {
		t.Fatal("request was not sent properly")
	}

	msg := receivedReq.Msg

	// Verify to_user_id
	if msg.ToUserID != "user123" {
		t.Errorf("expected to_user_id 'user123', got '%s'", msg.ToUserID)
	}

	// Verify message_type = bot (2)
	if msg.MessageType != MessageTypeBot {
		t.Errorf("expected message_type %d, got %d", MessageTypeBot, msg.MessageType)
	}

	// Verify message_state = finish (2)
	if msg.MessageState != MessageStateFinish {
		t.Errorf("expected message_state %d, got %d", MessageStateFinish, msg.MessageState)
	}

	// Verify context_token
	if msg.ContextToken != "ctx-token-abc" {
		t.Errorf("expected context_token 'ctx-token-abc', got '%s'", msg.ContextToken)
	}

	// Verify item_list
	if len(msg.ItemList) != 1 {
		t.Fatalf("expected 1 item, got %d", len(msg.ItemList))
	}

	item := msg.ItemList[0]
	if item.Type != ItemTypeText {
		t.Errorf("expected item type %d (text), got %d", ItemTypeText, item.Type)
	}

	if item.TextItem == nil {
		t.Fatal("text_item is nil")
	}

	if item.TextItem.Text != "Hello World" {
		t.Errorf("expected text 'Hello World', got '%s'", item.TextItem.Text)
	}
}

func TestReply(t *testing.T) {
	var receivedReq *SendMessageRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			receivedReq = &SendMessageRequest{}
			json.NewDecoder(r.Body).Decode(receivedReq)

			resp := SendMessageResponse{Ret: 0}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")

	// Create an incoming message to reply to
	incomingMsg := &Message{
		FromUserID:   "sender456",
		ToUserID:     "bot1",
		MessageType:  MessageTypeUser,
		ContextToken: "incoming-ctx-token",
		ItemList: []MessageItem{
			{Type: ItemTypeText, TextItem: &TextItem{Text: "Original message"}},
		},
	}

	err := Reply(context.Background(), client, incomingMsg, "This is my reply")
	if err != nil {
		t.Fatalf("Reply failed: %v", err)
	}

	if receivedReq == nil || receivedReq.Msg == nil {
		t.Fatal("request was not sent properly")
	}

	msg := receivedReq.Msg

	// Verify to_user_id is from_user_id of incoming message
	if msg.ToUserID != "sender456" {
		t.Errorf("expected to_user_id 'sender456', got '%s'", msg.ToUserID)
	}

	// Verify context_token from incoming message
	if msg.ContextToken != "incoming-ctx-token" {
		t.Errorf("expected context_token 'incoming-ctx-token', got '%s'", msg.ContextToken)
	}

	// Verify reply text
	if len(msg.ItemList) != 1 {
		t.Fatalf("expected 1 item, got %d", len(msg.ItemList))
	}

	if msg.ItemList[0].TextItem == nil || msg.ItemList[0].TextItem.Text != "This is my reply" {
		t.Errorf("reply text mismatch")
	}
}

func TestSendText_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			resp := SendMessageResponse{
				Ret:     -1,
				ErrCode: -1001,
				ErrMsg:  "send failed",
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")

	err := SendText(context.Background(), client, "user", "text", "token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}

	if apiErr.Code != -1001 {
		t.Errorf("expected error code -1001, got %d", apiErr.Code)
	}
}

func TestSendImage(t *testing.T) {
	var receivedReq *SendMessageRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			receivedReq = &SendMessageRequest{}
			json.NewDecoder(r.Body).Decode(receivedReq)

			resp := SendMessageResponse{Ret: 0}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")

	imageItem := &ImageItem{
		Media: &CDNMedia{
			EncryptQueryParam: "enc-param",
			AESKey:            "YWJjMTIz", // base64 encoded
			EncryptType:       1,
		},
		ThumbWidth:  800,
		ThumbHeight: 600,
	}

	err := SendImage(context.Background(), client, "user789", "img-ctx-token", imageItem)
	if err != nil {
		t.Fatalf("SendImage failed: %v", err)
	}

	if receivedReq == nil || receivedReq.Msg == nil {
		t.Fatal("request was not sent properly")
	}

	msg := receivedReq.Msg

	if len(msg.ItemList) != 1 {
		t.Fatalf("expected 1 item, got %d", len(msg.ItemList))
	}

	item := msg.ItemList[0]
	if item.Type != ItemTypeImage {
		t.Errorf("expected item type %d (image), got %d", ItemTypeImage, item.Type)
	}

	if item.ImageItem == nil {
		t.Fatal("image_item is nil")
	}

	if item.ImageItem.Media == nil {
		t.Fatal("image_item.Media is nil")
	}

	if item.ImageItem.ThumbWidth != 800 || item.ImageItem.ThumbHeight != 600 {
		t.Errorf("image dimensions mismatch")
	}
}

func TestSendFile(t *testing.T) {
	var receivedReq *SendMessageRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			receivedReq = &SendMessageRequest{}
			json.NewDecoder(r.Body).Decode(receivedReq)

			resp := SendMessageResponse{Ret: 0}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")

	fileItem := &FileItem{
		Media: &CDNMedia{
			EncryptQueryParam: "file-enc-param",
			AESKey:            "ZGVmNDU2", // base64 encoded
			EncryptType:       1,
		},
		FileName: "document.pdf",
		Length:   "54321",
	}

	err := SendFile(context.Background(), client, "user-file", "file-ctx-token", fileItem)
	if err != nil {
		t.Fatalf("SendFile failed: %v", err)
	}

	if receivedReq == nil || receivedReq.Msg == nil {
		t.Fatal("request was not sent properly")
	}

	msg := receivedReq.Msg

	if len(msg.ItemList) != 1 {
		t.Fatalf("expected 1 item, got %d", len(msg.ItemList))
	}

	item := msg.ItemList[0]
	if item.Type != ItemTypeFile {
		t.Errorf("expected item type %d (file), got %d", ItemTypeFile, item.Type)
	}

	if item.FileItem == nil {
		t.Fatal("file_item is nil")
	}

	if item.FileItem.FileName != "document.pdf" {
		t.Errorf("expected file name 'document.pdf', got '%s'", item.FileItem.FileName)
	}
}

func TestSendMessage_MultipleItems(t *testing.T) {
	var receivedReq *SendMessageRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ilink/bot/sendmessage" {
			receivedReq = &SendMessageRequest{}
			json.NewDecoder(r.Body).Decode(receivedReq)

			resp := SendMessageResponse{Ret: 0}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.Client(), slog.Default(), "1.0.3")

	items := []MessageItem{
		{Type: ItemTypeText, TextItem: &TextItem{Text: "Check out this image:"}},
		{Type: ItemTypeImage, ImageItem: &ImageItem{URL: "https://example.com/img.png"}},
	}

	err := SendMessage(context.Background(), client, "multi-user", "multi-ctx", items)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if receivedReq == nil || receivedReq.Msg == nil {
		t.Fatal("request was not sent properly")
	}

	msg := receivedReq.Msg

	if len(msg.ItemList) != 2 {
		t.Fatalf("expected 2 items, got %d", len(msg.ItemList))
	}

	if msg.ItemList[0].Type != ItemTypeText {
		t.Errorf("first item should be text")
	}

	if msg.ItemList[1].Type != ItemTypeImage {
		t.Errorf("second item should be image")
	}
}
