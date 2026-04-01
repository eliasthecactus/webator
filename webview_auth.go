package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/webview/webview"
)

type webviewMessage struct {
	Req   int       `json:"req"`
	Type  string    `json:"type"`
	Error string    `json:"error,omitempty"`
	Info  *pageInfo `json:"info,omitempty"`
}

type pageInfo struct {
	URL             string `json:"url"`
	UsernamePresent bool   `json:"usernamePresent"`
	PasswordPresent bool   `json:"passwordPresent"`
	SubmitPresent   bool   `json:"submitPresent"`
	TOTPPresent     bool   `json:"totpPresent"`
	DonePresent     bool   `json:"donePresent"`
	ReadyState      string `json:"readyState"`
}

type webviewController struct {
	mu      sync.Mutex
	nextReq int
	pending map[int]chan webviewMessage
}

func newWebviewController() *webviewController {
	return &webviewController{
		pending: map[int]chan webviewMessage{},
	}
}

func (c *webviewController) callback(_ webview.WebView, data string) {
	var msg webviewMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		return
	}

	c.mu.Lock()
	ch, ok := c.pending[msg.Req]
	c.mu.Unlock()
	if !ok {
		return
	}

	select {
	case ch <- msg:
	default:
	}
}

func (c *webviewController) nextRequestID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextReq++
	return c.nextReq
}

func (c *webviewController) requestPageInfo(w webview.WebView, cfg *Config, timeout time.Duration) (*pageInfo, error) {
	req := c.nextRequestID()
	ch := make(chan webviewMessage, 1)

	c.mu.Lock()
	c.pending[req] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, req)
		c.mu.Unlock()
	}()

	js := buildPageInfoJS(req, cfg)
	if err := c.evalJS(w, js); err != nil {
		return nil, fmt.Errorf("page state evaluation failed: %w", err)
	}

	select {
	case msg := <-ch:
		if msg.Error != "" {
			return nil, fmt.Errorf("page state JS error: %s", msg.Error)
		}
		if msg.Info == nil {
			return nil, fmt.Errorf("page state missing info payload")
		}
		return msg.Info, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for page state")
	}
}

func (c *webviewController) evalJS(w webview.WebView, js string) error {
	errCh := make(chan error, 1)
	w.Dispatch(func() {
		errCh <- w.Eval(js)
	})
	return <-errCh
}

func (c *webviewController) fillField(w webview.WebView, selector, value string) error {
	js := fmt.Sprintf(`(function(){
	function find(sel){
		if(!sel) return null;
		try {
			if(sel.indexOf('//') === 0 || sel.indexOf('(/') === 0 || sel.indexOf('.//') === 0) {
				var result = document.evaluate(sel, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
				return result ? result.singleNodeValue : null;
			}
			return document.querySelector(sel);
		} catch(e) {
			return null;
		}
	}
	var el = find(%s);
	if(!el) return;
	try { el.focus(); } catch(e) {}
	try {
		el.value = %s;
		el.dispatchEvent(new Event('input', { bubbles: true }));
		el.dispatchEvent(new Event('change', { bubbles: true }));
	} catch(e) {}
})();`, strconv.Quote(selector), strconv.Quote(value))
	return c.evalJS(w, js)
}

func (c *webviewController) clickElement(w webview.WebView, selector string) error {
	js := fmt.Sprintf(`(function(){
	function find(sel){
		if(!sel) return null;
		try {
			if(sel.indexOf('//') === 0 || sel.indexOf('(/') === 0 || sel.indexOf('.//') === 0) {
				var result = document.evaluate(sel, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
				return result ? result.singleNodeValue : null;
			}
			return document.querySelector(sel);
		} catch(e) {
			return null;
		}
	}
	var el = find(%s);
	if(!el) return;
	try { el.click(); return; } catch(e) {}
	try {
		var event = document.createEvent('MouseEvents');
		event.initEvent('click', true, true);
		el.dispatchEvent(event);
	} catch(e) {}
})();`, strconv.Quote(selector))
	return c.evalJS(w, js)
}

func (c *webviewController) navigateTo(w webview.WebView, url string) error {
	js := fmt.Sprintf(`window.location.href = %s;`, strconv.Quote(url))
	return c.evalJS(w, js)
}

func buildPageInfoJS(req int, cfg *Config) string {
	return fmt.Sprintf(`(function(){
	function find(sel){
		if(!sel) return null;
		try {
			if(sel.indexOf('//') === 0 || sel.indexOf('(/') === 0 || sel.indexOf('.//') === 0) {
				var result = document.evaluate(sel, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null);
				return result ? result.singleNodeValue : null;
			}
			return document.querySelector(sel);
		} catch(e) {
			return null;
		}
	}
	var info = {
		url: window.location.href,
		usernamePresent: find(%s) !== null,
		passwordPresent: find(%s) !== null,
		submitPresent: find(%s) !== null,
		totpPresent: find(%s) !== null,
		donePresent: find(%s) !== null,
		readyState: document.readyState,
	};
	try {
		window.external.invoke(JSON.stringify({type:'pageInfo', req:%d, info:info}));
	} catch(e) {
		if(window.external && window.external.invoke) {
			window.external.invoke(JSON.stringify({type:'pageInfo', req:%d, info:info, error:e.toString()}));
		}
	}
})();`, strconv.Quote(cfg.UsernameSelector), strconv.Quote(cfg.PasswordSelector), strconv.Quote(cfg.SubmitSelector), strconv.Quote(cfg.TOTPSelector), strconv.Quote(cfg.DoneSelector), req, req)
}

func runWebviewAuth(ctx context.Context, cfg *Config, logger *slog.Logger, statusFn func(string), w webview.WebView, controller *webviewController) error {
	mode := determineMode(cfg)
	manualOnly := mode == modeManual
	statusFn("Waiting for embedded login page...")

	var (
		usernameFilled      bool
		passwordFilled      bool
		intermediateClicked bool
		totpStep1Filled     bool
		submitClicked       bool
		totpStep2Filled     bool
	)

	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if manualOnly {
				return nil
			}
			return fmt.Errorf("authentication cancelled: %w", ctx.Err())
		case <-ticker.C:
			info, err := controller.requestPageInfo(w, cfg, 3*time.Second)
			if err != nil {
				logger.Debug("failed to read embedded page state", slog.Any("error", err))
				continue
			}

			if !manualOnly {
				if cfg.AuthDoneURL != "" && info.URL != "" && contains(info.URL, cfg.AuthDoneURL) {
					logger.Info("authentication detected by done URL", slog.String("url", info.URL))
					statusFn("Authentication complete")
					if cfg.NavigateURL != "" {
						statusFn("Navigating to target...")
						if err := controller.navigateTo(w, cfg.NavigateURL); err != nil {
							return err
						}
					}
					return nil
				}
				if cfg.DoneSelector != "" && info.DonePresent {
					logger.Info("authentication detected by done selector")
					statusFn("Authentication complete")
					if cfg.NavigateURL != "" {
						statusFn("Navigating to target...")
						if err := controller.navigateTo(w, cfg.NavigateURL); err != nil {
							return err
						}
					}
					return nil
				}
			}

			if !usernameFilled && cfg.UsernameSelector != "" && info.UsernamePresent {
				logger.Info("filling username field")
				statusFn("Filling credentials...")
				if err := controller.fillField(w, cfg.UsernameSelector, cfg.UsernameValue); err != nil {
					logger.Warn("failed to fill username", slog.Any("error", err))
				} else {
					usernameFilled = true
				}
			}

			if cfg.SubmitSelector != "" && cfg.PasswordSelector != "" && !intermediateClicked && !info.PasswordPresent && info.SubmitPresent {
				logger.Info("clicking intermediate submit")
				if err := controller.clickElement(w, cfg.SubmitSelector); err != nil {
					logger.Warn("failed to click intermediate submit", slog.Any("error", err))
				} else {
					intermediateClicked = true
				}
			}

			if !passwordFilled && cfg.PasswordSelector != "" && info.PasswordPresent {
				logger.Info("filling password field")
				statusFn("Filling credentials...")
				if err := controller.fillField(w, cfg.PasswordSelector, cfg.PasswordValue); err != nil {
					logger.Warn("failed to fill password", slog.Any("error", err))
				} else {
					passwordFilled = true
				}
			}

			totpStep := cfg.TOTPStep
			if totpStep == 0 {
				totpStep = 2
			}

			if cfg.TOTPSecret != "" && cfg.TOTPSelector != "" && totpStep == 1 && !totpStep1Filled && info.TOTPPresent {
				logger.Info("filling TOTP field before submit")
				statusFn("Filling two-factor code...")
				code, err := generateTOTP(cfg.TOTPSecret)
				if err != nil {
					logger.Warn("failed to generate TOTP", slog.Any("error", err))
				} else if err := controller.fillField(w, cfg.TOTPSelector, code); err != nil {
					logger.Warn("failed to fill TOTP", slog.Any("error", err))
				} else {
					totpStep1Filled = true
				}
			}

			if cfg.SubmitSelector != "" && !submitClicked && info.SubmitPresent && (passwordFilled || cfg.PasswordSelector == "") {
				logger.Info("clicking submit button")
				statusFn("Submitting login form...")
				if err := controller.clickElement(w, cfg.SubmitSelector); err != nil {
					logger.Warn("failed to click submit", slog.Any("error", err))
				} else {
					submitClicked = true
				}
			}

			if cfg.TOTPSecret != "" && cfg.TOTPSelector != "" && totpStep == 2 && submitClicked && !totpStep2Filled && info.TOTPPresent {
				logger.Info("filling TOTP field after submit")
				statusFn("Filling two-factor code...")
				code, err := generateTOTP(cfg.TOTPSecret)
				if err != nil {
					logger.Warn("failed to generate TOTP", slog.Any("error", err))
				} else if err := controller.fillField(w, cfg.TOTPSelector, code); err != nil {
					logger.Warn("failed to fill TOTP", slog.Any("error", err))
				} else {
					totpStep2Filled = true
					if cfg.SubmitSelector != "" {
						if err := controller.clickElement(w, cfg.SubmitSelector); err != nil {
							logger.Warn("failed to click submit after TOTP", slog.Any("error", err))
						}
					}
				}
			}

			if manualOnly {
				statusFn("Waiting for authentication in embedded webview...")
			}
		}
	}
}

func contains(subject, fragment string) bool {
	return fragment != "" && strings.Contains(subject, fragment)
}

func generateTOTP(secret string) (string, error) {
	return totp.GenerateCode(secret, time.Now())
}
