package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/chromedp/chromedp"
)

// WaitOptions controls how smartWait polls for element visibility.
type WaitOptions struct {
	Timeout      time.Duration
	PollInterval time.Duration
	Label        string
}

// smartWait polls for element visibility until the element becomes visible,
// the timeout elapses, or the context is cancelled.
//
//   - Emits a debug log on every poll attempt.
//   - Emits a single warn log if the element has not appeared after 3 seconds.
//   - Returns a descriptive error that includes the selector and elapsed time
//     on timeout.
func smartWait(ctx context.Context, sel string, opts WaitOptions, logger *slog.Logger) error {
	rs := resolveSelector(sel, logger)

	start := time.Now()
	deadline := time.NewTimer(opts.Timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()

	warnThreshold := 3 * time.Second
	warnEmitted := false

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for %q (label: %s, elapsed: %s): %w",
				sel, opts.Label, time.Since(start).Round(time.Millisecond), ctx.Err())

		case <-deadline.C:
			elapsed := time.Since(start).Round(time.Millisecond)
			return fmt.Errorf("timeout after %s waiting for element %q (label: %s)", elapsed, sel, opts.Label)

		case <-ticker.C:
			elapsed := time.Since(start)

			if !warnEmitted && elapsed > warnThreshold {
				logger.Warn("element taking longer than expected",
					slog.String("selector", sel),
					slog.String("label", opts.Label),
					slog.String("elapsed", elapsed.Round(time.Millisecond).String()),
				)
				warnEmitted = true
			}

			var visible bool
			err := chromedp.Run(ctx, chromedp.Evaluate(rs.visibilityJS(), &visible))

			logger.Debug("poll attempt",
				slog.String("selector", sel),
				slog.String("label", opts.Label),
				slog.String("elapsed", elapsed.Round(time.Millisecond).String()),
				slog.Bool("visible", visible),
				slog.Any("evalErr", err),
			)

			if err != nil {
				// Evaluation errors are transient (page still loading); keep polling.
				continue
			}
			if visible {
				logger.Debug("element became visible",
					slog.String("selector", sel),
					slog.String("label", opts.Label),
					slog.String("elapsed", elapsed.Round(time.Millisecond).String()),
				)
				return nil
			}
		}
	}
}

// quickCheck performs a single, immediate visibility check for the given
// selector.  It returns false on any error or if the element is not visible.
func quickCheck(ctx context.Context, sel string, logger *slog.Logger) bool {
	rs := resolveSelector(sel, logger)

	var visible bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(rs.visibilityJS(), &visible)); err != nil {
		logger.Debug("quickCheck eval error", slog.String("selector", sel), slog.Any("err", err))
		return false
	}
	return visible
}
