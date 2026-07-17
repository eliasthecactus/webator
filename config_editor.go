package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	triAuto     = "Auto"
	triEnabled  = "Enabled"
	triDisabled = "Disabled"
)

func runConfigEditor(path string) error {
	if path == "" {
		return errors.New("--edit-config requires a file path")
	}

	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	parent := filepath.Dir(cleanPath)
	info, err := os.Stat(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("path does not exist: %s", parent)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", parent)
	}

	exists := true
	if _, err := os.Stat(cleanPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			exists = false
		} else {
			return err
		}
	}

	cfg := defaultConfig()
	// The start URL is the only root-level setting required for a runnable
	// configuration. Every optional setting starts disabled in a new file.
	enabled := map[string]bool{"auth_start_url": !exists}
	if exists {
		loaded, loadedEnabled, err := loadEditorConfig(cleanPath)
		if err != nil {
			return err
		}
		cfg, enabled = loaded, loadedEnabled
	}

	workingPath, err := findWorkingConfig(cleanPath)
	if err != nil {
		return err
	}
	editor := newConfigEditor(cleanPath, cfg, !exists, enabled, workingPath)
	editor.run()
	return nil
}

func loadEditorConfig(path string) (Config, map[string]bool, error) {
	cfg, err := loadConfig(path)
	if err != nil {
		return Config{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, nil, err
	}
	enabled := make(map[string]bool, len(raw))
	for key := range raw {
		enabled[key] = true
	}
	return cfg, enabled, nil
}

func findWorkingConfig(configPath string) (string, error) {
	pattern := filepath.Join(filepath.Dir(configPath), "."+filepath.Base(configPath)+".webator-launch-*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	configModTime := time.Time{}
	if configInfo, err := os.Stat(configPath); err == nil {
		configModTime = configInfo.ModTime()
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	var newest string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || !info.ModTime().After(configModTime) {
			continue
		}
		if newest == "" {
			newest = path
			continue
		}
		newestInfo, err := os.Stat(newest)
		if err == nil && info.ModTime().After(newestInfo.ModTime()) {
			newest = path
		}
	}
	return newest, nil
}

type configEditor struct {
	app         fyne.App
	window      fyne.Window
	path        string
	missingFile bool
	workingPath string
	restoreWork bool
	savedState  []byte
	enabled     map[string]bool
	defaults    map[string]interface{}

	authStartURL *widget.Entry
	authDoneURL  *widget.Entry
	navigateURL  *widget.Entry

	usernameSelector *widget.Entry
	usernameValue    *widget.Entry
	passwordSelector *widget.Entry
	passwordValue    *widget.Entry

	totpSecret         *widget.Entry
	totpSelector       *widget.Entry
	totpSubmitSelector *widget.Entry
	totpStep           *widget.Entry

	submitSelector    *widget.Entry
	doneSelector      *widget.Entry
	waitAfterSubmitMs *widget.Entry

	headless                     *widget.Check
	appMode                      *widget.Check
	kiosk                        *widget.Check
	kioskCloseButton             *widget.Select
	kioskCloseButtonLabel        *widget.Entry
	kioskCloseButtonPosition     *widget.Select
	kioskCloseButtonSwapPosition *widget.Check
	browserControls              *widget.Check
	browserControlsPosition      *widget.Select
	browserControlsSwapPosition  *widget.Check
	startMaximized               *widget.Check
	webview                      *widget.Check
	webviewTitle                 *widget.Entry
	incognito                    *widget.Check
	disableContextMenu           *widget.Check
	disableDevTools              *widget.Check
	disableTranslate             *widget.Check
	disablePinch                 *widget.Check
	disableTouchAdjustment       *widget.Check
	kioskPrinting                *widget.Check
	noFirstRun                   *widget.Check
	noDefaultBrowserCheck        *widget.Check

	browserPath                 *widget.Entry
	viewportWidth               *widget.Entry
	viewportHeight              *widget.Entry
	userAgent                   *widget.Entry
	disableFeatures             *widget.Entry
	edgeKioskType               *widget.Entry
	overscrollHistoryNavigation *widget.Entry
	pullToRefresh               *widget.Entry

	proxy            *widget.Entry
	ignoreCertErrors *widget.Check

	timeout      *widget.Entry
	retryCount   *widget.Entry
	retryDelayMs *widget.Entry
	pollInterval *widget.Entry

	logLevel *widget.Select
	logFile  *widget.Entry

	launchAuthStartURL, launchUsername, launchPassword, launchTOTPSecret, launchDestinationTags, launchExtra string
	launchHeadless                                                                                           string
	launchDebug                                                                                              bool
	launchConfigPath                                                                                         string

	destinations      []Destination
	selectedDest      int
	selectedURL       int
	destList          *widget.List
	urlList           *widget.List
	destinationJSON   *widget.Entry
	urlJSON           *widget.Entry
	waitOverridesJSON *widget.Entry

	status *widget.Label
}

func newConfigEditor(path string, cfg Config, missingFile bool, enabled map[string]bool, workingPath string) *configEditor {
	a := app.NewWithID("ch.eliasthecactus.webator.config-editor")
	icon := fyne.NewStaticResource("icon.png", iconBytes)
	a.SetIcon(icon)

	e := &configEditor{
		app:         a,
		path:        path,
		missingFile: missingFile,
		workingPath: workingPath,
		status:      widget.NewLabel(""),
		enabled:     enabled,
	}
	defaultData, _ := json.Marshal(defaultConfig())
	_ = json.Unmarshal(defaultData, &e.defaults)
	e.window = a.NewWindow("webator - Config Editor")
	e.window.SetIcon(icon)

	e.authStartURL = entry(cfg.AuthStartURL)
	e.authDoneURL = entry(cfg.AuthDoneURL)
	e.navigateURL = entry(cfg.NavigateURL)
	e.usernameSelector = entry(cfg.UsernameSelector)
	e.usernameValue = entry(cfg.UsernameValue)
	e.passwordSelector = entry(cfg.PasswordSelector)
	e.passwordValue = passwordEntry(cfg.PasswordValue)
	e.totpSecret = passwordEntry(cfg.TOTPSecret)
	e.totpSelector = entry(cfg.TOTPSelector)
	e.totpSubmitSelector = entry(cfg.TOTPSubmitSelector)
	e.totpStep = entry(strconv.Itoa(cfg.TOTPStep))
	e.submitSelector = entry(cfg.SubmitSelector)
	e.doneSelector = entry(cfg.DoneSelector)
	e.waitAfterSubmitMs = entry(strconv.Itoa(cfg.WaitAfterSubmitMs))

	e.headless = check("Run headless", cfg.Headless)
	e.appMode = check("App mode", cfg.AppMode)
	e.kiosk = check("Kiosk", cfg.Kiosk)
	e.kioskCloseButton = widget.NewSelect([]string{triAuto, triEnabled, triDisabled}, nil)
	e.kioskCloseButton.SetSelected(boolPtrToTriState(cfg.KioskCloseButton))
	e.kioskCloseButtonLabel = entry(kioskCloseButtonLabel(&cfg))
	e.kioskCloseButtonPosition = positionSelect(cfg.KioskCloseButtonPosition, "top-right")
	e.kioskCloseButtonSwapPosition = check("Allow position swap", cfg.KioskCloseButtonSwapPosition)
	e.browserControls = check("Show browser controls", cfg.BrowserControls)
	e.browserControlsPosition = positionSelect(cfg.BrowserControlsPosition, "top-left")
	e.browserControlsSwapPosition = check("Allow position swap", cfg.BrowserControlsSwapPosition)
	e.startMaximized = check("Start maximized", cfg.StartMaximized)
	e.webview = check("Embedded webview", cfg.Webview)
	e.webviewTitle = entry(cfg.WebviewTitle)
	e.incognito = check("Incognito", cfg.Incognito)
	e.disableContextMenu = check("Allow context menu", !cfg.DisableContextMenu)
	e.disableDevTools = check("Allow developer tools", !cfg.DisableDevTools)
	e.disableTranslate = check("Allow translate prompts", !cfg.DisableTranslate)
	e.disablePinch = check("Allow pinch zoom", !cfg.DisablePinch)
	e.disableTouchAdjustment = check("Allow touch adjustment UI", !cfg.DisableTouchAdjustment)
	e.kioskPrinting = check("Kiosk printing", cfg.KioskPrinting)
	e.noFirstRun = check("Run first-run checks", !cfg.NoFirstRun)
	e.noDefaultBrowserCheck = check("Run default browser check", !cfg.NoDefaultBrowserCheck)

	e.browserPath = entry(cfg.BrowserPath)
	e.viewportWidth = entry(strconv.Itoa(cfg.ViewportWidth))
	e.viewportHeight = entry(strconv.Itoa(cfg.ViewportHeight))
	e.userAgent = entry(cfg.UserAgent)
	e.disableFeatures = entry(cfg.DisableFeatures)
	e.edgeKioskType = entry(cfg.EdgeKioskType)
	e.overscrollHistoryNavigation = entry(strconv.Itoa(cfg.OverscrollHistoryNavigation))
	e.pullToRefresh = entry(strconv.Itoa(cfg.PullToRefresh))

	e.proxy = entry(cfg.Proxy)
	e.ignoreCertErrors = check("Ignore certificate errors", cfg.IgnoreCertErrors)

	e.timeout = entry(strconv.Itoa(cfg.Timeout))
	e.retryCount = entry(strconv.Itoa(cfg.RetryCount))
	e.retryDelayMs = entry(strconv.Itoa(cfg.RetryDelayMs))
	e.pollInterval = entry(strconv.Itoa(cfg.PollIntervalMs))

	e.logLevel = widget.NewSelect([]string{"debug", "info", "warn", "error"}, nil)
	e.logLevel.SetSelected(cfg.LogLevel)
	e.logFile = entry(cfg.LogFile)

	e.destinations = append([]Destination(nil), cfg.Destinations...)
	e.selectedDest = -1
	e.selectedURL = -1
	e.destinationJSON = multiJSONEntry(map[string]interface{}{})
	e.urlJSON = multiJSONEntry(map[string]interface{}{})
	e.waitOverridesJSON = multiJSONEntry(cfg.WaitOverrides)

	e.initDestinationLists()
	e.window.SetContent(e.content())
	e.window.SetCloseIntercept(e.requestClose)
	e.window.Resize(fyne.NewSize(900, 720))
	e.markSaved()
	return e
}

func (e *configEditor) run() {
	e.window.Show()
	if e.workingPath != "" {
		e.askRestoreWork()
	} else if e.missingFile {
		e.askCreate()
	}
	e.app.Run()
	if e.restoreWork {
		cfg, enabled, err := loadEditorConfig(e.workingPath)
		if err != nil {
			return
		}
		newConfigEditor(e.path, cfg, false, enabled, "").run()
	}
}

func (e *configEditor) askCreate() {
	dialog.ShowConfirm(
		"Create config file?",
		fmt.Sprintf("%s does not exist. Create it when you save?", e.path),
		func(ok bool) {
			if !ok {
				e.app.Quit()
			}
		},
		e.window,
	)
}

func (e *configEditor) askRestoreWork() {
	dialog.ShowConfirm(
		"Continue previous work?",
		"A newer unsaved launch snapshot was found for this config. Load it and continue editing from that state?",
		func(ok bool) {
			if ok {
				e.restoreWork = true
				e.app.Quit()
			}
		},
		e.window,
	)
}

func (e *configEditor) content() fyne.CanvasObject {
	header := container.NewBorder(nil, nil,
		widget.NewLabel("File"),
		nil,
		widget.NewLabel(e.path),
	)

	tabs := container.NewAppTabs(
		container.NewTabItem("Auth", scroll(e.authForm())),
		container.NewTabItem("Browser", scroll(e.browserForm())),
		container.NewTabItem("Timing", scroll(e.timingForm())),
		container.NewTabItem("Destinations", e.destinationsPane()),
		container.NewTabItem("Wait Overrides", e.rawJSONPane(e.waitOverridesJSON, "wait_overrides")),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	launch := widget.NewButton("Launch", func() { e.launch() })
	save := widget.NewButton("Save", func() { e.save() })
	closeBtn := widget.NewButton("Close", e.requestClose)
	footer := container.NewBorder(nil, nil, e.status, container.NewHBox(closeBtn, launch, save), nil)

	return container.NewBorder(header, footer, nil, nil, tabs)
}

type settingRow struct {
	label string
	help  string
	field fyne.CanvasObject
}

func (e *configEditor) helpForm(rows []settingRow) fyne.CanvasObject {
	items := make([]fyne.CanvasObject, 0, len(rows))
	for _, row := range rows {
		row := row
		key := settingKey(row.label)
		label := widget.NewLabel(row.label)
		help := widget.NewButton("?", func() {
			dialog.ShowInformation(row.label, row.help+e.defaultHelp(key), e.window)
		})
		help.Importance = widget.LowImportance
		use := widget.NewCheck("Use", func(active bool) {
			e.enabled[key] = active
			setEnabled(row.field, active)
		})
		use.SetChecked(e.enabled[key])
		setEnabled(row.field, e.enabled[key])
		left := container.NewBorder(nil, nil, nil, container.NewHBox(use, help), label)
		items = append(items, container.NewBorder(nil, nil, left, nil, row.field))
	}
	return container.NewVBox(items...)
}

func settingKey(label string) string {
	overrides := map[string]string{
		"Auth done URL": "auth_done_url", "Embedded webview": "webview", "Kiosk close label": "kiosk_close_button_label",
		"Kiosk close button": "kiosk_close_button", "Kiosk printing": "kiosk_printing", "Start maximized": "start_maximized",
		"Kiosk close position": "kiosk_close_button_position", "Kiosk close swap position": "kiosk_close_button_swap_position",
		"Close button": "kiosk_close_button", "Close button label": "kiosk_close_button_label",
		"Close button position": "kiosk_close_button_position", "Close button swap position": "kiosk_close_button_swap_position",
		"Ignore certificate errors": "ignore_cert_errors", "Allow developer tools": "disable_dev_tools",
		"Allow context menu": "disable_context_menu", "Allow touch adjustment UI": "disable_touch_adjustment",
		"Allow translate prompts": "disable_translate", "Allow pinch zoom": "disable_pinch",
		"Run first-run checks": "no_first_run", "Run default browser check": "no_default_browser_check",
		"Timeout seconds": "timeout", "Retry delay ms": "retry_delay_ms", "Poll interval ms": "poll_interval_ms",
	}
	if key, ok := overrides[label]; ok {
		return key
	}
	return strings.ReplaceAll(strings.ToLower(label), " ", "_")
}

func (e *configEditor) defaultHelp(key string) string {
	if value, ok := e.defaults[key]; ok {
		return fmt.Sprintf("\n\nDefault: %v", value)
	}
	return "\n\nDefault: not set"
}

func setEnabled(object fyne.CanvasObject, enabled bool) {
	if field, ok := object.(interface {
		Enable()
		Disable()
	}); ok {
		if enabled {
			field.Enable()
		} else {
			field.Disable()
		}
	}
}

func (e *configEditor) authForm() fyne.CanvasObject {
	return e.helpForm([]settingRow{
		{"Auth start URL", "URL where the login flow begins.", e.authStartURL},
		{"Auth done URL", "URL substring that marks authentication as complete in full-auto mode.", e.authDoneURL},
		{"Navigate URL", "Target URL to open after authentication succeeds.", e.navigateURL},
		{"Username selector", "CSS or XPath selector for the username input.", e.usernameSelector},
		{"Username value", "Username or email to type into the username field.", e.usernameValue},
		{"Password selector", "CSS or XPath selector for the password input.", e.passwordSelector},
		{"Password value", "Password to type. This is saved into the JSON file.", e.passwordValue},
		{"TOTP secret", "Base32 TOTP secret used to generate MFA codes. This is saved into the JSON file.", e.totpSecret},
		{"TOTP selector", "CSS or XPath selector for the TOTP/MFA code input.", e.totpSelector},
		{"TOTP submit selector", "Optional button selector used after filling TOTP in step 2. Falls back to submit selector.", e.totpSubmitSelector},
		{"TOTP step", "1 means fill TOTP before the main submit. 2 means submit first, then fill TOTP.", e.totpStep},
		{"Submit selector", "CSS or XPath selector for the main submit button.", e.submitSelector},
		{"Done selector", "Optional selector whose appearance marks login success.", e.doneSelector},
		{"Wait after submit ms", "Extra delay after clicking submit, useful for slow multi-step login pages.", e.waitAfterSubmitMs},
	})
}

func (e *configEditor) browserForm() fyne.CanvasObject {
	return e.helpForm([]settingRow{
		{"Headless", "Run Chrome without a visible browser window.", e.headless},
		{"App mode", "Open Chrome with --app=URL, hiding the address bar and tabs.", e.appMode},
		{"Kiosk", "Run the browser fullscreen/kiosk-style.", e.kiosk},
		{"Close button", "Show the injected Close button. When not explicitly set, it is enabled only in kiosk mode.", e.kioskCloseButton},
		{"Close button label", "Text shown on the injected Close button.", e.kioskCloseButtonLabel},
		{"Close button position", "Corner for the injected Close button.", e.kioskCloseButtonPosition},
		{"Close button swap position", "Allow a right-click or press-and-hold to move the Close button across the screen.", e.kioskCloseButtonSwapPosition},
		{"Browser controls", "Show injected Back, Forward, and Refresh controls. Useful in kiosk or app mode.", e.browserControls},
		{"Browser controls position", "Corner for the injected browser controls.", e.browserControlsPosition},
		{"Browser controls swap position", "Allow a right-click or press-and-hold to move the controls across the screen.", e.browserControlsSwapPosition},
		{"Start maximized", "Maximize the browser window after launch, including app mode.", e.startMaximized},
		{"Embedded webview", "Use the native embedded webview instead of launching Chrome or Edge.", e.webview},
		{"Webview title", "Optional title for the embedded webview window.", e.webviewTitle},
		{"Incognito", "Launch Chrome/Edge in a private session.", e.incognito},
		{"Browser path", "Explicit path to Chrome or Edge. Leave empty for auto-detect.", e.browserPath},
		{"Viewport width", "Headless viewport width in pixels.", e.viewportWidth},
		{"Viewport height", "Headless viewport height in pixels.", e.viewportHeight},
		{"User agent", "Browser User-Agent string. Leave default unless a site requires a specific UA.", e.userAgent},
		{"Proxy", "HTTP/HTTPS proxy URL, for example http://proxy:8080.", e.proxy},
		{"Ignore cert errors", "Launch Chrome with certificate errors ignored. Use only for trusted internal/dev sites.", e.ignoreCertErrors},
		{"Allow context menu", "When enabled, right-click context menus are allowed. Disabled blocks them.", e.disableContextMenu},
		{"Allow developer tools", "When enabled, developer tools are allowed. Disabled blocks them.", e.disableDevTools},
		{"Allow translate prompts", "When enabled, browser translation prompts are allowed.", e.disableTranslate},
		{"Allow pinch zoom", "When enabled, pinch/zoom gestures are allowed.", e.disablePinch},
		{"Allow touch adjustment UI", "When enabled, browser touch adjustment UI is allowed.", e.disableTouchAdjustment},
		{"Kiosk printing", "Enable kiosk-friendly printing behavior.", e.kioskPrinting},
		{"Disable features", "Comma-separated Chrome features to disable in addition to webator defaults.", e.disableFeatures},
		{"Edge kiosk type", "Microsoft Edge kiosk type used when kiosk is enabled.", e.edgeKioskType},
		{"Run first-run checks", "When enabled, Chrome/Edge first-run checks are allowed.", e.noFirstRun},
		{"Run default browser check", "When enabled, default-browser checks and prompts are allowed.", e.noDefaultBrowserCheck},
		{"Overscroll history navigation", "Chrome overscroll history navigation setting.", e.overscrollHistoryNavigation},
		{"Pull to refresh", "Chrome pull-to-refresh setting.", e.pullToRefresh},
	})
}

func (e *configEditor) timingForm() fyne.CanvasObject {
	return e.helpForm([]settingRow{
		{"Timeout seconds", "Global timeout for the login flow.", e.timeout},
		{"Retry count", "Number of retry attempts after a failed login attempt.", e.retryCount},
		{"Retry delay ms", "Delay between retry attempts in milliseconds.", e.retryDelayMs},
		{"Poll interval ms", "How often webator checks for elements while waiting.", e.pollInterval},
		{"Log level", "Minimum log level written to the log output.", e.logLevel},
		{"Log file", "Path to the JSON log file. Leave empty for the default temp log file.", e.logFile},
	})
}

func (e *configEditor) initDestinationLists() {
	e.destList = widget.NewList(
		func() int { return len(e.destinations) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(destinationDisplayName(e.destinations[id], id))
		},
	)
	e.destList.OnSelected = func(id widget.ListItemID) {
		e.selectedDest = id
		e.selectedURL = -1
		e.urlList.Refresh()
	}

	e.urlList = widget.NewList(
		func() int {
			if e.selectedDest < 0 || e.selectedDest >= len(e.destinations) {
				return 0
			}
			return len(e.destinations[e.selectedDest].URLs)
		},
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(urlDisplayName(e.destinations[e.selectedDest].URLs[id], id))
		},
	)
	e.urlList.OnSelected = func(id widget.ListItemID) {
		e.selectedURL = id
	}

	if len(e.destinations) > 0 {
		e.destList.Select(0)
	}
}

func (e *configEditor) destinationsPane() fyne.CanvasObject {
	addDest := widget.NewButton("Add Destination", func() {
		e.destinations = append(e.destinations, Destination{Name: "New Destination"})
		e.enabled["destinations"] = true
		e.selectedDest = len(e.destinations) - 1
		e.selectedURL = -1
		e.destList.Refresh()
		e.destList.Select(e.selectedDest)
		e.openDestinationEditor(e.selectedDest)
	})
	removeDest := widget.NewButton("Remove Destination", func() {
		if e.selectedDest < 0 || e.selectedDest >= len(e.destinations) {
			return
		}
		idx := e.selectedDest
		e.destinations = append(e.destinations[:idx], e.destinations[idx+1:]...)
		if len(e.destinations) > 0 {
			if idx >= len(e.destinations) {
				idx = len(e.destinations) - 1
			}
			e.selectedDest = idx
			e.selectedURL = -1
		} else {
			e.selectedDest = -1
			e.selectedURL = -1
		}
		e.destList.Refresh()
		e.urlList.Refresh()
		if e.selectedDest >= 0 {
			e.destList.Select(idx)
		}
	})
	editDest := widget.NewButton("Edit Destination", func() {
		if e.selectedDest >= 0 && e.selectedDest < len(e.destinations) {
			e.openDestinationEditor(e.selectedDest)
		}
	})

	addURL := widget.NewButton("Add URL", func() {
		if e.selectedDest < 0 || e.selectedDest >= len(e.destinations) {
			e.setStatus("Select a destination first", true)
			return
		}
		e.destinations[e.selectedDest].URLs = append(e.destinations[e.selectedDest].URLs, DestinationURL{Label: "New URL"})
		e.urlList.Refresh()
		e.urlList.Select(len(e.destinations[e.selectedDest].URLs) - 1)
		e.openURLEditor(e.selectedDest, len(e.destinations[e.selectedDest].URLs)-1)
	})
	removeURL := widget.NewButton("Remove URL", func() {
		if e.selectedDest < 0 || e.selectedDest >= len(e.destinations) {
			return
		}
		urls := e.destinations[e.selectedDest].URLs
		if e.selectedURL < 0 || e.selectedURL >= len(urls) {
			return
		}
		idx := e.selectedURL
		e.destinations[e.selectedDest].URLs = append(urls[:idx], urls[idx+1:]...)
		if len(e.destinations[e.selectedDest].URLs) > 0 {
			if idx >= len(e.destinations[e.selectedDest].URLs) {
				idx = len(e.destinations[e.selectedDest].URLs) - 1
			}
			e.selectedURL = idx
		} else {
			e.selectedURL = -1
		}
		e.urlList.Refresh()
		if e.selectedURL >= 0 {
			e.urlList.Select(idx)
		}
	})
	editURL := widget.NewButton("Edit URL", func() {
		if e.selectedDest >= 0 && e.selectedDest < len(e.destinations) && e.selectedURL >= 0 && e.selectedURL < len(e.destinations[e.selectedDest].URLs) {
			e.openURLEditor(e.selectedDest, e.selectedURL)
		}
	})

	left := container.NewBorder(
		container.NewVBox(widget.NewLabelWithStyle("Destinations", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), container.NewGridWithColumns(2, addDest, removeDest)),
		editDest,
		nil,
		nil,
		e.destList,
	)
	right := container.NewBorder(
		container.NewVBox(widget.NewLabelWithStyle("URLs", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), container.NewGridWithColumns(2, addURL, removeURL)),
		editURL,
		nil,
		nil,
		e.urlList,
	)
	split := container.NewVSplit(left, right)
	split.SetOffset(0.5)
	return split
}

type scopedField struct {
	key, label, help, kind string
	required               bool
}

func (e *configEditor) openDestinationEditor(index int) {
	if index < 0 || index >= len(e.destinations) {
		return
	}
	fields := scopedFields(false)
	e.openScopedEditor("Destination settings", scopedJSON(e.destinations[index], "urls"), fields, func(raw map[string]json.RawMessage) error {
		previous := e.destinations[index]
		urls := e.destinations[index].URLs
		data, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		var updated Destination
		if err := json.Unmarshal(data, &updated); err != nil {
			return err
		}
		updated.URLs = urls
		e.destinations[index] = updated
		if err := validateDestinationTags(e.destinations); err != nil {
			e.destinations[index] = previous
			return err
		}
		e.destList.Refresh()
		e.urlList.Refresh()
		return nil
	})
}

func (e *configEditor) openURLEditor(destinationIndex, urlIndex int) {
	if destinationIndex < 0 || destinationIndex >= len(e.destinations) || urlIndex < 0 || urlIndex >= len(e.destinations[destinationIndex].URLs) {
		return
	}
	fields := scopedFields(true)
	e.openScopedEditor("URL settings", scopedJSON(e.destinations[destinationIndex].URLs[urlIndex]), fields, func(raw map[string]json.RawMessage) error {
		previous := e.destinations[destinationIndex].URLs[urlIndex]
		data, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		var updated DestinationURL
		if err := json.Unmarshal(data, &updated); err != nil {
			return err
		}
		e.destinations[destinationIndex].URLs[urlIndex] = updated
		if err := validateDestinationTags(e.destinations); err != nil {
			e.destinations[destinationIndex].URLs[urlIndex] = previous
			return err
		}
		e.urlList.Refresh()
		return nil
	})
}

func scopedFields(url bool) []scopedField {
	fields := []scopedField{
		{"tag", "Tag", "Optional tag used by destination filtering.", "string", false},
		{"username_selector", "Username selector", "CSS or XPath selector for the username input.", "string", false},
		{"username_value", "Username value", "Username or email to type.", "string", false},
		{"password_selector", "Password selector", "CSS or XPath selector for the password input.", "string", false},
		{"password_value", "Password value", "Password to type. It is saved in this config file.", "string", false},
		{"totp_secret", "TOTP secret", "Base32 secret used to generate MFA codes.", "string", false},
		{"totp_selector", "TOTP selector", "CSS or XPath selector for the MFA input.", "string", false},
		{"totp_step", "TOTP step", "1 fills MFA before submit; 2 fills MFA after submit.", "int", false},
		{"submit_selector", "Submit selector", "Selector for the main submit button.", "string", false},
		{"totp_submit_selector", "TOTP submit selector", "Optional selector clicked after MFA in step 2.", "string", false},
		{"done_selector", "Done selector", "Selector whose appearance confirms login success.", "string", false},
		{"wait_after_submit_ms", "Wait after submit ms", "Extra delay after submit.", "int", false},
		{"ignore_cert_errors", "Ignore certificate errors", "Override the root certificate-error behavior for this selection.", "bool", false},
		{"kiosk", "Kiosk", "Override the root kiosk behavior.", "bool", false},
		{"kiosk_close_button", "Close button", "Override visibility of the injected Close button.", "bool", false},
		{"kiosk_close_button_label", "Close button label", "Override the Close button label.", "string", false},
		{"kiosk_close_button_position", "Close button position", "Corner for the Close button.", "position", false},
		{"kiosk_close_button_swap_position", "Close button swap position", "Allow hold/right-click to move Close to the opposite side.", "bool", false},
		{"browser_controls", "Browser controls", "Override visibility of Back, Forward, and Refresh controls.", "bool", false},
		{"browser_controls_position", "Browser controls position", "Corner for browser controls.", "position", false},
		{"browser_controls_swap_position", "Browser controls swap position", "Allow hold/right-click to move controls to the opposite side.", "bool", false},
		{"start_maximized", "Start maximized", "Override whether the app/browser starts maximized.", "bool", false},
		{"app_mode", "App mode", "Override whether Chrome hides its regular browser chrome.", "bool", false},
		{"incognito", "Incognito", "Override whether the browser uses a private session.", "bool", false},
		{"disable_context_menu", "Context menu", "Enabled allows right-click context menus; disabled blocks them.", "inverted-bool", false},
		{"disable_dev_tools", "Developer tools", "Enabled allows developer tools; disabled blocks them.", "inverted-bool", false},
		{"disable_translate", "Translate prompts", "Enabled allows browser translation prompts; disabled blocks them.", "inverted-bool", false},
		{"disable_pinch", "Pinch zoom", "Enabled allows pinch/zoom gestures; disabled blocks them.", "inverted-bool", false},
		{"disable_touch_adjustment", "Touch adjustment UI", "Enabled allows browser touch adjustment UI; disabled blocks it.", "inverted-bool", false},
		{"kiosk_printing", "Kiosk printing", "Override kiosk-friendly printing behavior.", "bool", false},
		{"no_first_run", "First-run checks", "Enabled allows first-run checks; disabled skips them.", "inverted-bool", false},
		{"no_default_browser_check", "Default browser check", "Enabled allows default-browser checks; disabled skips them.", "inverted-bool", false},
	}
	if url {
		return append([]scopedField{{"label", "Label", "Name shown in the destination picker.", "string", true}, {"auth_start_url", "Auth start URL", "URL where this login begins.", "string", false}, {"auth_done_url", "Auth done URL", "URL substring that marks authentication complete.", "string", false}, {"navigate_url", "Navigate URL", "Target URL after login.", "string", false}}, fields...)
	}
	return append([]scopedField{{"name", "Name", "Name shown in the destination picker.", "string", true}}, fields...)
}

func (e *configEditor) openScopedEditor(title string, values map[string]json.RawMessage, fields []scopedField, onSave func(map[string]json.RawMessage) error) {
	window := e.app.NewWindow("webator - " + title)
	rows := make([]fyne.CanvasObject, 0, len(fields))
	writes := make([]func() error, 0, len(fields))
	updated := map[string]json.RawMessage{}
	for key, value := range values {
		updated[key] = value
	}
	for _, definition := range fields {
		definition := definition
		active := definition.required || values[definition.key] != nil
		var field fyne.CanvasObject
		var write func() error
		switch definition.kind {
		case "bool", "inverted-bool":
			selectField := widget.NewSelect([]string{triAuto, triEnabled, triDisabled}, nil)
			selectField.SetSelected(triAuto)
			if raw := values[definition.key]; raw != nil {
				var value bool
				if json.Unmarshal(raw, &value) == nil {
					if definition.kind == "inverted-bool" {
						value = !value
					}
					selectField.SetSelected(boolPtrToTriState(&value))
				}
			}
			field = selectField
			write = func() error {
				if value := triStateToBoolPtr(selectField.Selected); value == nil {
					delete(updated, definition.key)
				} else {
					if definition.kind == "inverted-bool" {
						*value = !*value
					}
					raw, _ := json.Marshal(*value)
					updated[definition.key] = raw
				}
				return nil
			}
		case "position":
			selectField := positionSelect(rawString(values[definition.key]), "top-right")
			field = selectField
			write = func() error { raw, _ := json.Marshal(selectField.Selected); updated[definition.key] = raw; return nil }
		default:
			text := entry(rawString(values[definition.key]))
			if definition.key == "password_value" || definition.key == "totp_secret" {
				text = passwordEntry(rawString(values[definition.key]))
			}
			field = text
			write = func() error {
				if definition.kind == "int" {
					if _, err := strconv.Atoi(text.Text); err != nil {
						return fmt.Errorf("%s must be an integer", definition.label)
					}
					raw, _ := json.Marshal(mustInt(text.Text))
					updated[definition.key] = raw
				} else {
					raw, _ := json.Marshal(text.Text)
					updated[definition.key] = raw
				}
				return nil
			}
		}
		use := widget.NewCheck("Use", func(enabled bool) {
			setEnabled(field, enabled)
			if !enabled {
				delete(updated, definition.key)
			}
		})
		use.SetChecked(active)
		setEnabled(field, active)
		help := widget.NewButton("?", func() {
			dialog.ShowInformation(definition.label, definition.help+"\n\nDefault: inherited from the parent setting.", window)
		})
		help.Importance = widget.LowImportance
		rows = append(rows, container.NewBorder(nil, nil, container.NewBorder(nil, nil, nil, container.NewHBox(use, help), widget.NewLabel(definition.label)), nil, field))
		originalWrite := write
		write = func() error {
			if !use.Checked {
				delete(updated, definition.key)
				return nil
			}
			return originalWrite()
		}
		_ = write
		writes = append(writes, write)
	}
	save := widget.NewButton("Save", func() {
		for _, write := range writes {
			if err := write(); err != nil {
				dialog.ShowError(err, window)
				return
			}
		}
		if err := onSave(updated); err != nil {
			dialog.ShowError(err, window)
			return
		}
		window.Close()
	})
	window.SetContent(container.NewBorder(nil, save, nil, nil, container.NewScroll(container.NewVBox(rows...))))
	window.Resize(fyne.NewSize(780, 700))
	window.Show()
}

func rawString(raw json.RawMessage) string {
	var value interface{}
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return fmt.Sprint(value)
}
func mustInt(value string) int { parsed, _ := strconv.Atoi(value); return parsed }

func scopedJSON(value interface{}, skip ...string) map[string]json.RawMessage {
	data, _ := json.Marshal(value)
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(data, &raw)
	skipSet := map[string]bool{}
	for _, key := range skip {
		skipSet[key] = true
	}
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		key := strings.Split(field.Tag.Get("json"), ",")[0]
		if key == "" || skipSet[key] {
			delete(raw, key)
			continue
		}
		if v.Field(i).IsZero() {
			delete(raw, key)
		}
	}
	return raw
}

func (e *configEditor) rawJSONPane(entry *widget.Entry, key string) fyne.CanvasObject {
	entry.Wrapping = fyne.TextWrapOff
	use := widget.NewCheck("Use", func(enabled bool) {
		e.enabled[key] = enabled
		setEnabled(entry, enabled)
	})
	use.SetChecked(e.enabled[key])
	setEnabled(entry, e.enabled[key])
	validate := widget.NewButton("Validate "+key, func() {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(entry.Text), &raw); err != nil {
			e.setStatus(err.Error(), true)
			return
		}
		e.setStatus(key+" JSON is valid", false)
	})
	return container.NewBorder(nil, container.NewBorder(nil, nil, use, validate, nil), nil, nil, entry)
}

func (e *configEditor) save() {
	if err := e.writeConfig(); err != nil {
		e.setStatus(err.Error(), true)
		return
	}
	e.markSaved()
	e.setStatus("Saved "+e.path, false)
}

func (e *configEditor) markSaved() {
	state, err := e.currentState()
	if err == nil {
		e.savedState = state
	}
}

func (e *configEditor) currentState() ([]byte, error) {
	cfg, err := e.config()
	if err != nil {
		return nil, err
	}
	return e.marshalEnabledConfig(cfg)
}

func (e *configEditor) requestClose() {
	state, err := e.currentState()
	if err == nil && string(state) == string(e.savedState) {
		e.app.Quit()
		return
	}
	dialog.ShowConfirm(
		"Discard unsaved changes?",
		"The selected config file has not been updated. Close without saving your editor changes?",
		func(ok bool) {
			if ok {
				e.app.Quit()
			}
		},
		e.window,
	)
}

func (e *configEditor) writeConfig() error {
	return e.writeConfigTo(e.path)
}

func (e *configEditor) writeConfigTo(path string) error {
	if err := e.commitDestinationEditors(); err != nil {
		return err
	}
	cfg, err := e.config()
	if err != nil {
		return err
	}

	data, err := e.marshalEnabledConfig(cfg)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return nil
}

func (e *configEditor) launch() {
	authStartURL := widget.NewEntry()
	authStartURL.SetText(e.launchAuthStartURL)
	username := widget.NewEntry()
	username.SetText(e.launchUsername)
	password := widget.NewPasswordEntry()
	password.SetText(e.launchPassword)
	totpSecret := widget.NewPasswordEntry()
	totpSecret.SetText(e.launchTOTPSecret)
	destinationTags := widget.NewEntry()
	destinationTags.SetText(e.launchDestinationTags)
	debug := widget.NewCheck("Enable debug logging", nil)
	debug.SetChecked(e.launchDebug)
	headless := widget.NewSelect([]string{triAuto, triEnabled, triDisabled}, nil)
	headless.SetSelected(e.launchHeadless)
	if headless.Selected == "" {
		headless.SetSelected(triAuto)
	}

	extra := widget.NewEntry()
	extra.SetText(e.launchExtra)
	extra.SetPlaceHolder("Optional, for example: --debug --headless=false")
	form := widget.NewForm(
		widget.NewFormItem("Auth start URL", authStartURL),
		widget.NewFormItem("Username", username),
		widget.NewFormItem("Password", password),
		widget.NewFormItem("TOTP secret", totpSecret),
		widget.NewFormItem("Destination tags", destinationTags),
		widget.NewFormItem("Headless", headless),
		widget.NewFormItem("Debug", debug),
		widget.NewFormItem("Additional arguments", extra),
	)
	content := container.NewScroll(form)
	content.SetMinSize(fyne.NewSize(620, 380))
	dialog.NewCustomConfirm("Launch webator", "Launch", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		e.launchAuthStartURL = authStartURL.Text
		e.launchUsername = username.Text
		e.launchPassword = password.Text
		e.launchTOTPSecret = totpSecret.Text
		e.launchDestinationTags = destinationTags.Text
		e.launchHeadless = headless.Selected
		e.launchDebug = debug.Checked
		e.launchExtra = extra.Text
		if err := e.writeLaunchConfig(); err != nil {
			e.setStatus(err.Error(), true)
			return
		}
		args, err := splitCLIArgs(extra.Text)
		if err != nil {
			e.setStatus(err.Error(), true)
			return
		}
		args = append(buildLaunchArgs(launchOptions{
			AuthStartURL:    authStartURL.Text,
			Username:        username.Text,
			Password:        password.Text,
			TOTPSecret:      totpSecret.Text,
			DestinationTags: destinationTags.Text,
			Headless:        headless.Selected,
			Debug:           debug.Checked,
		}), args...)
		executable, err := os.Executable()
		if err != nil {
			e.setStatus(err.Error(), true)
			return
		}
		cmd := exec.Command(executable, append([]string{"--config", e.launchConfigPath}, args...)...)
		if err := cmd.Start(); err != nil {
			e.setStatus(fmt.Sprintf("launch failed: %v", err), true)
			return
		}
		e.setStatus("Launched using the current unsaved editor state", false)
	}, e.window).Show()
}

// writeLaunchConfig keeps the selected config untouched until Save is used.
// The private snapshot remains available for the child process after launch.
func (e *configEditor) writeLaunchConfig() error {
	if e.launchConfigPath == "" {
		file, err := os.CreateTemp(filepath.Dir(e.path), "."+filepath.Base(e.path)+".webator-launch-*.json")
		if err != nil {
			return fmt.Errorf("create launch config: %w", err)
		}
		e.launchConfigPath = file.Name()
		if err := file.Close(); err != nil {
			return fmt.Errorf("close launch config: %w", err)
		}
	}
	return e.writeConfigTo(e.launchConfigPath)
}

type launchOptions struct {
	AuthStartURL, Username, Password, TOTPSecret, DestinationTags, Headless string
	Debug                                                                   bool
}

func buildLaunchArgs(options launchOptions) []string {
	args := make([]string, 0, 13)
	appendValue := func(flag, value string) {
		if strings.TrimSpace(value) != "" {
			args = append(args, flag, value)
		}
	}
	appendValue("--auth-start-url", options.AuthStartURL)
	appendValue("--username-value", options.Username)
	appendValue("--password-value", options.Password)
	appendValue("--totp-secret", options.TOTPSecret)
	appendValue("--destination-tags", options.DestinationTags)
	if options.Headless == triEnabled {
		args = append(args, "--headless=true")
	} else if options.Headless == triDisabled {
		args = append(args, "--headless=false")
	}
	if options.Debug {
		args = append(args, "--debug")
	}
	return args
}

// splitCLIArgs accepts quoted values without invoking a shell.
func splitCLIArgs(input string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}
	for _, char := range input {
		if escaped {
			current.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				current.WriteRune(char)
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case ' ', '\t', '\n':
			flush()
		default:
			current.WriteRune(char)
		}
	}
	if escaped {
		return nil, errors.New("additional arguments end with an unfinished escape")
	}
	if quote != 0 {
		return nil, errors.New("additional arguments contain an unclosed quote")
	}
	flush()
	return args, nil
}

func (e *configEditor) marshalEnabledConfig(cfg Config) ([]byte, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	for key := range raw {
		if key != "destinations" && !e.enabled[key] {
			delete(raw, key)
		}
	}
	if len(e.destinations) == 0 && !e.enabled["destinations"] {
		delete(raw, "destinations")
	} else if len(e.destinations) > 0 {
		destinations := make([]map[string]json.RawMessage, 0, len(e.destinations))
		for _, destination := range e.destinations {
			destinationRaw := scopedJSON(destination, "urls")
			if len(destination.URLs) > 0 {
				urls := make([]map[string]json.RawMessage, 0, len(destination.URLs))
				for _, target := range destination.URLs {
					urls = append(urls, scopedJSON(target))
				}
				urlsRaw, err := json.Marshal(urls)
				if err != nil {
					return nil, err
				}
				destinationRaw["urls"] = urlsRaw
			}
			destinations = append(destinations, destinationRaw)
		}
		destinationsRaw, err := json.Marshal(destinations)
		if err != nil {
			return nil, err
		}
		raw["destinations"] = destinationsRaw
	}
	return json.MarshalIndent(raw, "", "  ")
}

func (e *configEditor) commitDestinationEditors() error {
	return nil
}

func (e *configEditor) commitDestination() error {
	return nil
}

func (e *configEditor) commitURL() error {
	return nil
}

func (e *configEditor) config() (Config, error) {
	parseInt := func(name, value string) (int, error) {
		if value == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer", name)
		}
		return n, nil
	}

	overscrollHistoryNavigation, err := parseInt("overscroll_history_navigation", e.overscrollHistoryNavigation.Text)
	if err != nil {
		return Config{}, err
	}
	pullToRefresh, err := parseInt("pull_to_refresh", e.pullToRefresh.Text)
	if err != nil {
		return Config{}, err
	}
	viewportWidth, err := parseInt("viewport_width", e.viewportWidth.Text)
	if err != nil {
		return Config{}, err
	}
	viewportHeight, err := parseInt("viewport_height", e.viewportHeight.Text)
	if err != nil {
		return Config{}, err
	}
	totpStep, err := parseInt("totp_step", e.totpStep.Text)
	if err != nil {
		return Config{}, err
	}
	waitAfterSubmitMs, err := parseInt("wait_after_submit_ms", e.waitAfterSubmitMs.Text)
	if err != nil {
		return Config{}, err
	}
	timeout, err := parseInt("timeout", e.timeout.Text)
	if err != nil {
		return Config{}, err
	}
	retryCount, err := parseInt("retry_count", e.retryCount.Text)
	if err != nil {
		return Config{}, err
	}
	retryDelayMs, err := parseInt("retry_delay_ms", e.retryDelayMs.Text)
	if err != nil {
		return Config{}, err
	}
	pollIntervalMs, err := parseInt("poll_interval_ms", e.pollInterval.Text)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AuthStartURL:                 e.authStartURL.Text,
		AuthDoneURL:                  e.authDoneURL.Text,
		NavigateURL:                  e.navigateURL.Text,
		UsernameSelector:             e.usernameSelector.Text,
		UsernameValue:                e.usernameValue.Text,
		PasswordSelector:             e.passwordSelector.Text,
		PasswordValue:                e.passwordValue.Text,
		TOTPSecret:                   e.totpSecret.Text,
		TOTPSelector:                 e.totpSelector.Text,
		TOTPSubmitSelector:           e.totpSubmitSelector.Text,
		SubmitSelector:               e.submitSelector.Text,
		DoneSelector:                 e.doneSelector.Text,
		Headless:                     e.headless.Checked,
		AppMode:                      e.appMode.Checked,
		Kiosk:                        e.kiosk.Checked,
		KioskCloseButton:             triStateToBoolPtr(e.kioskCloseButton.Selected),
		KioskCloseButtonLabel:        e.kioskCloseButtonLabel.Text,
		KioskCloseButtonPosition:     e.kioskCloseButtonPosition.Selected,
		KioskCloseButtonSwapPosition: e.kioskCloseButtonSwapPosition.Checked,
		BrowserControls:              e.browserControls.Checked,
		BrowserControlsPosition:      e.browserControlsPosition.Selected,
		BrowserControlsSwapPosition:  e.browserControlsSwapPosition.Checked,
		StartMaximized:               e.startMaximized.Checked,
		Webview:                      e.webview.Checked,
		WebviewTitle:                 e.webviewTitle.Text,
		Incognito:                    e.incognito.Checked,
		BrowserPath:                  e.browserPath.Text,
		UserAgent:                    e.userAgent.Text,
		Proxy:                        e.proxy.Text,
		IgnoreCertErrors:             e.ignoreCertErrors.Checked,
		DisableContextMenu:           !e.disableContextMenu.Checked,
		DisableDevTools:              !e.disableDevTools.Checked,
		DisableTranslate:             !e.disableTranslate.Checked,
		DisablePinch:                 !e.disablePinch.Checked,
		DisableTouchAdjustment:       !e.disableTouchAdjustment.Checked,
		KioskPrinting:                e.kioskPrinting.Checked,
		DisableFeatures:              e.disableFeatures.Text,
		EdgeKioskType:                e.edgeKioskType.Text,
		NoFirstRun:                   !e.noFirstRun.Checked,
		NoDefaultBrowserCheck:        !e.noDefaultBrowserCheck.Checked,
		OverscrollHistoryNavigation:  overscrollHistoryNavigation,
		PullToRefresh:                pullToRefresh,
		ViewportWidth:                viewportWidth,
		ViewportHeight:               viewportHeight,
		TOTPStep:                     totpStep,
		WaitAfterSubmitMs:            waitAfterSubmitMs,
		Timeout:                      timeout,
		RetryCount:                   retryCount,
		RetryDelayMs:                 retryDelayMs,
		PollIntervalMs:               pollIntervalMs,
		LogLevel:                     e.logLevel.Selected,
		LogFile:                      e.logFile.Text,
	}
	cfg.Destinations = append([]Destination(nil), e.destinations...)
	if err := json.Unmarshal([]byte(e.waitOverridesJSON.Text), &cfg.WaitOverrides); err != nil {
		return Config{}, fmt.Errorf("wait_overrides JSON: %w", err)
	}
	return cfg, nil
}

func (e *configEditor) setStatus(text string, isError bool) {
	if isError {
		e.status.Importance = widget.DangerImportance
	} else {
		e.status.Importance = widget.SuccessImportance
	}
	e.status.SetText(text)
}

func entry(value string) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(value)
	return e
}

func passwordEntry(value string) *widget.Entry {
	e := widget.NewPasswordEntry()
	e.SetText(value)
	return e
}

func check(label string, value bool) *widget.Check {
	c := widget.NewCheck(label, nil)
	c.SetChecked(value)
	return c
}

func positionSelect(value, fallback string) *widget.Select {
	selectField := widget.NewSelect([]string{"top-left", "top-right", "bottom-left", "bottom-right"}, nil)
	selectField.SetSelected(controlPosition(value, fallback))
	return selectField
}

func scroll(content fyne.CanvasObject) fyne.CanvasObject {
	s := container.NewScroll(content)
	s.SetMinSize(fyne.NewSize(820, 560))
	return s
}

func multiJSONEntry(value interface{}) *widget.Entry {
	e := widget.NewMultiLineEntry()
	e.SetMinRowsVisible(24)
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		e.SetText("null")
		return e
	}
	e.SetText(string(data))
	return e
}

func boolPtrToTriState(value *bool) string {
	if value == nil {
		return triAuto
	}
	if *value {
		return triEnabled
	}
	return triDisabled
}

func triStateToBoolPtr(value string) *bool {
	switch value {
	case triEnabled:
		v := true
		return &v
	case triDisabled:
		v := false
		return &v
	default:
		return nil
	}
}

func destinationDisplayName(dest Destination, index int) string {
	if dest.Name != "" {
		return dest.Name
	}
	if dest.Tag != "" {
		return dest.Tag
	}
	return fmt.Sprintf("Destination %d", index+1)
}

func urlDisplayName(url DestinationURL, index int) string {
	if url.Label != "" {
		return url.Label
	}
	if url.AuthStartURL != "" {
		return url.AuthStartURL
	}
	if url.NavigateURL != "" {
		return url.NavigateURL
	}
	return fmt.Sprintf("URL %d", index+1)
}

func destinationEditorJSON(dest Destination) string {
	var raw map[string]json.RawMessage
	if err := roundTripJSON(dest, &raw); err != nil {
		return "{}"
	}
	delete(raw, "urls")
	return mustJSON(raw)
}

func mustJSON(value interface{}) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func roundTripJSON(in, out interface{}) error {
	data, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
