# Browser Automation API

Phase 4 adds headless browser automation backed by
[Playwright](https://playwright.dev/). A single browser service container runs
an HTTP control server; the gateway proxies `/browser/*` requests to it over the
internal network. Sessions are pooled and auto-expire after an idle timeout.

## Architecture

```
  gateway (:8080)  --/browser/*-->  browser service (:3000)  -->  Playwright
                                     - session pool (max 10)        chromium
                                     - idle timeout (30m)           firefox
                                     - telemetry disabled           webkit
```

- **Browsers:** Chromium (default), Firefox, and WebKit. WebKit can be disabled
  in `config.yaml` for a smaller footprint.
- **Session cap:** `browser.max_sessions` (default 10). Exceeding it returns
  HTTP `429`.
- **Idle timeout:** `browser.session_timeout_minutes` (default 30). Each action
  refreshes the timer.
- **Telemetry:** disabled globally via `CI=true`, `PWDEBUG=0` container env.

## Endpoints

| Method | Path                     | Purpose                        |
| ------ | ------------------------ | ------------------------------ |
| POST   | `/browser/session`       | Create a session               |
| GET    | `/browser/session/{id}`  | Get session metadata           |
| DELETE | `/browser/session/{id}`  | Destroy a session              |
| POST   | `/browser/action`        | Execute a DOM action           |

All endpoints are scoped to the `browser` tool token when auth mode is `token`.

## Session lifecycle

### Create

```json
POST /browser/session
{ "browserType": "chromium" }        // firefox | webkit also supported
```

```json
{ "id": "b1f2...", "browserType": "chromium", "createdAt": "2026-07-04T..." }
```

### Inspect

```json
GET /browser/session/b1f2...
```

```json
{
  "id": "b1f2...",
  "browserType": "chromium",
  "url": "https://go.dev",
  "title": "The Go Programming Language",
  "createdAt": "2026-07-04T...",
  "lastActivityAt": "2026-07-04T..."
}
```

### Destroy

```json
DELETE /browser/session/b1f2...     // -> 204 No Content
```

## Actions

`POST /browser/action` takes a `session_id` plus a `type` and action-specific
parameters.

| `type`                | Params                        | Result                          |
| --------------------- | ----------------------------- | ------------------------------- |
| `navigate`            | `url`, `waitUntil?`           | `{url, title}`                  |
| `click`               | `selector`, `timeout?`        | `{message}`                     |
| `type`                | `selector`, `text`, `delay?`  | `{message}`                     |
| `eval`                | `script`, `args?`             | `{result}`                      |
| `screenshot`          | `fullPage?` (default true)    | `{data (base64), type}`         |
| `pdf`                 | `format?` (default A4)        | `{data (base64), type}`         |
| `content`             | –                             | `{html}`                        |
| `wait_for_selector`   | `selector`, `timeout?`        | `{message}`                     |
| `wait_for_navigation` | `timeout?`                    | `{url}`                         |
| `get_title`           | –                             | `{title}`                       |
| `get_url`             | –                             | `{url}`                         |
| `set_viewport`        | `width`, `height`             | `{message}`                     |

Every action response includes a `status` of `"success"` or `"error"`; on error
a `message` (and `error`) field describes the failure.

### Examples

**Navigate**

```json
POST /browser/action
{ "session_id": "b1f2...", "type": "navigate", "url": "https://go.dev" }
```

```json
{ "status": "success", "url": "https://go.dev/", "title": "The Go Programming Language" }
```

**Type into a field**

```json
{ "session_id": "b1f2...", "type": "type", "selector": "#search", "text": "generics" }
```

**Screenshot (base64 PNG)**

```json
{ "session_id": "b1f2...", "type": "screenshot", "fullPage": true }
```

```json
{ "status": "success", "data": "iVBORw0KGgo...", "type": "image/png" }
```

**PDF export (base64 PDF)**

```json
{ "session_id": "b1f2...", "type": "pdf", "format": "A4" }
```

```json
{ "status": "success", "data": "JVBERi0xLj...", "type": "application/pdf" }
```

**Evaluate JavaScript**

```json
{ "session_id": "b1f2...", "type": "eval", "script": "() => document.title" }
```

## Multi-browser support

Set which engines the container ships with in `config.yaml`:

```yaml
browser:
  browsers:
    chromium: true
    firefox: true
    webkit: false
```

Browsers are launched lazily on first use, so disabling one avoids the launch
cost even if the base image includes it.

## Error handling

| Condition             | Gateway status | Notes                                   |
| --------------------- | -------------- | --------------------------------------- |
| Missing `session_id`  | `400`          | Action requests must reference a session |
| Invalid JSON body     | `400`          |                                         |
| Unknown session       | `404`          | Session expired or never existed        |
| Session limit reached | `429`          | `browser.max_sessions` exceeded         |
| Browser service down  | `502`          | Upstream unavailable                    |
| Action-level failure  | `200`          | Body carries `status: "error"`          |
