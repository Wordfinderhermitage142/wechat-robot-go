package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// TokenStore is the interface for persisting login credentials.
type TokenStore interface {
	// Load loads the saved credentials. Returns nil, nil if no credentials exist.
	Load() (*Credentials, error)
	// Save persists the credentials.
	Save(creds *Credentials) error
	// Clear removes saved credentials.
	Clear() error
}

// Credentials holds the login session data.
type Credentials struct {
	BotToken string `json:"bot_token"`
	BaseURL  string `json:"base_url,omitempty"`
}

// FileTokenStore implements TokenStore by persisting credentials to a JSON file.
type FileTokenStore struct {
	path string
}

// NewFileTokenStore creates a new FileTokenStore with the given file path.
func NewFileTokenStore(path string) *FileTokenStore {
	return &FileTokenStore{path: path}
}

// Load reads credentials from the JSON file.
// Returns nil, nil if the file does not exist.
func (f *FileTokenStore) Load() (*Credentials, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read token file: %w", err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}
	return &creds, nil
}

// Save writes credentials to the JSON file with 0600 permissions.
func (f *FileTokenStore) Save(creds *Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	if err := os.WriteFile(f.path, data, 0600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}
	return nil
}

// Clear removes the credentials file. Does not return an error if file doesn't exist.
func (f *FileTokenStore) Clear() error {
	err := os.Remove(f.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove token file: %w", err)
	}
	return nil
}

// QRCodeResponse is the response from GET /ilink/bot/get_bot_qrcode
type QRCodeResponse struct {
	QRCode           string `json:"qrcode"`             // QR code identifier for polling
	QRCodeImgURL     string `json:"qrcode_img_url"`     // URL of QR code image (if available)
	QRCodeImgContent string `json:"qrcode_img_content"` // base64 encoded QR image data
}

// QRCodeStatus is the response from GET /ilink/bot/get_qrcode_status
type QRCodeStatus struct {
	Status   string `json:"status"`    // "wait", "scaned", "confirmed", "expired"
	BotToken string `json:"bot_token"` // only when status=confirmed
	BaseURL  string `json:"baseurl"`   // optional, may override default API URL
}

// Auth handles QR code login and token management.
type Auth struct {
	client *Client
	store  TokenStore
	logger *slog.Logger
}

// NewAuth creates a new Auth instance.
func NewAuth(client *Client, store TokenStore, logger *slog.Logger) *Auth {
	if logger == nil {
		logger = slog.Default()
	}
	return &Auth{
		client: client,
		store:  store,
		logger: logger,
	}
}

// GetQRCode requests a new login QR code from the server.
// Returns the QR code response with the code identifier and image data.
// API: GET /ilink/bot/get_bot_qrcode?bot_type=3
func (a *Auth) GetQRCode(ctx context.Context) (*QRCodeResponse, error) {
	var resp QRCodeResponse
	if err := a.client.Get(ctx, "/ilink/bot/get_bot_qrcode?bot_type=3", &resp); err != nil {
		return nil, fmt.Errorf("get qr code: %w", err)
	}
	return &resp, nil
}

// PollQRCodeStatus polls the QR code scan status.
// Returns the status response. Caller should poll until status is "confirmed" or "expired".
// API: GET /ilink/bot/get_qrcode_status?qrcode={qrcode}
func (a *Auth) PollQRCodeStatus(ctx context.Context, qrcode string) (*QRCodeStatus, error) {
	path := fmt.Sprintf("/ilink/bot/get_qrcode_status?qrcode=%s", qrcode)
	var status QRCodeStatus
	if err := a.client.Get(ctx, path, &status); err != nil {
		return nil, fmt.Errorf("poll qr code status: %w", err)
	}
	return &status, nil
}

// ValidateCredentials validates the current credentials by making a test API call.
// Returns true if credentials are valid, false otherwise.
func (a *Auth) ValidateCredentials(ctx context.Context) (bool, error) {
	// Try a simple getupdates call to validate credentials
	// This is more reliable than getconfig which may require additional parameters
	var resp GetUpdatesResponse
	req := &GetUpdatesRequest{
		GetUpdatesBuf: "",
		BaseInfo: &BaseInfo{
			ChannelVersion: "1.0.3",
		},
	}

	if err := a.client.Post(ctx, "/ilink/bot/getupdates", req, &resp); err != nil {
		return false, err
	}

	// If we get ret=0, credentials are valid (even with empty messages)
	if resp.Ret == 0 {
		return true, nil
	}

	// If we get session expired, credentials are invalid
	if resp.Ret == -14 {
		return false, fmt.Errorf("session expired")
	}

	return false, fmt.Errorf("credential validation failed: ret=%d", resp.Ret)
}

// GetConfigResponse is the response from POST /ilink/bot/getconfig.
type GetConfigResponse struct {
	Ret          int    `json:"ret"`
	ErrCode      int    `json:"errcode,omitempty"`
	ErrMsg       string `json:"errmsg,omitempty"`
	TypingTicket string `json:"typing_ticket"`
}

// Login performs the full login flow:
// 1. Try to load existing credentials from store
// 2. If valid credentials exist, set token and return
// 3. Otherwise, get QR code, poll for scan, save credentials
// The onQRCode callback is called with QR code image data for display.
// If onQRCode is nil, the QR code URL is logged.
func (a *Auth) Login(ctx context.Context, onQRCode func(qrCodeImgContent string)) error {
	// Step 1: Try to load and validate existing credentials
	if a.store != nil {
		creds, err := a.store.Load()
		if err != nil {
			a.logger.Warn("failed to load credentials", "error", err)
		} else if creds != nil && creds.BotToken != "" {
			// Set token temporarily to validate
			a.client.SetToken(creds.BotToken)
			if creds.BaseURL != "" {
				a.client.SetBaseURL(creds.BaseURL)
			}

			// Validate credentials by making a test API call
			a.logger.Info("validating existing credentials...")
			valid, validationErr := a.ValidateCredentials(ctx)
			if validationErr != nil {
				a.logger.Warn("credential validation error", "error", validationErr)
			}
			if valid {
				a.logger.Info("using existing credentials (validated)")
				return nil
			}

			// Credentials invalid, clear and re-login
			a.logger.Info("existing credentials invalid, need to re-login")
			if err := a.store.Clear(); err != nil {
				a.logger.Warn("failed to clear invalid credentials", "error", err)
			}
		}
	}

	// Step 2: Perform QR code login
	const maxQRRetries = 3
	for attempt := 0; attempt < maxQRRetries; attempt++ {
		// Get new QR code
		qrResp, err := a.GetQRCode(ctx)
		if err != nil {
			return fmt.Errorf("get qr code (attempt %d): %w", attempt+1, err)
		}

		// Notify caller about QR code
		if onQRCode != nil {
			onQRCode(qrResp.QRCodeImgContent)
		} else if qrResp.QRCodeImgURL != "" {
			a.logger.Info("scan QR code to login", "url", qrResp.QRCodeImgURL)
		}

		// Poll for status
		status, err := a.pollUntilComplete(ctx, qrResp.QRCode)
		if err != nil {
			return err
		}

		if status.Status == "confirmed" {
			// Login successful
			creds := &Credentials{
				BotToken: status.BotToken,
				BaseURL:  status.BaseURL,
			}

			// Set token on client
			a.client.SetToken(status.BotToken)
			if status.BaseURL != "" {
				a.client.SetBaseURL(status.BaseURL)
			}

			// Save credentials
			if a.store != nil {
				if err := a.store.Save(creds); err != nil {
					a.logger.Warn("failed to save credentials", "error", err)
				}
			}

			a.logger.Info("login successful")
			return nil
		}

		// QR code expired, retry
		a.logger.Info("qr code expired, retrying", "attempt", attempt+1)
	}

	return ErrQRCodeExpired
}

// pollUntilComplete polls QR code status until confirmed or expired.
func (a *Auth) pollUntilComplete(ctx context.Context, qrcode string) (*QRCodeStatus, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			status, err := a.PollQRCodeStatus(ctx, qrcode)
			if err != nil {
				return nil, err
			}

			switch status.Status {
			case "confirmed":
				return status, nil
			case "expired":
				return status, nil
			case "scaned":
				a.logger.Info("qr code scanned, waiting for confirmation")
			case "wait":
				// Continue polling
			default:
				a.logger.Warn("unknown qr code status", "status", status.Status)
			}
		}
	}
}
