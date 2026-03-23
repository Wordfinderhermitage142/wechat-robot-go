package wechat

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const (
	// TypingStatusStart indicates typing has started.
	TypingStatusStart = 1
	// TypingStatusStop indicates typing has stopped.
	TypingStatusStop = 2

	// ticketCacheDuration is how long to cache the typing_ticket.
	ticketCacheDuration = 24 * time.Hour
)

// TypingManager manages "typing" status indicators.
type TypingManager struct {
	client       *Client
	logger       *slog.Logger
	typingTicket string    // cached typing_ticket from getconfig
	ticketExpiry time.Time // cache for 24 hours
	mu           sync.RWMutex
}

// SendTypingRequest is the request body for POST /ilink/bot/sendtyping.
type SendTypingRequest struct {
	ToUserID     string `json:"to_user_id"`
	TypingTicket string `json:"typing_ticket"`
	Status       int    `json:"status"` // 1=typing, 2=cancel
}

// SendTypingResponse is the response from POST /ilink/bot/sendtyping.
type SendTypingResponse struct {
	Ret     int    `json:"ret"`
	ErrCode int    `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

// NewTypingManager creates a new TypingManager instance.
func NewTypingManager(client *Client, logger *slog.Logger) *TypingManager {
	return &TypingManager{
		client: client,
		logger: logger,
	}
}

// GetConfig fetches the typing_ticket from the server.
// Caches the ticket for 24 hours.
func (tm *TypingManager) GetConfig(ctx context.Context) (string, error) {
	// Check cache first
	tm.mu.RLock()
	if tm.typingTicket != "" && time.Now().Before(tm.ticketExpiry) {
		ticket := tm.typingTicket
		tm.mu.RUnlock()
		return ticket, nil
	}
	tm.mu.RUnlock()

	// Fetch from server
	var resp GetConfigResponse
	if err := tm.client.Post(ctx, "/ilink/bot/getconfig", struct{}{}, &resp); err != nil {
		return "", err
	}

	if resp.Ret != 0 {
		return "", &APIError{Code: resp.Ret, Message: resp.ErrMsg}
	}

	// Cache the ticket
	tm.mu.Lock()
	tm.typingTicket = resp.TypingTicket
	tm.ticketExpiry = time.Now().Add(ticketCacheDuration)
	tm.mu.Unlock()

	return resp.TypingTicket, nil
}

// SendTyping sends a "typing" indicator to the user.
func (tm *TypingManager) SendTyping(ctx context.Context, toUserID string) error {
	ticket, err := tm.GetConfig(ctx)
	if err != nil {
		return err
	}

	req := &SendTypingRequest{
		ToUserID:     toUserID,
		TypingTicket: ticket,
		Status:       TypingStatusStart,
	}

	var resp SendTypingResponse
	if err := tm.client.Post(ctx, "/ilink/bot/sendtyping", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.Ret, Message: resp.ErrMsg}
	}

	return nil
}

// StopTyping cancels the "typing" indicator.
func (tm *TypingManager) StopTyping(ctx context.Context, toUserID string) error {
	ticket, err := tm.GetConfig(ctx)
	if err != nil {
		return err
	}

	req := &SendTypingRequest{
		ToUserID:     toUserID,
		TypingTicket: ticket,
		Status:       TypingStatusStop,
	}

	var resp SendTypingResponse
	if err := tm.client.Post(ctx, "/ilink/bot/sendtyping", req, &resp); err != nil {
		return err
	}

	if resp.Ret != 0 {
		return &APIError{Code: resp.Ret, Message: resp.ErrMsg}
	}

	return nil
}

// ClearCache clears the cached typing ticket, forcing a refresh on next call.
func (tm *TypingManager) ClearCache() {
	tm.mu.Lock()
	tm.typingTicket = ""
	tm.ticketExpiry = time.Time{}
	tm.mu.Unlock()
}
