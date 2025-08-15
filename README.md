# gokit

## To-Do

- [ ] Rework go-chi to change handler interface from `func(http.ResponseWriter, *http.Request)` => `func(Context) Response`
- [ ] ability to set custom context type via generics
- [ ] extend default context with helpers for different default types of responses and request parsers
- [ ] router new function must be configurable via options pattern
- [ ] robust default configuration, especially for my personal purpose
- [ ] default middlewares with new interface
- [ ] default http server with graceful shutdown
- [ ] default handler for files uploading with customizable storage provider
- [ ] default SSE and WebSocket support
