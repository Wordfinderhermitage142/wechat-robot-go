package wechat

import (
	"errors"
	"fmt"
)

// APIError represents an error returned by the iLink Bot API.
type APIError struct {
	Code    int    `json:"errcode"`
	Message string `json:"errmsg"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ilink api error: code=%d, msg=%s", e.Code, e.Message)
}

// IsSessionExpired reports whether the error indicates a session expiration (errcode -14).
func IsSessionExpired(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == -14
	}
	return false
}

// Sentinel errors
var (
	ErrSessionExpired = &APIError{Code: -14, Message: "session expired"}
	ErrNotLoggedIn    = errors.New("wechat: not logged in")
	ErrQRCodeExpired  = errors.New("wechat: qr code expired")
	ErrPollerStopped  = errors.New("wechat: poller stopped")
)
