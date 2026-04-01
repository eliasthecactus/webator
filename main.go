package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// ── Flag definitions ───────────────────────────────────────────────────
	configPath := flag.String("config", "", "Path to JSON config file (optional)")
	debug := flag.Bool("debug", false, "Enable debug mode: verbose logging to stdout, browser visible")

	// URL flags
	authStartURL := flag.String("auth-start-url", "", "URL to start authentication flow")
	authDoneURL := flag.String("auth-done-url", "", "URL fragment that indicates authentication success")
	navigateURL := flag.String("navigate-url", "", "URL to navigate to after successful authentication")

	// Credential flags
	usernameSelector := flag.String("username-selector", "", "CSS/XPath selector for the username input")
	usernameValue := flag.String("username-value", "", "Value to enter in the username field")
	passwordSelector := flag.String("password-selector", "", "CSS/XPath selector for the password input")
	passwordValue := flag.String("password-value", "", "Value to enter in the password field")

	// TOTP flags
	totpSecret := flag.String("totp-secret", "", "Base32-encoded TOTP secret")
	totpSelector := flag.String("totp-selector", "", "CSS/XPath selector for the TOTP input")
	totpStep := flag.Int("totp-step", 0, "TOTP step: 1=before first submit, 2=after first submit (default 2)")

	// Form flags
	submitSelector := flag.String("submit-selector", "", "CSS/XPath selector for the submit button")
	doneSelector := flag.String("done-selector", "", "CSS/XPath selector that confirms login success")

	// Browser flags
	browserPath := flag.String("browser-path", "", "Explicit path to Chrome/Edge binary")
	headless := flag.Bool("headless", true, "Run browser in headless mode (default true; overridden to false in debug mode)")
	viewportWidth := flag.Int("viewport-width", 0, "Browser viewport width in pixels")
	viewportHeight := flag.Int("viewport-height", 0, "Browser viewport height in pixels")
	userAgent := flag.String("user-agent", "", "Browser User-Agent string")
	kiosk := flag.Bool("kiosk", false, "Run the browser in kiosk mode")
	incognito := flag.Bool("incognito", false, "Run the browser in an incognito/private session")
	disableContextMenu := flag.Bool("disable-context-menu", true, "Disable the browser context menu")
	disableDevTools := flag.Bool("disable-dev-tools", true, "Prevent opening developer tools")
	disableTranslate := flag.Bool("disable-translate", true, "Disable browser translation prompts")
	disablePinch := flag.Bool("disable-pinch", true, "Disable pinch/zoom gestures")
	overscrollHistoryNavigation := flag.Int("overscroll-history-navigation", 0, "Overscroll history navigation setting")
	pullToRefresh := flag.Int("pull-to-refresh", 0, "Pull-to-refresh setting")
	disableTouchAdjustment := flag.Bool("disable-touch-adjustment", true, "Disable touch adjustment UI")
	kioskPrinting := flag.Bool("kiosk-printing", true, "Enable kiosk-friendly printing behavior")
	disableFeatures := flag.String("disable-features", "", "Additional browser features to disable")
	edgeKioskType := flag.String("edge-kiosk-type", "", "Edge kiosk type to use when running in kiosk mode")
	noFirstRun := flag.Bool("no-first-run", true, "Disable browser first-run checks")
	noDefaultBrowserCheck := flag.Bool("no-default-browser-check", true, "Disable default browser check")

	// Network flags
	proxy := flag.String("proxy", "", "HTTP/HTTPS proxy URL (e.g. http://proxy:8080)")
	ignoreCertErrors := flag.Bool("ignore-cert-errors", false, "Ignore TLS certificate errors")

	// Timing and retry flags
	timeout := flag.Int("timeout", 0, "Global timeout in seconds (default 60)")
	retryCount := flag.Int("retry-count", 0, "Number of retry attempts on failure (default 3)")
	retryDelayMs := flag.Int("retry-delay-ms", 0, "Delay in milliseconds between retries (default 1500)")

	// Logging flags
	logLevel := flag.String("log-level", "", "Log level: debug, info, warn, error (default info)")
	logFile := flag.String("log-file", "", "Path to the JSON log file")

	// Wait tuning flags
	waitAfterSubmitMs := flag.Int("wait-after-submit-ms", 0, "Milliseconds to wait after clicking submit")
	pollIntervalMs := flag.Int("poll-interval-ms", 0, "Milliseconds between element visibility polls (default 250)")

	flag.Parse()

	// Track which flags were explicitly set on the command line.
	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	// ── Load config file (optional) ────────────────────────────────────────
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// ── Apply explicit CLI overrides ───────────────────────────────────────
	if setFlags["auth-start-url"] {
		cfg.AuthStartURL = *authStartURL
	}
	if setFlags["auth-done-url"] {
		cfg.AuthDoneURL = *authDoneURL
	}
	if setFlags["navigate-url"] {
		cfg.NavigateURL = *navigateURL
	}
	if setFlags["username-selector"] {
		cfg.UsernameSelector = *usernameSelector
	}
	if setFlags["username-value"] {
		cfg.UsernameValue = *usernameValue
	}
	if setFlags["password-selector"] {
		cfg.PasswordSelector = *passwordSelector
	}
	if setFlags["password-value"] {
		cfg.PasswordValue = *passwordValue
	}
	if setFlags["totp-secret"] {
		cfg.TOTPSecret = *totpSecret
	}
	if setFlags["totp-selector"] {
		cfg.TOTPSelector = *totpSelector
	}
	if setFlags["totp-step"] {
		cfg.TOTPStep = *totpStep
	}
	if setFlags["submit-selector"] {
		cfg.SubmitSelector = *submitSelector
	}
	if setFlags["done-selector"] {
		cfg.DoneSelector = *doneSelector
	}
	if setFlags["browser-path"] {
		cfg.BrowserPath = *browserPath
	}
	if setFlags["headless"] {
		cfg.Headless = *headless
	}
	if setFlags["viewport-width"] {
		cfg.ViewportWidth = *viewportWidth
	}
	if setFlags["viewport-height"] {
		cfg.ViewportHeight = *viewportHeight
	}
	if setFlags["user-agent"] {
		cfg.UserAgent = *userAgent
	}
	if setFlags["kiosk"] {
		cfg.Kiosk = *kiosk
	}
	if setFlags["incognito"] {
		cfg.Incognito = *incognito
	}
	if setFlags["disable-context-menu"] {
		cfg.DisableContextMenu = *disableContextMenu
	}
	if setFlags["disable-dev-tools"] {
		cfg.DisableDevTools = *disableDevTools
	}
	if setFlags["disable-translate"] {
		cfg.DisableTranslate = *disableTranslate
	}
	if setFlags["disable-pinch"] {
		cfg.DisablePinch = *disablePinch
	}
	if setFlags["overscroll-history-navigation"] {
		cfg.OverscrollHistoryNavigation = *overscrollHistoryNavigation
	}
	if setFlags["pull-to-refresh"] {
		cfg.PullToRefresh = *pullToRefresh
	}
	if setFlags["disable-touch-adjustment"] {
		cfg.DisableTouchAdjustment = *disableTouchAdjustment
	}
	if setFlags["kiosk-printing"] {
		cfg.KioskPrinting = *kioskPrinting
	}
	if setFlags["disable-features"] {
		cfg.DisableFeatures = *disableFeatures
	}
	if setFlags["edge-kiosk-type"] {
		cfg.EdgeKioskType = *edgeKioskType
	}
	if setFlags["no-first-run"] {
		cfg.NoFirstRun = *noFirstRun
	}
	if setFlags["no-default-browser-check"] {
		cfg.NoDefaultBrowserCheck = *noDefaultBrowserCheck
	}
	if setFlags["proxy"] {
		cfg.Proxy = *proxy
	}
	if setFlags["ignore-cert-errors"] {
		cfg.IgnoreCertErrors = *ignoreCertErrors
	}
	if setFlags["timeout"] {
		cfg.Timeout = *timeout
	}
	if setFlags["retry-count"] {
		cfg.RetryCount = *retryCount
	}
	if setFlags["retry-delay-ms"] {
		cfg.RetryDelayMs = *retryDelayMs
	}
	if setFlags["log-level"] {
		cfg.LogLevel = *logLevel
	}
	if setFlags["log-file"] {
		cfg.LogFile = *logFile
	}
	if setFlags["wait-after-submit-ms"] {
		cfg.WaitAfterSubmitMs = *waitAfterSubmitMs
	}
	if setFlags["poll-interval-ms"] {
		cfg.PollIntervalMs = *pollIntervalMs
	}

	// ── Validate required fields ───────────────────────────────────────────
	if cfg.AuthStartURL == "" {
		fmt.Fprintln(os.Stderr, "error: --auth-start-url is required (or set auth_start_url in config file)")
		os.Exit(1)
	}

	// ── Find browser ───────────────────────────────────────────────────────
	browserExec, err := findBrowser(&cfg, slog.Default())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if cfg.Headless {
		runHeadless(&cfg, *debug, browserExec)
	} else {
		runGUI(&cfg, *debug, browserExec)
	}
}

func runHeadless(cfg *Config, debug bool, browserExec string) {
	logger, logCleanup, err := setupLogger(cfg, debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error setting up logger: %v\n", err)
		os.Exit(1)
	}
	defer logCleanup()
	slog.SetDefault(logger)

	logger.Info("webator starting",
		slog.String("authStartUrl", cfg.AuthStartURL),
		slog.String("mode", determineMode(cfg).String()),
		slog.Bool("headless", true),
	)

	baseCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	browserCtx, cancelBrowser, err := launchBrowser(baseCtx, cfg, browserExec, logger)
	if err != nil {
		logger.Error("failed to launch browser", slog.Any("error", err))
		os.Exit(1)
	}
	defer cancelBrowser()

	authCtx := browserCtx
	if cfg.Timeout > 0 {
		var cancelAuth context.CancelFunc
		authCtx, cancelAuth = context.WithTimeout(browserCtx, time.Duration(cfg.Timeout)*time.Second)
		defer cancelAuth()
	}

	if err := runAuth(authCtx, cfg, logger, func(string) {}); err != nil {
		logger.Error("authentication failed", slog.Any("error", err))
		os.Exit(1)
	}

	if determineMode(cfg) == modeFullAuto {
		logger.Info("browser ready — press Ctrl+C to exit")
		<-baseCtx.Done()
	}

	logger.Info("webator exiting")
}

func runGUI(cfg *Config, debug bool, browserExec string) {
	gui := NewWebatorGUI(cfg, debug)

	var logger *slog.Logger
	var logCleanup func()
	if debug {
		logger = gui.Logger()
		logCleanup = func() {}
	} else {
		var err error
		logger, logCleanup, err = setupLogger(cfg, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error setting up logger: %v\n", err)
			os.Exit(1)
		}
	}
	defer logCleanup()
	slog.SetDefault(logger)

	logger.Info("webator starting",
		slog.String("authStartUrl", cfg.AuthStartURL),
		slog.String("mode", determineMode(cfg).String()),
		slog.Bool("debug", debug),
	)

	baseCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	gui.Run(stopSignal, func() error {
		gui.SetStatus("Starting browser...")

		browserCtx, cancelBrowser, err := launchBrowser(baseCtx, cfg, browserExec, logger)
		if err != nil {
			return err
		}
		gui.AddCleanup(cancelBrowser)

		authCtx := browserCtx
		if cfg.Timeout > 0 {
			var cancelAuth context.CancelFunc
			authCtx, cancelAuth = context.WithTimeout(browserCtx, time.Duration(cfg.Timeout)*time.Second)
			gui.AddCleanup(cancelAuth)
		}

		if err := runAuth(authCtx, cfg, logger, gui.SetStatus); err != nil {
			return err
		}
		gui.WatchBrowser(browserCtx, baseCtx)
		return nil
	})
}
