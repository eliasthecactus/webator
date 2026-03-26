package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/pquerna/otp/totp"
)

// operationMode describes how the tool should behave after submitting
// credentials.
type operationMode int

const (
	modeManual   operationMode = iota
	modeFullAuto operationMode = iota
)

func (m operationMode) String() string {
	switch m {
	case modeManual:
		return "manual"
	case modeFullAuto:
		return "full-auto"
	default:
		return "unknown"
	}
}

// determineMode returns the appropriate operationMode based on cfg.
func determineMode(cfg *Config) operationMode {
	if cfg.AuthDoneURL != "" && cfg.NavigateURL != "" {
		return modeFullAuto
	}
	return modeManual
}

// runAuth executes the authentication flow with retry logic.
func runAuth(ctx context.Context, cfg *Config, logger *slog.Logger, statusFn func(string)) error {
	mode := determineMode(cfg)
	logger.Info("operation mode determined", slog.String("mode", mode.String()))

	maxAttempts := cfg.RetryCount + 1
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			logger.Warn("retrying authentication",
				slog.Int("attempt", attempt),
				slog.Int("maxAttempts", maxAttempts),
				slog.Any("previousError", lastErr),
			)
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry delay: %w", ctx.Err())
			case <-time.After(time.Duration(cfg.RetryDelayMs) * time.Millisecond):
			}
		}

		if err := loginOnce(ctx, cfg, logger, mode, statusFn); err != nil {
			lastErr = err
			logger.Warn("authentication attempt failed",
				slog.Int("attempt", attempt),
				slog.Any("error", err),
			)
			continue
		}

		// Success.
		return nil
	}

	logger.Error("all authentication attempts exhausted",
		slog.Int("attempts", maxAttempts),
		slog.Any("lastError", lastErr),
	)
	return fmt.Errorf("authentication failed after %d attempt(s): %w", maxAttempts, lastErr)
}

// loginOnce performs a single end-to-end authentication attempt.
func loginOnce(ctx context.Context, cfg *Config, logger *slog.Logger, mode operationMode, statusFn func(string)) error {
	// waitOpts builds WaitOptions for a named step, honouring per-step overrides.
	waitOpts := func(label string) WaitOptions {
		to := time.Duration(cfg.Timeout) * time.Second
		if cfg.WaitOverrides != nil {
			if ov, ok := cfg.WaitOverrides[label]; ok && ov.TimeoutMs > 0 {
				to = time.Duration(ov.TimeoutMs) * time.Millisecond
			}
		}
		return WaitOptions{
			Timeout:      to,
			PollInterval: time.Duration(cfg.PollIntervalMs) * time.Millisecond,
			Label:        label,
		}
	}

	// ── Step 1: Navigate to the auth start URL ──────────────────────────────
	logger.Info("navigating to auth start URL", slog.String("url", cfg.AuthStartURL))
	if err := chromedp.Run(ctx, chromedp.Navigate(cfg.AuthStartURL)); err != nil {
		return fmt.Errorf("navigate to auth start URL: %w", err)
	}
	statusFn("Navigating to login page...")

	// ── Step 2: Check if already authenticated ──────────────────────────────
	if cfg.AuthDoneURL != "" {
		var currentURL string
		if err := chromedp.Run(ctx, chromedp.Location(&currentURL)); err == nil {
			if strings.Contains(currentURL, cfg.AuthDoneURL) {
				logger.Info("already authenticated, skipping login",
					slog.String("currentUrl", currentURL),
				)
				if mode == modeFullAuto && cfg.NavigateURL != "" {
					return navigateToTarget(ctx, cfg, logger)
				}
				return nil
			}
		}
	}

	// ── Step 3: Wait for the username field ─────────────────────────────────
	if cfg.UsernameSelector != "" {
		if err := smartWait(ctx, cfg.UsernameSelector, waitOpts("username"), logger); err != nil {
			return fmt.Errorf("waiting for username field: %w", err)
		}

		rs := resolveSelector(cfg.UsernameSelector, logger)
		statusFn("Filling credentials...")
		logger.Debug("filling username field")
		if err := chromedp.Run(ctx,
			chromedp.Clear(cfg.UsernameSelector, rs.byOption),
			chromedp.SendKeys(cfg.UsernameSelector, cfg.UsernameValue, rs.byOption),
		); err != nil {
			return fmt.Errorf("fill username: %w", err)
		}
	}

	// ── Step 4: Intermediate submit (multi-step flows like Microsoft / Okta) ─
	// Click submit before the password field is presented, but only when the
	// password field isn't already visible (i.e. this is a multi-step flow).
	if cfg.SubmitSelector != "" && cfg.PasswordSelector != "" {
		if !quickCheck(ctx, cfg.PasswordSelector, logger) {
			logger.Info("password field not yet visible, clicking intermediate submit")
			rsSubmit := resolveSelector(cfg.SubmitSelector, logger)
			if err := chromedp.Run(ctx,
				chromedp.Click(cfg.SubmitSelector, rsSubmit.byOption),
			); err != nil {
				return fmt.Errorf("click intermediate submit: %w", err)
			}
			if cfg.WaitAfterSubmitMs > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(cfg.WaitAfterSubmitMs) * time.Millisecond):
				}
			}
		}
	}

	// ── Step 5: Fill password ───────────────────────────────────────────────
	if cfg.PasswordSelector != "" {
		if err := smartWait(ctx, cfg.PasswordSelector, waitOpts("password"), logger); err != nil {
			return fmt.Errorf("waiting for password field: %w", err)
		}

		rsPass := resolveSelector(cfg.PasswordSelector, logger)
		logger.Debug("filling password field")
		if err := chromedp.Run(ctx,
			chromedp.Clear(cfg.PasswordSelector, rsPass.byOption),
			chromedp.SendKeys(cfg.PasswordSelector, cfg.PasswordValue, rsPass.byOption),
		); err != nil {
			return fmt.Errorf("fill password: %w", err)
		}
	}

	// ── Step 6: TOTP at step 1 (before first main submit) ───────────────────
	totpStep := cfg.TOTPStep
	if totpStep == 0 {
		totpStep = 2 // 0 is treated as 2 (the default)
	}

	if cfg.TOTPSecret != "" && cfg.TOTPSelector != "" && totpStep == 1 {
		if err := fillTOTP(ctx, cfg, logger, waitOpts); err != nil {
			return err
		}
	}

	// ── Step 7: Click the main submit button ────────────────────────────────
	if cfg.SubmitSelector != "" {
		rsSubmit := resolveSelector(cfg.SubmitSelector, logger)
		logger.Info("clicking submit button")
		if err := chromedp.Run(ctx,
			chromedp.Click(cfg.SubmitSelector, rsSubmit.byOption),
		); err != nil {
			return fmt.Errorf("click submit: %w", err)
		}
		statusFn("Waiting for authentication...")
		if cfg.WaitAfterSubmitMs > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(cfg.WaitAfterSubmitMs) * time.Millisecond):
			}
		}
	}

	// ── Step 8: TOTP at step 2 (after first main submit) ────────────────────
	if cfg.TOTPSecret != "" && cfg.TOTPSelector != "" && (totpStep == 2) {
		if err := fillTOTP(ctx, cfg, logger, waitOpts); err != nil {
			return err
		}
		// Click submit again after filling TOTP.
		if cfg.SubmitSelector != "" {
			rsSubmit := resolveSelector(cfg.SubmitSelector, logger)
			logger.Info("clicking submit after TOTP")
			if err := chromedp.Run(ctx,
				chromedp.Click(cfg.SubmitSelector, rsSubmit.byOption),
			); err != nil {
				return fmt.Errorf("click submit after TOTP: %w", err)
			}
		}
	}

	// ── Step 9: Post-submit wait ─────────────────────────────────────────────
	switch mode {
	case modeManual:
		return waitManual(ctx, cfg, logger, statusFn)
	case modeFullAuto:
		return waitForAuthDone(ctx, cfg, logger, statusFn)
	}

	return nil
}

// fillTOTP generates a fresh TOTP code and types it into the configured field.
func fillTOTP(ctx context.Context, cfg *Config, logger *slog.Logger, waitOpts func(string) WaitOptions) error {
	if err := smartWait(ctx, cfg.TOTPSelector, waitOpts("totp"), logger); err != nil {
		return fmt.Errorf("waiting for TOTP field: %w", err)
	}

	code, err := totp.GenerateCode(cfg.TOTPSecret, time.Now())
	if err != nil {
		return fmt.Errorf("generate TOTP code: %w", err)
	}

	rsTOTP := resolveSelector(cfg.TOTPSelector, logger)
	logger.Debug("filling TOTP field")
	if err := chromedp.Run(ctx,
		chromedp.Clear(cfg.TOTPSelector, rsTOTP.byOption),
		chromedp.SendKeys(cfg.TOTPSelector, code, rsTOTP.byOption),
	); err != nil {
		return fmt.Errorf("fill TOTP: %w", err)
	}

	return nil
}

// waitManual blocks until the context is cancelled, giving the user time to
// interact with the browser manually.
func waitManual(ctx context.Context, cfg *Config, logger *slog.Logger, statusFn func(string)) error {
	logger.Info("manual mode: browser ready for user interaction; waiting for context cancellation")
	statusFn("Browser ready — credentials filled")
	<-ctx.Done()
	return nil
}

// waitForAuthDone polls the browser until authentication is confirmed, then
// navigates to the target URL.
func waitForAuthDone(ctx context.Context, cfg *Config, logger *slog.Logger, statusFn func(string)) error {
	statusFn("Waiting for authentication...")
	to := time.Duration(cfg.Timeout) * time.Second
	deadline := time.NewTimer(to)
	defer deadline.Stop()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	movedAway := false // true once the browser left the AuthStartURL

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for auth completion: %w", ctx.Err())

		case <-deadline.C:
			return fmt.Errorf("timeout after %s waiting for authentication to complete", to)

		case <-ticker.C:
			var currentURL string
			if err := chromedp.Run(ctx, chromedp.Location(&currentURL)); err != nil {
				logger.Debug("could not read current URL during auth wait", slog.Any("err", err))
				continue
			}

			// Track when we have moved away from the start URL.
			if !movedAway && cfg.AuthStartURL != "" && !strings.Contains(currentURL, cfg.AuthStartURL) {
				movedAway = true
				logger.Debug("navigated away from auth start URL", slog.String("url", currentURL))
			}

			// Logout / session-expired detection.
			if movedAway && cfg.AuthStartURL != "" && strings.Contains(currentURL, cfg.AuthStartURL) {
				logger.Warn("logout detected: returned to auth start URL",
					slog.String("url", currentURL),
				)
				return fmt.Errorf("logout detected: browser returned to auth start URL %q", cfg.AuthStartURL)
			}

			// Check DoneSelector if configured.
			if cfg.DoneSelector != "" && quickCheck(ctx, cfg.DoneSelector, logger) {
				logger.Info("done selector matched, authentication complete",
					slog.String("selector", cfg.DoneSelector),
					slog.String("url", currentURL),
				)
				statusFn("Navigating to target...")
				return navigateToTarget(ctx, cfg, logger)
			}

			// Check AuthDoneURL.
			if cfg.AuthDoneURL != "" && strings.Contains(currentURL, cfg.AuthDoneURL) {
				logger.Info("auth done URL matched, authentication complete",
					slog.String("authDoneUrl", cfg.AuthDoneURL),
					slog.String("currentUrl", currentURL),
				)
				statusFn("Navigating to target...")
				return navigateToTarget(ctx, cfg, logger)
			}
		}
	}
}

// navigateToTarget navigates the browser to cfg.NavigateURL.
func navigateToTarget(ctx context.Context, cfg *Config, logger *slog.Logger) error {
	logger.Info("navigating to target URL", slog.String("url", cfg.NavigateURL))
	if err := chromedp.Run(ctx, chromedp.Navigate(cfg.NavigateURL)); err != nil {
		return fmt.Errorf("navigate to target URL %q: %w", cfg.NavigateURL, err)
	}
	return nil
}
