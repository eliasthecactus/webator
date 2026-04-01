<p align="center">
  <img src="icon.png" width="110" alt="webator" />
</p>

<h1 align="center">webator</h1>

<p align="center">Enterprise-grade browser automation — SSO, MFA, and post-login navigation, driven by a real Chrome window.</p>

---
It drives a real Chrome or Edge browser (via [chromedp](https://github.com/chromedp/chromedp))
to automate web-based authentication flows — including multi-step SSO logins,
TOTP/MFA challenges, and post-login navigation.

---

## Download

Pre-built binaries are available on the [Releases](../../releases) page for every tagged version.

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `webator-darwin-arm64` |
| macOS (Intel) | `webator-darwin-amd64` |
| Linux (x86-64) | `webator-linux-amd64` |
| Linux (ARM64) | `webator-linux-arm64` |
| Windows (x86-64) | `webator-windows-amd64.exe` |

```bash
# macOS example
curl -L https://github.com/eliasthecactus/webator/releases/latest/download/webator-darwin-arm64 \
  -o webator && chmod +x webator
```

---

## Build from Source

```bash
git clone https://github.com/eliasthecactus/webator
cd webator
go mod tidy
go build -o webator .
```

Requires Go 1.21+.

---
## VirusTotal uploads

The release workflow does upload built binaries to VirusTotal automatically when a `VIRUSTOTAL_API_KEY` repository secret is configured.


---
## Overview

webator supports two operating modes:

| Mode | When | Description |
|------|------|-------------|
| **Manual** | `--auth-done-url` / `--navigate-url` not set | Fills credentials and keeps the browser open for you to take over. |
| **Full-Auto** | Both `--auth-done-url` and `--navigate-url` set | Fills credentials, waits for the auth-done URL, navigates to the target, and keeps the browser open until Ctrl+C. |

### Browser visibility vs. logging

These two things are independent:

| Flag | Controls |
|------|----------|
| `--headless` | Whether a browser window is shown. Default: **off** (window is always visible unless you pass `--headless`). |
| `--debug` | Whether logs go to stdout (human-readable) or to the log file (JSON). Does **not** affect the browser window. |

---

## Quick Start

### Manual Mode

```bash
./webator \
  --auth-start-url "https://auth.immoscout24.ch/u/login" \
  --username-selector "#username" \
  --username-value "mail@eliasthecactus.ch" \
  --password-selector "#password" \
  --password-value "s3cret" \
  --submit-selector "//html/body/div/main/div/div/div[1]/main/section/div/div/div/form/div[2]/button"
```

### Full-Auto Mode

```bash
./webator \
  --auth-start-url   "https://auth.immoscout24.ch/u/login" \
  --auth-done-url    "https://www.immoscout24.ch/de/account-overview" \
  --navigate-url     "https://www.immoscout24.ch/de/immobilienbewertung/my-realestate" \
  --username-selector "#email" \
  --username-value   "mail@eliasthecactus.ch" \
  --password-selector "#password" \
  --password-value   "s3cret" \
  --submit-selector "//html/body/div/main/div/div/div[1]/main/section/div/div/div/form/div[2]/button"
```

The browser stays open after navigation. Press **Ctrl+C** or close the browser to exit.

### Headless / Script Mode

```bash
./webator --headless --config ./my-login.json
```

No window is shown. Logs go to `$TMPDIR/browser-automation.log`.

### Kiosk Mode

```bash
./webator --kiosk --incognito --auth-start-url "https://login.example.com" --browser-path "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"
```

This enables kiosk-friendly browser options such as fullscreen mode, disabled context menu, disabled dev tools and translation, and touchscreen/pinch restrictions.


### Debug Mode (verbose stdout logs)

```bash
./webator --debug --config ./my-login.json
```

Logs are printed to stdout in human-readable format. The browser window follows the `--headless` flag as normal.

---

## Using a Config File

All flags can be set in a JSON config file. CLI flags always override the file.

```bash
./webator --config ./my-login.json
```

A blank template is included at [`config.json`](./config.json). Config keys use **snake_case**.

### Example config — simple login (not tested by me)

```json
{
  "auth_start_url": "https://login.example.com",
  "auth_done_url":  "https://app.example.com/home",
  "navigate_url":   "https://app.example.com/reports",
  "username_selector": "#email",
  "username_value":    "alice@example.com",
  "password_selector": "#password",
  "password_value":    "hunter2",
  "submit_selector":   "button[type='submit']",
  "timeout": 90,
  "retry_count": 3
}
```

### Example config — TOTP / MFA (not tested by me)

```json
{
  "auth_start_url":  "https://login.example.com",
  "auth_done_url":   "https://app.example.com/",
  "navigate_url":    "https://app.example.com/data",
  "username_selector": "#username",
  "username_value":    "alice@example.com",
  "password_selector": "#password",
  "password_value":    "s3cret",
  "submit_selector":   "#loginBtn",
  "totp_secret":    "JBSWY3DPEHPK3PXP",
  "totp_selector":  "#mfa-code",
  "totp_step":      2,
  "wait_after_submit_ms": 1000
}
```

---

## Enterprise SSO Scenarios

### Microsoft 365 / Azure AD (multi-step) (not tested by me)

Microsoft shows username first, then reveals the password field after clicking Next.
webator detects that the password field is not yet visible and clicks the intermediate
submit automatically.

```json
{
  "auth_start_url":  "https://login.microsoftonline.com",
  "auth_done_url":   "https://portal.office.com",
  "navigate_url":    "https://portal.office.com/onedrive",
  "username_selector": "input[type='email']",
  "username_value":    "alice@corp.onmicrosoft.com",
  "password_selector": "input[type='password']",
  "password_value":    "P@ssw0rd!",
  "submit_selector":   "input[type='submit']",
  "totp_secret":    "YOUR_TOTP_SECRET",
  "totp_selector":  "input[name='otc']",
  "totp_step":      2,
  "wait_after_submit_ms": 2000,
  "timeout": 120,
  "wait_overrides": {
    "password": { "timeout_ms": 15000 },
    "totp":     { "timeout_ms": 20000 }
  }
}
```

### Okta (not tested by me)

```json
{
  "auth_start_url":  "https://your-org.okta.com",
  "auth_done_url":   "https://your-org.okta.com/app/UserHome",
  "navigate_url":    "https://internal.app.example.com",
  "username_selector": "#okta-signin-username",
  "username_value":    "alice@example.com",
  "password_selector": "#okta-signin-password",
  "password_value":    "s3cret!",
  "submit_selector":   "#okta-signin-submit",
  "totp_secret":    "YOUR_TOTP_SECRET",
  "totp_selector":  "input[name='answer']",
  "totp_step":      2,
  "timeout": 90
}
```

### ADFS (Active Directory Federation Services) (not tested by me)

```json
{
  "auth_start_url":  "https://adfs.corp.example.com/adfs/ls/",
  "auth_done_url":   "https://app.corp.example.com/",
  "navigate_url":    "https://app.corp.example.com/dashboard",
  "username_selector": "#userNameInput",
  "username_value":    "CORP\\alice",
  "password_selector": "#passwordInput",
  "password_value":    "C0rp!Pass",
  "submit_selector":   "#submitButton",
  "ignore_cert_errors": true,
  "timeout": 60
}
```

---

## TOTP: Step 1 vs Step 2

`totp_step` controls **when** the TOTP code is entered relative to the main submit click.

| Value | Behaviour |
|-------|-----------|
| `1` | TOTP field is filled **before** clicking submit (all fields on one page) |
| `2` *(default)* | Submit is clicked first; webator waits for the TOTP field to appear on the next step, then fills and submits again |

Most MFA flows (Microsoft, Okta) use step 2. Some older portals present all fields on a single page, requiring step 1.

---

## All CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | *(none)* | Path to JSON config file |
| `--debug` | `false` | Write logs to stdout in human-readable format instead of JSON log file |
| `--auth-start-url` | **required** | URL where the login flow begins |
| `--auth-done-url` | *(none)* | URL substring that signals successful login |
| `--navigate-url` | *(none)* | URL to open after authentication (enables full-auto mode when combined with `--auth-done-url`) |
| `--username-selector` | *(none)* | CSS or XPath selector for the username input |
| `--username-value` | *(none)* | Username / email to type |
| `--password-selector` | *(none)* | CSS or XPath selector for the password input |
| `--password-value` | *(none)* | Password to type |
| `--totp-secret` | *(none)* | Base32-encoded TOTP secret (RFC 6238) |
| `--totp-selector` | *(none)* | CSS or XPath selector for the TOTP input |
| `--totp-step` | `2` | `1` = before submit, `2` = after submit |
| `--submit-selector` | *(none)* | CSS or XPath selector for the submit button |
| `--done-selector` | *(none)* | CSS or XPath selector whose appearance confirms login success |
| `--browser-path` | *(auto-detect)* | Explicit path to the Chrome or Edge binary |
| `--headless` | `false` | Run browser without a visible window |
| `--viewport-width` | `1920` | Viewport width in pixels |
| `--viewport-height` | `1080` | Viewport height in pixels |
| `--user-agent` | Chrome 124 UA | Browser User-Agent string |
| `--kiosk` | `false` | Run the browser in kiosk/fullscreen mode |
| `--incognito` | `false` | Open the browser in an incognito/private session |
| `--disable-context-menu` | `true` | Disable the browser context menu |
| `--disable-dev-tools` | `true` | Prevent opening developer tools |
| `--disable-features` | `DevTools` | Additional browser features to disable |
| `--kiosk-printing` | `true` | Enable kiosk-friendly printing behavior |
| `--disable-pinch` | `true` | Disable pinch/zoom gestures |
| `--overscroll-history-navigation` | `0` | Overscroll history navigation setting |
| `--pull-to-refresh` | `0` | Pull-to-refresh setting |
| `--disable-touch-adjustment` | `true` | Disable touch adjustment UI |
| `--disable-translate` | `true` | Disable browser translation prompts |
| `--edge-kiosk-type` | `fullscreen` | Edge kiosk type when using `--kiosk` |
| `--no-first-run` | `true` | Disable first-run browser checks |
| `--no-default-browser-check` | `true` | Disable default browser check |
| `--proxy` | *(none)* | Proxy URL (e.g. `http://proxy.corp:8080`) |
| `--ignore-cert-errors` | `false` | Ignore TLS/SSL certificate errors |
| `--timeout` | `60` | Timeout in seconds for the login flow |
| `--retry-count` | `3` | Number of retry attempts after failure |
| `--retry-delay-ms` | `1500` | Delay between retries in milliseconds |
| `--log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `--log-file` | `$TMPDIR/browser-automation.log` | Path to the JSON log file |
| `--wait-after-submit-ms` | `0` | Extra wait in ms after clicking submit |
| `--poll-interval-ms` | `250` | Polling interval for element visibility checks |

### Selector syntax

| Prefix | Interpreted as |
|--------|---------------|
| `//` or `.//` | XPath |
| `#id` | CSS id selector |
| Anything else | CSS selector |

### Per-step timeout overrides (config file only)

```json
"wait_overrides": {
  "username": { "timeout_ms": 10000 },
  "password": { "timeout_ms": 15000 },
  "totp":     { "timeout_ms": 30000 }
}
```

---

## Logging

| Mode | Format | Destination |
|------|--------|-------------|
| Normal | JSON (structured) | `--log-file` (default: `$TMPDIR/browser-automation.log`) |
| `--debug` | Human-readable text | stdout |

No output is printed to the console during normal operation.

---

## Releases

Releases are built automatically by GitHub Actions on every `v*` tag push and published to the [Releases](../../releases) page with binaries for all supported platforms.

To cut a new release:

```bash
git tag v1.0.0
git push origin v1.0.0
```

---

## License

[MIT](LICENSE)
