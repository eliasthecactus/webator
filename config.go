package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// WaitOverride allows per-step timeout configuration.
type WaitOverride struct {
	TimeoutMs int `json:"timeout_ms"`
}

// DestinationURL represents a single navigable URL inside a Destination category.
// Fields left empty fall back to the enclosing Destination, then to the root Config.
type DestinationURL struct {
	Label             string `json:"label"`
	Tag               string `json:"tag"`
	AuthStartURL      string `json:"auth_start_url"`
	AuthDoneURL       string `json:"auth_done_url"`
	NavigateURL       string `json:"navigate_url"`
	UsernameSelector  string `json:"username_selector"`
	UsernameValue     string `json:"username_value"`
	PasswordSelector  string `json:"password_selector"`
	PasswordValue     string `json:"password_value"`
	TOTPSecret        string `json:"totp_secret"`
	TOTPSelector      string `json:"totp_selector"`
	TOTPStep          int    `json:"totp_step"`
	SubmitSelector    string `json:"submit_selector"`
	DoneSelector      string `json:"done_selector"`
	WaitAfterSubmitMs int    `json:"wait_after_submit_ms"`
}

// Destination groups one or more URLs under a named category.
// Selector/credential fields here apply to every URL in the group unless
// overridden at the DestinationURL level.
type Destination struct {
	Name              string           `json:"name"`
	Tag               string           `json:"tag"`
	UsernameSelector  string           `json:"username_selector"`
	UsernameValue     string           `json:"username_value"`
	PasswordSelector  string           `json:"password_selector"`
	PasswordValue     string           `json:"password_value"`
	TOTPSecret        string           `json:"totp_secret"`
	TOTPSelector      string           `json:"totp_selector"`
	TOTPStep          int              `json:"totp_step"`
	SubmitSelector    string           `json:"submit_selector"`
	DoneSelector      string           `json:"done_selector"`
	WaitAfterSubmitMs int              `json:"wait_after_submit_ms"`
	URLs              []DestinationURL `json:"urls"`
}

// Config holds all configuration for the automation run.
type Config struct {
	// Authentication URLs
	AuthStartURL string `json:"auth_start_url"`
	AuthDoneURL  string `json:"auth_done_url"`
	NavigateURL  string `json:"navigate_url"`

	// Credential selectors and values
	UsernameSelector string `json:"username_selector"`
	UsernameValue    string `json:"username_value"`
	PasswordSelector string `json:"password_selector"`
	PasswordValue    string `json:"password_value"`

	// TOTP configuration
	TOTPSecret   string `json:"totp_secret"`
	TOTPSelector string `json:"totp_selector"`
	TOTPStep     int    `json:"totp_step"` // 1 = before first submit, 2 = after first submit (default)

	// Form submit selectors
	SubmitSelector string `json:"submit_selector"`
	DoneSelector   string `json:"done_selector"`

	// Browser settings
	BrowserPath                 string `json:"browser_path"`
	Headless                    bool   `json:"headless"`
	ViewportWidth               int    `json:"viewport_width"`
	ViewportHeight              int    `json:"viewport_height"`
	UserAgent                   string `json:"user_agent"`
	Kiosk                       bool   `json:"kiosk"`
	AppMode                     bool   `json:"app_mode"`
	Webview                     bool   `json:"webview"`
	Incognito                   bool   `json:"incognito"`
	DisableContextMenu          bool   `json:"disable_context_menu"`
	DisableDevTools             bool   `json:"disable_dev_tools"`
	DisableTranslate            bool   `json:"disable_translate"`
	DisablePinch                bool   `json:"disable_pinch"`
	DisableTouchAdjustment      bool   `json:"disable_touch_adjustment"`
	KioskPrinting               bool   `json:"kiosk_printing"`
	OverscrollHistoryNavigation int    `json:"overscroll_history_navigation"`
	PullToRefresh               int    `json:"pull_to_refresh"`
	DisableFeatures             string `json:"disable_features"`
	EdgeKioskType               string `json:"edge_kiosk_type"`
	NoFirstRun                  bool   `json:"no_first_run"`
	NoDefaultBrowserCheck       bool   `json:"no_default_browser_check"`
	WebviewTitle                string `json:"webview_title"`

	// Network settings
	Proxy            string `json:"proxy"`
	IgnoreCertErrors bool   `json:"ignore_cert_errors"`

	// Timing and retry settings
	Timeout      int `json:"timeout"` // seconds
	RetryCount   int `json:"retry_count"`
	RetryDelayMs int `json:"retry_delay_ms"`

	// Logging
	LogLevel string `json:"log_level"`
	LogFile  string `json:"log_file"`

	// Wait tuning
	WaitAfterSubmitMs int                     `json:"wait_after_submit_ms"`
	PollIntervalMs    int                     `json:"poll_interval_ms"`
	WaitOverrides     map[string]WaitOverride `json:"wait_overrides"`

	// Multi-destination mode — when set, the user picks a destination before
	// the auth flow starts. AuthStartURL (and the selector/credential fields)
	// are populated from the chosen destination, overriding root-level values.
	Destinations []Destination `json:"destinations"`
}

// defaultConfig returns a Config populated with safe, sensible defaults.
func defaultConfig() Config {
	return Config{
		TOTPStep:                    2,
		Headless:                    false,
		ViewportWidth:               1920,
		ViewportHeight:              1080,
		UserAgent:                   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		Kiosk:                       false,
		AppMode:                     true,
		Webview:                     false,
		Incognito:                   false,
		DisableContextMenu:          true,
		DisableDevTools:             true,
		DisableTranslate:            true,
		DisablePinch:                true,
		DisableTouchAdjustment:      true,
		KioskPrinting:               true,
		OverscrollHistoryNavigation: 0,
		PullToRefresh:               0,
		DisableFeatures:             "DevTools",
		EdgeKioskType:               "fullscreen",
		NoFirstRun:                  true,
		NoDefaultBrowserCheck:       true,
		Timeout:                     60,
		RetryCount:                  3,
		RetryDelayMs:                1500,
		LogLevel:                    "info",
		LogFile:                     filepath.Join(os.TempDir(), "browser-automation.log"),
		PollIntervalMs:              250,
	}
}

// loadConfig reads the JSON config file at path into a Config.
// If the file does not exist, it returns the defaults without error.
func loadConfig(path string) (Config, error) {
	cfg := defaultConfig()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	// Unmarshal into a temporary map so we only override keys that are
	// explicitly present in the file, leaving defaults intact for absent keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg, err
	}

	// Re-marshal raw back into the struct, layering on top of defaults.
	merged, err := json.Marshal(raw)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(merged, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}
