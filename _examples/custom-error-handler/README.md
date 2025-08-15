# Custom Error Handler Example

Demonstrates how to implement a custom error handler for the gokit router that returns JSON-formatted error responses.

## Features Demonstrated

- Custom error handler implementation
- Panic recovery with custom formatting
- JSON error responses
- Error logging

## Running the Example

```bash
go run main.go
```

The server will start on http://localhost:8080

## Available Endpoints

- `GET /` - Home page (normal response)
- `GET /panic` - Triggers a panic to test error recovery

## Testing

```bash
# Normal endpoint
curl http://localhost:8080/

# Trigger panic - returns JSON error
curl http://localhost:8080/panic
```

### Expected Response from `/panic`:

```json
{
    "error": "Internal Server Error",
    "message": "panic: 💥 Something went terribly wrong! Database connection lost!",
    "status": 500
}
```

## Key Concepts

- **Custom Error Handler**: Replaces the default error handler with `gokit.WithErrorHandler()`
- **Error Recovery**: Panics are caught and converted to structured error responses
- **JSON Responses**: Error handler returns JSON instead of plain text
- **Logging**: Errors are logged to the console for debugging
- **Stability**: Server continues running after handling panics

## Use Cases

This pattern is useful for:

- API servers that need consistent JSON error responses
- Production applications requiring detailed error logging
- Services that need custom error formatting
- Applications requiring specific error status codes
