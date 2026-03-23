package wechat

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the SDK configuration file.
// This follows the openclaw-weixin config structure.
type Config struct {
	BaseURL    string `json:"baseUrl,omitempty"`
	CDNBaseURL string `json:"cdnBaseUrl,omitempty"`
}

// ChannelsConfig represents the channels section in openclaw.json.
type ChannelsConfig struct {
	Weixin Config `json:"openclaw-weixin,omitempty"`
}

// OpenClawConfig represents the full openclaw.json structure.
type OpenClawConfig struct {
	Channels ChannelsConfig `json:"channels,omitempty"`
}

// DefaultConfigDir returns the default configuration directory.
// Priority: $WECHAT_ROBOT_CONFIG_DIR > ~/.config/wechat-robot > current directory
func DefaultConfigDir() string {
	if dir := os.Getenv("WECHAT_ROBOT_CONFIG_DIR"); dir != "" {
		return dir
	}
	if home, err := os.UserConfigDir(); err == nil {
		return filepath.Join(home, "wechat-robot")
	}
	return "."
}

// LoadConfig loads configuration from a JSON file.
// Supported paths:
//   - Absolute path if path starts with /
//   - Relative to current directory
//   - Default: {configDir}/config.json
func LoadConfig(paths ...string) (*Config, error) {
	var configPaths []string

	if len(paths) > 0 {
		configPaths = paths
	} else {
		configDir := DefaultConfigDir()
		configPaths = []string{
			filepath.Join(configDir, "config.json"),
			filepath.Join(configDir, "wechat.json"),
			filepath.Join(".", "config.json"),
			filepath.Join(".", "wechat.json"),
		}
	}

	for _, p := range configPaths {
		// Handle absolute paths or paths starting with ./
		if !filepath.IsAbs(p) && !filepath.IsAbs(filepath.Dir(p)) {
			// Try as-is first (relative to cwd)
			if data, err := os.ReadFile(p); err == nil {
				return parseConfig(data)
			}
		}
		// Try as absolute path
		if data, err := os.ReadFile(p); err == nil {
			return parseConfig(data)
		}
	}

	// Return default config if no file found
	return &Config{
		BaseURL:    DefaultBaseURL,
		CDNBaseURL: DefaultCDNBaseURL,
	}, nil
}

// LoadOpenClawConfig loads configuration from openclaw.json format.
func LoadOpenClawConfig(paths ...string) (*Config, error) {
	var configPaths []string

	if len(paths) > 0 {
		configPaths = paths
	} else {
		home, _ := os.UserConfigDir()
		configPaths = []string{
			filepath.Join(home, "openclaw", "openclaw.json"),
			filepath.Join(".openclaw", "openclaw.json"),
			filepath.Join(".", "openclaw.json"),
		}
	}

	for _, p := range configPaths {
		if data, err := os.ReadFile(p); err == nil {
			var openclawCfg OpenClawConfig
			if err := json.Unmarshal(data, &openclawCfg); err == nil {
				if openclawCfg.Channels.Weixin.CDNBaseURL != "" {
					return &Config{
						BaseURL:    openclawCfg.Channels.Weixin.BaseURL,
						CDNBaseURL: openclawCfg.Channels.Weixin.CDNBaseURL,
					}, nil
				}
			}
		}
	}

	// Return default config if no file found
	return &Config{
		BaseURL:    DefaultBaseURL,
		CDNBaseURL: DefaultCDNBaseURL,
	}, nil
}

func parseConfig(data []byte) (*Config, error) {
	// Try to parse as direct config
	var directCfg Config
	if err := json.Unmarshal(data, &directCfg); err == nil && directCfg.CDNBaseURL != "" {
		return &directCfg, nil
	}

	// Try to parse as openclaw.json format
	var openclawCfg OpenClawConfig
	if err := json.Unmarshal(data, &openclawCfg); err == nil {
		return &Config{
			BaseURL:    openclawCfg.Channels.Weixin.BaseURL,
			CDNBaseURL: openclawCfg.Channels.Weixin.CDNBaseURL,
		}, nil
	}

	return nil, nil
}

// SaveConfig saves configuration to a JSON file.
func SaveConfig(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
