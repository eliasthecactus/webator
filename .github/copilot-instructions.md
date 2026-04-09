# Webator — Copilot Instructions

Enterprise Chrome/Edge browser automation tool for SSO, MFA, and post-login navigation. Written in Go, built as a single binary.

## Architecture

All code lives in `package main` (flat structure, no sub-packages).

| File | Role |
|------|------|
| `main.go` | Entry point; 60+ CLI flags; routes to headless / GUI (Fyne) / webview mode |
| `auth.go` | Core 9-step login orchestration with retry logic |
| `browser.go` | chromedp allocator setup; Chrome/Edge auto-detection per platform |
| `config.go` | `Config` struct (60+ fields); merges defaults → JSON file → CLI flags |
| `selector.go` | `resolvedSelector`; detects CSS vs XPath at runtime; generates JS visibility snippets |
| `wait.go` | `smartWait()` element polling with adaptive logging and timeouts |
| `gui.go` | Fyne desktop GUI (spinner + log area) |
| `webview_auth.go` | Embedded webview; JS↔Go async messaging via JSON RPC over channels |
| `logger.go` | `slog` dual-mode: text→stdout (debug) or JSON→file (production) |

**Key types:** `Config`, `operationMode` (manual vs full-auto), `WaitOptions`, `WaitOverride`, `WebatorGUI`, `webviewController`.

## Build & Release

```bash
go mod tidy
go build -o webator .   # requires Go 1.21+
```

- Version is read from `FyneApp.toml` (`Version = "x.y.z"`)
- CI (`release.yml`) cross-compiles for macOS (Intel + ARM), Linux (x86_64 + ARM64), Windows
- No test suite exists — validate by running the binary against a real login page

## Conventions

### Selector handling
`selector.go` decides CSS vs XPath at runtime: strings starting with `//` or `.//' are XPath, everything else is CSS. When adding new element interactions, use `resolvedSelector` and its visibility-check JS helpers — **do not** hardcode raw CSS or XPath strings.

### Config layering
`loadConfig()` applies in order: struct defaults → JSON file → explicit CLI flags (detected via `flag.Visit()`). When adding new config fields:
1. Add to `Config` struct in `config.go` with a sensible zero-value default
2. Add the corresponding CLI flag in `main.go`
3. Add the merge logic in `loadConfig()` following the existing pattern

### JavaScript in webview mode
`webview_auth.go` sends JS to the embedded webview and receives responses via a keyed channel map. Always use `json.Marshal()` to embed dynamic values into JS strings — never interpolate user-controlled values directly.

### Browser flags
`browser.go` configures 30+ chromedp flags explicitly. Add new browser flags here alongside the existing block rather than relying on chromedp defaults, which vary across versions.

### Logging
Use the package-level `logger` (slog). Debug mode emits human-readable text to stdout; production emits JSON to the log file. Do not use `fmt.Print*` for operational messages.

## Key External Dependencies

- `github.com/chromedp/chromedp` — Chrome DevTools Protocol automation
- `fyne.io/fyne/v2` — Desktop GUI
- `github.com/pquerna/otp` — TOTP/MFA code generation
- `github.com/webview/webview` — Embedded webview fallback

See [README.md](../README.md) for full usage examples and CLI flag reference.
