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

### Webview Mode

```bash
./webator --webview --debug --auth-start-url "https://login.example.com" --webview-title "Login"
```

This opens the auth page in an embedded native webview window instead of launching an external browser. The embedded window title can be set explicitly with `--webview-title` or `webview_title` in config. If not provided, the title is derived from `navigate-url` when present, otherwise from `auth-start-url`.

In `--debug` mode, a GUI window is shown with a log box. Full-auto authentication is not supported in this mode; the user must complete login manually.

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

The embedded webview title can be configured with `webview_title` in JSON. If omitted, webview mode will derive a title from `navigate_url` first, then `auth_start_url`.

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

## Multi-Destination Mode

When you need webator to authenticate against several different systems and choose the target at launch time, define a `destinations` list in your config file. A Fyne GUI picker (or a numbered stdin prompt in `--headless` mode) appears before the browser starts.

If `--destination-tags` narrows the list to exactly **one** selectable URL, the picker is skipped entirely and that URL opens directly — useful for scripted invocations.

### Config structure

```json
{
  "username_value": "global-user",
  "password_value": "global-pass",
  "destinations": [
    {
      "name": "My App",
      "tag":  "myapp",
      "username_selector":    "#user",
      "password_selector":    "#pass",
      "submit_selector":      "button[type=submit]",
      "totp_secret":          "BASE32TOTPSECRET",
      "totp_selector":        "#otp",
      "totp_step":            2,
      "wait_after_submit_ms": 1000,
      "done_selector":        "#dashboard",
      "urls": [
        {
          "label":          "Production",
          "tag":            "myapp-prod",
          "auth_start_url": "https://prod.example.com",
          "auth_done_url":  "https://prod.example.com/home",
          "navigate_url":   "https://prod.example.com/home",
          "username_value": "prod-admin",
          "password_value": "prod-secret"
        }
      ]
    }
  ]
}
```

See [`multi-config.json`](./multi-config.json) for a full annotated example covering Zabbix, phpIPAM, and Microsoft 365 with per-category TOTP.

### Field precedence

Fields are merged in the following order — the highest level that provides a non-empty value wins:

```
CLI flag  >  URL-level  >  Category-level  >  Root Config
```

| Field | Root config | Category (`destinations[]`) | URL (`destinations[].urls[]`) |
|-------|:-----------:|:---------------------------:|:-----------------------------:|
| `username_selector`   | ✓ | ✓ | ✓ |
| `username_value`      | ✓ | ✓ | ✓ |
| `password_selector`   | ✓ | ✓ | ✓ |
| `password_value`      | ✓ | ✓ | ✓ |
| `submit_selector`     | ✓ | ✓ | ✓ |
| `done_selector`       | ✓ | ✓ | ✓ |
| `totp_secret`         | ✓ | ✓ | ✓ |
| `totp_selector`       | ✓ | ✓ | ✓ |
| `totp_step`           | ✓ | ✓ | ✓ |
| `wait_after_submit_ms`| ✓ | ✓ | ✓ |
| `auth_start_url`      | ✓ | — | ✓ |
| `auth_done_url`       | ✓ | — | ✓ |
| `navigate_url`        | ✓ | — | ✓ |

> **CLI flags** (e.g. `--username-value`) override everything, including per-destination values. Use them to inject secrets at runtime without storing them in the config file.

### Tagging and filtering

Each category and URL can carry a `tag` string. The `--destination-tags` flag accepts a comma-separated list of tags to restrict what appears in the picker:

```bash
# Show every destination (no filter)
./webator --config multi-config.json

# Show only Zabbix entries in the picker
./webator --config multi-config.json --destination-tags "zabbix"

# Open Zabbix Prod immediately — single match, picker is skipped
./webator --config multi-config.json --destination-tags "zabbix-prod"

# Show two specific URLs in the picker
./webator --config multi-config.json --destination-tags "zabbix-prod,ipam-dev"
```

A category tag matches **all** of its child URLs. A URL tag matches only that one URL.

### Injecting credentials at runtime

```bash
./webator --config multi-config.json \
  --username-value "alice" \
  --password-value "s3cret"
```

Root-level `username_value` / `password_value` apply globally to every destination that does not override them at category or URL level.

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
| `--webview` | `false` | Render the auth page in an embedded webview instead of using an external browser |
| `--webview-title` | *(none)* | Explicit title for the embedded webview window |
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
| `--destination-tags` | *(none)* | Comma-separated tags to restrict the destination picker (e.g. `zabbix,ipam-prod`). If the filter yields a single URL, the picker is skipped and that URL opens directly. |

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
