package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"github.com/webview/webview"
)

// showError prints msg to stderr and, on Windows or when guiMode is true,
// shows a blocking Fyne dialog before exiting with code 1. Never returns.
func showError(msg string, guiMode bool) {
	fmt.Fprintln(os.Stderr, msg)
	if guiMode || runtime.GOOS == "windows" {
		ShowFatalErrorDialog(msg) // internally calls os.Exit(1)
	}
	os.Exit(1)
}

func main() {
	// Use ContinueOnError so we can catch bad flags and show a dialog instead
	// of writing to stderr (invisible on Windows GUI builds) and calling os.Exit.
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

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
	appMode := flag.Bool("app-mode", true, "Open the browser in app mode: no address bar, tabs, or toolbar (--app=URL). Default: true")
	webviewFlag := flag.Bool("webview", false, "Render the auth page in an embedded webview instead of launching an external browser")
	webviewTitle := flag.String("webview-title", "", "Title for the embedded webview window")
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

	// Destination tag filter — restricts which destinations defined in the
	// config file are presented in the picker. Comma-separated list of tags.
	destinationTagsArg := flag.String("destination-tags", "", "Comma-separated tags to restrict the destination picker (e.g. 'zabbix,ipam')")

	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		// Bad flag — always show a dialog (interactive invocation).
		showError(fmt.Sprintf("invalid arguments: %v", err), true)
	}

	// Track which flags were explicitly set on the command line.
	setFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	// ── Load config file (optional) ────────────────────────────────────────
	cfg, err := loadConfig(*configPath)
	if err != nil {
		showError(fmt.Sprintf("error loading config: %v", err), !setFlags["headless"] || !*headless)
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
	if setFlags["app-mode"] {
		cfg.AppMode = *appMode
	}
	if setFlags["webview"] {
		cfg.Webview = *webviewFlag
	}
	if setFlags["webview-title"] {
		cfg.WebviewTitle = *webviewTitle
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

	// ── Apply destination tag filter ───────────────────────────────────────
	var tagsFilter []string
	if setFlags["destination-tags"] {
		for _, t := range strings.Split(*destinationTagsArg, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tagsFilter = append(tagsFilter, t)
			}
		}
	}
	cfg.Destinations = filterDestinations(cfg.Destinations, tagsFilter)

	// ── Validate required fields ───────────────────────────────────────────
	if cfg.AuthStartURL == "" && len(cfg.Destinations) == 0 {
		showError("error: --auth-start-url is required, or define destinations in the config file", !cfg.Headless)
	}

	if cfg.Webview {
		if cfg.Headless {
			cfg.Headless = false
		}
		runWebview(&cfg, *debug)
		return
	}

	// ── Find browser ───────────────────────────────────────────────────────
	browserExec, err := findBrowser(&cfg, slog.Default())
	if err != nil {
		showError(fmt.Sprintf("error: %v", err), !cfg.Headless)
	}

	if cfg.Headless {
		runHeadless(&cfg, *debug, browserExec)
	} else {
		runGUI(&cfg, *debug, browserExec)
	}
}

func runHeadless(cfg *Config, debug bool, browserExec string) {
	// Destination selection (headless: numbered stdout/stdin menu).
	if len(cfg.Destinations) > 0 {
		entries := buildPickerEntries(cfg.Destinations)
		var chosen DestinationURL
		var parent *Destination
		if e, ok := singleSelectableEntry(entries); ok {
			// Only one selectable entry — skip the interactive prompt.
			chosen, parent = e.url, e.parent
		} else {
			var err error
			chosen, parent, err = selectDestinationHeadless(entries)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: destination selection: %v\n", err)
				os.Exit(1)
			}
		}
		applyDestination(cfg, parent, chosen)
	}

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
			gui.ShowStartupError(fmt.Sprintf("failed to set up logger: %v", err))
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
		// Destination selection (GUI: Fyne picker or auto-select).
		if len(cfg.Destinations) > 0 {
			entries := buildPickerEntries(cfg.Destinations)
			var chosen DestinationURL
			var parent *Destination
			if e, ok := singleSelectableEntry(entries); ok {
				// Only one selectable entry — skip the picker.
				chosen, parent = e.url, e.parent
			} else {
				chosen, parent = gui.ShowDestinationPicker(entries)
			}
			applyDestination(cfg, parent, chosen)
			gui.UpdateInfoLabels(cfg.AuthStartURL, cfg.UsernameValue)
		}

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

func runWebview(cfg *Config, debug bool) {
	controller := newWebviewController()

	if debug {
		gui := NewWebatorGUI(cfg, true)
		logger := gui.Logger()
		logCleanup := func() {}
		defer logCleanup()
		slog.SetDefault(logger)

		baseCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stopSignal()

		logger.Info("webator starting",
			slog.String("authStartUrl", cfg.AuthStartURL),
			slog.Bool("debug", debug),
			slog.Bool("webview", true),
		)

		logger.Info("opening embedded webview", slog.String("url", cfg.AuthStartURL))
		w, err := newWebviewWindow(cfg, controller)
		if err != nil {
			logger.Error("failed to create embedded webview", slog.Any("error", err))
			os.Exit(1)
		}
		gui.AddCleanup(func() {
			w.Dispatch(func() {
				w.Exit()
			})
		})

		loopDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(16 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					fyne.Do(func() {
						if !w.Loop(false) {
							select {
							case <-loopDone:
							default:
								close(loopDone)
								gui.SetStatus("Embedded webview closed")
								gui.window.Close()
							}
						}
					})
				case <-baseCtx.Done():
					w.Dispatch(func() {
						w.Exit()
					})
					return
				}
			}
		}()

		gui.Run(stopSignal, func() error {
			gui.SetStatus("Starting embedded webview...")
			logger.Info("embedded webview running")

			authCtx := baseCtx
			if cfg.Timeout > 0 {
				var cancelAuth context.CancelFunc
				authCtx, cancelAuth = context.WithTimeout(baseCtx, time.Duration(cfg.Timeout)*time.Second)
				gui.AddCleanup(cancelAuth)
			}

			if err := runWebviewAuth(authCtx, cfg, logger, gui.SetStatus, w, controller); err != nil {
				return err
			}

			return nil
		})
		return
	}

	logger, logCleanup, err := setupLogger(cfg, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error setting up logger: %v\n", err)
		os.Exit(1)
	}
	defer logCleanup()
	slog.SetDefault(logger)

	baseCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	logger.Info("webator starting",
		slog.String("authStartUrl", cfg.AuthStartURL),
		slog.Bool("debug", debug),
		slog.Bool("webview", true),
	)

	logger.Info("opening embedded webview", slog.String("url", cfg.AuthStartURL))
	w, err := newWebviewWindow(cfg, controller)
	if err != nil {
		logger.Error("failed to create embedded webview", slog.Any("error", err))
		os.Exit(1)
	}

	go func() {
		authCtx := baseCtx
		if cfg.Timeout > 0 {
			var cancelAuth context.CancelFunc
			authCtx, cancelAuth = context.WithTimeout(baseCtx, time.Duration(cfg.Timeout)*time.Second)
			defer cancelAuth()
		}
		if err := runWebviewAuth(authCtx, cfg, logger, func(string) {}, w, controller); err != nil {
			logger.Error("embedded auth failed", slog.Any("error", err))
			w.Dispatch(func() {
				w.Exit()
			})
		}
	}()

	w.Run()

	logger.Info("embedded webview closed")
	logger.Info("webator exiting")
}

func deriveWebviewTitle(cfg *Config) string {
	if cfg.WebviewTitle != "" {
		return cfg.WebviewTitle
	}
	if cfg.NavigateURL != "" {
		return titleFromURL(cfg.NavigateURL)
	}
	return titleFromURL(cfg.AuthStartURL)
}

func titleFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return raw
	}
	return parsed.Host
}

func newWebviewWindow(cfg *Config, controller *webviewController) (webview.WebView, error) {
	title := deriveWebviewTitle(cfg)
	if title == "" {
		title = "webator"
	}
	w := webview.New(webview.Settings{
		Title:                  title,
		URL:                    cfg.AuthStartURL,
		Width:                  cfg.ViewportWidth,
		Height:                 cfg.ViewportHeight,
		Resizable:              true,
		Debug:                  false,
		ExternalInvokeCallback: controller.callback,
	})
	if w == nil {
		return nil, fmt.Errorf("failed to create embedded webview")
	}
	return w, nil
}
