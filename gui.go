package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

//go:embed icon.png
var iconBytes []byte

// WebatorGUI manages the Fyne application window.
type WebatorGUI struct {
	fyneApp     fyne.App
	window      fyne.Window
	activity    *widget.Activity
	statusLabel *widget.Label
	logLabel    *widget.Label
	logScroll   *container.Scroll
	debugMode   bool

	statusContent fyne.CanvasObject // the normal status/spinner view
	statusWidth   float32
	statusHeight  float32

	authURLLabel *widget.Label // info form labels — updated after destination selection
	userLabel    *widget.Label

	mu       sync.Mutex
	logLines []string
	cleanups []func()
}

// NewWebatorGUI creates the window but does not show it yet.
func NewWebatorGUI(cfg *Config, debug bool) *WebatorGUI {
	a := app.NewWithID("ch.eliasthecactus.webator")
	icon := fyne.NewStaticResource("icon.png", iconBytes)
	a.SetIcon(icon)

	g := &WebatorGUI{
		fyneApp:   a,
		debugMode: debug,
	}

	g.activity = widget.NewActivity()
	g.activity.Start()

	g.statusLabel = widget.NewLabel("Starting browser...")
	g.statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	statusRow := container.NewHBox(g.activity, g.statusLabel)

	authURLDisplay := cfg.AuthStartURL
	if authURLDisplay == "" && len(cfg.Destinations) > 0 {
		authURLDisplay = "— select destination —"
	}
	g.authURLLabel = widget.NewLabel(truncate(authURLDisplay, 52))
	g.userLabel = widget.NewLabel(cfg.UsernameValue)
	infoForm := widget.NewForm(
		widget.NewFormItem("Auth URL", g.authURLLabel),
		widget.NewFormItem("User", g.userLabel),
	)

	var content fyne.CanvasObject
	if debug {
		g.logLabel = widget.NewLabel("")
		g.logLabel.Wrapping = fyne.TextWrapOff
		g.logScroll = container.NewScroll(g.logLabel)
		g.logScroll.SetMinSize(fyne.NewSize(580, 220))

		content = container.NewVBox(
			statusRow,
			widget.NewSeparator(),
			infoForm,
			widget.NewSeparator(),
			g.logScroll,
		)
	} else {
		content = container.NewVBox(
			statusRow,
			widget.NewSeparator(),
			infoForm,
		)
	}

	w := a.NewWindow("webator")
	w.SetIcon(icon)
	g.statusContent = container.NewPadded(content)
	w.SetContent(g.statusContent)
	if debug {
		g.statusWidth = 620
		g.statusHeight = 460
		w.Resize(fyne.NewSize(g.statusWidth, g.statusHeight))
	} else {
		g.statusWidth = 440
		g.statusHeight = 170
		w.Resize(fyne.NewSize(g.statusWidth, g.statusHeight))
		w.SetFixedSize(true)
	}

	g.window = w
	return g
}

// ShowFatalErrorDialog creates a minimal Fyne application to display a fatal
// error to the user before the main GUI is started (e.g. bad config, missing
// browser). It blocks until the user dismisses the window, then exits with
// code 1.  Must NOT be called from within runGUI — use ShowStartupError there.
func ShowFatalErrorDialog(msg string) {
	a := app.NewWithID("ch.eliasthecactus.webator.err")
	icon := fyne.NewStaticResource("icon.png", iconBytes)
	a.SetIcon(icon)
	w := a.NewWindow("webator \u2014 Error")
	w.SetIcon(icon)
	lbl := widget.NewLabel(msg)
	lbl.Wrapping = fyne.TextWrapWord
	btn := widget.NewButton("OK", func() { a.Quit() })
	btn.Importance = widget.DangerImportance
	w.SetContent(container.NewPadded(container.NewVBox(lbl, widget.NewSeparator(), btn)))
	w.Resize(fyne.NewSize(440, 160))
	w.SetFixedSize(true)
	w.SetOnClosed(func() { os.Exit(1) })
	w.ShowAndRun()
	os.Exit(1)
}

// ShowStartupError displays the already-created webator window with a fatal
// error message and blocks until the user closes it, then exits with code 1.
// Use this for errors that occur inside runGUI before gui.Run() is called.
func (g *WebatorGUI) ShowStartupError(msg string) {
	lbl := widget.NewLabel(msg)
	lbl.Wrapping = fyne.TextWrapWord
	g.window.SetTitle("webator \u2014 Error")
	g.window.SetContent(container.NewPadded(container.NewVBox(
		widget.NewLabelWithStyle("Fatal error", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		lbl,
	)))
	g.window.Resize(fyne.NewSize(440, 200))
	g.window.SetFixedSize(true)
	g.window.SetOnClosed(func() { os.Exit(1) })
	g.window.ShowAndRun()
	os.Exit(1)
}

// UpdateInfoLabels refreshes the Auth URL and User fields in the info form.
// Call this after applyDestination so the GUI reflects the selected destination.
func (g *WebatorGUI) UpdateInfoLabels(authURL, user string) {
	fyne.Do(func() {
		g.authURLLabel.SetText(truncate(authURL, 52))
		if user != "" {
			g.userLabel.SetText(user)
		}
	})
}

// SetStatus updates the status label safely from any goroutine.
func (g *WebatorGUI) SetStatus(msg string) {
	fyne.Do(func() {
		g.statusLabel.SetText(msg)
	})
}

// SetDone stops the spinner and shows a session-active message.
func (g *WebatorGUI) SetDone() {
	fyne.Do(func() {
		g.activity.Stop()
		g.statusLabel.SetText("Session active")
	})
}

// SetError stops the spinner and displays the error.
func (g *WebatorGUI) SetError(err error) {
	fyne.Do(func() {
		g.activity.Stop()
		g.statusLabel.SetText(fmt.Sprintf("Error: %v", err))
	})
}

// AppendLog appends a line to the debug log area. No-op in non-debug mode.
func (g *WebatorGUI) AppendLog(line string) {
	if !g.debugMode || g.logLabel == nil {
		return
	}
	g.mu.Lock()
	g.logLines = append(g.logLines, line)
	if len(g.logLines) > 500 {
		g.logLines = g.logLines[len(g.logLines)-500:]
	}
	text := strings.Join(g.logLines, "\n")
	g.mu.Unlock()

	fyne.Do(func() {
		g.logLabel.SetText(text)
		if g.logScroll != nil {
			g.logScroll.ScrollToBottom()
		}
	})
}

// WatchBrowser starts a goroutine that detects when the browser is closed
// externally (not because the GUI window was already closed). When detected,
// debug mode shows "Session closed"; non-debug mode closes the GUI window.
func (g *WebatorGUI) WatchBrowser(browserCtx, baseCtx context.Context) {
	go func() {
		<-browserCtx.Done()
		// If baseCtx is also done, the GUI window was closed first and
		// triggered this cancellation — nothing more to do.
		select {
		case <-baseCtx.Done():
			return
		default:
		}
		if g.debugMode {
			g.SetStatus("Session closed")
		} else {
			// App.Quit() is goroutine-safe in Fyne — no fyne.Do wrapper needed.
			// Wrapping it in fyne.Do would try to run synchronously on the main
			// goroutine while Quit() posts back to the same loop, causing it to
			// be silently swallowed.
			g.fyneApp.Quit()
		}
	}()
}

// AddCleanup registers a function called when the window closes.
func (g *WebatorGUI) AddCleanup(fn func()) {
	g.mu.Lock()
	g.cleanups = append(g.cleanups, fn)
	g.mu.Unlock()
}

// Logger returns an slog.Logger that writes to the GUI log area (debug mode)
// or silently discards records (non-debug mode).
func (g *WebatorGUI) Logger() *slog.Logger {
	if !g.debugMode {
		return slog.New(discardHandler{})
	}
	return slog.New(&guiLogHandler{gui: g, minLevel: slog.LevelDebug})
}

// Run shows the window, launches authFn in a goroutine, and blocks until the
// window is closed. On close, all registered cleanups run and stopSignal is
// called to cancel the base context.
func (g *WebatorGUI) Run(stopSignal context.CancelFunc, authFn func() error) {
	g.window.SetOnClosed(func() {
		g.mu.Lock()
		fns := g.cleanups
		g.cleanups = nil
		g.mu.Unlock()
		for _, fn := range fns {
			fn()
		}
		stopSignal()
	})

	g.window.Show()

	go func() {
		if err := authFn(); err != nil {
			g.SetError(err)
			return
		}
		g.SetDone()
	}()

	g.fyneApp.Run()
}

// ShowDestinationPicker replaces the window content with an inline destination
// list so the user can pick a target without a constrained dialog overlay.
// Category headers are shown as bold uppercase separators; URL entries are
// full-width buttons. After selection the window snaps back to the status view.
func (g *WebatorGUI) ShowDestinationPicker(entries []pickerEntry) (DestinationURL, *Destination) {
	ch := make(chan pickerEntry, 1)

	fyne.Do(func() {
		const entryH = float32(44)
		const headerH = float32(42)

		naturalH := float32(0)
		var rows []fyne.CanvasObject

		for _, e := range entries {
			e := e
			if e.isHeader {
				naturalH += headerH
				// Visually distinct category heading: separator line + uppercase bold label.
				lbl := widget.NewLabelWithStyle(
					strings.ToUpper(e.label),
					fyne.TextAlignLeading,
					fyne.TextStyle{Bold: true},
				)
				rows = append(rows, widget.NewSeparator(), lbl)
			} else {
				naturalH += entryH
				btn := widget.NewButton(e.label, func() {
					// Restore the status content before signalling.
					g.window.SetContent(g.statusContent)
					if !g.debugMode {
						g.window.SetFixedSize(true)
					}
					g.window.Resize(fyne.NewSize(g.statusWidth, g.statusHeight))
					ch <- e
				})
				btn.Alignment = widget.ButtonAlignLeading
				rows = append(rows, btn)
			}
		}

		// Cap display height; add scroll only when content overflows.
		const maxH = float32(480)
		list := container.NewVBox(rows...)
		var listWidget fyne.CanvasObject
		if naturalH > maxH {
			sc := container.NewVScroll(list)
			sc.SetMinSize(fyne.NewSize(0, maxH))
			listWidget = sc
		} else {
			listWidget = list
		}

		title := widget.NewLabelWithStyle(
			"Select Destination",
			fyne.TextAlignCenter,
			fyne.TextStyle{Bold: true},
		)
		pickerContent := container.NewPadded(
			container.NewVBox(title, widget.NewSeparator(), listWidget),
		)

		// Window width: at least as wide as the status view.
		pickerW := g.statusWidth
		if pickerW < 460 {
			pickerW = 460
		}
		dispH := naturalH + 60 // account for title + separators + padding
		if dispH > maxH+60 {
			dispH = maxH + 60
		}

		g.window.SetFixedSize(false)
		g.window.SetContent(pickerContent)
		g.window.Resize(fyne.NewSize(pickerW, dispH))
	})

	chosen := <-ch
	return chosen.url, chosen.parent
}

// truncate shortens s to maxLen runes, appending "…" if trimmed.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

// ── slog handlers ─────────────────────────────────────────────────────────────

type guiLogHandler struct {
	gui      *WebatorGUI
	minLevel slog.Level
	attrs    []slog.Attr
}

var chromedpNoise = []string{"could not unmarshal event"}

func (h *guiLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

func (h *guiLogHandler) Handle(_ context.Context, r slog.Record) error {
	for _, f := range chromedpNoise {
		if strings.Contains(r.Message, f) {
			return nil
		}
	}
	var sb strings.Builder
	sb.WriteString(r.Time.Format("15:04:05"))
	sb.WriteByte(' ')
	sb.WriteString(r.Level.String())
	sb.WriteByte(' ')
	sb.WriteString(r.Message)
	for _, a := range h.attrs {
		sb.WriteByte(' ')
		sb.WriteString(a.Key)
		sb.WriteByte('=')
		sb.WriteString(fmt.Sprintf("%v", a.Value))
	}
	r.Attrs(func(a slog.Attr) bool {
		sb.WriteByte(' ')
		sb.WriteString(a.Key)
		sb.WriteByte('=')
		sb.WriteString(fmt.Sprintf("%v", a.Value))
		return true
	})
	h.gui.AppendLog(sb.String())
	return nil
}

func (h *guiLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	n := *h
	n.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &n
}

func (h *guiLogHandler) WithGroup(_ string) slog.Handler { return h }

// discardHandler silently drops all log records.
type discardHandler struct{}

func (discardHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (discardHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (d discardHandler) WithAttrs(_ []slog.Attr) slog.Handler        { return d }
func (d discardHandler) WithGroup(_ string) slog.Handler             { return d }
