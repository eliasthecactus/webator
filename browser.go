package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/chromedp/chromedp"
)

// chromePaths returns the list of candidate Chrome executable paths for the
// current operating system, in priority order.
func chromePaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"/usr/bin/google-chrome",
			"/usr/local/bin/google-chrome",
		}
	case "windows":
		return []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			os.Getenv("LOCALAPPDATA") + `\Google\Chrome\Application\chrome.exe`,
		}
	default: // linux and others
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
			"/usr/local/bin/google-chrome",
		}
	}
}

// edgePaths returns the list of candidate Microsoft Edge executable paths for
// the current operating system, in priority order.
func edgePaths() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "windows":
		return []string{
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		}
	default: // linux
		return []string{
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
			"/usr/bin/microsoft-edge-beta",
		}
	}
}

// findBrowser locates a suitable browser binary to use for automation.
//
//   - If cfg.BrowserPath is set, it is verified to exist and returned directly.
//   - Otherwise, Chrome candidates are tried first, followed by Edge candidates.
//   - An error is returned if no usable binary is found.
func findBrowser(cfg *Config, logger *slog.Logger) (string, error) {
	if cfg.BrowserPath != "" {
		if _, err := os.Stat(cfg.BrowserPath); err != nil {
			return "", fmt.Errorf("specified browser path not found: %s", cfg.BrowserPath)
		}
		logger.Info("using configured browser path", slog.String("path", cfg.BrowserPath))
		return cfg.BrowserPath, nil
	}

	candidates := append(chromePaths(), edgePaths()...)
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			logger.Info("browser detected", slog.String("path", p))
			return p, nil
		}
	}

	return "", fmt.Errorf("no supported browser found; install Chrome or Edge, or set --browser-path")
}

// launchBrowser starts a browser instance using the given path and
// configuration, and returns the chromedp context along with a combined cancel
// function that tears down both the browser allocator and the chromedp context.
func launchBrowser(parentCtx context.Context, cfg *Config, browserPath string, logger *slog.Logger) (context.Context, context.CancelFunc, error) {
	// Build allocator options explicitly so we can conditionally exclude
	// chromedp.Headless. Using Flag("headless", false) on top of
	// DefaultExecAllocatorOptions is not reliable across chromedp versions.
	allocOpts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		// chromedp.Headless is added below only when cfg.Headless == true.
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-features", "site-per-process,Translate,BlinkGenPropertyTrees"),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
	}

	if cfg.Headless {
		allocOpts = append(allocOpts, chromedp.Headless)
	}

	allocOpts = append(allocOpts,
		chromedp.ExecPath(browserPath),
		chromedp.UserAgent(cfg.UserAgent),
		chromedp.Flag("ignore-certificate-errors", cfg.IgnoreCertErrors),
	)
	if cfg.Headless {
		allocOpts = append(allocOpts, chromedp.WindowSize(cfg.ViewportWidth, cfg.ViewportHeight))
	} else {
		allocOpts = append(allocOpts, chromedp.Flag("start-maximized", true))
	}

	if cfg.Proxy != "" {
		allocOpts = append(allocOpts, chromedp.ProxyServer(cfg.Proxy))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(parentCtx, allocOpts...)

	cdpCtx, cdpCancel := chromedp.NewContext(allocCtx)

	// Combined cancel: tear down chromedp context first, then the allocator.
	combinedCancel := func() {
		cdpCancel()
		allocCancel()
	}

	// Initialise the browser; this also ensures the browser process actually
	// starts so we can report a meaningful error here.
	// EmulateViewport is a headless-only override — in non-headless mode the
	// window size is already set via WindowSize above, and EmulateViewport
	// would cause content to appear cut off.
	var initAction chromedp.Action
	if cfg.Headless {
		initAction = chromedp.EmulateViewport(int64(cfg.ViewportWidth), int64(cfg.ViewportHeight))
	} else {
		initAction = chromedp.Navigate("about:blank")
	}
	if err := chromedp.Run(cdpCtx, initAction); err != nil {
		combinedCancel()
		return nil, nil, fmt.Errorf("browser initialisation failed: %w", err)
	}

	logger.Info("browser launched",
		slog.String("path", browserPath),
		slog.Bool("headless", cfg.Headless),
		slog.Int("viewportWidth", cfg.ViewportWidth),
		slog.Int("viewportHeight", cfg.ViewportHeight),
	)

	return cdpCtx, combinedCancel, nil
}
