package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// WebatorGUI manages the Fyne application window.
type WebatorGUI struct {
	fyneApp     fyne.App
	window      fyne.Window
	activity    *widget.Activity
	statusLabel *widget.Label
	logLabel    *widget.Label
	logScroll   *container.Scroll
	debugMode   bool

	mu       sync.Mutex
	logLines []string
	cleanups []func()
}

// NewWebatorGUI creates the window but does not show it yet.
func NewWebatorGUI(cfg *Config, debug bool) *WebatorGUI {
	a := app.NewWithID("io.webator.app")

	g := &WebatorGUI{
		fyneApp:   a,
		debugMode: debug,
	}

	g.activity = widget.NewActivity()
	g.activity.Start()

	g.statusLabel = widget.NewLabel("Starting browser...")
	g.statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	statusRow := container.NewHBox(g.activity, g.statusLabel)

	infoForm := widget.NewForm(
		widget.NewFormItem("Auth URL", widget.NewLabel(truncate(cfg.AuthStartURL, 52))),
		widget.NewFormItem("User", widget.NewLabel(cfg.UsernameValue)),
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
	w.SetContent(container.NewPadded(content))
	if debug {
		w.Resize(fyne.NewSize(620, 460))
	} else {
		w.Resize(fyne.NewSize(440, 170))
		w.SetFixedSize(true)
	}

	g.window = w
	return g
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
