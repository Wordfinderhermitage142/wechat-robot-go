package wechat

import (
	"log/slog"
	"net/http"
)

const (
	DefaultBaseURL         = "https://ilinkai.weixin.qq.com"
	DefaultCDNBaseURL      = "https://novac2c.cdn.weixin.qq.com/c2c"
	DefaultTokenFile       = ".weixin-token.json"
	DefaultChannelVersion  = "1.0.3"
	DefaultContextTokenDir = ".wechat-context-tokens"
)

// Option configures a Bot instance.
type Option func(*botConfig)

type botConfig struct {
	baseURL           string
	cdnBaseURL        string
	tokenFile         string
	contextTokenDir   string
	contextTokenStore ContextTokenStore
	httpClient        *http.Client
	logger            *slog.Logger
	channelVersion    string
}

func defaultConfig() *botConfig {
	return &botConfig{
		baseURL:         DefaultBaseURL,
		cdnBaseURL:      DefaultCDNBaseURL,
		tokenFile:       DefaultTokenFile,
		contextTokenDir: DefaultContextTokenDir,
		httpClient:      &http.Client{},
		logger:          slog.Default(),
		channelVersion:  DefaultChannelVersion,
	}
}

// WithBaseURL sets the API base URL.
func WithBaseURL(url string) Option {
	return func(c *botConfig) { c.baseURL = url }
}

// WithCDNBaseURL sets the CDN base URL for media upload/download.
// Default: https://novac2c.cdn.weixin.qq.com/c2c
func WithCDNBaseURL(url string) Option {
	return func(c *botConfig) { c.cdnBaseURL = url }
}

// WithTokenFile sets the path for token persistence.
func WithTokenFile(path string) Option {
	return func(c *botConfig) { c.tokenFile = path }
}

// WithContextTokenDir sets the directory path for persisting context tokens.
// Context tokens are required for sending messages and must be persisted
// to support outbound messages even after gateway restarts.
func WithContextTokenDir(dir string) Option {
	return func(c *botConfig) { c.contextTokenDir = dir }
}

// WithContextTokenStore sets a custom context token store.
// This allows using custom storage implementations (e.g., database, Redis).
func WithContextTokenStore(store ContextTokenStore) Option {
	return func(c *botConfig) { c.contextTokenStore = store }
}

// WithHTTPClient sets a custom HTTP client.
// Note: Do not set http.Client.Timeout as it conflicts with long-polling.
// Use context-based timeouts instead. The poller uses per-request context
// timeouts which work correctly with long-polling.
func WithHTTPClient(client *http.Client) Option {
	return func(c *botConfig) {
		if client.Timeout > 0 {
			slog.Warn("wechat: HTTP client has Timeout set, this may interfere with long-polling; consider removing it")
		}
		c.httpClient = client
	}
}

// WithLogger sets the logger to use.
func WithLogger(logger *slog.Logger) Option {
	return func(c *botConfig) { c.logger = logger }
}

// WithChannelVersion sets the channel version for API requests.
func WithChannelVersion(version string) Option {
	return func(c *botConfig) { c.channelVersion = version }
}
