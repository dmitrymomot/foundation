# Basic gokit Example

A minimal example demonstrating the core features of the gokit HTTP router.

## Features Demonstrated

- Basic routing (GET, POST)
- URL parameters (`/hello/{name}`)
- Request body handling
- HTTP redirects
- Panic recovery
- Custom middleware

## Running the Example

```bash
go run main.go
```

The server will start on http://localhost:8080

## Available Endpoints

- `GET /` - Home page with endpoint list
- `GET /hello/{name}` - Greeting with URL parameter
- `POST /echo` - Echoes back the request body
- `GET /redirect` - Redirects to home page
- `GET /panic` - Triggers a panic to test error recovery

## Testing

```bash
# Home page
curl http://localhost:8080/

# URL parameters
curl http://localhost:8080/hello/world

# POST with body
curl -X POST http://localhost:8080/echo -d 'Hello gokit!'

# Redirect (use -L to follow)
curl -L http://localhost:8080/redirect

# Test panic recovery
curl http://localhost:8080/panic

# Check response headers (see custom middleware)
curl -I http://localhost:8080/
```

## Key Concepts

- **Router Creation**: Uses `gokit.NewRouter[*gokit.Context]()` with the default context
- **Middleware**: Adds custom headers to all responses
- **Response Helpers**: Uses `gokit.String()` and `gokit.Redirect()` helpers
- **Error Handling**: Framework automatically recovers from panics
