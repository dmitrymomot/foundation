# gokit

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/dmitrymomot/gokit)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/dmitrymomot/gokit)](https://github.com/dmitrymomot/gokit/tags)
[![Go Reference](https://pkg.go.dev/badge/github.com/dmitrymomot/gokit.svg)](https://pkg.go.dev/github.com/dmitrymomot/gokit)
[![License](https://img.shields.io/github/license/dmitrymomot/gokit)](https://github.com/dmitrymomot/gokit/blob/main/LICENSE)

[![Tests](https://github.com/dmitrymomot/gokit/actions/workflows/tests.yml/badge.svg)](https://github.com/dmitrymomot/gokit/actions/workflows/tests.yml)
[![CodeQL Analysis](https://github.com/dmitrymomot/gokit/actions/workflows/codeql.yml/badge.svg)](https://github.com/dmitrymomot/gokit/actions/workflows/codeql.yml)
[![GolangCI Lint](https://github.com/dmitrymomot/gokit/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/dmitrymomot/gokit/actions/workflows/golangci-lint.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/dmitrymomot/gokit)](https://goreportcard.com/report/github.com/dmitrymomot/gokit)

## To-Do

- [x] Rework go-chi to change handler interface from `func(http.ResponseWriter, *http.Request)` => `func(Context) Response`
- [x] ability to set custom context type via generics
- [x] extend default context with helpers for different default types of responses and request parsers
- [x] router new function must be configurable via options pattern
- [ ] robust default configuration, especially for my personal purpose
- [ ] default middlewares with new interface
- [ ] default http server with graceful shutdown
- [ ] default handler for files uploading with customizable storage provider
- [ ] default SSE and WebSocket support

## Additional Response Types (Planned)

### Streaming Responses

- [x] `Stream(writer func(w io.Writer) error)` - For chunked/streaming responses (implemented)
- [x] `SSE(events <-chan any, opts ...EventOption)` - Server-Sent Events support (implemented)
- [x] `StreamJSON(items <-chan any)` - JSON streaming (newline-delimited) (implemented)

### Error Responses

- `Error(message string, status int)` - Simple error response
- `ValidationError(errors map[string][]string)` - Field-level validation errors
- `ProblemDetails(title, detail string, status int)` - RFC 7807 compliant error responses

### Header Manipulation

- `WithHeaders(response Response, headers map[string]string)` - Wrap any response with custom headers
- `WithCookie(response Response, cookie *http.Cookie)` - Add cookies to response
- `WithCache(response Response, maxAge int)` - Add cache control headers

### Data Export Responses

- `CSV(records [][]string)` - Export CSV data
- `CSVFromStruct(data any)` - Convert structs to CSV format
- `Excel(data any)` - Excel file generation (requires external library)

### Template Responses

- `Template(name string, data any)` - Render HTML templates
- `TemplateString(tmpl string, data any)` - Inline template rendering

### Partial Content

- `PartialContent(content []byte, start, end, total int64)` - For range requests (206 status)

### WebSocket Upgrade

- `WebSocketUpgrade(handler func(conn *websocket.Conn))` - Handle WebSocket connections
